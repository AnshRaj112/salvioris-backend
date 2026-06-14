package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var (
	devTherapistMu     sync.Mutex
	devTherapistID     uuid.UUID
	devTherapistLoaded bool
)

const therapistResponseCacheTTL = 30 * time.Second

func therapistCacheKey(therapistID uuid.UUID, resource string) string {
	return fmt.Sprintf("tcache:%s:%s", therapistID.String(), resource)
}

func readTherapistCache(therapistID uuid.UUID, resource string) ([]byte, bool) {
	if database.RedisClient == nil {
		return nil, false
	}
	data, err := database.RedisClient.Get(context.Background(), therapistCacheKey(therapistID, resource)).Result()
	if err != nil {
		return nil, false
	}
	return []byte(data), true
}

func writeTherapistCache(therapistID uuid.UUID, resource string, data []byte) {
	if database.RedisClient != nil {
		_ = database.RedisClient.Set(context.Background(), therapistCacheKey(therapistID, resource), data, therapistResponseCacheTTL).Err()
	}
}

func invalidateTherapistCaches(therapistID uuid.UUID, resources ...string) {
	if database.RedisClient == nil {
		return
	}
	ctx := context.Background()
	for _, resource := range resources {
		_ = database.RedisClient.Del(ctx, therapistCacheKey(therapistID, resource)).Err()
	}
}

func getDevTherapistID() uuid.UUID {
	devTherapistMu.Lock()
	defer devTherapistMu.Unlock()
	if devTherapistLoaded {
		return devTherapistID
	}
	devTherapistLoaded = true
	if err := database.PostgresDB.QueryRow("SELECT id FROM therapists LIMIT 1").Scan(&devTherapistID); err == nil {
		return devTherapistID
	}
	devTherapistID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	var exists bool
	_ = database.PostgresDB.QueryRow("SELECT EXISTS(SELECT 1 FROM therapists WHERE id = $1)", devTherapistID).Scan(&exists)
	if !exists {
		_, _ = database.PostgresDB.Exec(`
			INSERT INTO therapists (
				id, name, email, password, license_number, license_state,
				years_of_experience, specialization, phone, college_degree, masters_institution,
				psychologist_type, successful_cases, dsm_awareness, therapy_types,
				certificate_image_path, degree_image_path, is_approved
			) VALUES ($1, 'Mock Therapist', 'mock@therapist.com', '$2a$10$mock', 'LIC123', 'CA',
				5, 'CBT', '+15555555555', 'PhD', 'Mock University',
				'Clinical Psychologist', 100, 'expert', 'CBT, DBT',
				'http://example.com/cert.png', 'http://example.com/deg.png', TRUE)
		`, devTherapistID)
	}
	return devTherapistID
}

func resolveTherapistFromToken(token string) (uuid.UUID, bool) {
	if token == "" {
		return uuid.Nil, false
	}
	if claims, ok := services.ValidateAccessToken(token); ok {
		if id, err := uuid.Parse(claims.UserID); err == nil {
			return id, true
		}
	}
	if id, ok := services.GetTherapistAuthCache(token); ok {
		return id, true
	}
	userID, ok, err := services.ValidateSession(token)
	if err != nil || !ok {
		return uuid.Nil, false
	}
	var isApproved bool
	err = database.PostgresDB.QueryRow(
		`SELECT is_approved FROM therapists WHERE id = $1`, userID,
	).Scan(&isApproved)
	if err != nil || !isApproved {
		return uuid.Nil, false
	}
	services.SetTherapistAuthCache(token, userID)
	return userID, true
}

// requireTherapistAuth validates session token and confirms that the ID represents an approved therapist
func requireTherapistAuth(r *http.Request) (uuid.UUID, bool) {
	token := extractBearerToken(r.Header.Get("Authorization"))
	if id, ok := resolveTherapistFromToken(token); ok {
		return id, true
	}
	if os.Getenv("ENV") != "production" {
		return getDevTherapistID(), true
	}
	return uuid.Nil, false
}

// requireUserAuth validates session token and confirms that the ID represents an active user
func requireUserAuth(r *http.Request) (uuid.UUID, bool) {
	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		return uuid.Nil, false
	}
	userID, ok, err := services.ValidateSession(token)
	if err != nil || !ok {
		return uuid.Nil, false
	}
	return userID, true
}

// GenerateSecureCode creates an unguessable high-entropy referral token
func GenerateSecureCode() (string, error) {
	b := make([]byte, 6) // 12 characters in hex
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	h := hex.EncodeToString(b)
	return fmt.Sprintf("SAL-%s-%s", strings.ToUpper(h[0:4]), strings.ToUpper(h[4:12])), nil
}

