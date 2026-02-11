package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

const (
	// AdminSessionDuration is 7 days
	AdminSessionDuration = 7 * 24 * time.Hour
	// AdminSessionKeyPrefix is the Redis key prefix for admin sessions
	AdminSessionKeyPrefix = "admin_session:"
	// AdminToSessionKeyPrefix is the Redis key prefix for admin->session mapping
	AdminToSessionKeyPrefix = "admin_to_session:"
)

// CreateAdminSession creates a new session for an admin and stores it in Redis.
// If admin already has a session, it invalidates the old one and creates a new one.
// Returns the session token.
func CreateAdminSession(adminID uuid.UUID) (string, error) {
	// Invalidate any existing session for this admin (so 7-day timer resets)
	_ = InvalidateAdminSessions(adminID)

	// Generate secure session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	sessionToken := base64.URLEncoding.EncodeToString(tokenBytes)

	ctx := context.Background()
	sessionKey := AdminSessionKeyPrefix + sessionToken
	adminToSessionKey := AdminToSessionKeyPrefix + adminID.String()

	// Store session with 7-day expiration
	if err := database.RedisClient.Set(ctx, sessionKey, adminID.String(), AdminSessionDuration).Err(); err != nil {
		return "", err
	}

	// Store admin->session mapping
	if err := database.RedisClient.Set(ctx, adminToSessionKey, sessionToken, AdminSessionDuration).Err(); err != nil {
		return "", err
	}

	return sessionToken, nil
}

// ValidateAdminSession checks if a session token is valid and returns the admin ID.
func ValidateAdminSession(sessionToken string) (uuid.UUID, bool, error) {
	if sessionToken == "" {
		return uuid.Nil, false, nil
	}

	ctx := context.Background()
	sessionKey := AdminSessionKeyPrefix + sessionToken

	adminIDStr, err := database.RedisClient.Get(ctx, sessionKey).Result()
	if err != nil {
		return uuid.Nil, false, nil
	}

	adminID, err := uuid.Parse(adminIDStr)
	if err != nil {
		return uuid.Nil, false, err
	}

	return adminID, true, nil
}

// RefreshAdminSession extends the session expiration by 7 days from now.
func RefreshAdminSession(sessionToken string) error {
	if sessionToken == "" {
		return fmt.Errorf("session token is empty")
	}

	ctx := context.Background()
	sessionKey := AdminSessionKeyPrefix + sessionToken

	adminIDStr, err := database.RedisClient.Get(ctx, sessionKey).Result()
	if err != nil {
		return err
	}

	adminID, err := uuid.Parse(adminIDStr)
	if err != nil {
		return err
	}

	adminToSessionKey := AdminToSessionKeyPrefix + adminID.String()

	// Extend both keys by 7 days from now
	if err := database.RedisClient.Expire(ctx, sessionKey, AdminSessionDuration).Err(); err != nil {
		return err
	}
	if err := database.RedisClient.Expire(ctx, adminToSessionKey, AdminSessionDuration).Err(); err != nil {
		return err
	}

	return nil
}

// InvalidateAdminSession removes a session from Redis.
func InvalidateAdminSession(sessionToken string) error {
	if sessionToken == "" {
		return nil
	}

	ctx := context.Background()
	sessionKey := AdminSessionKeyPrefix + sessionToken

	// Get admin ID before deleting
	adminIDStr, err := database.RedisClient.Get(ctx, sessionKey).Result()
	if err == nil && adminIDStr != "" {
		adminToSessionKey := AdminToSessionKeyPrefix + adminIDStr
		_ = database.RedisClient.Del(ctx, adminToSessionKey).Err()
	}

	// Delete session
	return database.RedisClient.Del(ctx, sessionKey).Err()
}

// InvalidateAdminSessions invalidates all sessions for an admin.
func InvalidateAdminSessions(adminID uuid.UUID) error {
	ctx := context.Background()
	adminToSessionKey := AdminToSessionKeyPrefix + adminID.String()

	// Get current session token
	sessionToken, err := database.RedisClient.Get(ctx, adminToSessionKey).Result()
	if err == nil && sessionToken != "" {
		sessionKey := AdminSessionKeyPrefix + sessionToken
		_ = database.RedisClient.Del(ctx, sessionKey).Err()
	}

	// Delete admin->session mapping
	return database.RedisClient.Del(ctx, adminToSessionKey).Err()
}


