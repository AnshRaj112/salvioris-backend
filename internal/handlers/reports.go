package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/AnshRaj112/serenify-backend/pkg/clientip"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type SubmitReportRequest struct {
	ReportedBy       string `json:"reported_by"`
	GroupID          string `json:"group_id"`
	EncryptedPayload string `json:"encrypted_payload"`
}

// SubmitAbuseReport receives and persists ECIES-X25519-AES-GCM report disclosure payloads.
func SubmitAbuseReport(w http.ResponseWriter, r *http.Request) {
	var req SubmitReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	reporterID, err := uuid.Parse(req.ReportedBy)
	if err != nil {
		http.Error(w, "invalid reporter user UUID", http.StatusBadRequest)
		return
	}

	groupUUID, err := uuid.Parse(req.GroupID)
	if err != nil {
		http.Error(w, "invalid group UUID", http.StatusBadRequest)
		return
	}

	if req.EncryptedPayload == "" {
		http.Error(w, "encrypted payload is required", http.StatusBadRequest)
		return
	}

	// Insert into PostgreSQL abuse_reports ledger
	reportID := uuid.New()
	_, err = database.PostgresDB.ExecContext(r.Context(), `
		INSERT INTO abuse_reports (id, reported_by, group_id, encrypted_payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, reportID, reporterID, groupUUID, req.EncryptedPayload, "pending", time.Now().UTC())

	if err != nil {
		http.Error(w, "failed to submit report", http.StatusInternalServerError)
		return
	}

	// Safe Logging only (no plaintext or encrypted payload is logged)
	_, _ = database.PostgresDB.ExecContext(r.Context(), `
		INSERT INTO security_audit_logs (id, event_type, target_id, actor_id, reason, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, uuid.New(), "USER_REPORT_SUBMITTED", reportID.String(), reporterID.String(), "Abuse/Harassment report filed securely", clientip.RealClientIP(r), time.Now().UTC())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"report_id": reportID.String(),
		"message":   "Report submitted securely to the mental health safety moderation team",
	})
}

type DecryptReportRequest struct {
	ModeratorID string `json:"moderator_id"`
	Reason      string `json:"reason"`
}

// ReviewAbuseReport decrypts and exposes user-disclosed message packages under strict PostgreSQL audit controls.
func ReviewAbuseReport(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(reportID); err != nil {
		http.Error(w, "invalid report id format", http.StatusBadRequest)
		return
	}

	var req DecryptReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ModeratorID == "" || req.Reason == "" {
		http.Error(w, "moderator_id and justification reason are required", http.StatusBadRequest)
		return
	}

	// Retrieve disclosure service and decrypt reported envelope in secure KMS enclave
	if services.ActiveDisclosureService == nil {
		http.Error(w, "governed disclosure service is not initialized", http.StatusInternalServerError)
		return
	}

	ipAddress := clientip.RealClientIP(r)
	plaintextJSON, err := services.ActiveDisclosureService.DecryptReportPayload(r.Context(), reportID, req.ModeratorID, req.Reason, ipAddress)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"report_id":       reportID,
		"disclosed_data":  plaintextJSON,
		"security_status": "Audited governed disclosure access verified and authorized",
	})
}
