package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
)

const (
	// CacheKeyPrefix is the Redis key prefix for cached data
	CacheKeyPrefix = "cache:"
	// DefaultCacheTTL is 6-12 hours (we'll use 8 hours as default)
	DefaultCacheTTL = 8 * time.Hour
	// MinCacheTTL is 6 hours
	MinCacheTTL = 6 * time.Hour
	// MaxCacheTTL is 12 hours
	MaxCacheTTL = 12 * time.Hour
)

// CacheService provides caching functionality for important data
type CacheService struct{}

// Get retrieves a value from cache
func (c *CacheService) Get(key string, dest interface{}) (bool, error) {
	ctx := context.Background()
	cacheKey := CacheKeyPrefix + key

	val, err := database.RedisClient.Get(ctx, cacheKey).Result()
	if err != nil {
		return false, nil // Cache miss, not an error
	}

	// Unmarshal JSON
	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return false, err
	}

	return true, nil
}

// Set stores a value in cache with default TTL
func (c *CacheService) Set(key string, value interface{}) error {
	return c.SetWithTTL(key, value, DefaultCacheTTL)
}

// SetWithTTL stores a value in cache with custom TTL (clamped to 6-12 hours)
func (c *CacheService) SetWithTTL(key string, value interface{}, ttl time.Duration) error {
	// Clamp TTL to 6-12 hours
	if ttl < MinCacheTTL {
		ttl = MinCacheTTL
	}
	if ttl > MaxCacheTTL {
		ttl = MaxCacheTTL
	}

	ctx := context.Background()
	cacheKey := CacheKeyPrefix + key

	// Marshal to JSON
	jsonData, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return database.RedisClient.Set(ctx, cacheKey, jsonData, ttl).Err()
}

// Delete removes a value from cache
func (c *CacheService) Delete(key string) error {
	ctx := context.Background()
	cacheKey := CacheKeyPrefix + key
	return database.RedisClient.Del(ctx, cacheKey).Err()
}

// Exists checks if a key exists in cache
func (c *CacheService) Exists(key string) (bool, error) {
	ctx := context.Background()
	cacheKey := CacheKeyPrefix + key
	count, err := database.RedisClient.Exists(ctx, cacheKey).Result()
	return count > 0, err
}

// GetString retrieves a string value from cache
func (c *CacheService) GetString(key string) (string, bool, error) {
	ctx := context.Background()
	cacheKey := CacheKeyPrefix + key

	val, err := database.RedisClient.Get(ctx, cacheKey).Result()
	if err != nil {
		return "", false, nil // Cache miss
	}

	return val, true, nil
}

// SetString stores a string value in cache
func (c *CacheService) SetString(key string, value string) error {
	return c.SetStringWithTTL(key, value, DefaultCacheTTL)
}

// SetStringWithTTL stores a string value in cache with custom TTL
func (c *CacheService) SetStringWithTTL(key string, value string, ttl time.Duration) error {
	// Clamp TTL to 6-12 hours
	if ttl < MinCacheTTL {
		ttl = MinCacheTTL
	}
	if ttl > MaxCacheTTL {
		ttl = MaxCacheTTL
	}

	ctx := context.Background()
	cacheKey := CacheKeyPrefix + key
	return database.RedisClient.Set(ctx, cacheKey, value, ttl).Err()
}

// CacheKey generates a cache key for a specific resource
func CacheKey(resource string, identifier string) string {
	return fmt.Sprintf("%s:%s", resource, identifier)
}

// Global cache service instance
var Cache = &CacheService{}

