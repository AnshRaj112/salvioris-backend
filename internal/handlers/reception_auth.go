package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/AnshRaj112/serenify-backend/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ──────────────────────────────────────────────────────────────────────────────
// Request / Response types
// ──────────────────────────────────────────────────────────────────────────────

type createReceptionistRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type receptionistSigninRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Public: Receptionist Login
// ──────────────────────────────────────────────────────────────────────────────

// ReceptionistSignin authenticates a receptionist and issues a JWT pair.
func ReceptionistSignin(w http.ResponseWriter, r *http.Request) {
	var req receptionistSigninRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	var id uuid.UUID
	var tenantID, therapistID uuid.UUID
	var name, storedHash string
	var isActive bool
	var createdAt time.Time

	err := database.PostgresDB.QueryRow(`
		SELECT r.id, r.tenant_id, r.therapist_id, r.name, r.password_hash, r.is_active, r.created_at
		FROM receptionists r
		WHERE r.email = $1
		LIMIT 1
	`, req.Email).Scan(&id, &tenantID, &therapistID, &name, &storedHash, &isActive, &createdAt)
	if err == sql.ErrNoRows {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if !isActive {
		http.Error(w, "Account has been deactivated. Contact your therapist.", http.StatusForbidden)
		return
	}

	valid, err := utils.VerifyPassword(req.Password, storedHash)
	if err != nil || !valid {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Fetch therapist name for context
	var therapistName string
	_ = database.PostgresDB.QueryRow(`SELECT name FROM therapists WHERE id = $1`, therapistID).Scan(&therapistName)

	tokenPair, err := services.IssueReceptionistTokens(id, tenantID)
	if err != nil {
		log.Printf("ERROR: Failed to issue receptionist tokens for %s: %v", req.Email, err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Also cache in Redis for fast middleware lookups
	services.SetReceptionistAuthCache(tokenPair.AccessToken, id)

	receptionistMap := map[string]interface{}{
		"id":             id.String(),
		"name":           name,
		"email":          req.Email,
		"tenant_id":      tenantID.String(),
		"therapist_id":   therapistID.String(),
		"therapist_name": therapistName,
		"created_at":     createdAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"message":       "Login successful",
		"receptionist":  receptionistMap,
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_in":    tokenPair.ExpiresIn,
	})
}

// ReceptionistSignout clears the Redis cache for the token.
func ReceptionistSignout(w http.ResponseWriter, r *http.Request) {
	// Clear Redis cache for the token
	authHeader := r.Header.Get("Authorization")
	var token string
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}
	if token != "" {
		services.ClearReceptionistAuthCache(token)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Signed out"})
}

// ──────────────────────────────────────────────────────────────────────────────
// Therapist-managed: CRUD for receptionists
// ──────────────────────────────────────────────────────────────────────────────

// TherapistCreateReceptionist creates a new receptionist account for a tenant.
// Protected by TenantAuth — only the owning therapist can call this.
func TherapistCreateReceptionist(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())

	var req createReceptionistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Name == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "name, email, and password are required", http.StatusBadRequest)
		return
	}

	// Check for duplicate within tenant
	var existingID uuid.UUID
	err := database.PostgresDB.QueryRow(`
		SELECT id FROM receptionists WHERE tenant_id = $1 AND email = $2
	`, tenantID, req.Email).Scan(&existingID)
	if err == nil {
		http.Error(w, "A receptionist with this email already exists in your practice", http.StatusConflict)
		return
	}
	if err != sql.ErrNoRows {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	var newID uuid.UUID
	var createdAt time.Time
	err = database.PostgresDB.QueryRow(`
		INSERT INTO receptionists (tenant_id, therapist_id, name, email, password_hash)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, tenantID, therapistID, req.Name, req.Email, hashedPassword).Scan(&newID, &createdAt)
	if err != nil {
		log.Printf("ERROR: Failed to create receptionist: %v", err)
		http.Error(w, "Failed to create receptionist", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id":         newID.String(),
			"name":       req.Name,
			"email":      req.Email,
			"tenant_id":  tenantID.String(),
			"is_active":  true,
			"created_at": createdAt,
		},
	})
}

// TherapistListReceptionists returns all receptionists for the current tenant.
func TherapistListReceptionists(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())

	rows, err := database.PostgresDB.Query(`
		SELECT id, name, email, is_active, created_at, updated_at
		FROM receptionists
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type receptionistRow struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		Email     string    `json:"email"`
		IsActive  bool      `json:"is_active"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	var list []receptionistRow
	for rows.Next() {
		var rec receptionistRow
		var rid uuid.UUID
		if err := rows.Scan(&rid, &rec.Name, &rec.Email, &rec.IsActive, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			continue
		}
		rec.ID = rid.String()
		list = append(list, rec)
	}
	if list == nil {
		list = []receptionistRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": list})
}

// TherapistDeactivateReceptionist soft-deletes a receptionist by setting is_active = false.
func TherapistDeactivateReceptionist(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	receptionistID, err := uuid.Parse(chi.URLParam(r, "receptionistId"))
	if err != nil {
		http.Error(w, "Invalid receptionist ID", http.StatusBadRequest)
		return
	}

	result, err := database.PostgresDB.Exec(`
		UPDATE receptionists SET is_active = FALSE, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
	`, receptionistID, tenantID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		http.Error(w, "Receptionist not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Receptionist deactivated"})
}

// TherapistReactivateReceptionist re-enables a deactivated receptionist.
func TherapistReactivateReceptionist(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	receptionistID, err := uuid.Parse(chi.URLParam(r, "receptionistId"))
	if err != nil {
		http.Error(w, "Invalid receptionist ID", http.StatusBadRequest)
		return
	}

	result, err := database.PostgresDB.Exec(`
		UPDATE receptionists SET is_active = TRUE, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
	`, receptionistID, tenantID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		http.Error(w, "Receptionist not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Receptionist reactivated"})
}
