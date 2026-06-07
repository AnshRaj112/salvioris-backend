package services

import (
	"context"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

const (
	therapistAuthKeyPrefix = "therapist_auth:"
	therapistAuthTTL       = 30 * time.Minute
)

// SetTherapistAuthCache marks a session token as an approved therapist session.
func SetTherapistAuthCache(sessionToken string, therapistID uuid.UUID) {
	if sessionToken == "" || database.RedisClient == nil {
		return
	}
	_ = database.RedisClient.Set(
		context.Background(),
		therapistAuthKeyPrefix+sessionToken,
		therapistID.String(),
		therapistAuthTTL,
	).Err()
}

// GetTherapistAuthCache returns a cached therapist ID for a session token.
func GetTherapistAuthCache(sessionToken string) (uuid.UUID, bool) {
	if sessionToken == "" || database.RedisClient == nil {
		return uuid.Nil, false
	}
	val, err := database.RedisClient.Get(context.Background(), therapistAuthKeyPrefix+sessionToken).Result()
	if err != nil {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// ClearTherapistAuthCache removes cached therapist auth for a session.
func ClearTherapistAuthCache(sessionToken string) {
	if sessionToken == "" || database.RedisClient == nil {
		return
	}
	_ = database.RedisClient.Del(context.Background(), therapistAuthKeyPrefix+sessionToken).Err()
}
