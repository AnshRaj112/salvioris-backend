package services

import (
	"context"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

const (
	receptionistAuthKeyPrefix = "receptionist_auth:"
	receptionistAuthTTL       = 7 * 24 * time.Hour
)

// SetReceptionistAuthCache marks a session token as a valid receptionist session.
func SetReceptionistAuthCache(sessionToken string, receptionistID uuid.UUID) {
	if sessionToken == "" || database.RedisClient == nil {
		return
	}
	_ = database.RedisClient.Set(
		context.Background(),
		receptionistAuthKeyPrefix+sessionToken,
		receptionistID.String(),
		receptionistAuthTTL,
	).Err()
}

// GetReceptionistAuthCache returns the cached receptionist ID for a token.
// Slides the TTL on every successful lookup.
func GetReceptionistAuthCache(sessionToken string) (uuid.UUID, bool) {
	if sessionToken == "" || database.RedisClient == nil {
		return uuid.Nil, false
	}
	ctx := context.Background()
	key := receptionistAuthKeyPrefix + sessionToken
	val, err := database.RedisClient.Get(ctx, key).Result()
	if err != nil {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, false
	}
	database.RedisClient.Expire(ctx, key, receptionistAuthTTL)
	return id, true
}

// ClearReceptionistAuthCache removes a receptionist's cached auth token.
func ClearReceptionistAuthCache(sessionToken string) {
	if sessionToken == "" || database.RedisClient == nil {
		return
	}
	_ = database.RedisClient.Del(context.Background(), receptionistAuthKeyPrefix+sessionToken).Err()
}
