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

type ctxKey string

const (
	CtxTherapistID ctxKey = "therapist_id"
	CtxTenantID    ctxKey = "tenant_id"
)

func TenantAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, err := uuid.Parse(chi.URLParam(r, "tenantId"))
		if err != nil {
			http.Error(w, "Invalid tenant ID", http.StatusBadRequest)
			return
		}

		token := bearerToken(r.Header.Get("Authorization"))
		var therapistID uuid.UUID
		var ok bool

		if claims, valid := services.ValidateAccessToken(token); valid {
			therapistID, err = uuid.Parse(claims.UserID)
			ok = err == nil
		} else {
			therapistID, ok = resolveTherapistSession(token)
		}
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var approved bool
		err = database.PostgresDB.QueryRow(
			`SELECT is_approved FROM therapists WHERE id = $1`, therapistID,
		).Scan(&approved)
		if err == sql.ErrNoRows || !approved {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		owns, err := services.TherapistOwnsTenant(therapistID, tenantID)
		if err != nil || !owns {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), CtxTherapistID, therapistID)
		ctx = context.WithValue(ctx, CtxTenantID, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(h string) string {
	if len(h) > 7 && h[:7] == "Bearer " {
		return h[7:]
	}
	return ""
}

func resolveTherapistSession(token string) (uuid.UUID, bool) {
	if token == "" {
		return uuid.Nil, false
	}
	if id, ok := services.GetTherapistAuthCache(token); ok {
		return id, true
	}
	userID, ok, err := services.ValidateSession(token)
	if err != nil || !ok {
		return uuid.Nil, false
	}
	return userID, true
}

func TherapistIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(CtxTherapistID).(uuid.UUID)
	return id, ok
}

func TenantIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(CtxTenantID).(uuid.UUID)
	return id, ok
}
