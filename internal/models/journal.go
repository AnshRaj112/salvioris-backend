package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Journal represents a private journaling entry for a user
type Journal struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at"`
	UserIDString string             `bson:"user_id_string,omitempty" json:"user_id,omitempty"`
	Title        string             `bson:"title" json:"title"`
	Content      string             `bson:"content" json:"content"`
}


