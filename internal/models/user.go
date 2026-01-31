package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`

	Name     string `bson:"name" json:"name"`
	Email    string `bson:"email" json:"email"`
	Password string `bson:"password" json:"-"` // Don't return password in JSON

	// Address fields
	Street  string `bson:"street,omitempty" json:"street,omitempty"`
	City    string `bson:"city,omitempty" json:"city,omitempty"`
	State   string `bson:"state,omitempty" json:"state,omitempty"`
	ZipCode string `bson:"zip_code,omitempty" json:"zip_code,omitempty"`
	Country string `bson:"country,omitempty" json:"country,omitempty"`
}

