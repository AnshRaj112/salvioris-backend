package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AnshRaj112/serenify-backend/pkg/clientip"
	"golang.org/x/time/rate"
)

// Chat history rate limit: per-IP, different limits for auth vs anonymous.
// Auth: 30 req/min, burst 20. Anonymous: 10 req/min, burst 5.
// Prevents 429 from rapid group switching while blocking abuse.

const (
	chatHistoryAuthRPS    = 0.5  // 30/min
	chatHistoryAuthBurst  = 20
	chatHistoryAnonRPS    = 0.17 // ~10/min
	chatHistoryAnonBurst  = 5
	chatHistoryCleanupMin = 5 * time.Minute
	chatHistoryLimiterTTL = 30 * time.Minute
)

type chatLimiterEntry struct {
	limiter *rate.Limiter
	lastUse time.Time
}

var (
	chatHistoryEntries   = make(map[string]*chatLimiterEntry)
	chatHistoryEntriesMu sync.Mutex
	chatHistoryCleanup   bool
)

func getChatHistoryLimiter(ip string, authenticated bool) *rate.Limiter {
	key := ip
	if authenticated {
		key = "auth:" + ip
	} else {
		key = "anon:" + ip
	}

	chatHistoryEntriesMu.Lock()
	defer chatHistoryEntriesMu.Unlock()
	startChatHistoryCleanupOnce()

	e, ok := chatHistoryEntries[key]
	if !ok {
		if authenticated {
			e = &chatLimiterEntry{
				limiter: rate.NewLimiter(rate.Limit(chatHistoryAuthRPS), chatHistoryAuthBurst),
				lastUse: time.Now(),
			}
		} else {
			e = &chatLimiterEntry{
				limiter: rate.NewLimiter(rate.Limit(chatHistoryAnonRPS), chatHistoryAnonBurst),
				lastUse: time.Now(),
			}
		}
		chatHistoryEntries[key] = e
	}
	e.lastUse = time.Now()
	return e.limiter
}

func startChatHistoryCleanupOnce() {
	if chatHistoryCleanup {
		return
	}
	chatHistoryCleanup = true
	go func() {
		ticker := time.NewTicker(chatHistoryCleanupMin)
		defer ticker.Stop()
		for range ticker.C {
			chatHistoryEntriesMu.Lock()
			now := time.Now()
			for k, e := range chatHistoryEntries {
				if now.Sub(e.lastUse) > chatHistoryLimiterTTL {
					delete(chatHistoryEntries, k)
				}
			}
			chatHistoryEntriesMu.Unlock()
		}
	}()
}

// isAuthenticated checks for Bearer token in Authorization header.
func chatHistoryIsAuthenticated(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	return strings.HasPrefix(auth, "Bearer ") && len(strings.TrimPrefix(auth, "Bearer ")) > 0
}

// ChatHistoryRateLimit applies rate limiting only to GET /api/chat/history.
// Auth: 30/min burst 20. Anonymous: 10/min burst 5. Returns 429 with headers when exceeded.
func ChatHistoryRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/api/chat/history") {
			next.ServeHTTP(w, r)
			return
		}

		ip := clientip.RealClientIP(r)
		auth := chatHistoryIsAuthenticated(r)
		limiter := getChatHistoryLimiter(ip, auth)

		if !limiter.Allow() {
			limit := chatHistoryAnonBurst
			if auth {
				limit = chatHistoryAuthBurst
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"success":false,"message":"Too many chat history requests. Please slow down."}`))
			return
		}

		limit := chatHistoryAnonBurst
		if auth {
			limit = chatHistoryAuthBurst
		}
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(limit-1)) // Best-effort for debugging
		next.ServeHTTP(w, r)
	})
}
