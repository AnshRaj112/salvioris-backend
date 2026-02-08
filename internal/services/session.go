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
	// SessionDuration is 7 days
	SessionDuration = 7 * 24 * time.Hour
	// SessionKeyPrefix is the Redis key prefix for sessions
	SessionKeyPrefix = "session:"
	// UserSessionKeyPrefix is the Redis key prefix for user->session mapping
	UserSessionKeyPrefix = "user_session:"
)

// CreateSession creates a new session for a user and stores it in Redis
// If user already has a session, it invalidates the old one and creates a new one
// This ensures the 7-day timer resets from the current login
// Returns the session token
func CreateSession(userID uuid.UUID) (string, error) {
	// Invalidate any existing session for this user (so 7-day timer resets)
	InvalidateUserSessions(userID)

	// Generate secure session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	sessionToken := base64.URLEncoding.EncodeToString(tokenBytes)

	ctx := context.Background()
	sessionKey := SessionKeyPrefix + sessionToken
	userSessionKey := UserSessionKeyPrefix + userID.String()

	// Store session with 7-day expiration
	err := database.RedisClient.Set(ctx, sessionKey, userID.String(), SessionDuration).Err()
	if err != nil {
		return "", err
	}

	// Store user->session mapping
	err = database.RedisClient.Set(ctx, userSessionKey, sessionToken, SessionDuration).Err()
	if err != nil {
		return "", err
	}

	return sessionToken, nil
}

// ValidateSession checks if a session token is valid and returns the user ID
func ValidateSession(sessionToken string) (uuid.UUID, bool, error) {
	if sessionToken == "" {
		return uuid.Nil, false, nil
	}

	ctx := context.Background()
	sessionKey := SessionKeyPrefix + sessionToken

	// Get user ID from session
	userIDStr, err := database.RedisClient.Get(ctx, sessionKey).Result()
	if err != nil {
		return uuid.Nil, false, nil
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, false, err
	}

	return userID, true, nil
}

// RefreshSession extends the session expiration by 7 days from now
// This is called when user logs in again - resets the 7-day timer
func RefreshSession(sessionToken string) error {
	if sessionToken == "" {
		return fmt.Errorf("session token is empty")
	}

	ctx := context.Background()
	sessionKey := SessionKeyPrefix + sessionToken

	// Check if session exists
	userIDStr, err := database.RedisClient.Get(ctx, sessionKey).Result()
	if err != nil {
		return err
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return err
	}

	userSessionKey := UserSessionKeyPrefix + userID.String()

	// Extend both keys by 7 days from now
	err = database.RedisClient.Expire(ctx, sessionKey, SessionDuration).Err()
	if err != nil {
		return err
	}

	err = database.RedisClient.Expire(ctx, userSessionKey, SessionDuration).Err()
	if err != nil {
		return err
	}

	return nil
}

// InvalidateSession removes a session from Redis
func InvalidateSession(sessionToken string) error {
	if sessionToken == "" {
		return nil
	}

	ctx := context.Background()
	sessionKey := SessionKeyPrefix + sessionToken

	// Get user ID before deleting
	userIDStr, err := database.RedisClient.Get(ctx, sessionKey).Result()
	if err == nil && userIDStr != "" {
		userSessionKey := UserSessionKeyPrefix + userIDStr
		database.RedisClient.Del(ctx, userSessionKey)
	}

	// Delete session
	return database.RedisClient.Del(ctx, sessionKey).Err()
}

// InvalidateUserSessions invalidates all sessions for a user (useful when password changes)
func InvalidateUserSessions(userID uuid.UUID) error {
	ctx := context.Background()
	userSessionKey := UserSessionKeyPrefix + userID.String()

	// Get current session token
	sessionToken, err := database.RedisClient.Get(ctx, userSessionKey).Result()
	if err == nil && sessionToken != "" {
		sessionKey := SessionKeyPrefix + sessionToken
		database.RedisClient.Del(ctx, sessionKey)
	}

	// Delete user session mapping
	return database.RedisClient.Del(ctx, userSessionKey).Err()
}

