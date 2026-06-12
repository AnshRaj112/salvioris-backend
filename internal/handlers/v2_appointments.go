package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type appointmentRequest struct {
	PatientID   string `json:"patient_id"`
	TherapistID string `json:"therapist_id,omitempty"`
	Type        string `json:"type"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at,omitempty"`
	DurationMin int    `json:"duration_min,omitempty"`
	MeetingLink string `json:"meeting_link,omitempty"`
	Location    string `json:"location,omitempty"`
	Notes       string `json:"notes,omitempty"`
	Status      string `json:"status,omitempty"`
}

type cancelRequest struct {
	Reason string `json:"reason,omitempty"`
}

func ListAppointmentsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	from, to := parseAptRange(r)

	query := `
		SELECT id, tenant_id, patient_id, therapist_id, type, status, starts_at, ends_at,
			meeting_link, location, notes, reminder_sent, created_by, cancelled_at, cancel_reason, created_at, updated_at
		FROM appointments WHERE tenant_id = $1 AND starts_at >= $2 AND starts_at <= $3
	`
	args := []interface{}{tenantID, from, to}

	if v := r.URL.Query().Get("therapist_id"); v != "" {
		query += ` AND therapist_id = $4`
		args = append(args, v)
	}
	if v := r.URL.Query().Get("patient_id"); v != "" {
		query += ` AND patient_id = $` + itoa(len(args)+1)
		args = append(args, v)
	}
	if v := r.URL.Query().Get("status"); v != "" {
		query += ` AND status = $` + itoa(len(args)+1)
		args = append(args, v)
	}
	query += ` ORDER BY starts_at ASC`

	rows, err := database.PostgresDB.Query(query, args...)
	if err != nil {
		http.Error(w, "Failed to list appointments", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	appointments := make([]models.Appointment, 0)
	for rows.Next() {
		a, err := scanAppointment(rows)
		if err != nil {
			http.Error(w, "Failed to read appointments", http.StatusInternalServerError)
			return
		}
		appointments = append(appointments, a)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": appointments})
}

func CreateAppointmentV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())

	var req appointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	patientID, err := uuid.Parse(req.PatientID)
	if err != nil || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Invalid patient", http.StatusBadRequest)
		return
	}

	aptTherapist := therapistID
	if req.TherapistID != "" {
		if tid, e := uuid.Parse(req.TherapistID); e == nil && services.TherapistInTenant(tenantID, tid) {
			aptTherapist = tid
		}
	}

	aptType := strings.TrimSpace(req.Type)
	if !services.ValidateAppointmentType(aptType) {
		http.Error(w, "Invalid appointment type", http.StatusBadRequest)
		return
	}

	startsAt, err := services.ParseRFC3339(req.StartsAt)
	if err != nil {
		http.Error(w, "Invalid starts_at (RFC3339)", http.StatusBadRequest)
		return
	}
	var endsAt time.Time
	if req.EndsAt != "" {
		endsAt, err = services.ParseRFC3339(req.EndsAt)
		if err != nil {
			http.Error(w, "Invalid ends_at", http.StatusBadRequest)
			return
		}
	} else {
		endsAt = startsAt.Add(services.DefaultDuration(req.DurationMin))
	}
	if !endsAt.After(startsAt) {
		http.Error(w, "ends_at must be after starts_at", http.StatusBadRequest)
		return
	}

	conflict, err := services.TherapistHasConflict(aptTherapist, startsAt, endsAt, nil)
	if err != nil || conflict {
		http.Error(w, "Therapist has a scheduling conflict", http.StatusConflict)
		return
	}

	var id uuid.UUID
	err = database.PostgresDB.QueryRow(`
		INSERT INTO appointments (
			tenant_id, patient_id, therapist_id, type, status, starts_at, ends_at,
			meeting_link, location, notes, created_by
		) VALUES ($1,$2,$3,$4,'scheduled',$5,$6,$7,$8,$9,$10)
		RETURNING id
	`, tenantID, patientID, aptTherapist, aptType, startsAt, endsAt,
		nullStr(req.MeetingLink), nullStr(req.Location), nullStr(req.Notes), therapistID).Scan(&id)
	if err != nil {
		http.Error(w, "Failed to create appointment", http.StatusInternalServerError)
		return
	}

	services.EnqueueCalendarSync("create", tenantID, id)
	a, _ := getAppointment(tenantID, id)
	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": a})
}

func GetAppointmentV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	aptID, ok := parsePatientIDParam(chi.URLParam(r, "appointmentId"))
	if !ok {
		http.Error(w, "Invalid appointment ID", http.StatusBadRequest)
		return
	}
	a, err := getAppointment(tenantID, aptID)
	if err == sql.ErrNoRows {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load appointment", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": a})
}

