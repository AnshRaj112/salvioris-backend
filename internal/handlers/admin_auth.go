package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/pkg/utils"
	"github.com/google/uuid"
)

// AdminSignupRequest represents the request to create an admin account
type AdminSignupRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AdminSigninRequest represents the request to sign in as admin
type AdminSigninRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AdminSignupResponse represents the response after creating admin account
type AdminSignupResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Admin   map[string]interface{} `json:"admin,omitempty"`
}

// AdminSigninResponse represents the response after admin signin
type AdminSigninResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Admin   map[string]interface{} `json:"admin,omitempty"`
	Token   string `json:"token,omitempty"`
}

// AdminSignup handles creating a new admin account (backend only, no frontend)
func AdminSignup(w http.ResponseWriter, r *http.Request) {
	var req AdminSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AdminSignupResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate required fields
	if req.Username == "" || req.Email == "" || req.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AdminSignupResponse{
			Success: false,
			Message: "Username, email, and password are required",
		})
		return
	}

	// Validate password length
	if len(req.Password) < 8 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AdminSignupResponse{
			Success: false,
			Message: "Password must be at least 8 characters long",
		})
		return
	}

	// Check if admin with username already exists
	var existingUsername string
	err := database.PostgresDB.QueryRow("SELECT username FROM admins WHERE username = $1", req.Username).Scan(&existingUsername)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(AdminSignupResponse{
			Success: false,
			Message: "Admin with this username already exists",
		})
		return
	} else if err != sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AdminSignupResponse{
			Success: false,
			Message: "Database error",
		})
		return
	}

	// Check if admin with email already exists
	var existingEmail string
	err = database.PostgresDB.QueryRow("SELECT email FROM admins WHERE email = $1", req.Email).Scan(&existingEmail)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(AdminSignupResponse{
			Success: false,
			Message: "Admin with this email already exists",
		})
		return
	} else if err != sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AdminSignupResponse{
			Success: false,
			Message: "Database error",
		})
		return
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AdminSignupResponse{
			Success: false,
			Message: "Failed to hash password",
		})
		return
	}

	// Create admin
	adminID := uuid.New()
	now := time.Now()
	_, err = database.PostgresDB.Exec(`
		INSERT INTO admins (id, created_at, updated_at, username, email, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, adminID, now, now, req.Username, req.Email, hashedPassword, true)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AdminSignupResponse{
			Success: false,
			Message: "Failed to create admin account",
		})
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(AdminSignupResponse{
		Success: true,
		Message: "Admin account created successfully",
		Admin: map[string]interface{}{
			"id":       adminID.String(),
			"username": req.Username,
			"email":    req.Email,
			"created_at": now,
		},
	})
}

// AdminSignin handles admin login
func AdminSignin(w http.ResponseWriter, r *http.Request) {
	var req AdminSigninRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AdminSigninResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate required fields
	if req.Username == "" || req.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AdminSigninResponse{
			Success: false,
			Message: "Username and password are required",
		})
		return
	}

	// Find admin by username
	var adminID uuid.UUID
	var username, email, passwordHash string
	var isActive bool
	var createdAt time.Time

	err := database.PostgresDB.QueryRow(`
		SELECT id, created_at, username, email, password_hash, is_active
		FROM admins
		WHERE username = $1
	`, req.Username).Scan(&adminID, &createdAt, &username, &email, &passwordHash, &isActive)
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(AdminSigninResponse{
				Success: false,
				Message: "Invalid username or password",
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AdminSigninResponse{
			Success: false,
			Message: "Database error",
		})
		return
	}

	// Check if admin is active
	if !isActive {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(AdminSigninResponse{
			Success: false,
			Message: "Admin account is inactive",
		})
		return
	}

	// Verify password
	valid, err := utils.VerifyPassword(req.Password, passwordHash)
	if err != nil || !valid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(AdminSigninResponse{
			Success: false,
			Message: "Invalid username or password",
		})
		return
	}

	// Generate JWT token (you can use your existing JWT utility)
	// For now, we'll return a simple success response
	// TODO: Add JWT token generation if needed

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(AdminSigninResponse{
		Success: true,
		Message: "Admin signed in successfully",
		Admin: map[string]interface{}{
			"id":        adminID.String(),
			"username":  username,
			"email":     email,
			"created_at": createdAt,
		},
		Token: "", // Add JWT token here if needed
	})
}

