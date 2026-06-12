package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/AnshRaj112/serenify-backend/pkg/clientip"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

// GetPendingTherapists returns all therapists with is_approved = false
func GetPendingTherapists(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	rows, err := database.PostgresDB.Query(`
		SELECT id, created_at, name, email, license_number, license_state,
			years_of_experience, specialization, phone, college_degree, masters_institution,
			psychologist_type, successful_cases, dsm_awareness, therapy_types,
			certificate_image_path, degree_image_path, is_approved
		FROM therapists
		WHERE is_approved = false
		ORDER BY created_at DESC
	`)
	if err != nil {
		http.Error(w, "Failed to fetch therapists: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	therapistList := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, name, email, licenseNumber, licenseState, phone sql.NullString
		var collegeDegree, mastersInstitution, psychologistType, dsmAwareness, therapyTypes sql.NullString
		var specialization, certificateImagePath, degreeImagePath sql.NullString
		var yearsOfExperience, successfulCases int
		var isApproved bool
		var createdAt time.Time

		if err := rows.Scan(&id, &createdAt, &name, &email, &licenseNumber, &licenseState,
			&yearsOfExperience, &specialization, &phone, &collegeDegree, &mastersInstitution,
			&psychologistType, &successfulCases, &dsmAwareness, &therapyTypes,
			&certificateImagePath, &degreeImagePath, &isApproved); err != nil {
			http.Error(w, "Failed to scan therapists: "+err.Error(), http.StatusInternalServerError)
			return
		}

		therapistList = append(therapistList, map[string]interface{}{
			"id":                     id.String,
			"name":                   name.String,
			"email":                  email.String,
			"created_at":             createdAt,
			"license_number":         licenseNumber.String,
			"license_state":          licenseState.String,
			"years_of_experience":    yearsOfExperience,
			"specialization":         specialization.String,
			"phone":                  phone.String,
			"college_degree":         collegeDegree.String,
			"masters_institution":    mastersInstitution.String,
			"psychologist_type":      psychologistType.String,
			"successful_cases":       successfulCases,
			"dsm_awareness":          dsmAwareness.String,
			"therapy_types":          therapyTypes.String,
			"certificate_image_path": certificateImagePath.String,
			"degree_image_path":      degreeImagePath.String,
			"is_approved":            isApproved,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"therapists": therapistList,
		"count":      len(therapistList),
	})
}

// ApproveTherapist approves a therapist by ID
func ApproveTherapist(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	therapistID := r.URL.Query().Get("id")
	if therapistID == "" {
		http.Error(w, "Therapist ID is required", http.StatusBadRequest)
		return
	}

	// Validate UUID format
	if _, err := uuid.Parse(therapistID); err != nil {
		http.Error(w, "Invalid therapist ID", http.StatusBadRequest)
		return
	}

	// Update therapist to approved
	result, err := database.PostgresDB.Exec(`
		UPDATE therapists
		SET is_approved = true, updated_at = NOW()
		WHERE id = $1 AND is_approved = false
	`, therapistID)
	if err != nil {
		http.Error(w, "Failed to approve therapist: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to approve therapist: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Therapist not found or already approved", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Therapist approved successfully",
	})
}

// RejectTherapist rejects a therapist by ID (deletes the application)
func RejectTherapist(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	therapistID := r.URL.Query().Get("id")
	if therapistID == "" {
		http.Error(w, "Therapist ID is required", http.StatusBadRequest)
		return
	}

	// Validate UUID format
	if _, err := uuid.Parse(therapistID); err != nil {
		http.Error(w, "Invalid therapist ID", http.StatusBadRequest)
		return
	}

	// Delete therapist application
	result, err := database.PostgresDB.Exec(`
		DELETE FROM therapists
		WHERE id = $1 AND is_approved = false
	`, therapistID)
	if err != nil {
		http.Error(w, "Failed to reject therapist: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to reject therapist: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Therapist not found or already approved", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Therapist application rejected and removed",
	})
}

// DeleteTherapist deletes any therapist account and all related tenant/onboarding data.
func DeleteTherapist(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	therapistID, err := parseRequiredUUIDQuery(r, "id")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	tx, err := database.PostgresDB.BeginTx(r.Context(), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Database error"})
		return
	}
	defer tx.Rollback()

	var tenantIDs []uuid.UUID
	rows, err := tx.QueryContext(r.Context(), `SELECT id FROM tenants WHERE therapist_id = $1`, therapistID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to load therapist tenants"})
		return
	}
	for rows.Next() {
		var tenantID uuid.UUID
		if err := rows.Scan(&tenantID); err == nil {
			tenantIDs = append(tenantIDs, tenantID)
		}
	}
	rows.Close()

	result, err := tx.ExecContext(r.Context(), `DELETE FROM therapists WHERE id = $1`, therapistID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to delete therapist: " + err.Error()})
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Therapist not found"})
		return
	}

	_, _ = tx.ExecContext(r.Context(), `DELETE FROM notifications WHERE recipient_id = $1 OR (data->>'user_id') = $1`, therapistID.String())

	if err := tx.Commit(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Database error"})
		return
	}

	deleteTherapistMongoData(r.Context(), therapistID, tenantIDs)
	_ = services.InvalidateUserSessions(therapistID)
	services.Cache.Delete(services.CacheKey("therapists", "approved"))
	services.Cache.Delete(services.CacheKey("therapists", "pending"))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Therapist and related data deleted successfully",
	})
}