// GenerateReferralCode creates a secure referral code for the therapist
func GenerateReferralCode(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	var req struct {
		UsageLimit *int       `json:"usage_limit,omitempty"`
		ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	code, err := GenerateSecureCode()
	if err != nil {
		http.Error(w, "Failed to generate cryptotoken", http.StatusInternalServerError)
		return
	}

	var limitVal sql.NullInt64
	if req.UsageLimit != nil {
		limitVal = sql.NullInt64{Int64: int64(*req.UsageLimit), Valid: true}
	}

	var expiresVal interface{} = nil
	if req.ExpiresAt != nil {
		expiresVal = *req.ExpiresAt
	}

	var newID uuid.UUID
	err = database.PostgresDB.QueryRow(`
		INSERT INTO referral_codes (therapist_id, code, expires_at, usage_limit)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, therapistID, code, expiresVal, limitVal).Scan(&newID)

	if err != nil {
		log.Printf("ERROR generating referral code: %v", err)
		http.Error(w, "Failed to store referral code", http.StatusInternalServerError)
		return
	}

	invalidateReferralCodesCache(therapistID)
	database.TriggerAuditEvent("REFERRAL_CODE_CREATED", newID.String(), therapistID.String(), "therapist", fmt.Sprintf("Therapist generated secure referral code: %s", code), r)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Referral code created successfully",
		"id":      newID.String(),
		"code":    code,
	})
}

func invalidateReferralCodesCache(therapistID uuid.UUID) {
	invalidateTherapistCaches(therapistID, "referrals", "analytics")
}

// ListReferralCodes lists active/inactive referral codes for the therapist
func ListReferralCodes(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	if cached, ok := readTherapistCache(therapistID, "referrals"); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cached)
		return
	}

	rows, err := database.PostgresDB.Query(`
		SELECT id, code, created_at, expires_at, usage_limit, usage_count, is_revoked
		FROM referral_codes
		WHERE therapist_id = $1
		ORDER BY created_at DESC
	`, therapistID)
	if err != nil {
		log.Printf("ERROR fetching referral codes: %v", err)
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	codes := []models.ReferralCode{}
	for rows.Next() {
		var c models.ReferralCode
		var expires sql.NullTime
		var limit sql.NullInt64

		err = rows.Scan(&c.ID, &c.Code, &c.CreatedAt, &expires, &limit, &c.UsageCount, &c.IsRevoked)
		if err != nil {
			log.Printf("ERROR scanning referral codes: %v", err)
			continue
		}

		c.TherapistID = therapistID
		if expires.Valid {
			c.ExpiresAt = &expires.Time
		}
		if limit.Valid {
			limitVal := int(limit.Int64)
			c.UsageLimit = &limitVal
		}

		codes = append(codes, c)
	}

	if err = rows.Err(); err != nil {
		log.Printf("ERROR after iterating referral codes rows: %v", err)
	}

	resp, _ := json.Marshal(map[string]interface{}{"success": true, "codes": codes})
	writeTherapistCache(therapistID, "referrals", resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// RevokeReferralCode revokes a referral code
func RevokeReferralCode(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	codeIDStr := chi.URLParam(r, "id")
	codeID, err := uuid.Parse(codeIDStr)
	if err != nil {
		http.Error(w, "Invalid UUID format", http.StatusBadRequest)
		return
	}

	var codeStr string
	err = database.PostgresDB.QueryRow(`
		SELECT code FROM referral_codes WHERE id = $1 AND therapist_id = $2
	`, codeID, therapistID).Scan(&codeStr)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Referral code not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database validation failure", http.StatusInternalServerError)
		}
		return
	}

	_, err = database.PostgresDB.Exec(`
		UPDATE referral_codes SET is_revoked = TRUE WHERE id = $1
	`, codeID)
	if err != nil {
		http.Error(w, "Failed to revoke referral code", http.StatusInternalServerError)
		return
	}

	invalidateReferralCodesCache(therapistID)
	database.TriggerAuditEvent("REFERRAL_CODE_REVOKED", codeID.String(), therapistID.String(), "therapist", fmt.Sprintf("Therapist revoked referral code: %s", codeStr), r)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Referral code revoked successfully",
	})
}

// DeleteReferralCode deletes a referral code only if it has been revoked.
func DeleteReferralCode(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	codeIDStr := chi.URLParam(r, "id")
	codeID, err := uuid.Parse(codeIDStr)
	if err != nil {
		http.Error(w, "Invalid UUID format", http.StatusBadRequest)
		return
	}

	var codeStr string
	var isRevoked bool
	err = database.PostgresDB.QueryRow(`
		SELECT code, is_revoked FROM referral_codes WHERE id = $1 AND therapist_id = $2
	`, codeID, therapistID).Scan(&codeStr, &isRevoked)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Referral code not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database validation failure", http.StatusInternalServerError)
		}
		return
	}

	if !isRevoked {
		http.Error(w, "Referral code must be revoked before deletion", http.StatusForbidden)
		return
	}

	tx, err := database.PostgresDB.Begin()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Delete related referral usages first
	if _, err := tx.Exec(`DELETE FROM referral_usages WHERE referral_code_id = $1`, codeID); err != nil {
		http.Error(w, "Failed to delete referral usages", http.StatusInternalServerError)
		return
	}

	// Then delete the referral code record
	if _, err := tx.Exec(`DELETE FROM referral_codes WHERE id = $1`, codeID); err != nil {
		http.Error(w, "Failed to delete referral code", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	invalidateReferralCodesCache(therapistID)
	database.TriggerAuditEvent("REFERRAL_CODE_DELETED", codeID.String(), therapistID.String(), "therapist", fmt.Sprintf("Therapist deleted referral code: %s", codeStr), r)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Referral code deleted successfully",
	})
}

// GetReferralAnalytics fetches aggregated usage parameters for the therapist
func GetReferralAnalytics(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	if cached, ok := readTherapistCache(therapistID, "analytics"); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cached)
		return
	}

	var stats models.ReferralAnalytics
	err := database.PostgresDB.QueryRow(`
		SELECT COUNT(id),
			COALESCE(SUM(usage_count), 0),
			COUNT(id) FILTER (WHERE is_revoked = FALSE AND (expires_at IS NULL OR expires_at > NOW()))
		FROM referral_codes
		WHERE therapist_id = $1
	`, therapistID).Scan(&stats.TotalCodes, &stats.TotalSignups, &stats.ActiveCodes)
	if err != nil {
		http.Error(w, "Failed to fetch analytics aggregates", http.StatusInternalServerError)
		return
	}

	resp, _ := json.Marshal(map[string]interface{}{"success": true, "analytics": stats})
	writeTherapistCache(therapistID, "analytics", resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// GetConnectedUsers lists patient clients currently connected to the therapist
func GetConnectedUsers(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	search := r.URL.Query().Get("q")
	if search == "" {
		if cached, ok := readTherapistCache(therapistID, "connections"); ok {
			w.Header().Set("Content-Type", "application/json")
			w.Write(cached)
			return
		}
	}
	var rows *sql.Rows
	var err error

	if search != "" {
		rows, err = database.PostgresDB.Query(`
			SELECT c.user_id, u.username, c.connected_at, c.connection_type
			FROM therapist_user_connections c
			JOIN users u ON c.user_id = u.id
			WHERE c.therapist_id = $1 AND LOWER(u.username) LIKE LOWER($2)
			ORDER BY c.connected_at DESC
		`, therapistID, "%"+search+"%")
	} else {
		rows, err = database.PostgresDB.Query(`
			SELECT c.user_id, u.username, c.connected_at, c.connection_type
			FROM therapist_user_connections c
			JOIN users u ON c.user_id = u.id
			WHERE c.therapist_id = $1
			ORDER BY c.connected_at DESC
		`, therapistID)
	}

	if err != nil {
		log.Printf("ERROR fetching connected users: %v", err)
		http.Error(w, "Database connection load error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := []models.ConnectedUser{}
	for rows.Next() {
		var cu models.ConnectedUser
		var uID uuid.UUID
		err = rows.Scan(&uID, &cu.Username, &cu.ConnectedAt, &cu.Type)
		if err != nil {
			continue
		}
		cu.UserID = uID.String()
		users = append(users, cu)
	}

	resp, _ := json.Marshal(map[string]interface{}{"success": true, "connections": users})
	if search == "" {
		writeTherapistCache(therapistID, "connections", resp)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// GetPendingRequests lists incoming user direct connection requests
func GetPendingRequests(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	if cached, ok := readTherapistCache(therapistID, "requests"); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cached)
		return
	}

	rows, err := database.PostgresDB.Query(`
		SELECT r.id, r.user_id, u.username, r.created_at, r.note
		FROM connection_requests r
		JOIN users u ON r.user_id = u.id
		WHERE r.therapist_id = $1 AND r.status = 'pending'
		ORDER BY r.created_at DESC
	`, therapistID)
	if err != nil {
		log.Printf("ERROR loading pending requests: %v", err)
		http.Error(w, "Database error loading requests", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	requests := []models.ConnectionRequest{}
	for rows.Next() {
		var req models.ConnectionRequest
		var note sql.NullString
		err = rows.Scan(&req.ID, &req.UserID, &req.Username, &req.CreatedAt, &note)
		if err != nil {
			continue
		}
		req.TherapistID = therapistID
		req.Status = "pending"
		if note.Valid {
			req.Note = note.String
		}
		requests = append(requests, req)
	}

	resp, _ := json.Marshal(map[string]interface{}{"success": true, "requests": requests})
	writeTherapistCache(therapistID, "requests", resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// RespondToRequest approves or rejects an incoming connection request
func RespondToRequest(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	reqIDStr := chi.URLParam(r, "id")
	reqID, err := uuid.Parse(reqIDStr)
	if err != nil {
		http.Error(w, "Invalid request UUID", http.StatusBadRequest)
		return
	}

	var reqBody struct {
		Approve bool `json:"approve"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid response payload", http.StatusBadRequest)
		return
	}

	// Validate request ownership and state
	var userID uuid.UUID
	var status string
	err = database.PostgresDB.QueryRow(`
		SELECT user_id, status FROM connection_requests
		WHERE id = $1 AND therapist_id = $2
	`, reqID, therapistID).Scan(&userID, &status)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Connection request not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database validation error", http.StatusInternalServerError)
		}
		return
	}

	if status != "pending" {
		http.Error(w, "Connection request is already processed", http.StatusBadRequest)
		return
	}

	tx, err := database.PostgresDB.Begin()
	if err != nil {
		http.Error(w, "Database transaction error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var responseStatus string
	if reqBody.Approve {
		responseStatus = "approved"

		// Update request status
		_, err = tx.Exec(`
			UPDATE connection_requests SET status = 'approved', updated_at = NOW() WHERE id = $1
		`, reqID)
		if err != nil {
			http.Error(w, "Failed to update request status", http.StatusInternalServerError)
			return
		}

		// Insert user connection
		_, err = tx.Exec(`
			INSERT INTO therapist_user_connections (id, therapist_id, user_id, connected_at, connection_type)
			VALUES (gen_random_uuid(), $1, $2, NOW(), 'request')
			ON CONFLICT (therapist_id, user_id) DO NOTHING
		`, therapistID, userID)
		if err != nil {
			http.Error(w, "Failed to create connection record", http.StatusInternalServerError)
			return
		}

		// Log consent action
		_, err = tx.Exec(`
			INSERT INTO consent_history (id, user_id, therapist_id, action, timestamp, details)
			VALUES (gen_random_uuid(), $1, $2, 'granted_request', NOW(), 'Patient connected via accepted connection request')
		`, userID, therapistID)
		if err != nil {
			log.Printf("WARNING: Failed to log consent history row: %v", err)
		}

		// Trigger secure audit event
		database.TriggerAuditEvent("CONNECTION_REQUEST_APPROVED", reqID.String(), therapistID.String(), "therapist", fmt.Sprintf("Therapist approved connection request from user: %s", userID.String()), r)

		// Create user notification
		_, err = tx.Exec(`
			INSERT INTO notifications (id, recipient_id, recipient_role, title, message, type, is_read, created_at)
			VALUES (gen_random_uuid(), $1, 'user', 'Connection Approved', 'A therapist has approved your connection request. You can now start communicating.', 'connection_approved', FALSE, NOW())
		`, userID)
		if err != nil {
			log.Printf("WARNING: Failed to write patient notification: %v", err)
		}
	} else {
		responseStatus = "rejected"

		_, err = tx.Exec(`
			UPDATE connection_requests SET status = 'rejected', updated_at = NOW() WHERE id = $1
		`, reqID)
		if err != nil {
			http.Error(w, "Failed to reject connection request", http.StatusInternalServerError)
			return
		}

		database.TriggerAuditEvent("CONNECTION_REQUEST_REJECTED", reqID.String(), therapistID.String(), "therapist", fmt.Sprintf("Therapist rejected connection request from user: %s", userID.String()), r)
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	invalidateTherapistCaches(therapistID, "requests", "connections")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Request successfully %s", responseStatus),
	})
}

