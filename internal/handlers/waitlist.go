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

// SubmitUserWaitlistRequest represents the request to join user waitlist
type SubmitUserWaitlistRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// SubmitTherapistWaitlistRequest represents the request to join therapist waitlist
type SubmitTherapistWaitlistRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone,omitempty"`
}

// SubmitWaitlistResponse represents the response after submitting waitlist
type SubmitWaitlistResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetWaitlistResponse represents the response for getting waitlist entries
type GetWaitlistResponse struct {
	Success   bool                     `json:"success"`
	Message   string                   `json:"message,omitempty"`
	Entries   []map[string]interface{} `json:"entries"`
	Total     int64                    `json:"total"`
}

// SubmitUserWaitlist handles submitting user waitlist form
func SubmitUserWaitlist(w http.ResponseWriter, r *http.Request) {
	var req SubmitUserWaitlistRequest
	var err error

	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitWaitlistResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate waitlist form fields
	if req.Name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitWaitlistResponse{
			Success: false,
			Message: "Name is required",
		})
		return
	}

	if req.Email == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitWaitlistResponse{
			Success: false,
			Message: "Email is required",
		})
		return
	}

	// Get IP address (for analytics, not personal info)
	ipAddress := services.GetIPAddress(r)

	// Insert waitlist entry into PostgreSQL database
	_, err = database.PostgresDB.Exec(`
		INSERT INTO user_waitlist (id, created_at, name, email, ip_address)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), time.Now(), req.Name, req.Email, ipAddress)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SubmitWaitlistResponse{
			Success: false,
			Message: "Failed to join waitlist",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(SubmitWaitlistResponse{
		Success: true,
		Message: "Successfully joined the user waitlist! We'll notify you when we launch.",
	})
}

// SubmitTherapistWaitlist handles submitting therapist waitlist form
func SubmitTherapistWaitlist(w http.ResponseWriter, r *http.Request) {
	var req SubmitTherapistWaitlistRequest
	var err error

	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitWaitlistResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate waitlist form fields
	if req.Name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitWaitlistResponse{
			Success: false,
			Message: "Name is required",
		})
		return
	}

	if req.Email == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitWaitlistResponse{
			Success: false,
			Message: "Email is required",
		})
		return
	}

	// Get IP address (for analytics, not personal info)
	ipAddress := services.GetIPAddress(r)

	// Insert waitlist entry into PostgreSQL database
	_, err = database.PostgresDB.Exec(`
		INSERT INTO therapist_waitlist (id, created_at, name, email, phone, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New(), time.Now(), req.Name, req.Email, req.Phone, ipAddress)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SubmitWaitlistResponse{
			Success: false,
			Message: "Failed to join waitlist",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(SubmitWaitlistResponse{
		Success: true,
		Message: "Successfully joined the therapist waitlist! We'll notify you when we launch.",
	})
}

// GetUserWaitlist handles getting all user waitlist entries (admin only)
func GetUserWaitlist(w http.ResponseWriter, r *http.Request) {
	// Count total entries
	var total int64
	err := database.PostgresDB.QueryRow("SELECT COUNT(*) FROM user_waitlist").Scan(&total)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetWaitlistResponse{
			Success: false,
			Message: "Failed to fetch waitlist",
			Entries: []map[string]interface{}{},
			Total:   0,
		})
		return
	}

	// Query all entries (sorted by created_at descending - newest first)
	rows, err := database.PostgresDB.Query(`
		SELECT id, created_at, name, email, ip_address
		FROM user_waitlist
		ORDER BY created_at DESC
	`)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetWaitlistResponse{
			Success: false,
			Message: "Failed to fetch waitlist",
			Entries: []map[string]interface{}{},
			Total:   0,
		})
		return
	}
	defer rows.Close()

	// Convert to response format
	entryMaps := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id string
		var createdAt time.Time
		var name, email string
		var ipAddress sql.NullString

		if err := rows.Scan(&id, &createdAt, &name, &email, &ipAddress); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(GetWaitlistResponse{
				Success: false,
				Message: "Failed to scan waitlist entries",
				Entries: []map[string]interface{}{},
				Total:   0,
			})
			return
		}

		entryMap := map[string]interface{}{
			"id":         id,
			"name":       name,
			"email":      email,
			"created_at": createdAt,
		}
		if ipAddress.Valid {
			entryMap["ip_address"] = ipAddress.String
		}
		entryMaps = append(entryMaps, entryMap)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetWaitlistResponse{
		Success: true,
		Entries: entryMaps,
		Total:   total,
	})
}

// GetTherapistWaitlist handles getting all therapist waitlist entries (admin only)
func GetTherapistWaitlist(w http.ResponseWriter, r *http.Request) {
	// Count total entries
	var total int64
	err := database.PostgresDB.QueryRow("SELECT COUNT(*) FROM therapist_waitlist").Scan(&total)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetWaitlistResponse{
			Success: false,
			Message: "Failed to fetch waitlist",
			Entries: []map[string]interface{}{},
			Total:   0,
		})
		return
	}

	// Query all entries (sorted by created_at descending - newest first)
	rows, err := database.PostgresDB.Query(`
		SELECT id, created_at, name, email, phone, ip_address
		FROM therapist_waitlist
		ORDER BY created_at DESC
	`)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetWaitlistResponse{
			Success: false,
			Message: "Failed to fetch waitlist",
			Entries: []map[string]interface{}{},
			Total:   0,
		})
		return
	}
	defer rows.Close()

	// Convert to response format
	entryMaps := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id string
		var createdAt time.Time
		var name, email string
		var phone, ipAddress sql.NullString

		if err := rows.Scan(&id, &createdAt, &name, &email, &phone, &ipAddress); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(GetWaitlistResponse{
				Success: false,
				Message: "Failed to scan waitlist entries",
				Entries: []map[string]interface{}{},
				Total:   0,
			})
			return
		}

		entryMap := map[string]interface{}{
			"id":         id,
			"name":       name,
			"email":      email,
			"created_at": createdAt,
		}
		if phone.Valid {
			entryMap["phone"] = phone.String
		}
		if ipAddress.Valid {
			entryMap["ip_address"] = ipAddress.String
		}
		entryMaps = append(entryMaps, entryMap)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetWaitlistResponse{
		Success: true,
		Entries: entryMaps,
		Total:   total,
	})
}

