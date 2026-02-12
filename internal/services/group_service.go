package services

import (
	"database/sql"
	"strings"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/google/uuid"
)

// slugify generates a URL-friendly slug from a group name.
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, s)
	s = strings.Trim(s, "-")
	if s == "" {
		s = "group"
	}
	return s
}

// GenerateUniqueGroupSlug generates a globally-unique slug for a group.
func GenerateUniqueGroupSlug(name string) string {
	base := slugify(name)
	slug := base

	var exists bool
	counter := 1
	for {
		err := database.PostgresDB.QueryRow(`
			SELECT EXISTS(SELECT 1 FROM groups WHERE LOWER(slug) = LOWER($1))
		`, slug).Scan(&exists)
		if err != nil {
			// On any error, fall back to a UUID-based slug to avoid blocking group creation.
			return base + "-" + uuid.New().String()
		}
		if !exists {
			return slug
		}
		counter++
		slug = base + "-" + strings.TrimSpace(strings.ToLower(strings.ReplaceAll(uuid.New().String()[:8], " ", "")))
	}
}

// CanUserSendToGroup validates that a user is allowed to send to a group.
// Rules:
//  - user must be creator OR have a membership in group_members
//  - group must exist
func CanUserSendToGroup(userID string, groupID string) (bool, string) {
	u, err := uuid.Parse(userID)
	if err != nil {
		return false, ""
	}
	g, err := uuid.Parse(groupID)
	if err != nil {
		return false, ""
	}

	// Check creator or member
	var username sql.NullString
	err = database.PostgresDB.QueryRow(`
		SELECT u.username
		FROM users u
		WHERE u.id = $1
	`, u).Scan(&username)
	if err != nil {
		return false, ""
	}

	var exists bool
	err = database.PostgresDB.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM groups g
			WHERE g.id = $1 AND (g.created_by = $2 OR EXISTS (
				SELECT 1 FROM group_members gm WHERE gm.group_id = g.id AND gm.user_id = $2
			))
		)
	`, g, u).Scan(&exists)
	if err != nil || !exists {
		return false, ""
	}

	return true, username.String
}


