package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ViolationType string

const (
	ViolationTypeThreat    ViolationType = "threat"
	ViolationTypeSelfHarm  ViolationType = "self_harm"
)

type Violation struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`

	// User information
	UserID    *primitive.ObjectID `bson:"user_id,omitempty" json:"user_id,omitempty"`
	IPAddress string              `bson:"ip_address" json:"ip_address"`

	// Violation details
	Type        ViolationType `bson:"type" json:"type"`
	Message     string        `bson:"message" json:"message"`
	VentID      string        `bson:"vent_id,omitempty" json:"vent_id,omitempty"`

	// Action taken
	ActionTaken string `bson:"action_taken" json:"action_taken"` // "warning", "blocked"
}

type BlockedIP struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	ExpiresAt time.Time          `bson:"expires_at" json:"expires_at"`

	IPAddress string `bson:"ip_address" json:"ip_address"`
	Reason    string `bson:"reason" json:"reason"`
	IsActive  bool   `bson:"is_active" json:"is_active"`
}

