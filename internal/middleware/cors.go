package middleware

import (
	"net/http"
	"strings"
)

// allowedOrigin returns the request's Origin if it is in the allowed list (case-insensitive), else "".
func allowedOrigin(r *http.Request, allowed []string) string {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return ""
	}
	originLower := strings.ToLower(origin)
	for _, a := range allowed {
		if strings.TrimSpace(strings.ToLower(a)) == originLower {
			return origin
		}
	}
	return ""
}

// CORS sets CORS headers and responds to OPTIONS with 200 so preflight never gets 403.
// allowedOrigins is the list of allowed origins (e.g. https://www.salvioris.com, http://localhost:3000).
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := allowedOrigin(r, allowedOrigins)
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, X-Requested-With")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "300")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
