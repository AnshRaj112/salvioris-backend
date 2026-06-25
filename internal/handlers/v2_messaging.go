package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type sendMessageRequest struct {
	Content       string `json:"content"`
	Type          string `json:"type,omitempty"`
	AttachmentURL string `json:"attachment_url,omitempty"`
}

type DMConversationResponse struct {
	models.DMConversation
	PatientName string `json:"patient_name"`
}

func ListConversationsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	ctx, cancel := mongoCtx()
	defer cancel()

	cursor, err := database.DB.Collection("dm_conversations").Find(ctx,
		bson.M{"tenant_id": tenantID.String()},
		options.Find().SetSort(bson.D{{Key: "last_message_at", Value: -1}}))
	if err != nil {
		http.Error(w, "Failed to list conversations", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var convos []models.DMConversation
	_ = cursor.All(ctx, &convos)

	patientNames := make(map[string]string)
	rows, err := database.PostgresDB.Query(`SELECT id, full_name FROM patients WHERE tenant_id = $1`, tenantID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pid uuid.UUID
			var name string
			if err := rows.Scan(&pid, &name); err == nil {
				patientNames[pid.String()] = name
			}
		}
	}

	responseList := make([]DMConversationResponse, 0)
	for _, c := range convos {
		name := patientNames[c.PatientID]
		if name == "" {
			name = "Unknown Patient"
		}
		responseList = append(responseList, DMConversationResponse{
			DMConversation: c,
			PatientName:    name,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": responseList})
}

func GetOrCreatePatientConversationV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}

	var therapistID uuid.UUID
	_ = database.PostgresDB.QueryRow(`
		SELECT COALESCE(assigned_therapist_id, (SELECT therapist_id FROM tenants WHERE id = $1))
		FROM patients WHERE id = $2
	`, tenantID, patientID).Scan(&therapistID)

	convo, err := ensureConversation(tenantID, patientID, therapistID)
	if err != nil {
		http.Error(w, "Failed to get conversation", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": convo})
}

func ListConversationMessagesV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	convoID := chi.URLParam(r, "conversationId")
	if !conversationInTenant(tenantID, convoID) {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}
	listMessages(w, r, tenantID.String(), convoID)
}

func SendConversationMessageV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())
	convoID := chi.URLParam(r, "conversationId")
	if !conversationInTenant(tenantID, convoID) {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	msg, err := insertDMMessage(tenantID.String(), convoID, therapistID.String(), "therapist", req)
	if err != nil {
		http.Error(w, "Failed to send message", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": msg})
}

func MarkConversationReadV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	convoID := chi.URLParam(r, "conversationId")
	ctx, cancel := mongoCtx()
	defer cancel()

	_, _ = database.DB.Collection("dm_conversations").UpdateOne(ctx,
		bson.M{"_id": mustObjectID(convoID), "tenant_id": tenantID.String()},
		bson.M{"$set": bson.M{"unread_count_therapist": 0}},
	)
	_, _ = database.DB.Collection("dm_messages").UpdateMany(ctx,
		bson.M{"conversation_id": convoID, "sender_role": "patient", "read_at": nil},
		bson.M{"$set": bson.M{"read_at": time.Now()}},
	)
	w.WriteHeader(http.StatusNoContent)
}

func GetMyConversationV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())

	var therapistID uuid.UUID
	_ = database.PostgresDB.QueryRow(`
		SELECT COALESCE(assigned_therapist_id, (SELECT therapist_id FROM tenants WHERE id = $1))
		FROM patients WHERE id = $2
	`, tenantID, patientID).Scan(&therapistID)

	convo, err := ensureConversation(tenantID, patientID, therapistID)
	if err != nil {
		http.Error(w, "Failed to get conversation", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": convo})
}

func ListMyMessagesV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())

	var therapistID uuid.UUID
	_ = database.PostgresDB.QueryRow(`
		SELECT COALESCE(assigned_therapist_id, (SELECT therapist_id FROM tenants WHERE id = $1))
		FROM patients WHERE id = $2
	`, tenantID, patientID).Scan(&therapistID)

	convo, err := ensureConversation(tenantID, patientID, therapistID)
	if err != nil {
		http.Error(w, "Failed to get conversation", http.StatusInternalServerError)
		return
	}
	listMessages(w, r, tenantID.String(), convo.ID.Hex())
}