func UpdateAppointmentV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	aptID, ok := parsePatientIDParam(chi.URLParam(r, "appointmentId"))
	if !ok {
		http.Error(w, "Invalid appointment ID", http.StatusBadRequest)
		return
	}

	var req appointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	existing, err := getAppointment(tenantID, aptID)
	if err == sql.ErrNoRows {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if err != nil || existing.Status == "cancelled" {
		http.Error(w, "Cannot update appointment", http.StatusConflict)
		return
	}

	startsAt := existing.StartsAt
	endsAt := existing.EndsAt
	if req.StartsAt != "" {
		startsAt, _ = services.ParseRFC3339(req.StartsAt)
	}
	if req.EndsAt != "" {
		endsAt, _ = services.ParseRFC3339(req.EndsAt)
	} else if req.DurationMin > 0 || req.StartsAt != "" {
		endsAt = startsAt.Add(services.DefaultDuration(req.DurationMin))
	}

	conflict, err := services.TherapistHasConflict(existing.TherapistID, startsAt, endsAt, &aptID)
	if err != nil || conflict {
		http.Error(w, "Scheduling conflict", http.StatusConflict)
		return
	}

	aptType := existing.Type
	if req.Type != "" && services.ValidateAppointmentType(req.Type) {
		aptType = req.Type
	}

	newStatus := existing.Status
	if req.Status == "completed" && existing.Status != "cancelled" {
		newStatus = "completed"
	} else if existing.Status == "scheduled" {
		newStatus = "confirmed"
	}

	_, err = database.PostgresDB.Exec(`
		UPDATE appointments SET
			type = $3, starts_at = $4, ends_at = $5,
			meeting_link = COALESCE(NULLIF($6,''), meeting_link),
			location = COALESCE(NULLIF($7,''), location),
			notes = COALESCE(NULLIF($8,''), notes),
			status = $9,
			updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
	`, aptID, tenantID, aptType, startsAt, endsAt, req.MeetingLink, req.Location, req.Notes, newStatus)
	if err != nil {
		http.Error(w, "Failed to update appointment", http.StatusInternalServerError)
		return
	}

	if newStatus == "completed" {
		_ = services.CreateDraftInvoiceFromAppointment(tenantID, aptID)
	}

	services.EnqueueCalendarSync("update", tenantID, aptID)
	a, _ := getAppointment(tenantID, aptID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": a})
}

func CancelAppointmentV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	aptID, ok := parsePatientIDParam(chi.URLParam(r, "appointmentId"))
	if !ok {
		http.Error(w, "Invalid appointment ID", http.StatusBadRequest)
		return
	}

	var req cancelRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	res, err := database.PostgresDB.Exec(`
		UPDATE appointments SET status = 'cancelled', cancelled_at = NOW(),
			cancel_reason = $3, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND status NOT IN ('cancelled', 'completed')
	`, aptID, tenantID, strings.TrimSpace(req.Reason))
	if err != nil {
		http.Error(w, "Failed to cancel", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.Error(w, "Not found or already cancelled", http.StatusNotFound)
		return
	}

	services.EnqueueCalendarSync("delete", tenantID, aptID)
	a, _ := getAppointment(tenantID, aptID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": a})
}

func ListMyAppointmentsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())
	from, to := parseAptRange(r)

	rows, err := database.PostgresDB.Query(`
		SELECT id, tenant_id, patient_id, therapist_id, type, status, starts_at, ends_at,
			meeting_link, location, notes, reminder_sent, created_by, cancelled_at, cancel_reason, created_at, updated_at
		FROM appointments
		WHERE tenant_id = $1 AND patient_id = $2 AND starts_at >= $3 AND starts_at <= $4
		ORDER BY starts_at ASC
	`, tenantID, patientID, from, to)
	if err != nil {
		http.Error(w, "Failed to list appointments", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	appointments := make([]models.Appointment, 0)
	for rows.Next() {
		a, _ := scanAppointment(rows)
		appointments = append(appointments, a)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": appointments})
}

func getAppointment(tenantID, id uuid.UUID) (models.Appointment, error) {
	row := database.PostgresDB.QueryRow(`
		SELECT id, tenant_id, patient_id, therapist_id, type, status, starts_at, ends_at,
			meeting_link, location, notes, reminder_sent, created_by, cancelled_at, cancel_reason, created_at, updated_at
		FROM appointments WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	return scanAppointmentRow(row)
}

func scanAppointment(rows *sql.Rows) (models.Appointment, error) {
	var a models.Appointment
	var meeting, location, notes, cancelReason sql.NullString
	var createdBy sql.NullString
	var cancelledAt sql.NullTime
	err := rows.Scan(
		&a.ID, &a.TenantID, &a.PatientID, &a.TherapistID, &a.Type, &a.Status,
		&a.StartsAt, &a.EndsAt, &meeting, &location, &notes, &a.ReminderSent,
		&createdBy, &cancelledAt, &cancelReason, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return a, err
	}
	a.MeetingLink = meeting.String
	a.Location = location.String
	a.Notes = notes.String
	a.CancelReason = cancelReason.String
	if createdBy.Valid {
		id := uuid.MustParse(createdBy.String)
		a.CreatedBy = &id
	}
	if cancelledAt.Valid {
		t := cancelledAt.Time
		a.CancelledAt = &t
	}
	return a, nil
}

func scanAppointmentRow(row *sql.Row) (models.Appointment, error) {
	var a models.Appointment
	var meeting, location, notes, cancelReason sql.NullString
	var createdBy sql.NullString
	var cancelledAt sql.NullTime
	err := row.Scan(
		&a.ID, &a.TenantID, &a.PatientID, &a.TherapistID, &a.Type, &a.Status,
		&a.StartsAt, &a.EndsAt, &meeting, &location, &notes, &a.ReminderSent,
		&createdBy, &cancelledAt, &cancelReason, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return a, err
	}
	a.MeetingLink = meeting.String
	a.Location = location.String
	a.Notes = notes.String
	a.CancelReason = cancelReason.String
	if createdBy.Valid {
		id := uuid.MustParse(createdBy.String)
		a.CreatedBy = &id
	}
	if cancelledAt.Valid {
		t := cancelledAt.Time
		a.CancelledAt = &t
	}
	return a, nil
}

func parseAptRange(r *http.Request) (time.Time, time.Time) {
	now := time.Now()
	from := now.AddDate(0, -1, 0)
	to := now.AddDate(0, 2, 0)
	if s := r.URL.Query().Get("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			from = t
		}
	}
	if s := r.URL.Query().Get("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			to = t
		}
	}
	return from, to
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
