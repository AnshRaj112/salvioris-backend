package models

import (
	"time"
)

type ViolationType string

const (
	ViolationTypeThreat    ViolationType = "threat"
	ViolationTypeSelfHarm  ViolationType = "self_harm"
)

type Violation struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`

	// User information
	UserID    *string `json:"user_id,omitempty"`
	IPAddress string  `json:"ip_address"`

	// Violation details
	Type        ViolationType `json:"type"`
	Message     string        `json:"message"`
	VentID      string        `json:"vent_id,omitempty"`

	// Action taken
	ActionTaken string `json:"action_taken"` // "warning", "blocked"
}

type BlockedIP struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`

	IPAddress string `json:"ip_address"`
	Reason    string `json:"reason"`
	IsActive  bool   `json:"is_active"`
}

