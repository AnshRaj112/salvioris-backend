package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetPendingTherapists returns all therapists with is_approved = false
func GetPendingTherapists(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Content-Type", "application/json")
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find all pending therapists
	cursor, err := database.DB.Collection("therapists").Find(ctx, bson.M{"is_approved": false}, options.Find().SetSort(bson.M{"created_at": -1}))
	if err != nil {
		http.Error(w, "Failed to fetch therapists: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var therapists []models.Therapist
	if err = cursor.All(ctx, &therapists); err != nil {
		http.Error(w, "Failed to decode therapists: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	therapistList := make([]map[string]interface{}, len(therapists))
	for i, therapist := range therapists {
		therapistList[i] = map[string]interface{}{
			"id":                    therapist.ID.Hex(),
			"name":                  therapist.Name,
			"email":                 therapist.Email,
			"created_at":            therapist.CreatedAt,
			"license_number":       therapist.LicenseNumber,
			"license_state":        therapist.LicenseState,
			"years_of_experience":  therapist.YearsOfExperience,
			"specialization":        therapist.Specialization,
			"phone":                therapist.Phone,
			"college_degree":       therapist.CollegeDegree,
			"masters_institution":  therapist.MastersInstitution,
			"psychologist_type":    therapist.PsychologistType,
			"successful_cases":      therapist.SuccessfulCases,
			"dsm_awareness":         therapist.DSMAwareness,
			"therapy_types":         therapist.TherapyTypes,
			"certificate_image_path": therapist.CertificateImagePath,
			"degree_image_path":      therapist.DegreeImagePath,
			"is_approved":           therapist.IsApproved,
		}
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
	therapistID := r.URL.Query().Get("id")
	if therapistID == "" {
		http.Error(w, "Therapist ID is required", http.StatusBadRequest)
		return
	}

	objectID, err := primitive.ObjectIDFromHex(therapistID)
	if err != nil {
		http.Error(w, "Invalid therapist ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Update therapist to approved
	result, err := database.DB.Collection("therapists").UpdateOne(
		ctx,
		bson.M{"_id": objectID},
		bson.M{"$set": bson.M{"is_approved": true, "updated_at": time.Now()}},
	)
	if err != nil {
		http.Error(w, "Failed to approve therapist: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if result.MatchedCount == 0 {
		http.Error(w, "Therapist not found", http.StatusNotFound)
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
	therapistID := r.URL.Query().Get("id")
	if therapistID == "" {
		http.Error(w, "Therapist ID is required", http.StatusBadRequest)
		return
	}

	objectID, err := primitive.ObjectIDFromHex(therapistID)
	if err != nil {
		http.Error(w, "Invalid therapist ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete therapist application
	result, err := database.DB.Collection("therapists").DeleteOne(ctx, bson.M{"_id": objectID, "is_approved": false})
	if err != nil {
		http.Error(w, "Failed to reject therapist: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if result.DeletedCount == 0 {
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := database.DB.Collection("therapists").Find(ctx, bson.M{"is_approved": true}, options.Find().SetSort(bson.M{"created_at": -1}))
	if err != nil {
		http.Error(w, "Failed to fetch therapists: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var therapists []models.Therapist
	if err = cursor.All(ctx, &therapists); err != nil {
		http.Error(w, "Failed to decode therapists: "+err.Error(), http.StatusInternalServerError)
		return
	}

	therapistList := make([]map[string]interface{}, len(therapists))
	for i, therapist := range therapists {
		therapistList[i] = map[string]interface{}{
			"id":                    therapist.ID.Hex(),
			"name":                  therapist.Name,
			"email":                 therapist.Email,
			"created_at":            therapist.CreatedAt,
			"license_number":        therapist.LicenseNumber,
			"license_state":        therapist.LicenseState,
			"years_of_experience":  therapist.YearsOfExperience,
			"specialization":        therapist.Specialization,
			"phone":                 therapist.Phone,
			"college_degree":       therapist.CollegeDegree,
			"masters_institution":  therapist.MastersInstitution,
			"psychologist_type":    therapist.PsychologistType,
			"successful_cases":      therapist.SuccessfulCases,
			"dsm_awareness":         therapist.DSMAwareness,
			"therapy_types":         therapist.TherapyTypes,
			"certificate_image_path": therapist.CertificateImagePath,
			"degree_image_path":     therapist.DegreeImagePath,
			"is_approved":           therapist.IsApproved,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"therapists": therapistList,
		"count":      len(therapistList),
	})
}

// GetViolations returns all content violations
func GetViolations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find all violations, sorted by most recent
	cursor, err := database.DB.Collection("violations").Find(ctx, bson.M{}, options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(100))
	if err != nil {
		http.Error(w, "Failed to fetch violations: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var violations []models.Violation
	if err = cursor.All(ctx, &violations); err != nil {
		http.Error(w, "Failed to decode violations: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	violationList := make([]map[string]interface{}, len(violations))
	for i, violation := range violations {
		violationMap := map[string]interface{}{
			"id":           violation.ID.Hex(),
			"created_at":   violation.CreatedAt,
			"ip_address":   violation.IPAddress,
			"type":         violation.Type,
			"message":      violation.Message,
			"vent_id":      violation.VentID,
			"action_taken": violation.ActionTaken,
		}
		if violation.UserID != nil {
			violationMap["user_id"] = violation.UserID.Hex()
		}
		violationList[i] = violationMap
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"violations": violationList,
		"count":      len(violationList),
	})
}

// GetBlockedIPs returns all currently blocked IP addresses
func GetBlockedIPs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find all active blocked IPs
	cursor, err := database.DB.Collection("blocked_ips").Find(ctx, bson.M{
		"is_active": true,
		"expires_at": bson.M{"$gt": time.Now()},
	}, options.Find().SetSort(bson.M{"created_at": -1}))
	if err != nil {
		http.Error(w, "Failed to fetch blocked IPs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var blockedIPs []models.BlockedIP
	if err = cursor.All(ctx, &blockedIPs); err != nil {
		http.Error(w, "Failed to decode blocked IPs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	blockedList := make([]map[string]interface{}, len(blockedIPs))
	for i, blocked := range blockedIPs {
		blockedList[i] = map[string]interface{}{
			"id":         blocked.ID.Hex(),
			"ip_address": blocked.IPAddress,
			"reason":     blocked.Reason,
			"created_at": blocked.CreatedAt,
			"expires_at": blocked.ExpiresAt,
			"is_active":  blocked.IsActive,
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"blocked_ips": blockedList,
		"count":       len(blockedList),
	})
}

// UnblockIP unblocks an IP address
func UnblockIP(w http.ResponseWriter, r *http.Request) {
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

