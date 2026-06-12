package models

import (
	"time"

	"github.com/google/uuid"
)

type Prescription struct {
	ID            uuid.UUID  `json:"id"`
	TenantID      uuid.UUID  `json:"tenant_id"`
	PatientID     uuid.UUID  `json:"patient_id"`
	TherapistID   uuid.UUID  `json:"therapist_id"`
	MedicineName  string     `json:"medicine_name"`
	Dosage        string     `json:"dosage"`
	Frequency     string     `json:"frequency"`
	DurationDays  *int       `json:"duration_days,omitempty"`
	Notes         string     `json:"notes,omitempty"`
	Status        string     `json:"status"`
	PrescribedAt  time.Time  `json:"prescribed_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	DiscontinuedAt *time.Time `json:"discontinued_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Task struct {
	ID          uuid.UUID  `json:"id"`
	TenantID    uuid.UUID  `json:"tenant_id"`
	PatientID   uuid.UUID  `json:"patient_id"`
	AssignedBy  uuid.UUID  `json:"assigned_by"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Category    string     `json:"category,omitempty"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	ReminderAt  *time.Time `json:"reminder_at,omitempty"`
	Status      string     `json:"status"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	PatientNotes string    `json:"patient_notes,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