// GetApprovedTherapists returns all approved therapists
func GetApprovedTherapists(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	// Try to get from cache
	cacheKey := services.CacheKey("therapists", "approved")
	var cachedResponse map[string]interface{}
	if found, err := services.Cache.Get(cacheKey, &cachedResponse); err == nil && found {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		json.NewEncoder(w).Encode(cachedResponse)
		return
	}

	rows, err := database.PostgresDB.Query(`
		SELECT id, created_at, name, email, license_number, license_state,
			years_of_experience, specialization, phone, college_degree, masters_institution,
			psychologist_type, successful_cases, dsm_awareness, therapy_types,
			certificate_image_path, degree_image_path, is_approved
		FROM therapists
		WHERE is_approved = true
		ORDER BY created_at DESC
	`)
	if err != nil {
		http.Error(w, "Failed to fetch therapists: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	therapistList := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, name, email, licenseNumber, licenseState, phone sql.NullString
		var collegeDegree, mastersInstitution, psychologistType, dsmAwareness, therapyTypes sql.NullString
		var specialization, certificateImagePath, degreeImagePath sql.NullString
		var yearsOfExperience, successfulCases int
		var isApproved bool
		var createdAt time.Time

		if err := rows.Scan(&id, &createdAt, &name, &email, &licenseNumber, &licenseState,
			&yearsOfExperience, &specialization, &phone, &collegeDegree, &mastersInstitution,
			&psychologistType, &successfulCases, &dsmAwareness, &therapyTypes,
			&certificateImagePath, &degreeImagePath, &isApproved); err != nil {
			http.Error(w, "Failed to scan therapists: "+err.Error(), http.StatusInternalServerError)
			return
		}

		therapistList = append(therapistList, map[string]interface{}{
			"id":                     id.String,
			"name":                   name.String,
			"email":                  email.String,
			"created_at":             createdAt,
			"license_number":         licenseNumber.String,
			"license_state":          licenseState.String,
			"years_of_experience":    yearsOfExperience,
			"specialization":         specialization.String,
			"phone":                  phone.String,
			"college_degree":         collegeDegree.String,
			"masters_institution":    mastersInstitution.String,
			"psychologist_type":      psychologistType.String,
			"successful_cases":       successfulCases,
			"dsm_awareness":          dsmAwareness.String,
			"therapy_types":          therapyTypes.String,
			"certificate_image_path": certificateImagePath.String,
			"degree_image_path":      degreeImagePath.String,
			"is_approved":            isApproved,
		})
	}

	response := map[string]interface{}{
		"success":    true,
		"therapists": therapistList,
		"count":      len(therapistList),
	}

	// Cache the response
	services.Cache.Set(cacheKey, response)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(response)
}

// GetViolations returns all content violations
func GetViolations(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	rows, err := database.PostgresDB.Query(`
		SELECT id, created_at, user_id, ip_address, type, message, vent_id, action_taken
		FROM violations
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		http.Error(w, "Failed to fetch violations: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	violationList := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, ipAddress, violationType, message, ventID, actionTaken string
		var createdAt time.Time
		var userID sql.NullString

		if err := rows.Scan(&id, &createdAt, &userID, &ipAddress, &violationType, &message, &ventID, &actionTaken); err != nil {
			http.Error(w, "Failed to scan violations: "+err.Error(), http.StatusInternalServerError)
			return
		}

		violationMap := map[string]interface{}{
			"id":           id,
			"created_at":   createdAt,
			"ip_address":   ipAddress,
			"type":         violationType,
			"message":      message,
			"vent_id":      ventID,
			"action_taken": actionTaken,
		}
		if userID.Valid {
			violationMap["user_id"] = userID.String
		}
		violationList = append(violationList, violationMap)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"violations": violationList,
		"count":      len(violationList),
	})
}

// GetBlockedIPs returns all currently blocked IP addresses
func GetBlockedIPs(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	rows, err := database.PostgresDB.Query(`
		SELECT id, created_at, expires_at, ip_address, reason, is_active
		FROM blocked_ips
		WHERE is_active = true AND expires_at > $1
		ORDER BY created_at DESC
	`, time.Now())
	if err != nil {
		http.Error(w, "Failed to fetch blocked IPs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	blockedList := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, ipAddress, reason string
		var createdAt, expiresAt time.Time
		var isActive bool

		if err := rows.Scan(&id, &createdAt, &expiresAt, &ipAddress, &reason, &isActive); err != nil {
			http.Error(w, "Failed to scan blocked IPs: "+err.Error(), http.StatusInternalServerError)
			return
		}

		blockedList = append(blockedList, map[string]interface{}{
			"id":         id,
			"ip_address": ipAddress,
			"reason":     reason,
			"created_at": createdAt,
			"expires_at": expiresAt,
			"is_active":  isActive,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"blocked_ips": blockedList,
		"count":       len(blockedList),
	})
}

// UnblockIP unblocks an IP address
func UnblockIP(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	ipAddress := r.URL.Query().Get("ip")
	if ipAddress == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "IP address is required",
		})
		return
	}

	// Check if IP is actually blocked before unblocking
	isBlocked, blockedIP, err := services.IsIPBlocked(ipAddress)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to check block status: " + err.Error(),
		})
		return
	}

	if !isBlocked {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "IP address is not currently blocked",
		})
		return
	}

	// Unblock the IP
	err = services.UnblockIP(ipAddress)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to unblock IP: " + err.Error(),
		})
		return
	}

	// Return success with details
	response := map[string]interface{}{
		"success":    true,
		"message":    "IP address unblocked successfully",
		"ip_address": ipAddress,
	}

	if blockedIP != nil {
		response["previous_reason"] = blockedIP.Reason
		response["was_blocked_until"] = blockedIP.ExpiresAt
	}

	json.NewEncoder(w).Encode(response)
}

// GetUsers returns all users (admin only). Does not include password_hash.
func GetUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	rows, err := database.PostgresDB.Query(`
		SELECT id, username, created_at, is_active
		FROM users
		ORDER BY created_at DESC
	`)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to fetch users",
			"users":   []interface{}{},
		})
		return
	}
	defer rows.Close()

	userList := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, username string
		var createdAt time.Time
		var isActive bool
		if err := rows.Scan(&id, &username, &createdAt, &isActive); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to scan users",
				"users":   []interface{}{},
			})
			return
		}
		userList = append(userList, map[string]interface{}{
			"id":         id,
			"username":   username,
			"created_at": createdAt,
			"is_active":  isActive,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"users":   userList,
		"count":   len(userList),
	})
}

// DeleteUser deletes a user by ID (admin only), including therapist onboarding and V2 patient records.
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	userID, err := parseRequiredUUIDQuery(r, "id")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	tx, err := database.PostgresDB.BeginTx(r.Context(), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Database error"})
		return
	}
	defer tx.Rollback()

	patientIDs, tenantIDs, err := loadPatientRefsForUser(r.Context(), tx, userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to load patient references"})
		return
	}

	if _, err = tx.ExecContext(r.Context(), `DELETE FROM patients WHERE user_id = $1`, userID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to delete patient profiles: " + err.Error()})
		return
	}
	_, _ = tx.ExecContext(r.Context(), `
		UPDATE groups g
		SET member_count = GREATEST(g.member_count - memberships.count, 0)
		FROM (
			SELECT group_id, COUNT(*)::int AS count
			FROM group_members
			WHERE user_id = $1
			GROUP BY group_id
		) memberships
		WHERE g.id = memberships.group_id
	`, userID)
	_, _ = tx.ExecContext(r.Context(), `DELETE FROM notifications WHERE recipient_id = $1 OR (data->>'user_id') = $1`, userID.String())

	result, err := tx.ExecContext(r.Context(), `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to delete user: " + err.Error()})
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "User not found",
		})
		return
	}

	if err := tx.Commit(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Database error"})
		return
	}

	deleteUserMongoData(r.Context(), userID, patientIDs, tenantIDs)
	_ = services.InvalidateUserSessions(userID)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "User deleted successfully",
	})
}

