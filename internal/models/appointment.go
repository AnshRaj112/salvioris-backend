package models

import (
	"time"

	"github.com/google/uuid"
)

type Appointment struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	PatientID    uuid.UUID  `json:"patient_id"`
	TherapistID  uuid.UUID  `json:"therapist_id"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	StartsAt     time.Time  `json:"starts_at"`
	EndsAt       time.Time  `json:"ends_at"`
	MeetingLink  string     `json:"meeting_link,omitempty"`
	Location     string     `json:"location,omitempty"`
	Notes        string     `json:"notes,omitempty"`
	ReminderSent bool       `json:"reminder_sent"`
	CreatedBy    *uuid.UUID `json:"created_by,omitempty"`
	CancelledAt  *time.Time `json:"cancelled_at,omitempty"`
	CancelReason string     `json:"cancel_reason,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type AvailabilitySlot struct {
	ID              uuid.UUID `json:"id"`
	TenantID        uuid.UUID `json:"tenant_id"`
	TherapistID     uuid.UUID `json:"therapist_id"`
	DayOfWeek       int       `json:"day_of_week"`
	StartTime       string    `json:"start_time"`
	EndTime         string    `json:"end_time"`
	SlotDurationMin int       `json:"slot_duration_min"`
	IsActive        bool      `json:"is_active"`
}
