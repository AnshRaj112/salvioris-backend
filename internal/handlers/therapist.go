package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// CheckTherapistStatus checks if a therapist is approved
func CheckTherapistStatus(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var therapist models.Therapist
	err := database.DB.Collection("therapists").FindOne(ctx, bson.M{"email": email}).Decode(&therapist)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Therapist not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	response := map[string]interface{}{
		"is_approved": therapist.IsApproved,
		"email":       therapist.Email,
		"name":        therapist.Name,
	}

	if therapist.IsApproved {
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

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var therapist models.Therapist
	err = database.DB.Collection("therapists").FindOne(ctx, bson.M{"_id": objectID}).Decode(&therapist)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Therapist not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	therapistMap := map[string]interface{}{
		"id":                   therapist.ID.Hex(),
		"name":                 therapist.Name,
		"email":                therapist.Email,
		"created_at":           therapist.CreatedAt,
		"license_number":       therapist.LicenseNumber,
		"license_state":       therapist.LicenseState,
		"years_of_experience":  therapist.YearsOfExperience,
		"specialization":       therapist.Specialization,
		"phone":                therapist.Phone,
		"college_degree":       therapist.CollegeDegree,
		"masters_institution":  therapist.MastersInstitution,
		"psychologist_type":    therapist.PsychologistType,
		"successful_cases":     therapist.SuccessfulCases,
		"dsm_awareness":        therapist.DSMAwareness,
		"therapy_types":        therapist.TherapyTypes,
		"certificate_image_path": therapist.CertificateImagePath,
		"degree_image_path":     therapist.DegreeImagePath,
		"is_approved":          therapist.IsApproved,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(therapistMap)
}

