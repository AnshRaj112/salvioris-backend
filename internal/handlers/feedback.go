package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SubmitFeedbackRequest represents the request to submit feedback
type SubmitFeedbackRequest struct {
	Feedback string `json:"feedback"`
}

// SubmitFeedbackResponse represents the response after submitting feedback
type SubmitFeedbackResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetFeedbacksResponse represents the response for getting feedbacks
type GetFeedbacksResponse struct {
	Success   bool                     `json:"success"`
	Message   string                   `json:"message,omitempty"`
	Feedbacks []map[string]interface{} `json:"feedbacks"`
	Total     int64                    `json:"total"`
}

// SubmitFeedback handles submitting feedback
func SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	var req SubmitFeedbackRequest
	var err error

	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitFeedbackResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate feedback
	if req.Feedback == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitFeedbackResponse{
			Success: false,
			Message: "Feedback is required",
		})
		return
	}

	if len(req.Feedback) < 10 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitFeedbackResponse{
			Success: false,
			Message: "Feedback must be at least 10 characters long",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get IP address (for analytics, not personal info)
	ipAddress := services.GetIPAddress(r)

	// Create feedback
	feedback := models.Feedback{
		ID:        primitive.NewObjectID(),
		CreatedAt: time.Now(),
		Feedback:  req.Feedback,
		IPAddress: ipAddress,
	}

	// Insert feedback into database
	_, err = database.DB.Collection("feedbacks").InsertOne(ctx, feedback)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SubmitFeedbackResponse{
			Success: false,
			Message: "Failed to submit feedback",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(SubmitFeedbackResponse{
		Success: true,
		Message: "Feedback submitted successfully. Thank you!",
	})
}

// GetFeedbacks handles getting all feedbacks (admin only)
func GetFeedbacks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Count total feedbacks
	total, err := database.DB.Collection("feedbacks").CountDocuments(ctx, bson.M{})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetFeedbacksResponse{
			Success:   false,
			Message:   "Failed to fetch feedbacks",
			Feedbacks: []map[string]interface{}{},
			Total:     0,
		})
		return
	}

	// Find all feedbacks (sorted by created_at descending - newest first)
	findOptions := options.Find()
	findOptions.SetSort(bson.M{"created_at": -1}) // Descending order

	cursor, err := database.DB.Collection("feedbacks").Find(ctx, bson.M{}, findOptions)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetFeedbacksResponse{
			Success:   false,
			Message:   "Failed to fetch feedbacks",
			Feedbacks: []map[string]interface{}{},
			Total:     0,
		})
		return
	}
	defer cursor.Close(ctx)

	var feedbacks []models.Feedback
	if err = cursor.All(ctx, &feedbacks); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetFeedbacksResponse{
			Success:   false,
			Message:   "Failed to fetch feedbacks",
			Feedbacks: []map[string]interface{}{},
			Total:     0,
		})
		return
	}

	// Convert to response format
	feedbackMaps := make([]map[string]interface{}, 0, len(feedbacks))
	for _, feedback := range feedbacks {
		feedbackMap := map[string]interface{}{
			"id":         feedback.ID.Hex(),
			"feedback":   feedback.Feedback,
			"created_at": feedback.CreatedAt,
		}
		if feedback.IPAddress != "" {
			feedbackMap["ip_address"] = feedback.IPAddress
		}
		feedbackMaps = append(feedbackMaps, feedbackMap)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetFeedbacksResponse{
		Success:   true,
		Feedbacks: feedbackMaps,
		Total:     total,
	})
}

