package models

import (
	"time"
)

// User represents the public profile (anonymous identity)
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	IsActive  bool      `json:"is_active"`
	
	// Internal only - never returned in JSON
	PasswordHash string `json:"-"`
}

// UserRecovery represents private recovery data (encrypted)
type UserRecovery struct {
	ID            string    `json:"-"`
	UserID        string    `json:"-"`
	EmailEncrypted string   `json:"-"`
	PhoneEncrypted string   `json:"-"`
	CreatedAt     time.Time `json:"-"`
	UpdatedAt     time.Time `json:"-"`
}

// UserDevice represents device tracking data (for support/security)
type UserDevice struct {
	ID         string    `json:"-"`
	UserID     string    `json:"-"`
	DeviceToken string   `json:"-"`
	IPAddress  string    `json:"-"`
	UserAgent  string    `json:"-"`
	LastUsed   time.Time `json:"-"`
	CreatedAt  time.Time `json:"-"`
}

