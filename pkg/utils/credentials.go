package utils

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

const tempPasswordLength = 12

var usernameChars = []byte("abcdefghijkmnopqrstuvwxyz23456789")

// GenerateTemporaryPassword creates a random password that meets the minimum 8-character requirement.
func GenerateTemporaryPassword() (string, error) {
	b := make([]byte, tempPasswordLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	password := make([]byte, tempPasswordLength)
	for i := range password {
		password[i] = usernameChars[int(b[i])%len(usernameChars)]
	}
	return string(password), nil
}

// GenerateUniqueUsername creates a unique anonymous username for a new patient account.
func GenerateUniqueUsername(db *sql.DB) (string, error) {
	for attempt := 0; attempt < 25; attempt++ {
		username, err := randomUsernameCandidate()
		if err != nil {
			return "", err
		}

		if err := ValidateUsername(username); err != nil {
			continue
		}

		normalized := NormalizeUsername(username)
		var existing string
		err = db.QueryRow(
			"SELECT username FROM users WHERE LOWER(username) = $1",
			normalized,
		).Scan(&existing)
		if err == sql.ErrNoRows {
			return normalized, nil
		}
		if err != nil {
			return "", err
		}
	}

	return "", errors.New("failed to generate unique username")
}

func randomUsernameCandidate() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Prefix with a letter and append 7 hex chars (11 chars total, always unique enough to retry on collision).
	return fmt.Sprintf("p%s", hex.EncodeToString(b)[:7]), nil
}
