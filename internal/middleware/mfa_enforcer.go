package middleware

import (
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
)

// MFAEnforcer verifies that the current administrator session has completed hardware-backed FIDO2/MFA authentication.
func MFAEnforcer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		actorID, _ := ctx.Value(UserIDKey).(string)
		role, _ := ctx.Value(UserRoleKey).(string)

		// Enforce strictly for privileged staff/admin roles
		if role == "admin" || role == "moderator" || role == "compliance" {
			var mfaVerified bool
			var lastMfaAt time.Time

			err := database.PostgresDB.QueryRowContext(ctx, `
				SELECT mfa_verified, last_mfa_at 
				FROM staff_sessions 
				WHERE actor_id = $1 AND active = true
				ORDER BY last_mfa_at DESC LIMIT 1
			`, actorID).Scan(&mfaVerified, &lastMfaAt)

			if err != nil || !mfaVerified || time.Since(lastMfaAt) > 12*time.Hour {
				database.TriggerAuditEvent("PRIVILEGED_MFA_CHALLENGE_FAILED", "MFA_ENFORCER", actorID, role, "Hardware MFA validation expired or missing", r)
				w.Header().Set("X-MFA-Challenge-Required", "true")
				http.Error(w, "Multi-Factor Authentication Required", http.StatusPreconditionRequired)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