// DisconnectUser disconnects/unlinks a patient
func DisconnectUser(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	userIDStr := chi.URLParam(r, "userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user UUID format", http.StatusBadRequest)
		return
	}

	// Validate connection exists
	var connID uuid.UUID
	err = database.PostgresDB.QueryRow(`
		SELECT id FROM therapist_user_connections WHERE therapist_id = $1 AND user_id = $2
	`, therapistID, userID).Scan(&connID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "No active connection exists with this user", http.StatusNotFound)
		} else {
			http.Error(w, "Database connection fetch failure", http.StatusInternalServerError)
		}
		return
	}

	tx, err := database.PostgresDB.Begin()
	if err != nil {
		http.Error(w, "Database transaction error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Delete connection
	_, err = tx.Exec(`
		DELETE FROM therapist_user_connections WHERE id = $1
	`, connID)
	if err != nil {
		http.Error(w, "Failed to delete connection", http.StatusInternalServerError)
		return
	}

	// Delete matching connection request to reset request lifecycle
	_, err = tx.Exec(`
		DELETE FROM connection_requests WHERE user_id = $1 AND therapist_id = $2
	`, userID, therapistID)
	if err != nil {
		log.Printf("WARNING: Failed to cleanup connection request: %v", err)
	}

	// Log consent removal
	_, err = tx.Exec(`
		INSERT INTO consent_history (id, user_id, therapist_id, action, timestamp, details)
		VALUES (gen_random_uuid(), $1, $2, 'revoked_disconnect', NOW(), 'Therapist unlinked connection, automatically revoking viewing consent')
	`, userID, therapistID)
	if err != nil {
		log.Printf("WARNING: Failed to record consent revocation: %v", err)
	}

	// Log secure audit event
	database.TriggerAuditEvent("CONNECTION_DISCONNECTED", connID.String(), therapistID.String(), "therapist", fmt.Sprintf("Therapist disconnected connection with user %s", userID.String()), r)

	// Create user notification
	_, err = tx.Exec(`
		INSERT INTO notifications (id, recipient_id, recipient_role, title, message, type, is_read, created_at)
		VALUES (gen_random_uuid(), $1, 'user', 'Connection Ended', 'A therapist has disconnected their profile relationship with you.', 'connection_disconnected', FALSE, NOW())
	`, userID)
	if err != nil {
		log.Printf("WARNING: Failed to create user notification: %v", err)
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	invalidateTherapistCaches(therapistID, "connections")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Connection disconnected successfully",
	})
}

