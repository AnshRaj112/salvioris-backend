package services

import (
	"context"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

const (
	therapistAuthKeyPrefix = "therapist_auth:"
	therapistAuthTTL       = 7 * 24 * time.Hour // 7 days — matches session and JWT duration
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
// It also slides the TTL — every successful lookup extends the cache entry by 7 days.
func GetTherapistAuthCache(sessionToken string) (uuid.UUID, bool) {
	if sessionToken == "" || database.RedisClient == nil {
		return uuid.Nil, false
	}
	ctx := context.Background()
	key := therapistAuthKeyPrefix + sessionToken
	val, err := database.RedisClient.Get(ctx, key).Result()
	if err != nil {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, false
	}
	// Slide the TTL on every successful use
	database.RedisClient.Expire(ctx, key, therapistAuthTTL)
	return id, true
}

// ClearTherapistAuthCache removes cached therapist auth for a session.
func ClearTherapistAuthCache(sessionToken string) {
	if sessionToken == "" || database.RedisClient == nil {
		return
	}
	_ = database.RedisClient.Del(context.Background(), therapistAuthKeyPrefix+sessionToken).Err()
}
