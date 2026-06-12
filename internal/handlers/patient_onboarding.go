package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/AnshRaj112/serenify-backend/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type onboardPatientRequest struct {
	PatientName  string `json:"patient_name"`
	PatientEmail string `json:"patient_email"`
}

// OnboardPatient creates a patient account, links referral, and emails credentials.
func OnboardPatient(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	var req onboardPatientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.PatientName = strings.TrimSpace(req.PatientName)
	req.PatientEmail = strings.TrimSpace(strings.ToLower(req.PatientEmail))

	if req.PatientName == "" {
		http.Error(w, "Patient name is required", http.StatusBadRequest)
		return
	}
	if req.PatientEmail == "" || !emailRegex.MatchString(req.PatientEmail) {
		http.Error(w, "A valid patient email is required", http.StatusBadRequest)
		return
	}

	var therapistName string
	err := database.PostgresDB.QueryRow(
		"SELECT name FROM therapists WHERE id = $1",
		therapistID,
	).Scan(&therapistName)
	if err != nil {
		http.Error(w, "Failed to load therapist profile", http.StatusInternalServerError)
		return
	}

	var existingID uuid.UUID
	var currentHash, initialHash sql.NullString
	var hasLoggedIn bool
	err = database.PostgresDB.QueryRow(`
		SELECT po.id, u.password_hash, po.initial_password_hash,
			EXISTS(SELECT 1 FROM user_devices ud WHERE ud.user_id = po.user_id)
		FROM patient_onboardings po
		JOIN users u ON u.id = po.user_id
		WHERE po.therapist_id = $1 AND LOWER(po.patient_email) = $2
	`, therapistID, req.PatientEmail).Scan(&existingID, &currentHash, &initialHash, &hasLoggedIn)
	if err == nil {
		activated := hasLoggedIn
		if initialHash.Valid && currentHash.Valid && currentHash.String != initialHash.String {
			activated = true
		}
		msg := "Onboarding already sent for this email. Awaiting patient login."
		if activated {
			msg = "This patient has already logged in or changed their password. Cannot onboard again."
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": msg})
		return
	}
	if err != sql.ErrNoRows {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	username, err := utils.GenerateUniqueUsername(database.PostgresDB)
	if err != nil {
		http.Error(w, "Failed to generate username", http.StatusInternalServerError)
		return
	}

	temporaryPassword, err := utils.GenerateTemporaryPassword()
	if err != nil {
		http.Error(w, "Failed to generate temporary password", http.StatusInternalServerError)
		return
	}

	hashedPassword, err := utils.HashPassword(temporaryPassword)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	referralCode, err := GenerateSecureCode()
	if err != nil {
		http.Error(w, "Failed to generate referral code", http.StatusInternalServerError)
		return
	}

	tx, err := database.PostgresDB.Begin()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	userID := uuid.New()
	_, err = tx.Exec(`
		INSERT INTO users (id, username, password_hash, created_at, is_active)
		VALUES ($1, $2, $3, NOW(), TRUE)
	`, userID, username, hashedPassword)
	if err != nil {
		http.Error(w, "Failed to create patient account", http.StatusInternalServerError)
		return
	}

	emailEncrypted, err := utils.Encrypt(req.PatientEmail)
	if err != nil {
		log.Printf("WARNING: Failed to encrypt recovery email for onboarded patient %s: %v", username, err)
	} else {
		_, err = tx.Exec(`
			INSERT INTO user_recovery (id, user_id, email_encrypted, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2, NOW(), NOW())
		`, userID, emailEncrypted)
		if err != nil {
			log.Printf("WARNING: Failed to save recovery email for onboarded patient %s: %v", username, err)
		}
	}

	usageLimit := 1
	var referralID uuid.UUID
	err = tx.QueryRow(`
		INSERT INTO referral_codes (therapist_id, code, usage_limit, usage_count)
		VALUES ($1, $2, $3, 1)
		RETURNING id
	`, therapistID, referralCode, usageLimit).Scan(&referralID)
	if err != nil {
		http.Error(w, "Failed to create referral code", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(`
		INSERT INTO referral_usages (id, referral_code_id, user_id, used_at)
		VALUES (gen_random_uuid(), $1, $2, NOW())
	`, referralID, userID)
	if err != nil {
		http.Error(w, "Failed to record referral usage", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(`
		INSERT INTO therapist_user_connections (id, therapist_id, user_id, connected_at, connection_type, referral_code_id)
		VALUES (gen_random_uuid(), $1, $2, NOW(), 'referral', $3)
	`, therapistID, userID, referralID)
	if err != nil {
		http.Error(w, "Failed to establish therapist connection", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(`
		INSERT INTO consent_history (id, user_id, therapist_id, action, timestamp, details)
		VALUES (gen_random_uuid(), $1, $2, 'granted_referral', NOW(), $3)
	`, userID, therapistID, fmt.Sprintf("Patient onboarded by therapist via referral code %s", referralCode))
	if err != nil {
		log.Printf("WARNING: Failed to log consent history during onboarding: %v", err)
	}

	var onboardingID uuid.UUID
	err = tx.QueryRow(`
		INSERT INTO patient_onboardings (
			therapist_id, user_id, patient_name, patient_email, username, referral_code_id, initial_password_hash, onboarded_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		RETURNING id
	`, therapistID, userID, req.PatientName, req.PatientEmail, username, referralID, hashedPassword).Scan(&onboardingID)
	if err != nil {
		http.Error(w, "Failed to record onboarding", http.StatusInternalServerError)
		return
	}

	// Sync to V2 patients table for patient self-service dashboard APIs.
	var tenantID uuid.UUID
	err = tx.QueryRow(`SELECT id FROM tenants WHERE therapist_id = $1`, therapistID).Scan(&tenantID)
	if err == sql.ErrNoRows {
		err = tx.QueryRow(`
			INSERT INTO tenants (therapist_id, display_name)
			VALUES ($1, $2)
			RETURNING id
		`, therapistID, therapistName).Scan(&tenantID)
	}
	if err != nil {
		log.Printf("ERROR: Failed to ensure tenant for onboarded patient %s: %v", userID, err)
		http.Error(w, "Failed to link patient dashboard profile", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(`
		INSERT INTO patients (tenant_id, user_id, full_name, email, assigned_therapist_id, status)
		VALUES ($1, $2, $3, $4, $5, 'active')
	`, tenantID, userID, req.PatientName, req.PatientEmail, therapistID)
	if err != nil {
		log.Printf("ERROR: Failed to create V2 patient profile for onboarded user %s: %v", userID, err)
		http.Error(w, "Failed to link patient dashboard profile", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	invalidateTherapistCaches(therapistID, "onboarded", "connections", "referrals", "analytics")

	emailData := services.OnboardingEmailData{
		Email: req.PatientEmail, PatientEmail: req.PatientEmail, PatientName: req.PatientName,
		TherapistName: therapistName, Username: username, TemporaryPassword: temporaryPassword,
	}
	go func() {
		if err := services.SendOnboardingEmail(emailData); err != nil {
			log.Printf("ERROR sending onboarding email to %s: %v", req.PatientEmail, err)
		}
	}()

	database.TriggerAuditEvent(
		"PATIENT_ONBOARDED",
		onboardingID.String(),
		therapistID.String(),
		"therapist",
		fmt.Sprintf("Therapist onboarded patient %s (%s)", req.PatientName, req.PatientEmail),
		r,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Patient onboarded successfully. Credentials have been emailed to the patient.",
		"patient": map[string]interface{}{
			"id":            onboardingID.String(),
			"user_id":       userID.String(),
			"patient_name":  req.PatientName,
			"patient_email": req.PatientEmail,
			"username":      username,
			"referral_code": referralCode,
			"onboarded_at":  time.Now(),
		},
	})
}

// ListOnboardedPatients returns patients onboarded by the authenticated therapist.
func ListOnboardedPatients(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	if cached, ok := readTherapistCache(therapistID, "onboarded"); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cached)
		return
	}

	rows, err := database.PostgresDB.Query(`
		SELECT po.id, po.user_id, po.patient_name, po.patient_email, po.username, rc.code, po.onboarded_at,
			(EXISTS(SELECT 1 FROM user_devices ud WHERE ud.user_id = po.user_id)
			 OR (po.initial_password_hash IS NOT NULL AND u.password_hash IS DISTINCT FROM po.initial_password_hash)) AS activated
		FROM patient_onboardings po
		JOIN referral_codes rc ON rc.id = po.referral_code_id
		JOIN users u ON u.id = po.user_id
		WHERE po.therapist_id = $1
		ORDER BY po.onboarded_at DESC
	`, therapistID)
	if err != nil {
		http.Error(w, "Failed to fetch onboarded patients", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type onboardedPatient struct {
		models.PatientOnboarding
		Status string `json:"status"`
	}
	patients := []onboardedPatient{}
	for rows.Next() {
		var patient onboardedPatient
		var activated bool
		if err := rows.Scan(
			&patient.ID, &patient.UserID, &patient.PatientName, &patient.PatientEmail,
			&patient.Username, &patient.ReferralCode, &patient.OnboardedAt, &activated,
		); err != nil {
			log.Printf("ERROR scanning onboarded patient: %v", err)
			continue
		}
		patient.TherapistID = therapistID
		patient.Status = "pending"
		if activated {
			patient.Status = "activated"
		}
		patients = append(patients, patient)
	}

	resp, _ := json.Marshal(map[string]interface{}{"success": true, "patients": patients})
	writeTherapistCache(therapistID, "onboarded", resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// RemoveOnboardedPatient removes a patient from the therapist onboarding list.
func RemoveOnboardedPatient(w http.ResponseWriter, r *http.Request) {
	therapistID, ok := requireTherapistAuth(r)
	if !ok {
		http.Error(w, "Unauthorized therapist access", http.StatusUnauthorized)
		return
	}

	onboardingID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid onboarding ID", http.StatusBadRequest)
		return
	}

	var userID uuid.UUID
	var activated bool
	err = database.PostgresDB.QueryRow(`
		SELECT po.user_id,
			(EXISTS(SELECT 1 FROM user_devices ud WHERE ud.user_id = po.user_id)
			 OR (po.initial_password_hash IS NOT NULL AND u.password_hash IS DISTINCT FROM po.initial_password_hash))
		FROM patient_onboardings po
		JOIN users u ON u.id = po.user_id
		WHERE po.id = $1 AND po.therapist_id = $2
	`, onboardingID, therapistID).Scan(&userID, &activated)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Onboarded patient not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	tx, err := database.PostgresDB.Begin()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM patient_onboardings WHERE id = $1`, onboardingID)
	if err != nil {
		http.Error(w, "Failed to remove onboarded patient", http.StatusInternalServerError)
		return
	}

	if !activated {
		_, _ = tx.Exec(`DELETE FROM therapist_user_connections WHERE therapist_id = $1 AND user_id = $2`, therapistID, userID)
		_, _ = tx.Exec(`DELETE FROM connection_requests WHERE therapist_id = $1 AND user_id = $2`, therapistID, userID)
		_, err = tx.Exec(`UPDATE users SET is_active = FALSE WHERE id = $1`, userID)
		if err != nil {
			http.Error(w, "Failed to deactivate patient account", http.StatusInternalServerError)
			return
		}
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	invalidateTherapistCaches(therapistID, "onboarded", "connections")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Patient removed from onboarding list",
	})
}