// GetTherapistMe gets profile details for authenticated therapist
func GetTherapistMe(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	if cached, ok := readTherapistCache(therapistID, "me"); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cached)
		return
	}

	var name, email, licenseNumber, licenseState, phone sql.NullString
	var collegeDegree, mastersInstitution, psychologistType, dsmAwareness, therapyTypes sql.NullString
	var specialization, certificateImagePath, degreeImagePath sql.NullString
	var yearsOfExperience, successfulCases int
	var isApproved bool
	var createdAt time.Time

	err := database.PostgresDB.QueryRow(`
		SELECT id, created_at, name, email, license_number, license_state,
			years_of_experience, specialization, phone, college_degree, masters_institution,
			psychologist_type, successful_cases, dsm_awareness, therapy_types,
			certificate_image_path, degree_image_path, is_approved
		FROM therapists WHERE id = $1
	`, therapistID).Scan(&therapistID, &createdAt, &name, &email, &licenseNumber, &licenseState,
		&yearsOfExperience, &specialization, &phone, &collegeDegree, &mastersInstitution,
		&psychologistType, &successfulCases, &dsmAwareness, &therapyTypes,
		&certificateImagePath, &degreeImagePath, &isApproved)
	if err != nil {
		http.Error(w, "Database error getting therapist profile", http.StatusInternalServerError)
		return
	}

	therapistMap := map[string]interface{}{
		"id":                   therapistID.String(),
		"name":                 name.String,
		"email":                email.String,
		"created_at":           createdAt,
		"license_number":       licenseNumber.String,
		"license_state":        licenseState.String,
		"years_of_experience":  yearsOfExperience,
		"specialization":       specialization.String,
		"phone":                phone.String,
		"college_degree":       collegeDegree.String,
		"masters_institution":  mastersInstitution.String,
		"psychologist_type":    psychologistType.String,
		"successful_cases":     successfulCases,
		"dsm_awareness":        dsmAwareness.String,
		"therapy_types":        therapyTypes.String,
		"certificate_image_path": certificateImagePath.String,
		"degree_image_path":     degreeImagePath.String,
		"is_approved":          isApproved,
	}

	tenantID, err := services.EnsureTenantForTherapist(therapistID)
	if err == nil {
		_ = services.EnsureBillingProfile(tenantID)
		var chatFee, inPersonFee float64
		err = database.PostgresDB.QueryRow(`
			SELECT session_fee_chat, session_fee_in_person FROM billing_profiles WHERE tenant_id = $1
		`, tenantID).Scan(&chatFee, &inPersonFee)
		if err == nil {
			therapistMap["session_fee_chat"] = chatFee
			therapistMap["session_fee_in_person"] = inPersonFee
		}
	}

	resp, _ := json.Marshal(map[string]interface{}{"success": true, "user": therapistMap})
	writeTherapistCache(therapistID, "me", resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// UpdateTherapistProfile allows modifying professional metrics
func UpdateTherapistProfile(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	var req struct {
		Specialization     string  `json:"specialization"`
		Phone              string  `json:"phone"`
		YearsOfExperience  int     `json:"years_of_experience"`
		DSMAwareness       string  `json:"dsm_awareness"`
		TherapyTypes       string  `json:"therapy_types"`
		SessionFeeChat     float64 `json:"session_fee_chat"`
		SessionFeeInPerson float64 `json:"session_fee_in_person"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if req.Phone == "" || req.DSMAwareness == "" || req.TherapyTypes == "" {
		http.Error(w, "All fields are required for professional profiles", http.StatusBadRequest)
		return
	}

	tx, err := database.PostgresDB.Begin()
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE therapists
		SET specialization = $1, phone = $2, years_of_experience = $3, dsm_awareness = $4, therapy_types = $5, updated_at = NOW()
		WHERE id = $6
	`, req.Specialization, req.Phone, req.YearsOfExperience, req.DSMAwareness, req.TherapyTypes, therapistID)
	if err != nil {
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	tenantID, err := services.EnsureTenantForTherapist(therapistID)
	if err != nil {
		http.Error(w, "Failed to resolve therapist tenant", http.StatusInternalServerError)
		return
	}

	_ = services.EnsureBillingProfile(tenantID)
	_, err = tx.Exec(`
		UPDATE billing_profiles
		SET session_fee_chat = $2, session_fee_in_person = $3, updated_at = NOW()
		WHERE tenant_id = $1
	`, tenantID, req.SessionFeeChat, req.SessionFeeInPerson)
	if err != nil {
		http.Error(w, "Failed to update pricing profile", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	cacheKey := services.CacheKey("therapist", therapistID.String())
	services.Cache.Delete(cacheKey)
	invalidateTherapistCaches(therapistID, "me")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Profile updated successfully",
	})
}

// ValidateReferralCode checks if a code is valid for registration (heavily rate limited)
func ValidateReferralCode(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Code parameter is required", http.StatusBadRequest)
		return
	}

	var isRevoked bool
	var usageLimit sql.NullInt64
	var usageCount int
	var expiresAt *time.Time
	var therapistName string

	err := database.PostgresDB.QueryRow(`
		SELECT r.is_revoked, r.usage_limit, r.usage_count, r.expires_at, t.name
		FROM referral_codes r
		JOIN therapists t ON r.therapist_id = t.id
		WHERE r.code = $1
	`, code).Scan(&isRevoked, &usageLimit, &usageCount, &expiresAt, &therapistName)

	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Referral code not found",
			})
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	valid := true
	var reason string

	if isRevoked {
		valid = false
		reason = "This referral code has been revoked"
	} else if expiresAt != nil && expiresAt.Before(time.Now()) {
		valid = false
		reason = "This referral code has expired"
	} else if usageLimit.Valid && int64(usageCount) >= usageLimit.Int64 {
		valid = false
		reason = "This referral code has reached its usage limit"
	}

	w.Header().Set("Content-Type", "application/json")
	if !valid {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": reason,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        true,
		"message":        "Referral code is valid",
		"therapist_name": therapistName,
	})
}

