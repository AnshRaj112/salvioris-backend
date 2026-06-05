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

	var existingOnboardingID uuid.UUID
	err = database.PostgresDB.QueryRow(`
		SELECT id FROM patient_onboardings
		WHERE therapist_id = $1 AND LOWER(patient_email) = $2
	`, therapistID, req.PatientEmail).Scan(&existingOnboardingID)
	if err == nil {
		http.Error(w, "This patient email has already been onboarded", http.StatusConflict)
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
			therapist_id, user_id, patient_name, patient_email, username, referral_code_id, onboarded_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id
	`, therapistID, userID, req.PatientName, req.PatientEmail, username, referralID).Scan(&onboardingID)
	if err != nil {
		http.Error(w, "Failed to record onboarding", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if err := services.SendOnboardingEmail(services.OnboardingEmailData{
		Email:             req.PatientEmail,
		PatientEmail:      req.PatientEmail,
		PatientName:       req.PatientName,
		TherapistName:     therapistName,
		Username:          username,
		TemporaryPassword: temporaryPassword,
	}); err != nil {
		log.Printf("ERROR sending onboarding email to %s: %v", req.PatientEmail, err)
		http.Error(w, "Patient account created but failed to send onboarding email", http.StatusInternalServerError)
		return
	}

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

	rows, err := database.PostgresDB.Query(`
		SELECT po.id, po.user_id, po.patient_name, po.patient_email, po.username, rc.code, po.onboarded_at
		FROM patient_onboardings po
		JOIN referral_codes rc ON rc.id = po.referral_code_id
		WHERE po.therapist_id = $1
		ORDER BY po.onboarded_at DESC
	`, therapistID)
	if err != nil {
		http.Error(w, "Failed to fetch onboarded patients", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	patients := []models.PatientOnboarding{}
	for rows.Next() {
		var patient models.PatientOnboarding
		if err := rows.Scan(
			&patient.ID,
			&patient.UserID,
			&patient.PatientName,
			&patient.PatientEmail,
			&patient.Username,
			&patient.ReferralCode,
			&patient.OnboardedAt,
		); err != nil {
			log.Printf("ERROR scanning onboarded patient: %v", err)
			continue
		}
		patient.TherapistID = therapistID
		patients = append(patients, patient)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"patients": patients,
	})
}
