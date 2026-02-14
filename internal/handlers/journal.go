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

// requireJournalAuth validates the session and returns the authenticated user's ID. Returns ("", false) if not authenticated.
func requireJournalAuth(r *http.Request) (string, bool) {
	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		return "", false
	}
	userID, ok, err := services.ValidateSession(token)
	if err != nil || !ok {
		return "", false
	}
	return userID.String(), true
}

type CreateJournalRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	UserID  string `json:"user_id"`
}

type CreateJournalResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Journal map[string]interface{} `json:"journal,omitempty"`
}

type GetJournalsResponse struct {
	Success  bool                     `json:"success"`
	Message  string                   `json:"message,omitempty"`
	Journals []map[string]interface{} `json:"journals"`
	Total    int64                    `json:"total"`
}

// CreateJournal creates a new journal entry for a logged-in user (requires authentication).
func CreateJournal(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireJournalAuth(r)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(CreateJournalResponse{
			Success: false,
			Message: "Authentication required",
		})
		return
	}

	var req CreateJournalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateJournalResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if req.Content == "" && req.Title == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateJournalResponse{
			Success: false,
			Message: "Title or content is required",
		})
		return
	}

	// Optional basic IP-based protection (reuse existing helper)
	ipAddress := services.GetIPAddress(r)
	_ = ipAddress // Currently not used for blocking, but kept for future auditing

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now()
	journal := models.Journal{
		ID:           primitive.NewObjectID(),
		CreatedAt:    now,
		UpdatedAt:    now,
		UserIDString: userID, // From session only; body user_id is ignored
		Title:        req.Title,
		Content:      req.Content,
	}

	_, err := database.DB.Collection("journals").InsertOne(ctx, journal)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CreateJournalResponse{
			Success: false,
			Message: "Failed to create journal entry",
		})
		return
	}

	journalMap := map[string]interface{}{
		"id":         journal.ID.Hex(),
		"title":      journal.Title,
		"content":    journal.Content,
		"created_at": journal.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(CreateJournalResponse{
		Success: true,
		Message: "Journal created successfully",
		Journal: journalMap,
	})
}

// GetJournals returns journal entries for the authenticated user only (requires authentication).
func GetJournals(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireJournalAuth(r)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(GetJournalsResponse{
			Success:  false,
			Message:  "Authentication required",
			Journals: []map[string]interface{}{},
			Total:    0,
		})
		return
	}

	limitStr := r.URL.Query().Get("limit")
	skipStr := r.URL.Query().Get("skip")

	limit := 20
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	skip := 0
	if skipStr != "" {
		if parsedSkip, err := strconv.Atoi(skipStr); err == nil && parsedSkip >= 0 {
			skip = parsedSkip
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"user_id_string": userID,
	}

	total, err := database.DB.Collection("journals").CountDocuments(ctx, filter)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetJournalsResponse{
			Success:  false,
			Journals: []map[string]interface{}{},
			Total:    0,
		})
		return
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.M{"created_at": -1})
	findOptions.SetLimit(int64(limit))
	findOptions.SetSkip(int64(skip))

	cursor, err := database.DB.Collection("journals").Find(ctx, filter, findOptions)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetJournalsResponse{
			Success:  false,
			Journals: []map[string]interface{}{},
			Total:    0,
		})
		return
	}
	defer cursor.Close(ctx)

	var journals []models.Journal
	if err = cursor.All(ctx, &journals); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetJournalsResponse{
			Success:  false,
			Journals: []map[string]interface{}{},
			Total:    0,
		})
		return
	}

	result := make([]map[string]interface{}, 0, len(journals))
	for _, j := range journals {
		journalMap := map[string]interface{}{
			"id":         j.ID.Hex(),
			"title":      j.Title,
			"content":    j.Content,
			"created_at": j.CreatedAt,
		}
		result = append(result, journalMap)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetJournalsResponse{
		Success:  true,
		Journals: result,
		Total:    total,
	})
}