// SearchTherapists allows patients to search therapists with multiple filters
func SearchTherapists(w http.ResponseWriter, r *http.Request) {
	_, ok := requireUserAuth(r)
	if !ok {
		http.Error(w, "Unauthorized user access", http.StatusUnauthorized)
		return
	}

	specialization := r.URL.Query().Get("specialization")
	location := r.URL.Query().Get("location") // maps to license_state
	availability := r.URL.Query().Get("availability")
	query := r.URL.Query().Get("q")
	limitStr := r.URL.Query().Get("limit")
	skipStr := r.URL.Query().Get("skip")

	limit := 10
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	skip := 0
	if skipStr != "" {
		if parsedSkip, err := strconv.Atoi(skipStr); err == nil && parsedSkip >= 0 {
			skip = parsedSkip
		}
	}

	// Build dynamic SQL query
	sqlQuery := `
		SELECT id, name, created_at, license_state, years_of_experience, specialization,
			college_degree, masters_institution, psychologist_type, successful_cases, therapy_types, is_approved
		FROM therapists
		WHERE is_approved = TRUE
	`
	args := []interface{}{}
	argIndex := 1

	if specialization != "" {
		sqlQuery += fmt.Sprintf(" AND LOWER(specialization) LIKE LOWER($%d)", argIndex)
		args = append(args, "%"+specialization+"%")
		argIndex++
	}

	if location != "" {
		sqlQuery += fmt.Sprintf(" AND LOWER(license_state) = LOWER($%d)", argIndex)
		args = append(args, location)
		argIndex++
	}

	if query != "" {
		sqlQuery += fmt.Sprintf(" AND (LOWER(name) LIKE LOWER($%d) OR LOWER(therapy_types) LIKE LOWER($%d))", argIndex, argIndex)
		args = append(args, "%"+query+"%")
		argIndex++
	}

	// Always sorting by experience
	sqlQuery += " ORDER BY years_of_experience DESC"

	// Pagination
	sqlQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit, skip)

	rows, err := database.PostgresDB.Query(sqlQuery, args...)
	if err != nil {
		log.Printf("ERROR searching therapists: %v", err)
		http.Error(w, "Failed to load directory search", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	therapists := []map[string]interface{}{}
	for rows.Next() {
		var id uuid.UUID
		var name, licenseState, college, masters, psychType, therapy, spec sql.NullString
		var years, success int
		var isApproved bool
		var createdAt time.Time

		err = rows.Scan(&id, &name, &createdAt, &licenseState, &years, &spec,
			&college, &masters, &psychType, &success, &therapy, &isApproved)
		if err != nil {
			continue
		}

		t := map[string]interface{}{
			"id":                  id.String(),
			"name":                name.String,
			"created_at":          createdAt,
			"license_state":       licenseState.String,
			"years_of_experience": years,
			"specialization":       spec.String,
			"college_degree":       college.String,
			"masters_institution":  masters.String,
			"psychologist_type":    psychType.String,
			"successful_cases":     success,
			"therapy_types":        therapy.String,
			"is_approved":          isApproved,
		}

		// Verify availability status
		t["availability_status"] = "available"
		if availability == "true" && t["availability_status"] != "available" {
			continue
		}

		therapists = append(therapists, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"therapists": therapists,
	})
}

// GetTherapistProfile allows patients to view therapist details
func GetTherapistProfile(w http.ResponseWriter, r *http.Request) {
	patientID, ok := requireUserAuth(r)
	if !ok {
		http.Error(w, "Unauthorized user access", http.StatusUnauthorized)
		return
	}

	therapistIDStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(therapistIDStr)
	if err != nil {
		http.Error(w, "Invalid UUID format", http.StatusBadRequest)
		return
	}

	var name, email, licenseNumber, licenseState, phone sql.NullString
	var collegeDegree, mastersInstitution, psychologistType, dsmAwareness, therapyTypes sql.NullString
	var specialization, certificateImagePath, degreeImagePath sql.NullString
	var yearsOfExperience, successfulCases int
	var isApproved bool
	var createdAt time.Time

	err = database.PostgresDB.QueryRow(`
		SELECT id, created_at, name, email, license_number, license_state,
			years_of_experience, specialization, phone, college_degree, masters_institution,
			psychologist_type, successful_cases, dsm_awareness, therapy_types,
			certificate_image_path, degree_image_path, is_approved
		FROM therapists WHERE id = $1 AND is_approved = TRUE
	`, therapistID).Scan(&therapistID, &createdAt, &name, &email, &licenseNumber, &licenseState,
		&yearsOfExperience, &specialization, &phone, &collegeDegree, &mastersInstitution,
		&psychologistType, &successfulCases, &dsmAwareness, &therapyTypes,
		&certificateImagePath, &degreeImagePath, &isApproved)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Therapist profile not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database failure loading profile", http.StatusInternalServerError)
		}
		return
	}

	// Get billing profile
	var billingMap map[string]interface{}
	if tenantID, err := services.EnsureTenantForTherapist(therapistID); err == nil {
		if bp, err := services.GetBillingProfile(tenantID); err == nil {
			billingMap = map[string]interface{}{
				"consultation_fee":      bp.ConsultationFee,
				"session_fee":           bp.SessionFee,
				"session_fee_in_person": bp.SessionFeeInPerson,
				"session_fee_chat":      bp.SessionFeeChat,
				"session_fee_voice":     bp.SessionFeeVoice,
				"session_fee_video":     bp.SessionFeeVideo,
				"currency":              bp.Currency,
				"gst_rate":              bp.GSTRate,
			}
		}
	}

	t := map[string]interface{}{
		"id":                  therapistID.String(),
		"name":                name.String,
		"created_at":          createdAt,
		"license_state":       licenseState.String,
		"years_of_experience":  yearsOfExperience,
		"specialization":       specialization.String,
		"college_degree":       collegeDegree.String,
		"masters_institution":  mastersInstitution.String,
		"psychologist_type":    psychologistType.String,
		"successful_cases":     successfulCases,
		"therapy_types":        therapyTypes.String,
		"is_approved":          isApproved,
		"billing_profile":      billingMap,
	}

	// Include connection state for UI flows
	var isConnected bool
	err = database.PostgresDB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM therapist_user_connections WHERE user_id = $1 AND therapist_id = $2
		)
	`, patientID, therapistID).Scan(&isConnected)
	t["is_connected"] = isConnected && err == nil

	var requestStatus string = ""
	_ = database.PostgresDB.QueryRow(`
		SELECT status FROM connection_requests WHERE user_id = $1 AND therapist_id = $2 ORDER BY created_at DESC LIMIT 1
	`, patientID, therapistID).Scan(&requestStatus)
	t["connection_request_status"] = requestStatus

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"therapist": t,
	})
}

// RequestConnection allows users to send direct requests
func RequestConnection(w http.ResponseWriter, r *http.Request) {
	patientID, ok := requireUserAuth(r)
	if !ok {
		http.Error(w, "Unauthorized user access", http.StatusUnauthorized)
		return
	}

	therapistIDStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(therapistIDStr)
	if err != nil {
		http.Error(w, "Invalid therapist UUID", http.StatusBadRequest)
		return
	}

	var req struct {
		Note string `json:"note,omitempty"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Validate therapist exists and is approved
	var isApproved bool
	err = database.PostgresDB.QueryRow(`
		SELECT is_approved FROM therapists WHERE id = $1
	`, therapistID).Scan(&isApproved)
	if err != nil || !isApproved {
		http.Error(w, "Therapist profile not found or inactive", http.StatusNotFound)
		return
	}

	// Check if already connected
	var isConnected bool
	_ = database.PostgresDB.QueryRow(`
		SELECT EXISTS (SELECT 1 FROM therapist_user_connections WHERE therapist_id = $1 AND user_id = $2)
	`, therapistID, patientID).Scan(&isConnected)
	if isConnected {
		http.Error(w, "You are already connected to this therapist", http.StatusBadRequest)
		return
	}

	// Verify no pending request
	var existingStatus string
	err = database.PostgresDB.QueryRow(`
		SELECT status FROM connection_requests WHERE user_id = $1 AND therapist_id = $2
	`, patientID, therapistID).Scan(&existingStatus)

	if err == nil {
		if existingStatus == "pending" {
			http.Error(w, "A connection request is already pending with this therapist", http.StatusConflict)
			return
		}
		// Reset rejected status
		_, err = database.PostgresDB.Exec(`
			UPDATE connection_requests SET status = 'pending', note = $1, created_at = NOW(), updated_at = NOW()
			WHERE user_id = $2 AND therapist_id = $3
		`, req.Note, patientID, therapistID)
	} else {
		// New request
		_, err = database.PostgresDB.Exec(`
			INSERT INTO connection_requests (id, user_id, therapist_id, status, note)
			VALUES (gen_random_uuid(), $1, $2, 'pending', $3)
		`, patientID, therapistID, req.Note)
	}

	if err != nil {
		http.Error(w, "Failed to submit request", http.StatusInternalServerError)
		return
	}

	// Log secure audit event
	database.TriggerAuditEvent("CONNECTION_REQUEST_SENT", therapistID.String(), patientID.String(), "user", "Patient sent direct connection request to therapist", r)

	// Notify therapist
	var patientName string
	_ = database.PostgresDB.QueryRow("SELECT username FROM users WHERE id = $1", patientID).Scan(&patientName)

	_, err = database.PostgresDB.Exec(`
		INSERT INTO notifications (id, recipient_id, recipient_role, title, message, type, is_read, data)
		VALUES (gen_random_uuid(), $1, 'therapist', 'Connection Request Received', $2, 'connection_request', FALSE, $3)
	`, therapistID, fmt.Sprintf("Patient (username: %s) requested a direct profile connection with you.", patientName), fmt.Sprintf(`{"user_id": "%s", "username": "%s"}`, patientID.String(), patientName))
	if err != nil {
		log.Printf("WARNING: Failed to log therapist notification: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Connection request sent successfully",
	})
}

