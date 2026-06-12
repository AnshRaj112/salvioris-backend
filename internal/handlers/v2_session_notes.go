package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type sessionNoteRequest struct {
	AppointmentID           string      `json:"appointment_id,omitempty"`
	Content                 interface{} `json:"content,omitempty"`
	PlainText               string      `json:"plain_text,omitempty"`
	FollowUpRecommendations string      `json:"follow_up_recommendations,omitempty"`
	ProgressRating          int         `json:"progress_rating,omitempty"`
	Attachments             []string    `json:"attachments,omitempty"`
	SessionDate             string      `json:"session_date,omitempty"`
}

func ListSessionNotesV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}

	ctx, cancel := mongoCtx()
	defer cancel()

	filter := bson.M{"tenant_id": tenantID.String(), "patient_id": patientID.String()}
	opts := options.Find().SetSort(bson.D{{Key: "session_number", Value: 1}})

	cursor, err := database.DB.Collection("session_notes").Find(ctx, filter, opts)
	if err != nil {
		http.Error(w, "Failed to list notes", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var notes []models.SessionNote
	if err = cursor.All(ctx, &notes); err != nil {
		http.Error(w, "Failed to read notes", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": notes})
}

func CreateSessionNoteV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}

	var req sessionNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx, cancel := mongoCtx()
	defer cancel()

	count, _ := database.DB.Collection("session_notes").CountDocuments(ctx, bson.M{
		"tenant_id": tenantID.String(), "patient_id": patientID.String(),
	})

	sessionDate := time.Now()
	if req.SessionDate != "" {
		if t, err := time.Parse("2006-01-02", req.SessionDate); err == nil {
			sessionDate = t
		}
	}

	now := time.Now()
	note := models.SessionNote{
		ID:                      primitive.NewObjectID(),
		TenantID:                tenantID.String(),
		PatientID:               patientID.String(),
		TherapistID:             therapistID.String(),
		SessionNumber:           int(count) + 1,
		AppointmentID:           strings.TrimSpace(req.AppointmentID),
		Status:                  "draft",
		SessionDate:             sessionDate,
		PatientSnapshot:         loadPatientSnapshot(patientID),
		TherapistSnapshot:       loadTherapistSnapshot(therapistID),
		Content:                 req.Content,
		PlainText:               strings.TrimSpace(req.PlainText),
		FollowUpRecommendations: strings.TrimSpace(req.FollowUpRecommendations),
		ProgressRating:          req.ProgressRating,
		Attachments:             req.Attachments,
		CreatedAt:               now,
		UpdatedAt:               now,
	}

	if _, err := database.DB.Collection("session_notes").InsertOne(ctx, note); err != nil {
		http.Error(w, "Failed to create note", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": note})
}

func GetSessionNoteV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	noteID, err := primitive.ObjectIDFromHex(chi.URLParam(r, "noteId"))
	if !ok || err != nil || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	ctx, cancel := mongoCtx()
	defer cancel()

	var note models.SessionNote
	err = database.DB.Collection("session_notes").FindOne(ctx, bson.M{
		"_id": noteID, "tenant_id": tenantID.String(), "patient_id": patientID.String(),
	}).Decode(&note)
	if err != nil {
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": note})
}

func UpdateSessionNoteV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	noteID, err := primitive.ObjectIDFromHex(chi.URLParam(r, "noteId"))
	if !ok || err != nil || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	var req sessionNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx, cancel := mongoCtx()
	defer cancel()

	var existing models.SessionNote
	err = database.DB.Collection("session_notes").FindOne(ctx, bson.M{
		"_id": noteID, "tenant_id": tenantID.String(), "patient_id": patientID.String(),
	}).Decode(&existing)
	if err != nil {
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	}
	if existing.Status != "draft" {
		http.Error(w, "Only draft notes can be edited", http.StatusConflict)
		return
	}

	saveNoteVersion(ctx, existing, therapistID.String())

	update := bson.M{
		"content":                   req.Content,
		"plain_text":                strings.TrimSpace(req.PlainText),
		"follow_up_recommendations": strings.TrimSpace(req.FollowUpRecommendations),
		"progress_rating":           req.ProgressRating,
		"attachments":               req.Attachments,
		"updated_at":                time.Now(),
	}
	if req.SessionDate != "" {
		if t, e := time.Parse("2006-01-02", req.SessionDate); e == nil {
			update["session_date"] = t
		}
	}

	_, err = database.DB.Collection("session_notes").UpdateByID(ctx, noteID, bson.M{"$set": update})
	if err != nil {
		http.Error(w, "Failed to update note", http.StatusInternalServerError)
		return
	}

	var note models.SessionNote
	_ = database.DB.Collection("session_notes").FindOne(ctx, bson.M{"_id": noteID}).Decode(&note)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": note})
}

func PublishSessionNoteV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	noteID, err := primitive.ObjectIDFromHex(chi.URLParam(r, "noteId"))
	if !ok || err != nil || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	ctx, cancel := mongoCtx()
	defer cancel()

	var existing models.SessionNote
	err = database.DB.Collection("session_notes").FindOne(ctx, bson.M{
		"_id": noteID, "tenant_id": tenantID.String(), "patient_id": patientID.String(),
	}).Decode(&existing)
	if err != nil {
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	}

	saveNoteVersion(ctx, existing, therapistID.String())
	now := time.Now()
	_, err = database.DB.Collection("session_notes").UpdateByID(ctx, noteID, bson.M{"$set": bson.M{
		"status": "published", "published_at": now, "updated_at": now,
	}})
	if err != nil {
		http.Error(w, "Failed to publish note", http.StatusInternalServerError)
		return
	}

	var note models.SessionNote
	_ = database.DB.Collection("session_notes").FindOne(ctx, bson.M{"_id": noteID}).Decode(&note)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": note})
}

func ListSessionNoteVersionsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	noteID := chi.URLParam(r, "noteId")
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	ctx, cancel := mongoCtx()
	defer cancel()

	cursor, err := database.DB.Collection("session_note_versions").Find(ctx, bson.M{
		"note_id": noteID, "tenant_id": tenantID.String(),
	}, options.Find().SetSort(bson.D{{Key: "version", Value: -1}}))
	if err != nil {
		http.Error(w, "Failed to list versions", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var versions []models.SessionNoteVersion
	_ = cursor.All(ctx, &versions)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": versions})
}

func SearchSessionNotesV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	patientFilter := strings.TrimSpace(r.URL.Query().Get("patient_id"))

	ctx, cancel := mongoCtx()
	defer cancel()

	filter := bson.M{"tenant_id": tenantID.String(), "status": "published"}
	if patientFilter != "" {
		if pid, err := uuid.Parse(patientFilter); err == nil {
			filter["patient_id"] = pid.String()
		}
	}
	if q != "" {
		filter["$text"] = bson.M{"$search": q}
	}

	cursor, err := database.DB.Collection("session_notes").Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "session_date", Value: -1}}).SetLimit(50))
	if err != nil {
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var notes []models.SessionNote
	_ = cursor.All(ctx, &notes)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": notes})
}

func saveNoteVersion(ctx context.Context, note models.SessionNote, changedBy string) {
	count, _ := database.DB.Collection("session_note_versions").CountDocuments(ctx, bson.M{"note_id": note.ID.Hex()})
	ver := models.SessionNoteVersion{
		ID:        primitive.NewObjectID(),
		NoteID:    note.ID.Hex(),
		TenantID:  note.TenantID,
		Version:   int(count) + 1,
		Content:   note.Content,
		PlainText: note.PlainText,
		ChangedBy: changedBy,
		CreatedAt: time.Now(),
	}
	_, _ = database.DB.Collection("session_note_versions").InsertOne(ctx, ver)
}
