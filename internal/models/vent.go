package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Vent struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`

	// User ID (optional - nil for non-logged-in users)
	UserID *primitive.ObjectID `bson:"user_id,omitempty" json:"user_id,omitempty"`

	// Message content
	Message string `bson:"message" json:"message"`
}