func parseRequiredUUIDQuery(r *http.Request, key string) (uuid.UUID, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return uuid.Nil, &badRequestError{message: key + " is required"}
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, &badRequestError{message: "Invalid " + key}
	}
	return id, nil
}

type badRequestError struct {
	message string
}

func (e *badRequestError) Error() string {
	return e.message
}

type queryer interface {
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
}

func loadPatientRefsForUser(ctx context.Context, q queryer, userID uuid.UUID) ([]uuid.UUID, []uuid.UUID, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, tenant_id FROM patients WHERE user_id = $1`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	patientIDs := []uuid.UUID{}
	tenantIDs := []uuid.UUID{}
	seenTenants := map[uuid.UUID]bool{}
	for rows.Next() {
		var patientID, tenantID uuid.UUID
		if err := rows.Scan(&patientID, &tenantID); err != nil {
			return nil, nil, err
		}
		patientIDs = append(patientIDs, patientID)
		if !seenTenants[tenantID] {
			tenantIDs = append(tenantIDs, tenantID)
			seenTenants[tenantID] = true
		}
	}
	return patientIDs, tenantIDs, rows.Err()
}

func deleteUserMongoData(ctx context.Context, userID uuid.UUID, patientIDs, tenantIDs []uuid.UUID) {
	if database.DB == nil {
		return
	}
	userIDStr := userID.String()
	patientIDStrs := uuidStrings(patientIDs)
	tenantIDStrs := uuidStrings(tenantIDs)

	_, _ = database.DB.Collection("vents").DeleteMany(ctx, bson.M{"user_id": userIDStr})
	_, _ = database.DB.Collection("journals").DeleteMany(ctx, bson.M{"user_id": userIDStr})
	_, _ = database.DB.Collection("chat_messages").DeleteMany(ctx, bson.M{"sender_id": userIDStr})
	_, _ = database.DB.Collection("chat_messages").DeleteMany(ctx, bson.M{"user_id": userIDStr})

	if len(patientIDStrs) > 0 {
		filter := bson.M{"patient_id": bson.M{"$in": patientIDStrs}}
		_, _ = database.DB.Collection("wellness_entries").DeleteMany(ctx, filter)
		_, _ = database.DB.Collection("patient_journals").DeleteMany(ctx, filter)
		_, _ = database.DB.Collection("session_notes").DeleteMany(ctx, filter)
		_, _ = database.DB.Collection("ai_insights").DeleteMany(ctx, filter)
		_, _ = database.DB.Collection("dm_conversations").DeleteMany(ctx, filter)
		_, _ = database.DB.Collection("dm_messages").DeleteMany(ctx, filter)
	}
	if len(tenantIDStrs) > 0 {
		_, _ = database.DB.Collection("session_note_versions").DeleteMany(ctx, bson.M{"tenant_id": bson.M{"$in": tenantIDStrs}})
	}
}

func deleteTherapistMongoData(ctx context.Context, therapistID uuid.UUID, tenantIDs []uuid.UUID) {
	if database.DB == nil {
		return
	}
	therapistIDStr := therapistID.String()
	tenantIDStrs := uuidStrings(tenantIDs)

	_, _ = database.DB.Collection("dm_conversations").DeleteMany(ctx, bson.M{"therapist_id": therapistIDStr})
	_, _ = database.DB.Collection("dm_messages").DeleteMany(ctx, bson.M{"therapist_id": therapistIDStr})
	if len(tenantIDStrs) > 0 {
		filter := bson.M{"tenant_id": bson.M{"$in": tenantIDStrs}}
		_, _ = database.DB.Collection("wellness_entries").DeleteMany(ctx, filter)
		_, _ = database.DB.Collection("patient_journals").DeleteMany(ctx, filter)
		_, _ = database.DB.Collection("session_notes").DeleteMany(ctx, filter)
		_, _ = database.DB.Collection("session_note_versions").DeleteMany(ctx, filter)
		_, _ = database.DB.Collection("ai_insights").DeleteMany(ctx, filter)
	}
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

// GetAbuseReports returns all secure abuse reports (admin only).
func GetAbuseReports(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	rows, err := database.PostgresDB.QueryContext(r.Context(), `
		SELECT ar.id, ar.reported_by, ur.username, ar.group_id, gr.name, ar.status, ar.created_at, ar.encrypted_payload
		FROM abuse_reports ar
		LEFT JOIN users ur ON ar.reported_by = ur.id
		LEFT JOIN groups gr ON ar.group_id = gr.id
		ORDER BY ar.created_at DESC
	`)
	if err != nil {
		http.Error(w, "Failed to fetch abuse reports: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	reportList := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, reportedBy, groupID, status, encryptedPayload string
		var reporterName, groupName sql.NullString
		var createdAt time.Time

		if err := rows.Scan(&id, &reportedBy, &reporterName, &groupID, &groupName, &status, &createdAt, &encryptedPayload); err != nil {
			http.Error(w, "Failed to scan abuse reports: "+err.Error(), http.StatusInternalServerError)
			return
		}

		reportList = append(reportList, map[string]interface{}{
			"id":                id,
			"reported_by":       reportedBy,
			"reporter_username": reporterName.String,
			"group_id":          groupID,
			"group_name":        groupName.String,
			"status":            status,
			"created_at":        createdAt,
			"encrypted_payload": encryptedPayload,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"reports": reportList,
		"count":   len(reportList),
	})
}

type AdminBlockMemberRequest struct {
	GroupID string `json:"group_id"`
	UserID  string `json:"user_id"`
}

// AdminBlockGroupMember evicts and blocks a user from a specific group chat.
func AdminBlockGroupMember(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var req AdminBlockMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	groupUUID, err := uuid.Parse(req.GroupID)
	if err != nil {
		http.Error(w, "invalid group ID formatting", http.StatusBadRequest)
		return
	}

	userUUID, err := uuid.Parse(req.UserID)
	if err != nil {
		http.Error(w, "invalid user ID formatting", http.StatusBadRequest)
		return
	}

	// 1. Evict the user from the group membership
	res, err := database.PostgresDB.ExecContext(r.Context(), `
		DELETE FROM group_members 
		WHERE group_id = $1 AND user_id = $2
	`, groupUUID, userUUID)
	if err != nil {
		http.Error(w, "failed to evict group member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	affected, _ := res.RowsAffected()
	if affected > 0 {
		// Decrease member count in group
		_, _ = database.PostgresDB.ExecContext(r.Context(), `
			UPDATE groups SET member_count = member_count - 1 
			WHERE id = $1 AND member_count > 0
		`, groupUUID)
	}

	// 2. Insert into group_blocks to prevent them from re-joining
	_, err = database.PostgresDB.ExecContext(r.Context(), `
		INSERT INTO group_blocks (group_id, user_id, blocked_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (group_id, user_id) DO NOTHING
	`, groupUUID, userUUID)
	if err != nil {
		http.Error(w, "failed to block member from group chat: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Immutably log administrative block action
	_, _ = database.PostgresDB.ExecContext(r.Context(), `
		INSERT INTO security_audit_logs (id, event_type, target_id, actor_id, actor_role, reason, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, uuid.New(), "ADMIN_MEMBER_GROUP_BLOCKED", userUUID.String(), "admin", "admin", "User blocked administratively from group chat: "+groupUUID.String(), clientip.RealClientIP(r), time.Now().UTC())

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "User evicted and blocked from group chat successfully",
	})
}
