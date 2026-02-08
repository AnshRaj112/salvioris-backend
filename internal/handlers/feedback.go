package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
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

	// Get IP address (for analytics, not personal info)
	ipAddress := services.GetIPAddress(r)

	// Insert feedback into PostgreSQL database
	_, err = database.PostgresDB.Exec(`
		INSERT INTO feedbacks (id, created_at, feedback, ip_address)
		VALUES ($1, $2, $3, $4)
	`, uuid.New(), time.Now(), req.Feedback, ipAddress)
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
	// Count total feedbacks
	var total int64
	err := database.PostgresDB.QueryRow("SELECT COUNT(*) FROM feedbacks").Scan(&total)
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

	// Query all feedbacks (sorted by created_at descending - newest first)
	rows, err := database.PostgresDB.Query(`
		SELECT id, created_at, feedback, ip_address
		FROM feedbacks
		ORDER BY created_at DESC
	`)
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
	defer rows.Close()

	// Convert to response format
	feedbackMaps := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id string
		var createdAt time.Time
		var feedback string
		var ipAddress sql.NullString

		if err := rows.Scan(&id, &createdAt, &feedback, &ipAddress); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(GetFeedbacksResponse{
				Success:   false,
				Message:   "Failed to scan feedbacks",
				Feedbacks: []map[string]interface{}{},
				Total:     0,
			})
			return
		}

		feedbackMap := map[string]interface{}{
			"id":         id,
			"feedback":   feedback,
			"created_at": createdAt,
		}
		if ipAddress.Valid {
			feedbackMap["ip_address"] = ipAddress.String
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

