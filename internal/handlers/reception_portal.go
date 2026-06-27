package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
)

// ──────────────────────────────────────────────────────────────────────────────
// Reception Portal — Appointments (read-only + walk-in scheduling)
// ──────────────────────────────────────────────────────────────────────────────

// ReceptionListAppointments returns all appointments for the tenant — same as
// the therapist's ListAppointmentsV2 but accessible via ReceptionistAuth.
func ReceptionListAppointments(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	from, to := parseAptRange(r)

	query := `
		SELECT id, tenant_id, patient_id, therapist_id, type, status, starts_at, ends_at,
			meeting_link, location, notes, reminder_sent, created_by, cancelled_at, cancel_reason, created_at, updated_at
		FROM appointments WHERE tenant_id = $1 AND starts_at >= $2 AND starts_at <= $3
	`
	args := []interface{}{tenantID, from, to}

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
			continue
		}
		appointments = append(appointments, a)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": appointments})
}

// ReceptionWalkIn schedules a walk-in appointment — uses the receptionist's
// linked therapist ID when no explicit therapist_id is supplied in the body.
func ReceptionWalkIn(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())

	var req walkInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.PatientName = strings.TrimSpace(req.PatientName)
	if req.PatientName == "" || req.StartsAt == "" {
		http.Error(w, "patient_name and starts_at are required", http.StatusBadRequest)
		return
	}

	aptTherapist := therapistID
	if req.TherapistID != "" {
		if tid, e := uuid.Parse(req.TherapistID); e == nil && services.TherapistInTenant(tenantID, tid) {
			aptTherapist = tid
		}
	}

	startsAt, err := services.ParseRFC3339(req.StartsAt)
	if err != nil {
		http.Error(w, "Invalid starts_at", http.StatusBadRequest)
		return
	}
	endsAt := startsAt.Add(services.DefaultDuration(req.DurationMin))

	conflict, _ := services.TherapistHasConflict(aptTherapist, startsAt, endsAt, nil)
	if conflict {
		http.Error(w, "Therapist has a scheduling conflict at this time", http.StatusConflict)
		return
	}

	var patientID uuid.UUID
	err = database.PostgresDB.QueryRow(`
		INSERT INTO patients (tenant_id, full_name, phone, email, assigned_therapist_id, status)
		VALUES ($1, $2, $3, $4, $5, 'active')
		RETURNING id
	`, tenantID, req.PatientName, nullStr(req.Phone), nullStr(strings.ToLower(req.Email)), aptTherapist).Scan(&patientID)
	if err != nil {
		http.Error(w, "Failed to register patient", http.StatusInternalServerError)
		return
	}

	var aptID uuid.UUID
	err = database.PostgresDB.QueryRow(`
		INSERT INTO appointments (
			tenant_id, patient_id, therapist_id, type, status, starts_at, ends_at,
			location, notes, created_by
		) VALUES ($1,$2,$3,'walk_in','confirmed',$4,$5,$6,$7,$8)
		RETURNING id
	`, tenantID, patientID, aptTherapist, startsAt, endsAt,
		nullStr(req.Location), nullStr(req.Notes), aptTherapist).Scan(&aptID)
	if err != nil {
		http.Error(w, "Failed to create appointment", http.StatusInternalServerError)
		return
	}

	services.EnqueueCalendarSync("create", tenantID, aptID)
	patient, _ := getPatientByID(tenantID, patientID)
	apt, _ := getAppointment(tenantID, aptID)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"data": map[string]interface{}{"patient": patient, "appointment": apt},
	})
}

// ReceptionQuickRegister registers a patient without scheduling an appointment.
func ReceptionQuickRegister(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())

	var req quickRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.PatientName = strings.TrimSpace(req.PatientName)
	if req.PatientName == "" {
		http.Error(w, "patient_name is required", http.StatusBadRequest)
		return
	}

	var patientID uuid.UUID
	err := database.PostgresDB.QueryRow(`
		INSERT INTO patients (tenant_id, full_name, phone, email, assigned_therapist_id, status)
		VALUES ($1, $2, $3, $4, $5, 'active')
		RETURNING id
	`, tenantID, req.PatientName, nullStr(req.Phone), nullStr(strings.ToLower(req.Email)), therapistID).Scan(&patientID)
	if err != nil {
		http.Error(w, "Failed to register patient", http.StatusInternalServerError)
		return
	}

	p, _ := getPatientByID(tenantID, patientID)
	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": p})
}

// ──────────────────────────────────────────────────────────────────────────────
// Reception Portal — Billing (invoices + payment collection)
// ──────────────────────────────────────────────────────────────────────────────

// ReceptionListInvoices returns all invoices for the tenant (same as therapist view).
func ReceptionListInvoices(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	status := r.URL.Query().Get("status")
	patientFilter := r.URL.Query().Get("patient_id")
	listInvoices(w, tenantID, patientFilter, status)
}

// ReceptionCollectPayment records a cash/card payment collected at reception.
func ReceptionCollectPayment(w http.ResponseWriter, r *http.Request) {
	ReceptionCollectPaymentV2(w, r)
}

// ──────────────────────────────────────────────────────────────────────────────
// Reception Portal — Referrals (read-only)
// ──────────────────────────────────────────────────────────────────────────────

// ReceptionListReferralCodes returns the referral codes for the therapist — read-only for reception.
func ReceptionListReferralCodes(w http.ResponseWriter, r *http.Request) {
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())

	rows, err := database.PostgresDB.Query(`
		SELECT id, code, created_at, expires_at, usage_limit, usage_count, is_revoked
		FROM referral_codes
		WHERE therapist_id = $1
		ORDER BY created_at DESC
	`, therapistID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type referralRow struct {
		ID         string  `json:"id"`
		Code       string  `json:"code"`
		CreatedAt  string  `json:"created_at"`
		ExpiresAt  *string `json:"expires_at"`
		UsageLimit *int    `json:"usage_limit"`
		UsageCount int     `json:"usage_count"`
		IsRevoked  bool    `json:"is_revoked"`
	}

	var list []referralRow
	for rows.Next() {
		var row referralRow
		var id uuid.UUID
		if err := rows.Scan(&id, &row.Code, &row.CreatedAt, &row.ExpiresAt, &row.UsageLimit, &row.UsageCount, &row.IsRevoked); err != nil {
			continue
		}
		row.ID = id.String()
		list = append(list, row)
	}
	if list == nil {
		list = []referralRow{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": list})
}

// ──────────────────────────────────────────────────────────────────────────────
// Reception Portal — Patients (list only)
// ──────────────────────────────────────────────────────────────────────────────

// ReceptionListPatients returns the active patient list for the tenant.
func ReceptionListPatients(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())

	rows, err := database.PostgresDB.Query(`
		SELECT id, full_name, phone, email, gender, status, created_at
		FROM patients
		WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY full_name ASC
	`, tenantID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type patientRow struct {
		ID        string  `json:"id"`
		FullName  string  `json:"full_name"`
		Phone     *string `json:"phone"`
		Email     *string `json:"email"`
		Gender    *string `json:"gender"`
		Status    string  `json:"status"`
		CreatedAt string  `json:"created_at"`
	}

	var list []patientRow
	for rows.Next() {
		var p patientRow
		var id uuid.UUID
		if err := rows.Scan(&id, &p.FullName, &p.Phone, &p.Email, &p.Gender, &p.Status, &p.CreatedAt); err != nil {
			continue
		}
		p.ID = id.String()
		list = append(list, p)
	}
	if list == nil {
		list = []patientRow{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": list})
}
