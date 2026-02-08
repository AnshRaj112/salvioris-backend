package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

// CheckTherapistStatus checks if a therapist is approved
func CheckTherapistStatus(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	var name sql.NullString
	var isApproved bool
	err := database.PostgresDB.QueryRow(`
		SELECT name, is_approved FROM therapists WHERE email = $1
	`, email).Scan(&name, &isApproved)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Therapist not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	response := map[string]interface{}{
		"is_approved": isApproved,
		"email":       email,
		"name":        name.String,
	}

	if isApproved {
		response["message"] = "Your application has been approved! You can now sign in."
	} else {
		response["message"] = "Your application is still pending approval."
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetTherapistByID gets therapist by ID (for admin use)
func GetTherapistByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	// Validate UUID format
	if _, err := uuid.Parse(id); err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	var therapistID, name, email, licenseNumber, licenseState, phone sql.NullString
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
	`, id).Scan(&therapistID, &createdAt, &name, &email, &licenseNumber, &licenseState,
		&yearsOfExperience, &specialization, &phone, &collegeDegree, &mastersInstitution,
		&psychologistType, &successfulCases, &dsmAwareness, &therapyTypes,
		&certificateImagePath, &degreeImagePath, &isApproved)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Therapist not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	therapistMap := map[string]interface{}{
		"id":                   therapistID.String,
		"name":                 name.String,
		"email":                email.String,
		"created_at":           createdAt,
		"license_number":       licenseNumber.String,
		"license_state":       licenseState.String,
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(therapistMap)
}

