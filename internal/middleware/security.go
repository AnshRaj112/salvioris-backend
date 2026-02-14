package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/AnshRaj112/serenify-backend/pkg/clientip"
	"golang.org/x/time/rate"
)

const (
	headerXContentTypeOptions        = "X-Content-Type-Options"
	headerXFrameOptions               = "X-Frame-Options"
	headerXXSSProtection              = "X-XSS-Protection"
	headerContentSecurityPolicy       = "Content-Security-Policy"
	headerStrictTransportSecurity    = "Strict-Transport-Security"
)

// SecurityHeaders sets security-related response headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerXContentTypeOptions, "nosniff")
		w.Header().Set(headerXFrameOptions, "DENY")
		w.Header().Set(headerXXSSProtection, "1; mode=block")
		w.Header().Set(headerContentSecurityPolicy, "default-src 'self'")
		w.Header().Set(headerStrictTransportSecurity, "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// HostCheck returns 403 when r.Host does not match allowedHost (e.g. backend.salvioris.com).
// Allows OPTIONS (preflight) through. Trusts X-Forwarded-Host when it matches allowedHost (proxies).
func HostCheck(allowedHost string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowedHost == "" {
				next.ServeHTTP(w, r)
				return
			}
			// Let preflight through so CORS works
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			normalize := func(h string) string {
				h = strings.TrimSpace(h)
				if host, _, err := net.SplitHostPort(h); err == nil {
					h = host
				}
				return strings.ToLower(h)
			}
			reqHost := normalize(r.Host)
			if reqHost == strings.ToLower(strings.TrimSpace(allowedHost)) {
				next.ServeHTTP(w, r)
				return
			}
			// Behind proxy: Cloudflare/Render may send original host in X-Forwarded-Host (use first if multiple)
			if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
				if idx := strings.Index(fwd, ","); idx != -1 {
					fwd = strings.TrimSpace(fwd[:idx])
				} else {
					fwd = strings.TrimSpace(fwd)
				}
				if normalize(fwd) == strings.ToLower(strings.TrimSpace(allowedHost)) {
					next.ServeHTTP(w, r)
					return
				}
			}
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Forbidden"))
		})
	}
}

// --- Global rate limiting (per-IP, 1/s, burst 10) ---

var (
	globalEntries   = make(map[string]*limiterEntry)
	globalEntriesMu sync.Mutex
	globalCleanupRun bool
)

const (
	globalRateLimitRPS   = 1
	globalRateLimitBurst = 10
	globalCleanupInterval = 5 * time.Minute
	globalLimiterTTL      = 30 * time.Minute
)

type limiterEntry struct {
	limiter *rate.Limiter
	lastUse time.Time
}

func getGlobalLimiter(ip string) *rate.Limiter {
	globalEntriesMu.Lock()
	defer globalEntriesMu.Unlock()
	startGlobalCleanupOnce()
	e, ok := globalEntries[ip]
	if !ok {
		e = &limiterEntry{
			limiter: rate.NewLimiter(rate.Limit(globalRateLimitRPS), globalRateLimitBurst),
			lastUse: time.Now(),
		}
		globalEntries[ip] = e
	}
	e.lastUse = time.Now()
	return e.limiter
}

func startGlobalCleanupOnce() {
	if globalCleanupRun {
		return
	}
	globalCleanupRun = true
	go func() {
		ticker := time.NewTicker(globalCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			globalEntriesMu.Lock()
			now := time.Now()
			for ip, e := range globalEntries {
				if now.Sub(e.lastUse) > globalLimiterTTL {
					delete(globalEntries, ip)
				}
			}
			globalEntriesMu.Unlock()
		}
	}()
}

// GlobalRateLimit limits each IP to 1 req/s, burst 10. Returns 429 when exceeded.
func GlobalRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientip.RealClientIP(r)
		if !getGlobalLimiter(ip).Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"success":false,"message":"Too many requests. Please slow down."}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Login route rate limiting (1 req/5s, burst 2) ---

var (
	loginEntries    = make(map[string]*limiterEntry)
	loginEntriesMu  sync.Mutex
	loginCleanupRun bool
)

const (
	loginRateLimitEvery  = 5 * time.Second
	loginRateLimitBurst  = 2
	loginCleanupInterval = 5 * time.Minute
	loginLimiterTTL      = 30 * time.Minute
)

var loginPaths = map[string]bool{
	"/api/auth/signin":           true,
	"/api/auth/user/signin":      true,
	"/api/auth/therapist/signin": true,
	"/api/admin/signin":          true,
}

func getLoginLimiter(ip string) *rate.Limiter {
	loginEntriesMu.Lock()
	defer loginEntriesMu.Unlock()
	startLoginCleanupOnce()
	e, ok := loginEntries[ip]
	if !ok {
		e = &limiterEntry{
			limiter: rate.NewLimiter(rate.Every(loginRateLimitEvery), loginRateLimitBurst),
			lastUse: time.Now(),
		}
		loginEntries[ip] = e
	}
	e.lastUse = time.Now()
	return e.limiter
}

func startLoginCleanupOnce() {
	if loginCleanupRun {
		return
	}
	loginCleanupRun = true
	go func() {
		ticker := time.NewTicker(loginCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			loginEntriesMu.Lock()
			now := time.Now()
			for ip, e := range loginEntries {
				if now.Sub(e.lastUse) > loginLimiterTTL {
					delete(loginEntries, ip)
				}
			}
			loginEntriesMu.Unlock()
		}
	}()
}

// LoginRateLimit applies stricter limit to sign-in routes only. Use after GlobalRateLimit.
func LoginRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loginPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		ip := clientip.RealClientIP(r)
		if !getLoginLimiter(ip).Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"success":false,"message":"Too many login attempts. Please try again later."}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ProductionSecurity returns middlewares for production: SecurityHeaders → HostCheck → GlobalRateLimit → LoginRateLimit.
func ProductionSecurity(allowedHost string) []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		SecurityHeaders,
		HostCheck(allowedHost),
		GlobalRateLimit,
		LoginRateLimit,
	}
}
