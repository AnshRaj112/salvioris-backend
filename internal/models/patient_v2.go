package models

import (
	"time"

	"github.com/google/uuid"
)

type Patient struct {
	ID                  uuid.UUID  `json:"id"`
	TenantID            uuid.UUID  `json:"tenant_id"`
	UserID              *uuid.UUID `json:"user_id,omitempty"`
	FullName            string     `json:"full_name"`
	DateOfBirth         *time.Time `json:"date_of_birth,omitempty"`
	Gender              string     `json:"gender,omitempty"`
	Phone               string     `json:"phone,omitempty"`
	Email               string     `json:"email,omitempty"`
	EmergencyContact    string     `json:"emergency_contact,omitempty"`
	Address             string     `json:"address,omitempty"`
	AssignedTherapistID *uuid.UUID `json:"assigned_therapist_id,omitempty"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
