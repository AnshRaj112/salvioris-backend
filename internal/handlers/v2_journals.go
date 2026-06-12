package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type journalRequest struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	MoodTag   string `json:"mood_tag,omitempty"`
	IsPrivate bool   `json:"is_private"`
}

type journalCommentRequest struct {
	Comment string `json:"comment"`
}

func CreateJournalV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())
	userID, _ := middleware.UserIDFromCtx(r.Context())

	var req journalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" && req.Content == "" {
		http.Error(w, "Title or content is required", http.StatusBadRequest)
		return
	}

	now := time.Now()
	journal := models.PatientJournal{
		ID:        primitive.NewObjectID(),
		TenantID:  tenantID.String(),
		PatientID: patientID.String(),
		UserID:    userID.String(),
		Title:     req.Title,
		Content:   req.Content,
		MoodTag:   strings.TrimSpace(req.MoodTag),
		IsPrivate: req.IsPrivate,
		CreatedAt: now,
		UpdatedAt: now,
	}

	ctx, cancel := mongoCtx()
	defer cancel()
	if _, err := database.DB.Collection("patient_journals").InsertOne(ctx, journal); err != nil {
		http.Error(w, "Failed to create journal", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": journal})
}

func ListMyJournalsV2(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromCtx(r.Context())
	limit, skip := pagination(r)

	ctx, cancel := mongoCtx()
	defer cancel()
	filter := bson.M{"user_id": userID.String()}
	total, _ := database.DB.Collection("patient_journals").CountDocuments(ctx, filter)

	cursor, err := database.DB.Collection("patient_journals").Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(int64(limit)).SetSkip(int64(skip)))
	if err != nil {
		http.Error(w, "Failed to list journals", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var journals []models.PatientJournal
	_ = cursor.All(ctx, &journals)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": journals, "meta": map[string]int64{"total": total, "limit": int64(limit), "skip": int64(skip)}})
}

func ListPatientJournalsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}

	limit, skip := pagination(r)
	ctx, cancel := mongoCtx()
	defer cancel()

	filter := bson.M{
		"tenant_id":  tenantID.String(),
		"patient_id": patientID.String(),
		"is_private": false,
	}
	total, _ := database.DB.Collection("patient_journals").CountDocuments(ctx, filter)
	cursor, err := database.DB.Collection("patient_journals").Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(int64(limit)).SetSkip(int64(skip)))
	if err != nil {
		http.Error(w, "Failed to list journals", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var journals []models.PatientJournal
	_ = cursor.All(ctx, &journals)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": journals, "meta": map[string]int64{"total": total, "limit": int64(limit), "skip": int64(skip)}})
}

func CommentOnJournalV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	journalID, err := primitive.ObjectIDFromHex(chi.URLParam(r, "journalId"))
	if !ok || err != nil || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	var req journalCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Comment = strings.TrimSpace(req.Comment)
	if req.Comment == "" {
		http.Error(w, "comment is required", http.StatusBadRequest)
		return
	}

	comment := models.TherapistComment{
		TherapistID: therapistID.String(),
		Comment:     req.Comment,
		CreatedAt:   time.Now(),
	}

	ctx, cancel := mongoCtx()
	defer cancel()
	result, err := database.DB.Collection("patient_journals").UpdateOne(ctx, bson.M{
		"_id": journalID, "tenant_id": tenantID.String(), "patient_id": patientID.String(),
	}, bson.M{
		"$push": bson.M{"therapist_comments": comment},
		"$set":  bson.M{"updated_at": time.Now()},
	})
	if err != nil || result.MatchedCount == 0 {
		http.Error(w, "Journal not found", http.StatusNotFound)
		return
	}

	var journal models.PatientJournal
	_ = database.DB.Collection("patient_journals").FindOne(ctx, bson.M{"_id": journalID}).Decode(&journal)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": journal})
}

func pagination(r *http.Request) (int, int) {
	limit, skip := 20, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("skip"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			skip = n
		}
	}
	return limit, skip
}
