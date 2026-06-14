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

type patientRequest struct {
	FullName         string `json:"full_name"`
	DateOfBirth      string `json:"date_of_birth,omitempty"`
	Gender           string `json:"gender,omitempty"`
	Phone            string `json:"phone,omitempty"`
	Email            string `json:"email,omitempty"`
	EmergencyContact string `json:"emergency_contact,omitempty"`
	Address          string `json:"address,omitempty"`
	Status           string `json:"status,omitempty"`
}

func ListPatientsV2(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	therapistID, _ := middleware.TherapistIDFromCtx(r.Context())
	_ = services.SyncConnectedUsersToPatients(tenantID, therapistID)

	rows, err := database.PostgresDB.Query(`
		SELECT id, tenant_id, user_id, full_name, date_of_birth, gender, phone, email,
			emergency_contact, address, assigned_therapist_id, status, created_at, updated_at
		FROM patients
		WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		http.Error(w, "Failed to list patients", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	patients := make([]models.Patient, 0)
	for rows.Next() {
		p, err := scanPatient(rows)
		if err != nil {
			http.Error(w, "Failed to read patients", http.StatusInternalServerError)
			return
		}
		patients = append(patients, p)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": patients})
}

func CreatePatientV2(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	therapistID, ok := middleware.TherapistIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req patientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.FullName = strings.TrimSpace(req.FullName)
	if req.FullName == "" {
		http.Error(w, "full_name is required", http.StatusBadRequest)
		return
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "active"
	}

	dob, _ := parseDate(req.DateOfBirth)
	var id uuid.UUID
	err := database.PostgresDB.QueryRow(`
		INSERT INTO patients (
			tenant_id, full_name, date_of_birth, gender, phone, email,
			emergency_contact, address, assigned_therapist_id, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id
	`, tenantID, req.FullName, dob, nullStr(req.Gender), nullStr(req.Phone), nullStr(strings.ToLower(req.Email)),
		nullStr(req.EmergencyContact), nullStr(req.Address), therapistID, status).Scan(&id)
	if err != nil {
		http.Error(w, "Failed to create patient", http.StatusInternalServerError)
		return
	}

	p, err := getPatientByID(tenantID, id)
	if err != nil {
		http.Error(w, "Failed to load patient", http.StatusInternalServerError)
		return
	}
	services.AuditV2Tenant(r, tenantID, "PATIENT_CREATED", "patient", id.String(), therapistID.String())
	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": p})
}

func GetPatientV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, err := uuid.Parse(chi.URLParam(r, "patientId"))
	if err != nil {
		http.Error(w, "Invalid patient ID", http.StatusBadRequest)
		return
	}

	p, err := getPatientByID(tenantID, patientID)
	if err == sql.ErrNoRows {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load patient", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": p})
}

func UpdatePatientV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, err := uuid.Parse(chi.URLParam(r, "patientId"))
	if err != nil {
		http.Error(w, "Invalid patient ID", http.StatusBadRequest)
		return
	}

	var req patientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	dob, _ := parseDate(req.DateOfBirth)
	_, err = database.PostgresDB.Exec(`
		UPDATE patients SET
			full_name = COALESCE(NULLIF($3,''), full_name),
			date_of_birth = COALESCE($4, date_of_birth),
			gender = COALESCE(NULLIF($5,''), gender),
			phone = COALESCE(NULLIF($6,''), phone),
			email = COALESCE(NULLIF($7,''), email),
			emergency_contact = COALESCE(NULLIF($8,''), emergency_contact),
			address = COALESCE(NULLIF($9,''), address),
			status = COALESCE(NULLIF($10,''), status),
			updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, patientID, tenantID, strings.TrimSpace(req.FullName), dob,
		req.Gender, req.Phone, strings.ToLower(strings.TrimSpace(req.Email)),
		req.EmergencyContact, req.Address, req.Status)
	if err != nil {
		http.Error(w, "Failed to update patient", http.StatusInternalServerError)
		return
	}

	p, err := getPatientByID(tenantID, patientID)
	if err == sql.ErrNoRows {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load patient", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": p})
}

func DeletePatientV2(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := middleware.TenantIDFromCtx(r.Context())
	patientID, err := uuid.Parse(chi.URLParam(r, "patientId"))
	if err != nil {
		http.Error(w, "Invalid patient ID", http.StatusBadRequest)
		return
	}

	res, err := database.PostgresDB.Exec(`
		UPDATE patients SET deleted_at = NOW(), status = 'inactive', updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, patientID, tenantID)
	if err != nil {
		http.Error(w, "Failed to delete patient", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func getPatientByID(tenantID, patientID uuid.UUID) (models.Patient, error) {
	row := database.PostgresDB.QueryRow(`
		SELECT id, tenant_id, user_id, full_name, date_of_birth, gender, phone, email,
			emergency_contact, address, assigned_therapist_id, status, created_at, updated_at
		FROM patients WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, patientID, tenantID)
	return scanPatientRow(row)
}

func scanPatient(rows *sql.Rows) (models.Patient, error) {
	var p models.Patient
	var userID, therapistID sql.NullString
	var dob sql.NullTime
	var gender, phone, email, emergency, address sql.NullString
	err := rows.Scan(
		&p.ID, &p.TenantID, &userID, &p.FullName, &dob, &gender, &phone, &email,
		&emergency, &address, &therapistID, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return p, err
	}
	applyPatientNulls(&p, userID, therapistID, dob, gender, phone, email, emergency, address)
	return p, nil
}

func scanPatientRow(row *sql.Row) (models.Patient, error) {
	var p models.Patient
	var userID, therapistID sql.NullString
	var dob sql.NullTime
	var gender, phone, email, emergency, address sql.NullString
	err := row.Scan(
		&p.ID, &p.TenantID, &userID, &p.FullName, &dob, &gender, &phone, &email,
		&emergency, &address, &therapistID, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return p, err
	}
	applyPatientNulls(&p, userID, therapistID, dob, gender, phone, email, emergency, address)
	return p, nil
}

func applyPatientNulls(p *models.Patient, userID, therapistID sql.NullString, dob sql.NullTime, gender, phone, email, emergency, address sql.NullString) {
	if userID.Valid {
		id := uuid.MustParse(userID.String)
		p.UserID = &id
	}
	if therapistID.Valid {
		id := uuid.MustParse(therapistID.String)
		p.AssignedTherapistID = &id
	}
	if dob.Valid {
		t := dob.Time
		p.DateOfBirth = &t
	}
	p.Gender = gender.String
	p.Phone = phone.String
	p.Email = email.String
	p.EmergencyContact = emergency.String
	p.Address = address.String
}

func parseDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func nullStr(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
