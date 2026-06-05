package models

import (
	"time"

	"github.com/google/uuid"
)

type PatientOnboarding struct {
	ID             uuid.UUID `json:"id"`
	TherapistID    uuid.UUID `json:"therapist_id"`
	UserID         uuid.UUID `json:"user_id"`
	PatientName    string    `json:"patient_name"`
	PatientEmail   string    `json:"patient_email"`
	Username       string    `json:"username"`
	ReferralCode   string    `json:"referral_code"`
	ReferralCodeID uuid.UUID `json:"referral_code_id"`
	OnboardedAt    time.Time `json:"onboarded_at"`
}
