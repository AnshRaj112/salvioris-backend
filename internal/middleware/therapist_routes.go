package middleware

import "strings"

// IsTherapistRoute returns true for therapist dashboard and auth API paths.
func IsTherapistRoute(path string) bool {
	return strings.HasPrefix(path, "/api/therapist/") ||
		strings.HasPrefix(path, "/api/auth/therapist/")
}
