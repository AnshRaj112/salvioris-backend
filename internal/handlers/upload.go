package handlers

import (
	"encoding/json"
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
	if cloudinaryService == nil {
		http.Error(w, "Cloudinary service not initialized", http.StatusInternalServerError)
		return
	}

	// Parse multipart form (max 10MB)
	err := r.ParseMultipartForm(10 << 20) // 10MB
	if err != nil {
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get file from form
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get folder from query parameter (default: "serenify")
	folder := r.URL.Query().Get("folder")
	if folder == "" {
		folder = "serenify"
	}

	// Upload to Cloudinary
	url, err := cloudinaryService.UploadFileFromHeader(r.Context(), fileHeader, folder)
	if err != nil {
		http.Error(w, "Failed to upload file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UploadResponse{
		Success: true,
		Message: "File uploaded successfully",
		URL:     url,
	})
}

