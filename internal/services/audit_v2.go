package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

func AuditV2(r *http.Request, eventType, targetID, actorID, role, details string) {
	database.LogSecurityEvent(context.Background(), database.AuditEvent{
		EventType:     "V2_" + eventType,
		TargetID:      targetID,
		ActorID:       actorID,
		ActorRole:     role,
		ActionDetails: details,
		IPAddress:     clientIP(r),
		UserAgent:     r.UserAgent(),
	})
}

func AuditV2Tenant(r *http.Request, tenantID uuid.UUID, eventType, resourceType, resourceID, actorID string) {
	AuditV2(r, eventType, resourceID, actorID, "therapist",
		fmt.Sprintf("tenant=%s resource=%s id=%s", tenantID.String(), resourceType, resourceID))
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}
