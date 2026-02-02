package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CreateVentRequest represents the request to create a vent message
type CreateVentRequest struct {
	Message string `json:"message"`
	UserID  string `json:"user_id,omitempty"` // Optional - for logged-in users
}

// CreateVentResponse represents the response after creating a vent
type CreateVentResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Vent    map[string]interface{} `json:"vent,omitempty"`
}

// GetVentsResponse represents the response for getting vents
type GetVentsResponse struct {
	Success bool                     `json:"success"`
	Message string                   `json:"message,omitempty"`
	Vents   []map[string]interface{} `json:"vents"`
	HasMore bool                     `json:"has_more"`
	Total   int64                    `json:"total"`
}

// CreateVent handles creating a new vent message
func CreateVent(w http.ResponseWriter, r *http.Request) {
	var req CreateVentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateVentResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate message
	if req.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateVentResponse{
			Success: false,
			Message: "Message is required",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create vent
	vent := models.Vent{
		ID:        primitive.NewObjectID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Message:   req.Message,
	}

	// Set user ID if provided
	if req.UserID != "" {
		userObjectID, err := primitive.ObjectIDFromHex(req.UserID)
		if err == nil {
			vent.UserID = &userObjectID
		}
	}

	// Insert vent into database
	_, err := database.DB.Collection("vents").InsertOne(ctx, vent)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CreateVentResponse{
			Success: false,
			Message: "Failed to create vent",
		})
		return
	}

	// Return vent (without sensitive data)
	ventMap := map[string]interface{}{
		"id":         vent.ID.Hex(),
		"message":    vent.Message,
		"created_at": vent.CreatedAt,
	}
	if vent.UserID != nil {
		ventMap["user_id"] = vent.UserID.Hex()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(CreateVentResponse{
		Success: true,
		Message: "Vent created successfully",
		Vent:    ventMap,
	})
}

// GetVents handles getting vent messages with pagination
func GetVents(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	userID := r.URL.Query().Get("user_id")
	limitStr := r.URL.Query().Get("limit")
	skipStr := r.URL.Query().Get("skip")

	// Parse limit (default: 20)
	limit := 20
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Parse skip (default: 0)
	skip := 0
	if skipStr != "" {
		if parsedSkip, err := strconv.Atoi(skipStr); err == nil && parsedSkip >= 0 {
			skip = parsedSkip
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Build filter
	filter := bson.M{}
	if userID != "" {
		userObjectID, err := primitive.ObjectIDFromHex(userID)
		if err == nil {
			filter["user_id"] = userObjectID
		}
	}

	// Count total vents for this user
	total, err := database.DB.Collection("vents").CountDocuments(ctx, filter)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetVentsResponse{
			Success: false,
			Vents:   []map[string]interface{}{},
			HasMore: false,
			Total:   0,
		})
		return
	}

	// Find vents with pagination (sorted by created_at descending - newest first)
	findOptions := options.Find()
	findOptions.SetSort(bson.M{"created_at": -1}) // Descending order
	findOptions.SetLimit(int64(limit))
	findOptions.SetSkip(int64(skip))

	cursor, err := database.DB.Collection("vents").Find(ctx, filter, findOptions)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetVentsResponse{
			Success: false,
			Vents:   []map[string]interface{}{},
			HasMore: false,
			Total:   0,
		})
		return
	}
	defer cursor.Close(ctx)

	var vents []models.Vent
	if err = cursor.All(ctx, &vents); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GetVentsResponse{
			Success: false,
			Vents:   []map[string]interface{}{},
			HasMore: false,
			Total:   0,
		})
		return
	}

	// Convert to response format
	ventMaps := make([]map[string]interface{}, 0, len(vents))
	for _, vent := range vents {
		ventMap := map[string]interface{}{
			"id":         vent.ID.Hex(),
			"message":    vent.Message,
			"created_at": vent.CreatedAt,
		}
		if vent.UserID != nil {
			ventMap["user_id"] = vent.UserID.Hex()
		}
		ventMaps = append(ventMaps, ventMap)
	}

	// Check if there are more vents
	hasMore := int64(skip+limit) < total

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetVentsResponse{
		Success: true,
		Vents:   ventMaps,
		HasMore: hasMore,
		Total:   total,
	})
}