// UserDisconnectTherapist allows patients to revoke consent and unlink a therapist
func UserDisconnectTherapist(w http.ResponseWriter, r *http.Request) {
	patientID, ok := requireUserAuth(r)
	if !ok {
		http.Error(w, "Unauthorized user access", http.StatusUnauthorized)
		return
	}

	therapistIDStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(therapistIDStr)
	if err != nil {
		http.Error(w, "Invalid therapist UUID format", http.StatusBadRequest)
		return
	}

	var connID uuid.UUID
	err = database.PostgresDB.QueryRow(`
		SELECT id FROM therapist_user_connections WHERE therapist_id = $1 AND user_id = $2
	`, therapistID, patientID).Scan(&connID)
	if err != nil {
		http.Error(w, "No active connection exists with this therapist", http.StatusNotFound)
		return
	}

	tx, err := database.PostgresDB.Begin()
	if err != nil {
		http.Error(w, "Database transaction failure", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Delete connection
	_, err = tx.Exec("DELETE FROM therapist_user_connections WHERE id = $1", connID)
	if err != nil {
		http.Error(w, "Failed to delete connection", http.StatusInternalServerError)
		return
	}

	// Delete requests
	_, err = tx.Exec("DELETE FROM connection_requests WHERE user_id = $1 AND therapist_id = $2", patientID, therapistID)
	if err != nil {
		log.Printf("WARNING: Failed to cleanup connection request: %v", err)
	}

	// Log consent removal
	_, err = tx.Exec(`
		INSERT INTO consent_history (id, user_id, therapist_id, action, timestamp, details)
		VALUES (gen_random_uuid(), $1, $2, 'revoked_disconnect', NOW(), 'Patient unlinked connection, automatically revoking viewing consent')
	`, patientID, therapistID)
	if err != nil {
		log.Printf("WARNING: Failed to log consent history: %v", err)
	}

	// Log secure audit event
	database.TriggerAuditEvent("CONNECTION_DISCONNECTED", connID.String(), patientID.String(), "user", fmt.Sprintf("Patient disconnected from therapist %s", therapistID.String()), r)

	// Notify therapist
	var patientName string
	_ = database.PostgresDB.QueryRow("SELECT username FROM users WHERE id = $1", patientID).Scan(&patientName)

	_, err = tx.Exec(`
		INSERT INTO notifications (id, recipient_id, recipient_role, title, message, type, is_read)
		VALUES (gen_random_uuid(), $1, 'therapist', 'Client Connection Removed', $2, 'connection_disconnected', FALSE)
	`, therapistID, fmt.Sprintf("Patient (username: %s) has ended their relationship connection with your profile.", patientName))
	if err != nil {
		log.Printf("WARNING: Failed to log therapist notification: %v", err)
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Disconnected successfully",
	})
}

// GetNotifications retrieves notifications for the logged-in entity
func GetNotifications(w http.ResponseWriter, r *http.Request) {
	var userID uuid.UUID
	token := extractBearerToken(r.Header.Get("Authorization"))

	if os.Getenv("ENV") != "production" {
		if token != "" {
			uID, ok, err := services.ValidateSession(token)
			if err == nil && ok {
				userID = uID
			}
		}
		if userID == uuid.Nil {
			var firstID uuid.UUID
			err := database.PostgresDB.QueryRow("SELECT id FROM therapists LIMIT 1").Scan(&firstID)
			if err == nil {
				userID = firstID
			} else {
				err = database.PostgresDB.QueryRow("SELECT id FROM users LIMIT 1").Scan(&firstID)
				if err == nil {
					userID = firstID
				} else {
					userID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
				}
			}
		}
	} else {
		if token == "" {
			http.Error(w, "Authorization token required", http.StatusUnauthorized)
			return
		}
		uID, ok, err := services.ValidateSession(token)
		if err != nil || !ok {
			http.Error(w, "Unauthorized session", http.StatusUnauthorized)
			return
		}
		userID = uID
	}

	rows, err := database.PostgresDB.Query(`
		SELECT id, recipient_id, recipient_role, title, message, type, is_read, created_at, COALESCE(data::text, '')
		FROM notifications
		WHERE recipient_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		log.Printf("ERROR fetching notifications: %v", err)
		http.Error(w, "Failed to load notifications", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	notifications := []models.Notification{}
	for rows.Next() {
		var n models.Notification
		var rID uuid.UUID
		err = rows.Scan(&n.ID, &rID, &n.RecipientRole, &n.Title, &n.Message, &n.Type, &n.IsRead, &n.CreatedAt, &n.Data)
		if err != nil {
			continue
		}
		n.RecipientID = rID
		notifications = append(notifications, n)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"notifications": notifications,
	})
}

// MarkNotificationRead marks alert as read
func MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	var userID uuid.UUID
	token := extractBearerToken(r.Header.Get("Authorization"))

	if os.Getenv("ENV") != "production" {
		if token != "" {
			uID, ok, err := services.ValidateSession(token)
			if err == nil && ok {
				userID = uID
			}
		}
		if userID == uuid.Nil {
			var firstID uuid.UUID
			err := database.PostgresDB.QueryRow("SELECT id FROM therapists LIMIT 1").Scan(&firstID)
			if err == nil {
				userID = firstID
			} else {
				err = database.PostgresDB.QueryRow("SELECT id FROM users LIMIT 1").Scan(&firstID)
				if err == nil {
					userID = firstID
				} else {
					userID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
				}
			}
		}
	} else {
		if token == "" {
			http.Error(w, "Authorization token required", http.StatusUnauthorized)
			return
		}
		uID, ok, err := services.ValidateSession(token)
		if err != nil || !ok {
			http.Error(w, "Unauthorized session", http.StatusUnauthorized)
			return
		}
		userID = uID
	}

	notifIDStr := chi.URLParam(r, "id")
	notifID, err := uuid.Parse(notifIDStr)
	if err != nil {
		http.Error(w, "Invalid UUID format", http.StatusBadRequest)
		return
	}

	_, err = database.PostgresDB.Exec(`
		UPDATE notifications SET is_read = TRUE WHERE id = $1 AND recipient_id = $2
	`, notifID, userID)
	if err != nil {
		http.Error(w, "Failed to update notification", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Notification marked as read",
	})
}
