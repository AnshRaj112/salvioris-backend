package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
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
	Success      bool                   `json:"success"`
	Message      string                 `json:"message"`
	Vent         map[string]interface{} `json:"vent,omitempty"`
	Warning      bool                   `json:"warning,omitempty"`
	Blocked      bool                   `json:"blocked,omitempty"`
	WarningCount int                    `json:"warning_count,omitempty"`
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
	var err error
	
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	// Get IP address
	ipAddress := services.GetIPAddress(r)

	// Check if IP is blocked
	var isBlocked bool
	isBlocked, _, err = services.IsIPBlocked(ipAddress)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CreateVentResponse{
			Success: false,
			Message: "Error checking access",
		})
		return
	}
	if isBlocked {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(CreateVentResponse{
			Success: false,
			Blocked: true,
			Message: "Your access has been temporarily restricted due to policy violations. Please contact support if you believe this is an error.",
		})
		return
	}

	// Check content for threats and self-harm
	hasThreat, hasSelfHarm, _ := services.CheckContent(req.Message)
	
	var userUUID *uuid.UUID
	if req.UserID != "" {
		parsedUUID, parseErr := uuid.Parse(req.UserID)
		if parseErr == nil {
			userUUID = &parsedUUID
		}
	}

	// Record violation if detected
	if hasThreat || hasSelfHarm {
		violationType := models.ViolationTypeThreat
		if hasSelfHarm {
			violationType = models.ViolationTypeSelfHarm
		}

		// Get violation count
		var violationCount int64
		violationCount, err = services.GetViolationCount(ipAddress)
		if err != nil {
			violationCount = 0
		}

		// Record the violation
		_ = services.RecordViolation(userUUID, ipAddress, violationType, req.Message, "", "warning")

		// If this is the 3rd violation (after 2 warnings), block the IP
		if violationCount >= 2 {
			// Block IP for 7 days
			reason := "Multiple content policy violations"
			if hasThreat {
				reason = "Threats against others detected"
			} else if hasSelfHarm {
				reason = "Self-harm content detected"
			}
			_ = services.BlockIP(ipAddress, reason, 7)
			
			// Record violation with blocked action
			_ = services.RecordViolation(userUUID, ipAddress, violationType, req.Message, "", "blocked")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(CreateVentResponse{
				Success: false,
				Blocked: true,
				Message: "Your message contains content that violates our policies. Your access has been temporarily restricted. If you need help, please contact support.",
			})
			return
		}

		// First or second violation - return warning
		warningMsg := "Your message contains content that may violate our community guidelines. "
		if hasThreat {
			warningMsg += "Threats against others are not permitted. "
		}
		if hasSelfHarm {
			warningMsg += "If you're experiencing thoughts of self-harm, please reach out to a mental health professional or crisis hotline. "
		}
		warningMsg += "Continued violations may result in temporary access restrictions."

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateVentResponse{
			Success:      false,
			Warning:      true,
			WarningCount: int(violationCount + 1),
			Message:      warningMsg,
		})
		return
	}

	// Only save to database if user is logged in
	// Guest messages pass moderation but are not saved (handled on frontend)
	if req.UserID == "" {
		// Guest message - moderation passed, return success but don't save
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(CreateVentResponse{
			Success: true,
			Message: "Message validated successfully",
			// No vent returned for guests - they handle storage locally
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create vent for logged-in user
	vent := models.Vent{
		ID:        primitive.NewObjectID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Message:   req.Message,
	}

	// Set user ID as string (UUID from PostgreSQL)
	// Store as string in MongoDB since user IDs are now UUIDs from PostgreSQL
	if req.UserID != "" {
		// Store user_id as string in MongoDB
		vent.UserIDString = req.UserID
	}

	// Insert vent into database
	_, err = database.DB.Collection("vents").InsertOne(ctx, vent)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CreateVentResponse{
			Success: false,
			Message: "Failed to create vent",
		})
		return
	}

	// Return vent with anonymous username only (privacy-first)
	ventMap := map[string]interface{}{
		"id":         vent.ID.Hex(),
		"message":    vent.Message,
		"created_at": vent.CreatedAt,
	}
	
	// Add anonymous username if user_id exists
	if vent.UserIDString != "" {
		username, _ := services.GetUsernameByID(vent.UserIDString)
		if username != "" {
			ventMap["username"] = username // Only return anonymous username
		}
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

	// Build cache key
	cacheKey := services.CacheKey("vents", fmt.Sprintf("%s:%d:%d", userID, limit, skip))
	
	// Try to get from cache
	var cachedResponse GetVentsResponse
	if found, err := services.Cache.Get(cacheKey, &cachedResponse); err == nil && found {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		json.NewEncoder(w).Encode(cachedResponse)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// For signed-in users: delete vents from previous days and only show today's vents
	var startOfTodayUTC time.Time
	if userID != "" {
		now := time.Now().UTC()
		startOfTodayUTC = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Delete all vents for this user that are from before today
		deleteFilter := bson.M{
			"created_at": bson.M{"$lt": startOfTodayUTC},
		}
		if _, err := uuid.Parse(userID); err == nil {
			deleteFilter["user_id_string"] = userID
		} else {
			userObjectID, err := primitive.ObjectIDFromHex(userID)
			if err == nil {
				deleteFilter["user_id"] = userObjectID
			}
		}
		_, _ = database.DB.Collection("vents").DeleteMany(ctx, deleteFilter)
	}

	// Build filter
	filter := bson.M{}
	if userID != "" {
		// Only return vents from today (after day change, previous days are already deleted above)
		filter["created_at"] = bson.M{"$gte": startOfTodayUTC}
		// Try to match either user_id_string (UUID) or user_id (ObjectID for backward compatibility)
		if _, err := uuid.Parse(userID); err == nil {
			filter["user_id_string"] = userID
		} else {
			userObjectID, err := primitive.ObjectIDFromHex(userID)
			if err == nil {
				filter["user_id"] = userObjectID
			}
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

	// Convert to response format (with anonymous usernames)
	ventMaps := make([]map[string]interface{}, 0, len(vents))
	for _, vent := range vents {
		ventMap := map[string]interface{}{
			"id":         vent.ID.Hex(),
			"message":    vent.Message,
			"created_at": vent.CreatedAt,
		}
		
		// Add anonymous username if user_id exists
		if vent.UserIDString != "" {
			username, _ := services.GetUsernameByID(vent.UserIDString)
			if username != "" {
				ventMap["username"] = username // Anonymous username only
			}
			// Don't expose user_id - only username for anonymity
		} else if vent.UserID != nil {
			// Legacy support - try to get username if it's a UUID string
			username, _ := services.GetUsernameByID(vent.UserID.Hex())
			if username != "" {
				ventMap["username"] = username
			}
		}
		
		ventMaps = append(ventMaps, ventMap)
	}

	// Check if there are more vents
	hasMore := int64(skip+limit) < total

	response := GetVentsResponse{
		Success: true,
		Vents:   ventMaps,
		HasMore: hasMore,
		Total:   total,
	}

	// Cache the response (only cache if no user filter or if it's a common query)
	// Don't cache user-specific queries as they change frequently
	if userID == "" {
		services.Cache.Set(cacheKey, response)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(response)
}

