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

// SubmitContactRequest represents the request to submit contact form
type SubmitContactRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// SubmitContactResponse represents the response after submitting contact form
type SubmitContactResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetContactsResponse represents the response for getting contact submissions
type GetContactsResponse struct {
	Success   bool                     `json:"success"`
	Message   string                   `json:"message,omitempty"`
	Contacts  []map[string]interface{} `json:"contacts"`
	Total     int64                    `json:"total"`
}

// SubmitContact handles submitting contact form
func SubmitContact(w http.ResponseWriter, r *http.Request) {
	var req SubmitContactRequest
	var err error

	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitContactResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate contact form fields
	if req.Name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitContactResponse{
			Success: false,
			Message: "Name is required",
		})
		return
	}

	if req.Email == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitContactResponse{
			Success: false,
			Message: "Email is required",
		})
		return
	}

	if req.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitContactResponse{
			Success: false,
			Message: "Message is required",
		})
		return
	}

	if len(req.Message) < 10 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitContactResponse{
			Success: false,
			Message: "Message must be at least 10 characters long",
		})
		return
	}

	// Get IP address (for analytics, not personal info)
	ipAddress := services.GetIPAddress(r)

	// Insert contact submission into PostgreSQL database
	_, err = database.PostgresDB.Exec(`
		INSERT INTO contact_us (id, created_at, name, email, message, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New(), time.Now(), req.Name, req.Email, req.Message, ipAddress)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SubmitContactResponse{
			Success: false,
			Message: "Failed to submit contact form",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(SubmitContactResponse{
		Success: true,
		Message: "Contact form submitted successfully. We'll get back to you soon!",
	})
}

// GetContacts handles getting all contact submissions (admin only)
func GetContacts(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	// Count total contacts
	var total int64
	err := database.PostgresDB.QueryRow("SELECT COUNT(*) FROM contact_us").Scan(&total)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetContactsResponse{
			Success:  false,
			Message:  "Failed to fetch contacts",
			Contacts: []map[string]interface{}{},
			Total:    0,
		})
		return
	}

	// Query all contacts (sorted by created_at descending - newest first)
	rows, err := database.PostgresDB.Query(`
		SELECT id, created_at, name, email, message, ip_address
		FROM contact_us
		ORDER BY created_at DESC
	`)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetContactsResponse{
			Success:  false,
			Message:  "Failed to fetch contacts",
			Contacts: []map[string]interface{}{},
			Total:    0,
		})
		return
	}
	defer rows.Close()

	// Convert to response format
	contactMaps := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id string
		var createdAt time.Time
		var name, email, message string
		var ipAddress sql.NullString

		if err := rows.Scan(&id, &createdAt, &name, &email, &message, &ipAddress); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(GetContactsResponse{
				Success:  false,
				Message:  "Failed to scan contacts",
				Contacts: []map[string]interface{}{},
				Total:    0,
			})
			return
		}

		contactMap := map[string]interface{}{
			"id":         id,
			"name":       name,
			"email":      email,
			"message":    message,
			"created_at": createdAt,
		}
		if ipAddress.Valid {
			contactMap["ip_address"] = ipAddress.String
		}
		contactMaps = append(contactMaps, contactMap)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetContactsResponse{
		Success:  true,
		Contacts: contactMaps,
		Total:    total,
	})
}

// DeleteContact deletes a contact submission by ID (admin only)
func DeleteContact(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	contactID := r.URL.Query().Get("id")
	if contactID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitContactResponse{
			Success: false,
			Message: "Contact ID is required",
		})
		return
	}
	if _, err := uuid.Parse(contactID); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SubmitContactResponse{
			Success: false,
			Message: "Invalid contact ID",
		})
		return
	}

	result, err := database.PostgresDB.Exec(`DELETE FROM contact_us WHERE id = $1`, contactID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SubmitContactResponse{
			Success: false,
			Message: "Failed to delete contact",
		})
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(SubmitContactResponse{
			Success: false,
			Message: "Contact not found",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(SubmitContactResponse{
		Success: true,
		Message: "Contact deleted successfully",
	})
}

