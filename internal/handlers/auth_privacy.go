package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/pkg/utils"
	"github.com/google/uuid"
)

// Privacy-First Signup Request
type PrivacySignupRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	RecoveryEmail string `json:"recovery_email,omitempty"` // Optional but recommended
}

// Privacy-First Signin Request
type PrivacySigninRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// CheckUsernameRequest for username availability
type CheckUsernameRequest struct {
	Username string `json:"username"`
}

// ForgotUsernameRequest for username recovery
type ForgotUsernameRequest struct {
	RecoveryEmail string `json:"recovery_email"`
}

// ForgotPasswordRequest for password reset
type ForgotPasswordRequest struct {
	RecoveryEmail string `json:"recovery_email"`
}

// PrivacyAuthResponse returns only anonymous data
type PrivacyAuthResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	User    map[string]interface{} `json:"user,omitempty"`
}

// CheckUsernameAvailability checks if a username is available
func CheckUsernameAvailability(w http.ResponseWriter, r *http.Request) {
	var req CheckUsernameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate username format
	if err := utils.ValidateUsername(req.Username); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"available": false,
			"message": err.Error(),
		})
		return
	}

	// Normalize username (lowercase)
	normalizedUsername := utils.NormalizeUsername(req.Username)

	// Check if username exists
	var existingUsername string
	err := database.PostgresDB.QueryRow(
		"SELECT username FROM users WHERE LOWER(username) = $1",
		normalizedUsername,
	).Scan(&existingUsername)

	available := err == sql.ErrNoRows

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"available": available,
		"username":  req.Username,
		"message":   map[bool]string{true: "Username is available", false: "Username is already taken"}[available],
	})
}

// PrivacySignup handles privacy-first user registration
func PrivacySignup(w http.ResponseWriter, r *http.Request) {
	var req PrivacySignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate username
	if err := utils.ValidateUsername(req.Username); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PrivacyAuthResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	// Validate password
	if len(req.Password) < 8 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PrivacyAuthResponse{
			Success: false,
			Message: "Password must be at least 8 characters",
		})
		return
	}

	// Normalize username
	normalizedUsername := utils.NormalizeUsername(req.Username)

	// Check if username already exists
	var existingUsername string
	err := database.PostgresDB.QueryRow(
		"SELECT username FROM users WHERE LOWER(username) = $1",
		normalizedUsername,
	).Scan(&existingUsername)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(PrivacyAuthResponse{
			Success: false,
			Message: "Username is already taken",
		})
		return
	} else if err != sql.ErrNoRows {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	// Start transaction
	tx, err := database.PostgresDB.Begin()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Create user (public profile only)
	userID := uuid.New()
	_, err = tx.Exec(`
		INSERT INTO users (id, username, password_hash, created_at, is_active)
		VALUES ($1, $2, $3, NOW(), TRUE)
	`, userID, normalizedUsername, hashedPassword)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// If recovery email provided, encrypt and store it
	if req.RecoveryEmail != "" {
		emailEncrypted, err := utils.Encrypt(req.RecoveryEmail)
		if err != nil {
			http.Error(w, "Failed to encrypt recovery email", http.StatusInternalServerError)
			return
		}

		_, err = tx.Exec(`
			INSERT INTO user_recovery (id, user_id, email_encrypted, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2, NOW(), NOW())
		`, userID, emailEncrypted)
		if err != nil {
			http.Error(w, "Failed to save recovery data", http.StatusInternalServerError)
			return
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Return anonymous user data only
	userMap := map[string]interface{}{
		"id":        userID.String(),
		"username":  normalizedUsername,
		"created_at": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(PrivacyAuthResponse{
		Success: true,
		Message: "Account created successfully",
		User:    userMap,
	})
}

// PrivacySignin handles privacy-first user login
func PrivacySignin(w http.ResponseWriter, r *http.Request) {
	var req PrivacySigninRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Normalize username
	normalizedUsername := utils.NormalizeUsername(req.Username)

	// Find user
	var userID uuid.UUID
	var passwordHash string
	var isActive bool
	var createdAt time.Time

	err := database.PostgresDB.QueryRow(`
		SELECT id, password_hash, created_at, is_active
		FROM users
		WHERE LOWER(username) = $1
	`, normalizedUsername).Scan(&userID, &passwordHash, &createdAt, &isActive)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Check if account is active
	if !isActive {
		http.Error(w, "Account is inactive", http.StatusForbidden)
		return
	}

	// Verify password
	valid, err := utils.VerifyPassword(req.Password, passwordHash)
	if err != nil || !valid {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// Track device for support purposes
	deviceToken := generateDeviceToken()
	ipAddress := getIPAddress(r)
	userAgent := r.UserAgent()

	// Try to insert or update device tracking
	// If device_token already exists for this user, update it; otherwise insert new
	_, err = database.PostgresDB.Exec(`
		INSERT INTO user_devices (id, user_id, device_token, ip_address, user_agent, last_used, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (device_token) DO UPDATE SET
			user_id = $1,
			last_used = NOW(),
			ip_address = $3,
			user_agent = $4
	`, userID, deviceToken, ipAddress, userAgent)
	// Note: Ignore device tracking errors - not critical for login

	// Return anonymous user data only
	userMap := map[string]interface{}{
		"id":        userID.String(),
		"username":  normalizedUsername,
		"created_at": createdAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PrivacyAuthResponse{
		Success: true,
		Message: "Login successful",
		User:    userMap,
	})
}

// ForgotUsername handles username recovery via email
func ForgotUsername(w http.ResponseWriter, r *http.Request) {
	var req ForgotUsernameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RecoveryEmail == "" {
		http.Error(w, "Recovery email is required", http.StatusBadRequest)
		return
	}

	// Find user by encrypted email (we need to search all encrypted emails)
	// This is a limitation - we'd need to decrypt all emails to search
	// For production, consider using a hash of email for searchable index
	rows, err := database.PostgresDB.Query(`
		SELECT ur.user_id, ur.email_encrypted, u.username
		FROM user_recovery ur
		JOIN users u ON u.id = ur.user_id
		WHERE ur.email_encrypted IS NOT NULL
	`)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var foundUserID string
	var foundUsername string

	for rows.Next() {
		var userID uuid.UUID
		var emailEncrypted sql.NullString
		var username string

		if err := rows.Scan(&userID, &emailEncrypted, &username); err != nil {
			continue
		}

		if emailEncrypted.Valid {
			decryptedEmail, err := utils.Decrypt(emailEncrypted.String)
			if err == nil && strings.EqualFold(decryptedEmail, req.RecoveryEmail) {
				foundUserID = userID.String()
				foundUsername = username
				break
			}
		}
	}

	// Always return success (privacy: don't reveal if email exists)
	// In production, send email if found
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "If an account exists with this email, you will receive your username via email.",
		// Don't return username in response - send via email only
	})
}

// ForgotPassword handles password reset via email
func ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RecoveryEmail == "" {
		http.Error(w, "Recovery email is required", http.StatusBadRequest)
		return
	}

	// Similar to ForgotUsername - find user by email
	// In production, generate reset token and send via email
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "If an account exists with this email, you will receive a password reset link.",
	})
}

// Helper functions
func generateDeviceToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func getIPAddress(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

