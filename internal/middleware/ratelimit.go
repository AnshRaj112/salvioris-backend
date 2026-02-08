package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/services"
)

const (
	// RateLimitWindow is 120 seconds
	RateLimitWindow = 120 * time.Second
	// RateLimitMaxRequests is the maximum number of requests allowed in the window
	RateLimitMaxRequests = 25 // Block after 25 requests (between 20-30 as requested)
	// RateLimitKeyPrefix is the Redis key prefix for rate limiting
	RateLimitKeyPrefix = "ratelimit:"
	// BlockedIPKeyPrefix is the Redis key prefix for blocked IPs
	BlockedIPKeyPrefix = "blocked_ip:"
	// BlockedIPDuration is how long an IP stays blocked (24 hours)
	BlockedIPDuration = 24 * time.Hour
)

// RateLimitMiddleware provides rate limiting with IP blocking
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ipAddress := services.GetIPAddress(r)
		
		// Check if IP is already blocked
		ctx := context.Background()
		blockedKey := BlockedIPKeyPrefix + ipAddress
		isBlocked, err := database.RedisClient.Exists(ctx, blockedKey).Result()
		if err == nil && isBlocked > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"success":false,"message":"Your IP has been temporarily blocked due to excessive requests. Please try again later."}`))
			return
		}

		// Rate limit key
		rateLimitKey := RateLimitKeyPrefix + ipAddress

		// Get current request count
		currentCount, err := database.RedisClient.Get(ctx, rateLimitKey).Int()
		if err != nil {
			// Key doesn't exist, start with 1
			currentCount = 0
		}

		// Increment counter
		newCount := currentCount + 1

		// Set or update the counter with TTL
		if currentCount == 0 {
			// First request in this window
			err = database.RedisClient.Set(ctx, rateLimitKey, "1", RateLimitWindow).Err()
		} else {
			// Increment existing counter
			err = database.RedisClient.Incr(ctx, rateLimitKey).Err()
			if err == nil {
				// Refresh TTL
				database.RedisClient.Expire(ctx, rateLimitKey, RateLimitWindow)
			}
		}

		if err != nil {
			// If Redis fails, allow the request (fail open)
			next.ServeHTTP(w, r)
			return
		}

		// Check if limit exceeded
		if newCount > RateLimitMaxRequests {
			// Block the IP
			blockErr := database.RedisClient.Set(ctx, blockedKey, "1", BlockedIPDuration).Err()
			if blockErr == nil {
				// Also record in database for admin visibility
				// This is handled by the existing moderation service
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(fmt.Sprintf(`{"success":false,"message":"Rate limit exceeded. Your IP has been temporarily blocked. Please try again later.","retry_after":%d}`, int(RateLimitWindow.Seconds()))))
			return
		}

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(RateLimitMaxRequests))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(RateLimitMaxRequests-newCount))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(RateLimitWindow).Unix(), 10))

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

// UnblockIP removes an IP from the blocked list (admin function)
func UnblockIP(ipAddress string) error {
	ctx := context.Background()
	blockedKey := BlockedIPKeyPrefix + ipAddress
	return database.RedisClient.Del(ctx, blockedKey).Err()
}

// IsIPBlocked checks if an IP is currently blocked
func IsIPBlocked(ipAddress string) (bool, error) {
	ctx := context.Background()
	blockedKey := BlockedIPKeyPrefix + ipAddress
	count, err := database.RedisClient.Exists(ctx, blockedKey).Result()
	return count > 0, err
}

