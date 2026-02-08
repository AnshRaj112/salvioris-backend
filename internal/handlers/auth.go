package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/pkg/utils"
	"github.com/google/uuid"
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
	var existingEmail string
	err := database.PostgresDB.QueryRow("SELECT email FROM users WHERE email = $1", req.Email).Scan(&existingEmail)
	if err == nil {
		http.Error(w, "User with this email already exists", http.StatusConflict)
		return
	} else if err != sql.ErrNoRows {
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
	userID := uuid.New()
	now := time.Now()
	
	_, err = database.PostgresDB.Exec(`
		INSERT INTO users (id, created_at, updated_at, name, email, password, street, city, state, zip_code, country)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, userID, now, now, req.Name, req.Email, hashedPassword, req.Street, req.City, req.State, req.ZipCode, req.Country)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Return user (without password)
	userMap := map[string]interface{}{
		"id":         userID.String(),
		"name":       req.Name,
		"email":      req.Email,
		"created_at": now,
		"street":     req.Street,
		"city":       req.City,
		"state":      req.State,
		"zip_code":   req.ZipCode,
		"country":    req.Country,
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
	var userID uuid.UUID
	var name, email, password, street, city, state, zipCode, country sql.NullString
	var createdAt time.Time
	
	err := database.PostgresDB.QueryRow(`
		SELECT id, created_at, name, email, password, street, city, state, zip_code, country
		FROM users WHERE email = $1
	`, req.Email).Scan(&userID, &createdAt, &name, &email, &password, &street, &city, &state, &zipCode, &country)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Verify password
	valid, err := utils.VerifyPassword(req.Password, password.String)
	if err != nil || !valid {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Return user (without password)
	userMap := map[string]interface{}{
		"id":         userID.String(),
		"name":       name.String,
		"email":      email.String,
		"created_at": createdAt,
		"street":     street.String,
		"city":       city.String,
		"state":      state.String,
		"zip_code":   zipCode.String,
		"country":    country.String,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Message: "Login successful",
		User:    userMap,
	})
}

// TherapistSignup handles therapist registration with multipart/form-data (includes file uploads)
func TherapistSignup(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 20MB for both images + form data)
	err := r.ParseMultipartForm(20 << 20) // 20MB
	if err != nil {
		log.Printf("ERROR: Failed to parse multipart form: %v", err)
		http.Error(w, "Invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Log all received files for debugging
	log.Printf("FILES RECEIVED: %v", r.MultipartForm.File)

	// Extract form values
	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")
	licenseNumber := r.FormValue("license_number")
	licenseState := r.FormValue("license_state")
	phone := r.FormValue("phone")
	collegeDegree := r.FormValue("college_degree")
	mastersInstitution := r.FormValue("masters_institution")
	psychologistType := r.FormValue("psychologist_type")
	dsmAwareness := r.FormValue("dsm_awareness")
	therapyTypes := r.FormValue("therapy_types")
	specialization := r.FormValue("specialization")

	// Parse integer fields
	yearsOfExperience, _ := strconv.Atoi(r.FormValue("years_of_experience"))
	successfulCases, _ := strconv.Atoi(r.FormValue("successful_cases"))

	// Validate required fields
	if name == "" || email == "" || password == "" ||
		licenseNumber == "" || licenseState == "" || phone == "" ||
		collegeDegree == "" || mastersInstitution == "" ||
		psychologistType == "" || dsmAwareness == "" || therapyTypes == "" {
		http.Error(w, "All required fields must be provided", http.StatusBadRequest)
		return
	}

	// Check if therapist already exists
	var existingEmail string
	err = database.PostgresDB.QueryRow("SELECT email FROM therapists WHERE email = $1", email).Scan(&existingEmail)
	if err == nil {
		http.Error(w, "Therapist with this email already exists", http.StatusConflict)
		return
	} else if err != sql.ErrNoRows {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Check if Cloudinary service is initialized
	if cloudinaryService == nil {
		log.Printf("ERROR: Cloudinary service not initialized")
		http.Error(w, "File upload service not available", http.StatusInternalServerError)
		return
	}

	// Extract BOTH files from multipart form
	var certificateURL, degreeURL string

	// Upload certificate image if provided
	certificateHeaders, certExists := r.MultipartForm.File["certificate_image"]
	if certExists && len(certificateHeaders) > 0 {
		certHeader := certificateHeaders[0]
		log.Printf("Uploading certificate image: %s, size: %d bytes", certHeader.Filename, certHeader.Size)

		file, err := certHeader.Open()
		if err != nil {
			log.Printf("ERROR: Failed to open certificate file: %v", err)
			http.Error(w, "Failed to process certificate image", http.StatusInternalServerError)
			return
		}

		// Upload to Cloudinary (empty folder = root)
		certificateURL, err = cloudinaryService.UploadFile(r.Context(), file, certHeader, "")
		file.Close()
		if err != nil {
			log.Printf("ERROR: Certificate upload failed: %v", err)
			http.Error(w, "Failed to upload certificate image: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("✅ Certificate uploaded to Cloudinary: %s", certificateURL)
	} else {
		log.Printf("⚠️  No certificate_image file provided")
	}

	// Upload degree image if provided
	degreeHeaders, degreeExists := r.MultipartForm.File["degree_image"]
	if degreeExists && len(degreeHeaders) > 0 {
		degreeHeader := degreeHeaders[0]
		log.Printf("Uploading degree image: %s, size: %d bytes", degreeHeader.Filename, degreeHeader.Size)

		file, err := degreeHeader.Open()
		if err != nil {
			log.Printf("ERROR: Failed to open degree file: %v", err)
			http.Error(w, "Failed to process degree image", http.StatusInternalServerError)
			return
		}

		// Upload to Cloudinary (empty folder = root)
		degreeURL, err = cloudinaryService.UploadFile(r.Context(), file, degreeHeader, "")
		file.Close()
		if err != nil {
			log.Printf("ERROR: Degree upload failed: %v", err)
			http.Error(w, "Failed to upload degree image: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("✅ Degree uploaded to Cloudinary: %s", degreeURL)
	} else {
		log.Printf("⚠️  No degree_image file provided")
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	// Create therapist - EXPLICITLY set image URLs from Cloudinary uploads
	therapistID := uuid.New()
	now := time.Now()
	
	// Verify what we're about to save
	log.Printf("Creating therapist: %s (%s)", name, email)
	log.Printf("Therapist struct before DB save:")
	log.Printf("  CertificateImagePath: %q (length: %d)", certificateURL, len(certificateURL))
	log.Printf("  DegreeImagePath: %q (length: %d)", degreeURL, len(degreeURL))

	_, err = database.PostgresDB.Exec(`
		INSERT INTO therapists (
			id, created_at, updated_at, name, email, password, license_number, license_state,
			years_of_experience, specialization, phone, college_degree, masters_institution,
			psychologist_type, successful_cases, dsm_awareness, therapy_types,
			certificate_image_path, degree_image_path, is_approved
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
	`, therapistID, now, now, name, email, hashedPassword, licenseNumber, licenseState,
		yearsOfExperience, specialization, phone, collegeDegree, mastersInstitution,
		psychologistType, successfulCases, dsmAwareness, therapyTypes,
		certificateURL, degreeURL, false)
	if err != nil {
		log.Printf("ERROR: Failed to insert therapist: %v", err)
		http.Error(w, "Failed to create therapist", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Therapist created successfully with ID: %s", therapistID.String())
	log.Printf("✅ Saved certificate_image_path: %q", certificateURL)
	log.Printf("✅ Saved degree_image_path: %q", degreeURL)

	// Return therapist (without password)
	therapistMap := map[string]interface{}{
		"id":                   therapistID.String(),
		"name":                 name,
		"email":                email,
		"created_at":           now,
		"license_number":       licenseNumber,
		"license_state":        licenseState,
		"years_of_experience":  yearsOfExperience,
		"specialization":       specialization,
		"phone":                phone,
		"college_degree":       collegeDegree,
		"masters_institution":  mastersInstitution,
		"psychologist_type":    psychologistType,
		"successful_cases":     successfulCases,
		"dsm_awareness":        dsmAwareness,
		"therapy_types":        therapyTypes,
		"certificate_image_path": certificateURL,
		"degree_image_path":     degreeURL,
		"is_approved":          false,
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
	var therapistID uuid.UUID
	var name, email, password, licenseNumber, licenseState, specialization, phone sql.NullString
	var collegeDegree, mastersInstitution, psychologistType, dsmAwareness, therapyTypes sql.NullString
	var certificateImagePath, degreeImagePath sql.NullString
	var yearsOfExperience, successfulCases int
	var isApproved bool
	var createdAt time.Time
	
	err := database.PostgresDB.QueryRow(`
		SELECT id, created_at, name, email, password, license_number, license_state,
			years_of_experience, specialization, phone, college_degree, masters_institution,
			psychologist_type, successful_cases, dsm_awareness, therapy_types,
			certificate_image_path, degree_image_path, is_approved
		FROM therapists WHERE email = $1
	`, req.Email).Scan(&therapistID, &createdAt, &name, &email, &password, &licenseNumber, &licenseState,
		&yearsOfExperience, &specialization, &phone, &collegeDegree, &mastersInstitution,
		&psychologistType, &successfulCases, &dsmAwareness, &therapyTypes,
		&certificateImagePath, &degreeImagePath, &isApproved)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Verify password
	valid, err := utils.VerifyPassword(req.Password, password.String)
	if err != nil || !valid {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Check if therapist is approved
	if !isApproved {
		http.Error(w, "Your application is pending approval. Please wait for admin approval before logging in.", http.StatusForbidden)
		return
	}

	// Return therapist (without password)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Message: "Login successful",
		User:    therapistMap,
	})
}

