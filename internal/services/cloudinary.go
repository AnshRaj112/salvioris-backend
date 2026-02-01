package services

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

type CloudinaryService struct {
	cld *cloudinary.Cloudinary
}

func NewCloudinaryService(cloudName, apiKey, apiSecret string) (*CloudinaryService, error) {
	cld, err := cloudinary.NewFromParams(cloudName, apiKey, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Cloudinary: %w", err)
	}

	return &CloudinaryService{
		cld: cld,
	}, nil
}

// UploadFile uploads a file stream directly to Cloudinary (no byte conversion)
func (s *CloudinaryService) UploadFile(
	ctx context.Context,
	file multipart.File,
	fileHeader *multipart.FileHeader,
	folder string,
) (string, error) {
	// Log file info
	log.Printf("Uploading file: %s, size: %d bytes", fileHeader.Filename, fileHeader.Size)

	// Build upload params using Cloudinary API helpers
	params := uploader.UploadParams{
		ResourceType:   "auto", // Automatically detect image, video, or raw
		UseFilename:    api.Bool(true),
		UniqueFilename: api.Bool(true),
	}

	// Only set folder if provided
	if folder != "" {
		params.Folder = folder
		log.Printf("Uploading to folder: %s", folder)
	} else {
		log.Printf("Uploading to root (no folder)")
	}

	// Upload file stream directly to Cloudinary (NOT bytes!)
	log.Printf("Calling Cloudinary Upload.Upload with file stream...")
	uploadResult, err := s.cld.Upload.Upload(ctx, file, params)
	if err != nil {
		log.Printf("ERROR: Cloudinary upload failed: %v", err)
		return "", fmt.Errorf("cloudinary upload failed: %w", err)
	}

	log.Printf("Cloudinary upload response received")
	log.Printf("  PublicID: %s", uploadResult.PublicID)
	log.Printf("  SecureURL: %s", uploadResult.SecureURL)
	log.Printf("  URL: %s", uploadResult.URL)

	if uploadResult.SecureURL == "" {
		log.Printf("ERROR: Upload succeeded but SecureURL is empty")
		if uploadResult.URL != "" {
			log.Printf("Using non-secure URL instead: %s", uploadResult.URL)
			return uploadResult.URL, nil
		}
		return "", fmt.Errorf("cloudinary returned empty secure_url")
	}

	log.Printf("✅ Successfully uploaded to Cloudinary: %s", uploadResult.SecureURL)
	return uploadResult.SecureURL, nil
}

