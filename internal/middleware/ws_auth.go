package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
)

// AuthenticateWebSocket verifies one-time-use tokens passed in WS handshakes to mitigate session sniffing
func AuthenticateWebSocket(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("ws_token")
		if token == "" {
			http.Error(w, "Unauthorized: Missing WS session validation token", http.StatusUnauthorized)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		// Retrieve one-time token from Redis to enforce single-use replay protection
		userSessionKey := "ws_otp:" + token
		val, err := database.RedisClient.Get(ctx, userSessionKey).Result()
		if err != nil || val == "" {
			database.TriggerAuditEvent("WS_SESSION_REPLAY_ATTEMPT", "WS_GATEWAY", "unknown", "guest", "Attempted login with stale/missing WS token", r)
			http.Error(w, "Unauthorized: Stale token session signature", http.StatusUnauthorized)
			return
		}
		
		// Immediately burn token to prevent replay attacks
		database.RedisClient.Del(ctx, userSessionKey)

		// Parse token details (format: user_id:role)
		parts := strings.Split(val, ":")
		if len(parts) != 2 {
			http.Error(w, "Unauthorized: Malformed session payload", http.StatusUnauthorized)
			return
		}

		rCtx := context.WithValue(r.Context(), UserIDKey, parts[0])
		rCtx = context.WithValue(rCtx, UserRoleKey, parts[1])

		next.ServeHTTP(w, r.WithContext(rCtx))
	})
}
