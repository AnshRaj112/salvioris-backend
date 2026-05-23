package database

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/pkg/clientip"
)

// AuditEvent represents a structured record in our append-only security logs.
type AuditEvent struct {
	EventType     string
	TargetID      string
	ActorID       string
	ActorRole     string
	ActionDetails string
	IPAddress     string
	UserAgent     string
}

// LogSecurityEvent writes a structured audit event to the append-only security_audit_logs PostgreSQL table.
// If any database error occurs, it prints to standard error but does NOT panic to avoid disrupting application flow.
func LogSecurityEvent(ctx context.Context, event AuditEvent) {
	if PostgresDB == nil {
		log.Println("⚠️ Audit log skipped: PostgreSQL connection is not active")
		return
	}

	query := `
		INSERT INTO security_audit_logs (event_type, target_id, actor_id, actor_role, reason, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	// Enforce default field values if empty
	if event.IPAddress == "" {
		event.IPAddress = "0.0.0.0"
	}
	if event.UserAgent == "" {
		event.UserAgent = "unknown-agent"
	}

	_, err := PostgresDB.ExecContext(ctx, query,
		event.EventType,
		event.TargetID,
		event.ActorID,
		event.ActorRole,
		event.ActionDetails,
		event.IPAddress,
		event.UserAgent,
		time.Now().UTC(),
	)

	if err != nil {
		log.Printf("🔴 SECURITY AUDIT LOG WRITE FAILURE: %v | Event details: %+v", err, event)
	}
}

// TriggerAuditEvent extracts metadata from an HTTP request and logs a security event.
func TriggerAuditEvent(eventType, targetID, actorID, actorRole, details string, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ip := "0.0.0.0"
	ua := "unknown"
	if r != nil {
		ip = clientip.RealClientIP(r)
		ua = r.UserAgent()
	}

	event := AuditEvent{
		EventType:     eventType,
		TargetID:      targetID,
		ActorID:       actorID,
		ActorRole:     actorRole,
		ActionDetails: details,
		IPAddress:     ip,
		UserAgent:     ua,
	}

	LogSecurityEvent(ctx, event)
}
