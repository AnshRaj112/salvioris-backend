package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ChatHistoryResponse represents paginated chat history.
type ChatHistoryResponse struct {
	Success  bool                   `json:"success"`
	Messages []models.ChatMessage   `json:"messages"`
	HasMore  bool                   `json:"has_more"`
	Total    int64                  `json:"total"`
}

// ChatSendRequest represents the body for sending a message via HTTP.
// WebSocket is preferred for realtime, but this endpoint is provided for compatibility.
type ChatSendRequest struct {
	GroupID string `json:"group_id"`
	Text    string `json:"text"`
}

// ChatSendResponse represents the response after sending a message via HTTP.
type ChatSendResponse struct {
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Data    *models.ChatMessage  `json:"data,omitempty"`
}

// ChatReadRequest represents a batch of messages to mark as read.
type ChatReadRequest struct {
	GroupID    string                         `json:"group_id"`
	MessageIDs []string                       `json:"message_ids"`
}

// LoadChatHistory loads chat messages for a group from MongoDB with pagination.
// Query parameters:
//   - group_id (required)
//   - limit (optional, default 50, max 100)
//   - before_id or before_ts (optional) for pagination backwards in time
func LoadChatHistory(w http.ResponseWriter, r *http.Request) {
	groupID := r.URL.Query().Get("group_id")
	if groupID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatHistoryResponse{
			Success:  false,
			Messages: []models.ChatMessage{},
			HasMore:  false,
			Total:    0,
		})
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := int64(50)
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 100 {
			limit = int64(v)
		}
	}

	beforeID := r.URL.Query().Get("before_id")
	beforeTsStr := r.URL.Query().Get("before_ts")

	filter := bson.M{
		"group_id": groupID,
	}

	if beforeID != "" {
		if objID, err := primitive.ObjectIDFromHex(beforeID); err == nil {
			filter["_id"] = bson.M{"$lt": objID}
		}
	} else if beforeTsStr != "" {
		if ts, err := time.Parse(time.RFC3339, beforeTsStr); err == nil {
			filter["created_at"] = bson.M{"$lt": ts}
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	coll := database.DB.Collection("chat_messages")

	total, err := coll.CountDocuments(ctx, bson.M{"group_id": groupID})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatHistoryResponse{
			Success:  false,
			Messages: []models.ChatMessage{},
			HasMore:  false,
			Total:    0,
		})
		return
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit)

	cur, err := coll.Find(ctx, filter, opts)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatHistoryResponse{
			Success:  false,
			Messages: []models.ChatMessage{},
			HasMore:  false,
			Total:    0,
		})
		return
	}
	defer cur.Close(ctx)

	var messages []models.ChatMessage
	for cur.Next(ctx) {
		var m models.ChatMessage
		if err := cur.Decode(&m); err != nil {
			continue
		}
		messages = append(messages, m)
	}

	// Reverse to oldest-first for chat UI
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	hasMore := false
	if len(messages) > 0 {
		// If we requested N and got N, it's likely there are more
		hasMore = int64(len(messages)) == limit && int64(len(messages)) < total
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ChatHistoryResponse{
		Success:  true,
		Messages: messages,
		HasMore:  hasMore,
		Total:    total,
	})
}

// SendChatMessageHTTP allows sending a group message over HTTP.
// This path still uses MongoDB for persistence and Redis for fan-out.
func SendChatMessageHTTP(w http.ResponseWriter, r *http.Request) {
	var req ChatSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatSendResponse{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if req.GroupID == "" || req.Text == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatSendResponse{
			Success: false,
			Message: "group_id and text are required",
		})
		return
	}

	// Authenticate user
	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ChatSendResponse{
			Success: false,
			Message: "missing Authorization bearer token",
		})
		return
	}

	userID, ok, err := services.ValidateSession(token)
	if err != nil || !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ChatSendResponse{
			Success: false,
			Message: "invalid session token",
		})
		return
	}

	// Ensure membership
	if !isUserMemberOfGroup(userID, req.GroupID) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(ChatSendResponse{
			Success: false,
			Message: "you must be a member of this group",
		})
		return
	}

	username, _ := services.GetUsernameByID(userID.String())

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	msg := &models.ChatMessage{
		GroupID:        req.GroupID,
		SenderID:       userID.String(),
		SenderUsername: username,
		Text:           req.Text,
		CreatedAt:      time.Now().UTC(),
		Status:         models.MessageStatusSent,
	}

	saved, err := services.SaveChatMessage(ctx, msg)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatSendResponse{
			Success: false,
			Message: "failed to persist message",
		})
		return
	}

	_ = services.PublishChatEvent(ctx, services.ChatEvent{
		Type:    services.EventTypeMessage,
		GroupID: req.GroupID,
		Message: saved,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ChatSendResponse{
		Success: true,
		Message: "message sent",
		Data:    saved,
	})
}

// MarkMessagesReadHTTP marks a batch of messages as read via HTTP.
func MarkMessagesReadHTTP(w http.ResponseWriter, r *http.Request) {
	var req ChatReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "invalid request body",
		})
		return
	}

	if req.GroupID == "" || len(req.MessageIDs) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "group_id and message_ids are required",
		})
		return
	}

	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "missing Authorization bearer token",
		})
		return
	}

	userID, ok, err := services.ValidateSession(token)
	if err != nil || !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "invalid session token",
		})
		return
	}

	if !isUserMemberOfGroup(userID, req.GroupID) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "you must be a member of this group",
		})
		return
	}

	username, _ := services.GetUsernameByID(userID.String())

	var updates []models.ChatMessageReadUpdate
	for _, id := range req.MessageIDs {
		updates = append(updates, models.ChatMessageReadUpdate{
			MessageID: id,
			GroupID:   req.GroupID,
		})
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := services.MarkMessagesRead(ctx, userID, username, updates); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "failed to mark messages as read",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "messages marked as read",
	})
}


