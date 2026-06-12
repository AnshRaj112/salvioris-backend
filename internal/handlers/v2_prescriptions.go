package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type prescriptionRequest struct {
	MedicineName string `json:"medicine_name"`
	Dosage       string `json:"dosage"`
	Frequency    string `json:"frequency"`
	DurationDays *int   `json:"duration_days,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

func ListPrescriptionsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}
	status := r.URL.Query().Get("status")
	listPrescriptions(w, tenantID, patientID, status)
}

func CreatePrescriptionV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())
	patientID, ok := parsePatientIDParam(chi.URLParam(r, "patientId"))
	if !ok || !patientBelongsToTenant(tenantID, patientID) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}

	var req prescriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.MedicineName = strings.TrimSpace(req.MedicineName)
	req.Dosage = strings.TrimSpace(req.Dosage)
	req.Frequency = strings.TrimSpace(req.Frequency)
	if req.MedicineName == "" || req.Dosage == "" || req.Frequency == "" {
		http.Error(w, "medicine_name, dosage, frequency required", http.StatusBadRequest)
		return
	}

	var expiresAt *time.Time
	if req.DurationDays != nil && *req.DurationDays > 0 {
		t := time.Now().AddDate(0, 0, *req.DurationDays)
		expiresAt = &t
	}

	var id uuid.UUID
	err := database.PostgresDB.QueryRow(`
		INSERT INTO prescriptions (
			tenant_id, patient_id, therapist_id, medicine_name, dosage, frequency,
			duration_days, notes, expires_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id
	`, tenantID, patientID, therapistID, req.MedicineName, req.Dosage, req.Frequency,
		req.DurationDays, nullStr(req.Notes), expiresAt).Scan(&id)
	if err != nil {
		http.Error(w, "Failed to create prescription", http.StatusInternalServerError)
		return
	}

	rx, _ := getPrescription(tenantID, id)
	services.NotifyPatientByID(patientID, "New prescription", req.MedicineName, "prescription")
	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": rx})
}

func UpdatePrescriptionV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	rxID, ok := parsePatientIDParam(chi.URLParam(r, "rxId"))
	if !ok {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	if body.Status != "discontinued" && body.Status != "completed" {
		http.Error(w, "status must be discontinued or completed", http.StatusBadRequest)
		return
	}

	_, err := database.PostgresDB.Exec(`
		UPDATE prescriptions SET status = $3,
			discontinued_at = CASE WHEN $3 = 'discontinued' THEN NOW() ELSE discontinued_at END,
			updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
	`, rxID, tenantID, body.Status)
	if err != nil {
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	}
	rx, err := getPrescription(tenantID, rxID)
	if err == sql.ErrNoRows {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": rx})
}

func ListMyPrescriptionsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, _ := middleware.PatientIDFromCtx(r.Context())
	listPrescriptions(w, tenantID, patientID, r.URL.Query().Get("status"))
}

func listPrescriptions(w http.ResponseWriter, tenantID, patientID uuid.UUID, status string) {
	query := `
		SELECT id, tenant_id, patient_id, therapist_id, medicine_name, dosage, frequency,
			duration_days, notes, status, prescribed_at, expires_at, discontinued_at, created_at, updated_at
		FROM prescriptions WHERE tenant_id = $1 AND patient_id = $2
	`
	args := []interface{}{tenantID, patientID}
	if status != "" {
		query += ` AND status = $3`
		args = append(args, status)
	}
	query += ` ORDER BY prescribed_at DESC`

	rows, err := database.PostgresDB.Query(query, args...)
	if err != nil {
		http.Error(w, "Failed to list prescriptions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := make([]models.Prescription, 0)
	for rows.Next() {
		rx, err := scanPrescription(rows)
		if err != nil {
			http.Error(w, "Failed to read prescriptions", http.StatusInternalServerError)
			return
		}
		items = append(items, rx)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

func getPrescription(tenantID, id uuid.UUID) (models.Prescription, error) {
	row := database.PostgresDB.QueryRow(`
		SELECT id, tenant_id, patient_id, therapist_id, medicine_name, dosage, frequency,
			duration_days, notes, status, prescribed_at, expires_at, discontinued_at, created_at, updated_at
		FROM prescriptions WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	return scanPrescriptionRow(row)
}

func scanPrescription(rows *sql.Rows) (models.Prescription, error) {
	var rx models.Prescription
	var dur sql.NullInt64
	var notes sql.NullString
	var expires, discontinued sql.NullTime
	err := rows.Scan(&rx.ID, &rx.TenantID, &rx.PatientID, &rx.TherapistID,
		&rx.MedicineName, &rx.Dosage, &rx.Frequency, &dur, &notes, &rx.Status,
		&rx.PrescribedAt, &expires, &discontinued, &rx.CreatedAt, &rx.UpdatedAt)
	if err != nil {
		return rx, err
	}
	if dur.Valid {
		d := int(dur.Int64)
		rx.DurationDays = &d
	}
	rx.Notes = notes.String
	if expires.Valid {
		t := expires.Time
		rx.ExpiresAt = &t
	}
	if discontinued.Valid {
		t := discontinued.Time
		rx.DiscontinuedAt = &t
	}
	return rx, nil
}

func scanPrescriptionRow(row *sql.Row) (models.Prescription, error) {
	var rx models.Prescription
	var dur sql.NullInt64
	var notes sql.NullString
	var expires, discontinued sql.NullTime
	err := row.Scan(&rx.ID, &rx.TenantID, &rx.PatientID, &rx.TherapistID,
		&rx.MedicineName, &rx.Dosage, &rx.Frequency, &dur, &notes, &rx.Status,
		&rx.PrescribedAt, &expires, &discontinued, &rx.CreatedAt, &rx.UpdatedAt)
	if err != nil {
		return rx, err
	}
	if dur.Valid {
		d := int(dur.Int64)
		rx.DurationDays = &d
	}
	rx.Notes = notes.String
	if expires.Valid {
		t := expires.Time
		rx.ExpiresAt = &t
	}
	if discontinued.Valid {
		t := discontinued.Time
		rx.DiscontinuedAt = &t
	}
	return rx, nil
}
