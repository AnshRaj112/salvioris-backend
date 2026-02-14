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
			"id":                    id.String,
			"name":                  name.String,
			"email":                 email.String,
			"created_at":            createdAt,
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
			"id":                    id.String,
			"name":                  name.String,
			"email":                 email.String,
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
		"success": true,
		"message": "IP address unblocked successfully",
		"ip_address": ipAddress,
	}
	
	if blockedIP != nil {
		response["previous_reason"] = blockedIP.Reason
		response["was_blocked_until"] = blockedIP.ExpiresAt
	}

	json.NewEncoder(w).Encode(response)
}

