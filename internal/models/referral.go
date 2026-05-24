package models

import (
	"time"

	"github.com/google/uuid"
)

type ReferralCode struct {
	ID          uuid.UUID  `json:"id"`
	TherapistID uuid.UUID  `json:"therapist_id"`
	Code        string     `json:"code"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	UsageLimit  *int       `json:"usage_limit,omitempty"`
	UsageCount  int        `json:"usage_count"`
	IsRevoked   bool       `json:"is_revoked"`
}

type ReferralAnalytics struct {
	TotalCodes   int `json:"total_codes"`
	ActiveCodes  int `json:"active_codes"`
	TotalSignups int `json:"total_signups"`
}

type ConnectedUser struct {
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	ConnectedAt  time.Time `json:"connected_at"`
	Type         string    `json:"connection_type"`
}

type ConnectionRequest struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username,omitempty"`
	TherapistID uuid.UUID `json:"therapist_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	Note        string    `json:"note,omitempty"`
}

type Notification struct {
	ID            uuid.UUID `json:"id"`
	RecipientID   uuid.UUID `json:"recipient_id"`
	RecipientRole string    `json:"recipient_role"`
	Title         string    `json:"title"`
	Message       string    `json:"message"`
	Type          string    `json:"type"`
	IsRead        bool      `json:"is_read"`
	CreatedAt     time.Time `json:"created_at"`
	Data          string    `json:"data,omitempty"`
}
