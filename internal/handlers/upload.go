package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/AnshRaj112/serenify-backend/internal/config"
	"github.com/AnshRaj112/serenify-backend/internal/services"
)

var cloudinaryService *services.CloudinaryService

func InitCloudinaryService(cfg *config.Config) error {
	service, err := services.NewCloudinaryService(
		cfg.CloudinaryName,
		cfg.CloudinaryAPIKey,
		cfg.CloudinaryAPISecret,
	)
	if err != nil {
		return err
	}
	cloudinaryService = service
	return nil
}

type UploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
}

// UploadFile handles file uploads to Cloudinary
func UploadFile(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if cloudinaryService == nil {
		log.Printf("ERROR: Cloudinary service is nil")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Message: "Cloudinary service not initialized",
		})
		return
	}

	log.Printf("Received upload request, Content-Type: %s", r.Header.Get("Content-Type"))

	// Parse multipart form (max 10MB)
	err := r.ParseMultipartForm(10 << 20) // 10MB
	if err != nil {
		log.Printf("ERROR: Failed to parse multipart form: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Message: "Failed to parse form: " + err.Error(),
		})
		return
	}

	// Get file from form (opens file ONCE)
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		log.Printf("ERROR: Failed to get file from form: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Message: "No file provided: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// Get folder from query parameter (optional - empty string means no folder)
	folder := r.URL.Query().Get("folder")

	// Log upload attempt
	log.Printf("Uploading file: %s, size: %d bytes, type: %s, folder: %s", 
		fileHeader.Filename, fileHeader.Size, fileHeader.Header.Get("Content-Type"), folder)

	// Upload to Cloudinary - pass file stream directly (NOT fileHeader.Open again!)
	url, err := cloudinaryService.UploadFile(r.Context(), file, fileHeader, folder)
	if err != nil {
		log.Printf("ERROR: Cloudinary upload failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Message: "Failed to upload file: " + err.Error(),
		})
		return
	}

	if url == "" {
		log.Printf("ERROR: Cloudinary returned empty URL")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Message: "Upload succeeded but no URL returned",
		})
		return
	}

	log.Printf("✅ File uploaded successfully to Cloudinary: %s", url)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UploadResponse{
		Success: true,
		Message: "File uploaded successfully",
		URL:     url,
	})
}