func SendMyMessageV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())
	userID, _ := middleware.UserIDFromCtx(r.Context())

	var therapistID uuid.UUID
	_ = database.PostgresDB.QueryRow(`
		SELECT COALESCE(assigned_therapist_id, (SELECT therapist_id FROM tenants WHERE id = $1))
		FROM patients WHERE id = $2
	`, tenantID, patientID).Scan(&therapistID)

	convo, err := ensureConversation(tenantID, patientID, therapistID)
	if err != nil {
		http.Error(w, "Failed to get conversation", http.StatusInternalServerError)
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	msg, err := insertDMMessage(tenantID.String(), convo.ID.Hex(), userID.String(), "patient", req)
	if err != nil {
		http.Error(w, "Failed to send message", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": msg})
}

func MarkMyConversationReadV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())

	var therapistID uuid.UUID
	_ = database.PostgresDB.QueryRow(`
		SELECT COALESCE(assigned_therapist_id, (SELECT therapist_id FROM tenants WHERE id = $1))
		FROM patients WHERE id = $2
	`, tenantID, patientID).Scan(&therapistID)

	convo, _ := ensureConversation(tenantID, patientID, therapistID)
	ctx, cancel := mongoCtx()
	defer cancel()

	_, _ = database.DB.Collection("dm_conversations").UpdateOne(ctx,
		bson.M{"_id": convo.ID},
		bson.M{"$set": bson.M{"unread_count_patient": 0}},
	)
	_, _ = database.DB.Collection("dm_messages").UpdateMany(ctx,
		bson.M{"conversation_id": convo.ID.Hex(), "sender_role": "therapist", "read_at": nil},
		bson.M{"$set": bson.M{"read_at": time.Now()}},
	)
	w.WriteHeader(http.StatusNoContent)
}

func ensureConversation(tenantID, patientID, therapistID uuid.UUID) (models.DMConversation, error) {
	ctx, cancel := mongoCtx()
	defer cancel()

	filter := bson.M{
		"tenant_id": tenantID.String(), "patient_id": patientID.String(), "therapist_id": therapistID.String(),
	}
	var convo models.DMConversation
	err := database.DB.Collection("dm_conversations").FindOne(ctx, filter).Decode(&convo)
	if err == nil {
		return convo, nil
	}

	now := time.Now()
	convo = models.DMConversation{
		ID:          primitive.NewObjectID(),
		TenantID:    tenantID.String(),
		PatientID:   patientID.String(),
		TherapistID: therapistID.String(),
		CreatedAt:   now,
		LastMessageAt: now,
	}
	_, err = database.DB.Collection("dm_conversations").InsertOne(ctx, convo)
	return convo, err
}

func insertDMMessage(tenantID, convoID, senderID, role string, req sendMessageRequest) (models.DMMessage, error) {
	content := strings.TrimSpace(req.Content)
	if content == "" && req.AttachmentURL == "" {
		return models.DMMessage{}, errEmptyMessage
	}
	msgType := req.Type
	if msgType == "" {
		msgType = "text"
		if req.AttachmentURL != "" {
			msgType = "attachment"
		}
	}

	now := time.Now()
	msg := models.DMMessage{
		ID:             primitive.NewObjectID(),
		TenantID:       tenantID,
		ConversationID: convoID,
		SenderID:       senderID,
		SenderRole:     role,
		Type:           msgType,
		Content:        content,
		AttachmentURL:  strings.TrimSpace(req.AttachmentURL),
		CreatedAt:      now,
	}

	ctx, cancel := mongoCtx()
	defer cancel()
	if _, err := database.DB.Collection("dm_messages").InsertOne(ctx, msg); err != nil {
		return msg, err
	}

	preview := content
	if len(preview) > 80 {
		preview = preview[:80] + "..."
	}
	inc := bson.M{"unread_count_therapist": 1}
	if role == "therapist" {
		inc = bson.M{"unread_count_patient": 1}
	}
	_, _ = database.DB.Collection("dm_conversations").UpdateOne(ctx,
		bson.M{"_id": mustObjectID(convoID)},
		bson.M{"$set": bson.M{"last_message_at": now, "last_message_preview": preview}, "$inc": inc},
	)

	services.BroadcastDM(tenantID, services.DMEvent{
		Type: "message.new", ConversationID: convoID, TenantID: tenantID,
		SenderID: senderID, SenderRole: role, MessageID: msg.ID.Hex(),
		Content: content, Timestamp: now.Format(time.RFC3339),
	}, senderID)

	return msg, nil
}

var errEmptyMessage = &emptyMsgErr{}

type emptyMsgErr struct{}

func (e *emptyMsgErr) Error() string { return "empty message" }

func listMessages(w http.ResponseWriter, r *http.Request, tenantID, convoID string) {
	limit, skip := pagination(r)
	ctx, cancel := mongoCtx()
	defer cancel()

	filter := bson.M{"tenant_id": tenantID, "conversation_id": convoID}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		filter["content"] = bson.M{"$regex": q, "$options": "i"}
	}

	total, _ := database.DB.Collection("dm_messages").CountDocuments(ctx, filter)
	cursor, err := database.DB.Collection("dm_messages").Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).
			SetLimit(int64(limit)).SetSkip(int64(skip)))
	if err != nil {
		http.Error(w, "Failed to list messages", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var messages []models.DMMessage
	_ = cursor.All(ctx, &messages)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": messages,
		"meta": map[string]int64{"total": total, "limit": int64(limit), "skip": int64(skip)},
	})
}

func conversationInTenant(tenantID uuid.UUID, convoID string) bool {
	ctx, cancel := mongoCtx()
	defer cancel()
	n, err := database.DB.Collection("dm_conversations").CountDocuments(ctx, bson.M{
		"_id": mustObjectID(convoID), "tenant_id": tenantID.String(),
	})
	return err == nil && n > 0
}

func mustObjectID(hex string) primitive.ObjectID {
	id, _ := primitive.ObjectIDFromHex(hex)
	return id
}
