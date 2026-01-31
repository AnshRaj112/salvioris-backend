package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/cloudinary/cloudinary-go/v2"
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

func (s *CloudinaryService) UploadFile(ctx context.Context, file multipart.File, folder string) (string, error) {
	// Read file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Upload to Cloudinary
	uploadResult, err := s.cld.Upload.Upload(ctx, fileBytes, uploader.UploadParams{
		Folder:   folder,
		ResourceType: "auto", // Automatically detect image, video, or raw
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to Cloudinary: %w", err)
	}

	return uploadResult.SecureURL, nil
}

func (s *CloudinaryService) UploadFileFromHeader(ctx context.Context, fileHeader *multipart.FileHeader, folder string) (string, error) {
	// Open the file
	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return s.UploadFile(ctx, file, folder)
}

