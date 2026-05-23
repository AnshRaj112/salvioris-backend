package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/AnshRaj112/serenify-backend/internal/database"
)

type contextKey string

const (
	UserRoleKey contextKey = "user_role"
	UserIDKey   contextKey = "user_id"
)

// RequireRole checks the authenticated request context for specific administrative/staff roles.
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			role, ok := ctx.Value(UserRoleKey).(string)
			if !ok || role == "" {
				database.TriggerAuditEvent("UNAUTHORIZED_ACCESS_ATTEMPT", "RBAC_GATEWAY", "unknown", "none", "Request blocked: missing active staff role context", r)
				http.Error(w, "Access Denied: Scoped authorization context missing", http.StatusForbidden)
				return
			}

			authorized := false
			for _, allowed := range allowedRoles {
				if strings.ToLower(role) == strings.ToLower(allowed) {
					authorized = true
					break
				}
			}

			if !authorized {
				actorID, _ := ctx.Value(UserIDKey).(string)
				database.TriggerAuditEvent("ACCESS_DENIED_INSUFFICIENT_PRIVILEGES", r.URL.Path, actorID, role, "Role "+role+" tried to access endpoint requiring "+strings.Join(allowedRoles, ","), r)
				http.Error(w, "Access Denied: Insufficient cryptographic privileges", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
