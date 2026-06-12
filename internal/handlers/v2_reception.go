package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/middleware"
	"github.com/AnshRaj112/serenify-backend/internal/services"
	"github.com/google/uuid"
)

type walkInRequest struct {
	PatientName string `json:"patient_name"`
	Phone       string `json:"phone,omitempty"`
	Email       string `json:"email,omitempty"`
	TherapistID string `json:"therapist_id,omitempty"`
	StartsAt    string `json:"starts_at"`
	DurationMin int    `json:"duration_min,omitempty"`
	Location    string `json:"location,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

type quickRegisterRequest struct {
	PatientName string `json:"patient_name"`
	Phone       string `json:"phone,omitempty"`
	Email       string `json:"email,omitempty"`
}

func ReceptionWalkInV2(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Therapist has a scheduling conflict", http.StatusConflict)
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
		nullStr(req.Location), nullStr(req.Notes), therapistID).Scan(&aptID)
	if err != nil {
		http.Error(w, "Failed to create walk-in appointment", http.StatusInternalServerError)
		return
	}

	services.EnqueueCalendarSync("create", tenantID, aptID)
	patient, _ := getPatientByID(tenantID, patientID)
	apt, _ := getAppointment(tenantID, aptID)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"data": map[string]interface{}{"patient": patient, "appointment": apt},
	})
}

func ReceptionQuickRegisterV2(w http.ResponseWriter, r *http.Request) {
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
