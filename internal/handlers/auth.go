package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/pkg/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// User Signup Request
type UserSignupRequest struct {
	Name            string `json:"name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	Street          string `json:"street,omitempty"`
	City            string `json:"city,omitempty"`
	State           string `json:"state,omitempty"`
	ZipCode         string `json:"zip_code,omitempty"`
	Country         string `json:"country,omitempty"`
}

// User Signin Request
type UserSigninRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Therapist Signup Request
type TherapistSignupRequest struct {
	Name                string `json:"name"`
	Email               string `json:"email"`
	Password            string `json:"password"`
	LicenseNumber       string `json:"license_number"`
	LicenseState        string `json:"license_state"`
	YearsOfExperience   int    `json:"years_of_experience"`
	Specialization      string `json:"specialization,omitempty"`
	Phone               string `json:"phone"`
	CollegeDegree       string `json:"college_degree"`
	MastersInstitution  string `json:"masters_institution"`
	PsychologistType    string `json:"psychologist_type"`
	SuccessfulCases     int    `json:"successful_cases"`
	DSMAwareness        string `json:"dsm_awareness"`
	TherapyTypes        string `json:"therapy_types"`
	CertificateImagePath string `json:"certificate_image_path,omitempty"`
	DegreeImagePath      string `json:"degree_image_path,omitempty"`
}

// Therapist Signin Request
type TherapistSigninRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Auth Response
type AuthResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	User    map[string]interface{} `json:"user,omitempty"`
	Token   string                 `json:"token,omitempty"`
}

// UserSignup handles user registration
func UserSignup(w http.ResponseWriter, r *http.Request) {
	var req UserSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "Name, email, and password are required", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var existingUser models.User
	err := database.DB.Collection("users").FindOne(ctx, bson.M{"email": req.Email}).Decode(&existingUser)
	if err == nil {
		http.Error(w, "User with this email already exists", http.StatusConflict)
		return
	} else if err != mongo.ErrNoDocuments {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	// Create user
	user := models.User{
		ID:        primitive.NewObjectID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      req.Name,
		Email:     req.Email,
		Password:  hashedPassword,
		Street:    req.Street,
		City:      req.City,
		State:     req.State,
		ZipCode:   req.ZipCode,
		Country:   req.Country,
	}

	_, err = database.DB.Collection("users").InsertOne(ctx, user)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Return user (without password)
	userMap := map[string]interface{}{
		"id":         user.ID.Hex(),
		"name":       user.Name,
		"email":      user.Email,
		"created_at": user.CreatedAt,
		"street":     user.Street,
		"city":       user.City,
		"state":      user.State,
		"zip_code":   user.ZipCode,
		"country":    user.Country,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Message: "User created successfully",
		User:    userMap,
	})
}

// UserSignin handles user login
func UserSignin(w http.ResponseWriter, r *http.Request) {
	var req UserSigninRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	// Find user
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := database.DB.Collection("users").FindOne(ctx, bson.M{"email": req.Email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Verify password
	valid, err := utils.VerifyPassword(req.Password, user.Password)
	if err != nil || !valid {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Return user (without password)
	userMap := map[string]interface{}{
		"id":         user.ID.Hex(),
		"name":       user.Name,
		"email":      user.Email,
		"created_at": user.CreatedAt,
		"street":     user.Street,
		"city":       user.City,
		"state":      user.State,
		"zip_code":   user.ZipCode,
		"country":    user.Country,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Message: "Login successful",
		User:    userMap,
	})
}

// TherapistSignup handles therapist registration
func TherapistSignup(w http.ResponseWriter, r *http.Request) {
	var req TherapistSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.Email == "" || req.Password == "" ||
		req.LicenseNumber == "" || req.LicenseState == "" || req.Phone == "" ||
		req.CollegeDegree == "" || req.MastersInstitution == "" ||
		req.PsychologistType == "" || req.DSMAwareness == "" || req.TherapyTypes == "" {
		http.Error(w, "All required fields must be provided", http.StatusBadRequest)
		return
	}

	// Check if therapist already exists
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var existingTherapist models.Therapist
	err := database.DB.Collection("therapists").FindOne(ctx, bson.M{"email": req.Email}).Decode(&existingTherapist)
	if err == nil {
		http.Error(w, "Therapist with this email already exists", http.StatusConflict)
		return
	} else if err != mongo.ErrNoDocuments {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	// Create therapist
	therapist := models.Therapist{
		ID:                  primitive.NewObjectID(),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
		Name:                req.Name,
		Email:               req.Email,
		Password:            hashedPassword,
		LicenseNumber:       req.LicenseNumber,
		LicenseState:        req.LicenseState,
		YearsOfExperience:   req.YearsOfExperience,
		Specialization:      req.Specialization,
		Phone:               req.Phone,
		CollegeDegree:       req.CollegeDegree,
		MastersInstitution:  req.MastersInstitution,
		PsychologistType:    req.PsychologistType,
		SuccessfulCases:     req.SuccessfulCases,
		DSMAwareness:        req.DSMAwareness,
		TherapyTypes:        req.TherapyTypes,
		CertificateImagePath: req.CertificateImagePath,
		DegreeImagePath:      req.DegreeImagePath,
		IsApproved:          false,
	}

	_, err = database.DB.Collection("therapists").InsertOne(ctx, therapist)
	if err != nil {
		http.Error(w, "Failed to create therapist", http.StatusInternalServerError)
		return
	}

	// Return therapist (without password)
	therapistMap := map[string]interface{}{
		"id":                   therapist.ID.Hex(),
		"name":                 therapist.Name,
		"email":                therapist.Email,
		"created_at":           therapist.CreatedAt,
		"license_number":       therapist.LicenseNumber,
		"license_state":        therapist.LicenseState,
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
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Message: "Therapist application submitted successfully. Awaiting approval.",
		User:    therapistMap,
	})
}

// TherapistSignin handles therapist login
func TherapistSignin(w http.ResponseWriter, r *http.Request) {
	var req TherapistSigninRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	// Find therapist
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var therapist models.Therapist
	err := database.DB.Collection("therapists").FindOne(ctx, bson.M{"email": req.Email}).Decode(&therapist)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Verify password
	valid, err := utils.VerifyPassword(req.Password, therapist.Password)
	if err != nil || !valid {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Check if therapist is approved
	if !therapist.IsApproved {
		http.Error(w, "Your application is pending approval. Please wait for admin approval before logging in.", http.StatusForbidden)
		return
	}

	// Return therapist (without password)
	therapistMap := map[string]interface{}{
		"id":                   therapist.ID.Hex(),
		"name":                 therapist.Name,
		"email":                therapist.Email,
		"created_at":           therapist.CreatedAt,
		"license_number":       therapist.LicenseNumber,
		"license_state":        therapist.LicenseState,
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
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Message: "Login successful",
		User:    therapistMap,
	})
}

