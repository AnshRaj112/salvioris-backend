package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Feedback struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`

	// Feedback content
	Feedback string `bson:"feedback" json:"feedback"`

	// Optional: IP address for analytics (not personal info)
	IPAddress string `bson:"ip_address,omitempty" json:"ip_address,omitempty"`
}

