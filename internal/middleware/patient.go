package middleware

import (
	"context"
	"net/http"

	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
)

const (
	CtxPatientID ctxKey = "patient_id"
	CtxUserID    ctxKey = "user_id"
)

func PatientAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		userID, ok, err := services.ValidateSession(token)
		if err != nil || !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		patientID, tenantID, err := services.EnsurePatientProfileForUser(userID)
		if err != nil {
			http.Error(w, "No patient profile linked to this account", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), CtxUserID, userID)
		ctx = context.WithValue(ctx, CtxPatientID, patientID)
		ctx = context.WithValue(ctx, CtxTenantID, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func PatientIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(CtxPatientID).(uuid.UUID)
	return id, ok
}

func UserIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(CtxUserID).(uuid.UUID)
	return id, ok
}
