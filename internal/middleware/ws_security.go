package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AnshRaj112/serenify-backend/pkg/clientip"
)

// WSTokenBucket represents a token bucket rate limiter for a specific client connection.
type WSTokenBucket struct {
	tokens float64
	last   time.Time
	mu     sync.Mutex
}

var (
	wsRateBuckets = make(map[string]*WSTokenBucket)
	wsBucketsMu   sync.Mutex
)

// WSSecurityMiddleware enforces security constraints on WebSocket upgrade requests.
func WSSecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Enforce TLS / Secure WebSockets (WSS) in production.
		// We inspect the X-Forwarded-Proto header set by load balancers, or the request's native TLS status.
		env := os.Getenv("ENV")
		if env == "production" {
			proto := r.Header.Get("X-Forwarded-Proto")
			if proto != "https" && r.TLS == nil {
				http.Error(w, "WebSocket upgrade rejected: must run over secure WSS protocol", http.StatusBadRequest)
				return
			}
		}

		// 2. Strict Origin Validation
		// Ensures that malicious third-party websites cannot initiate a WebSocket connection to bypass CORS (CSWSH).
		origin := r.Header.Get("Origin")
		allowedOrigin := os.Getenv("FRONTEND_URL")
		if allowedOrigin == "" {
			allowedOrigin = "http://localhost:3000" // Fallback for local development
		}

		if origin != "" {
			// In production, enforce exact origin match or subdomain validation
			if env == "production" {
				if !strings.HasPrefix(origin, allowedOrigin) {
					http.Error(w, "WebSocket upgrade rejected: origin not authorized", http.StatusForbidden)
					return
				}
			} else {
				// Local dev origin validation fallback
				if !strings.Contains(origin, "localhost") && !strings.Contains(origin, "127.0.0.1") {
					http.Error(w, "WebSocket upgrade rejected: origin not authorized in development", http.StatusForbidden)
					return
				}
			}
		}

		// 3. Token-Bucket Connection Rate Limiting per IP
		// Restricts clients from spamming multiple rapid reconnect requests.
		ip := clientip.RealClientIP(r)
		wsBucketsMu.Lock()
		tb, exists := wsRateBuckets[ip]
		if !exists {
			// Max 10 burst connections, refilling 1 token every 5 seconds (0.2 tokens/sec)
			tb = &WSTokenBucket{tokens: 10, last: time.Now()}
			wsRateBuckets[ip] = tb
		}
		wsBucketsMu.Unlock()

		tb.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(tb.last).Seconds()
		tb.tokens += elapsed * 0.2 // refilled at 0.2 tokens/sec
		if tb.tokens > 10 {
			tb.tokens = 10
		}
		tb.last = now

		if tb.tokens < 1.0 {
			tb.mu.Unlock()
			http.Error(w, "Too Many WebSocket Upgrade Requests", http.StatusTooManyRequests)
			return
		}
		tb.tokens -= 1.0
		tb.mu.Unlock()

		// Inject client IP and rate-limiting metadata into context
		ctx := context.WithValue(r.Context(), "client_ip", ip)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
