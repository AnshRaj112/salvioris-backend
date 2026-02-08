package utils

import (
	"regexp"
	"strings"
	"unicode"
)

const (
	MinUsernameLength = 3
	MaxUsernameLength = 20
)

var (
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
)

// ValidateUsername validates username format
// Rules: 3-20 characters, letters, numbers, underscores only
func ValidateUsername(username string) error {
	username = strings.TrimSpace(username)
	
	if len(username) < MinUsernameLength {
		return &ValidationError{Field: "username", Message: "Username must be at least 3 characters"}
	}
	
	if len(username) > MaxUsernameLength {
		return &ValidationError{Field: "username", Message: "Username must be at most 20 characters"}
	}
	
	if !usernameRegex.MatchString(username) {
		return &ValidationError{Field: "username", Message: "Username can only contain letters, numbers, and underscores"}
	}
	
	// Check if it starts with a letter or number (not underscore)
	if len(username) > 0 && !(unicode.IsLetter(rune(username[0])) || unicode.IsNumber(rune(username[0]))) {
		return &ValidationError{Field: "username", Message: "Username must start with a letter or number"}
	}
	
	return nil
}

// NormalizeUsername converts username to lowercase for storage
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

