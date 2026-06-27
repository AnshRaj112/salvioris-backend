package middleware

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	CtxReceptionistID ctxKey = "receptionist_id"
)

// ReceptionistAuth validates that the caller is an active receptionist belonging
// to the tenant specified in the URL path parameter {tenantId}.
func ReceptionistAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, err := uuid.Parse(chi.URLParam(r, "tenantId"))
		if err != nil {
			http.Error(w, "Invalid tenant ID", http.StatusBadRequest)
			return
		}

		token := bearerToken(r.Header.Get("Authorization"))
		var receptionistID uuid.UUID
		var ok bool

		if claims, valid := services.ValidateReceptionistAccessToken(token); valid {
			receptionistID, err = uuid.Parse(claims.UserID)
			ok = err == nil
		} else {
			receptionistID, ok = services.GetReceptionistAuthCache(token)
		}
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Verify this receptionist is active and belongs to the requested tenant
		var therapistID uuid.UUID
		var isActive bool
		err = database.PostgresDB.QueryRow(`
			SELECT therapist_id, is_active FROM receptionists
			WHERE id = $1 AND tenant_id = $2
		`, receptionistID, tenantID).Scan(&therapistID, &isActive)
		if err == sql.ErrNoRows {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		if !isActive {
			http.Error(w, "Account deactivated", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), CtxReceptionistID, receptionistID)
		ctx = context.WithValue(ctx, CtxTenantID, tenantID)
		ctx = context.WithValue(ctx, CtxTherapistID, therapistID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ReceptionistIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(CtxReceptionistID).(uuid.UUID)
	return id, ok
}
