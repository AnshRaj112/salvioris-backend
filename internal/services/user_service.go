package services

import (
	"database/sql"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

// GetUsernameByID retrieves username by user ID (for anonymous display)
func GetUsernameByID(userID string) (string, error) {
	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return "", err
	}

	var username string
	err = database.PostgresDB.QueryRow(`
		SELECT username FROM users WHERE id = $1 AND is_active = TRUE
	`, parsedID).Scan(&username)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // User not found or inactive
		}
		return "", err
	}

	return username, nil
}

// GetUserIDByUsername retrieves user ID by username
func GetUserIDByUsername(username string) (string, error) {
	var userID uuid.UUID
	err := database.PostgresDB.QueryRow(`
		SELECT id FROM users WHERE LOWER(username) = $1 AND is_active = TRUE
	`, username).Scan(&userID)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}

	return userID.String(), nil
}

