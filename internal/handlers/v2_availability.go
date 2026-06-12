package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type availabilityRequest struct {
	TherapistID     string `json:"therapist_id,omitempty"`
	DayOfWeek       int    `json:"day_of_week"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	SlotDurationMin int    `json:"slot_duration_min,omitempty"`
	IsActive        *bool  `json:"is_active,omitempty"`
}

func ListAvailabilityV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())

	tid := therapistID
	if v := r.URL.Query().Get("therapist_id"); v != "" {
		if parsed, err := uuid.Parse(v); err == nil && services.TherapistInTenant(tenantID, parsed) {
			tid = parsed
		}
	}

	rows, err := database.PostgresDB.Query(`
		SELECT id, tenant_id, therapist_id, day_of_week, start_time::text, end_time::text,
			slot_duration_min, is_active
		FROM availability_slots
		WHERE tenant_id = $1 AND therapist_id = $2
		ORDER BY day_of_week, start_time
	`, tenantID, tid)
	if err != nil {
		http.Error(w, "Failed to list availability", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	slots := make([]models.AvailabilitySlot, 0)
	for rows.Next() {
		var s models.AvailabilitySlot
		if err := rows.Scan(&s.ID, &s.TenantID, &s.TherapistID, &s.DayOfWeek,
			&s.StartTime, &s.EndTime, &s.SlotDurationMin, &s.IsActive); err != nil {
			http.Error(w, "Failed to read availability", http.StatusInternalServerError)
			return
		}
		slots = append(slots, s)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": slots})
}

func CreateAvailabilityV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())

	var req availabilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.DayOfWeek < 0 || req.DayOfWeek > 6 {
		http.Error(w, "day_of_week must be 0-6", http.StatusBadRequest)
		return
	}

	tid := therapistID
	if req.TherapistID != "" {
		if parsed, err := uuid.Parse(req.TherapistID); err == nil && services.TherapistInTenant(tenantID, parsed) {
			tid = parsed
		}
	}

	dur := req.SlotDurationMin
	if dur <= 0 {
		dur = 60
	}

	var id uuid.UUID
	err := database.PostgresDB.QueryRow(`
		INSERT INTO availability_slots (tenant_id, therapist_id, day_of_week, start_time, end_time, slot_duration_min)
		VALUES ($1, $2, $3, $4::time, $5::time, $6)
		RETURNING id
	`, tenantID, tid, req.DayOfWeek, req.StartTime, req.EndTime, dur).Scan(&id)
	if err != nil {
		http.Error(w, "Failed to create availability slot", http.StatusBadRequest)
		return
	}

	slot, _ := getAvailabilitySlot(tenantID, id)
	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": slot})
}

func DeleteAvailabilityV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	slotID, ok := parsePatientIDParam(chi.URLParam(r, "slotId"))
	if !ok {
		http.Error(w, "Invalid slot ID", http.StatusBadRequest)
		return
	}
	_, err := database.PostgresDB.Exec(`
		DELETE FROM availability_slots WHERE id = $1 AND tenant_id = $2
	`, slotID, tenantID)
	if err != nil {
		http.Error(w, "Failed to delete slot", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func GetOpenSlotsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}
	day, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "Invalid date", http.StatusBadRequest)
		return
	}

	rows, err := database.PostgresDB.Query(`
		SELECT start_time::text, end_time::text, slot_duration_min
		FROM availability_slots
		WHERE tenant_id = $1 AND therapist_id = $2 AND day_of_week = $3 AND is_active = TRUE
	`, tenantID, therapistID, int(day.Weekday()))
	if err != nil {
		http.Error(w, "Failed to load availability", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type slot struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	open := make([]slot, 0)
	for rows.Next() {
		var start, end string
		var dur int
		_ = rows.Scan(&start, &end, &dur)
		open = append(open, slot{Start: start, End: end})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": open, "date": dateStr})
}

func getAvailabilitySlot(tenantID, id uuid.UUID) (models.AvailabilitySlot, error) {
	var s models.AvailabilitySlot
	err := database.PostgresDB.QueryRow(`
		SELECT id, tenant_id, therapist_id, day_of_week, start_time::text, end_time::text,
			slot_duration_min, is_active
		FROM availability_slots WHERE id = $1 AND tenant_id = $2
	`, id, tenantID).Scan(&s.ID, &s.TenantID, &s.TherapistID, &s.DayOfWeek,
		&s.StartTime, &s.EndTime, &s.SlotDurationMin, &s.IsActive)
	return s, err
}
