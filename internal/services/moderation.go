package services

import (
	"database/sql"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/internal/models"
	"github.com/google/uuid"
)

// Base canonical words - the ONLY source of truth
// These are the clean, real words we're looking for
var baseThreatWords = []string{
	"rape",
	"kill",
	"murder",
	"death",
	"die",
	"assault",
	"attack",
	"harm",
	"hurt",
	"destroy",
	"eliminate",
	"execute",
	"shoot",
	"stab",
	"strangle",
	"threat",
	"threatening",
	"revenge",
	"retaliate",
	"slaughter",
	"massacre",
	"annihilate",
}

var baseSelfHarmWords = []string{
	"suicide",
	"kill myself",
	"end my life",
	"take my life",
	"end it all",
	"self harm",
	"cut myself",
	"hurt myself",
	"harm myself",
	"want to die",
	"wish I was dead",
	"not worth living",
	"better off dead",
	"end myself",
	"unalive",
}

// CleanText normalizes and cleans text to canonical form
// This is the ONLY function that transforms input for confirmation
func CleanText(text string) string {
	// Step 1: Convert to lowercase
	cleaned := strings.ToLower(text)
	
	// Step 2: Replace obfuscation characters with their letter equivalents
	replacements := map[string]string{
		"@": "a",
		"4": "a",
		"3": "e",
		"!": "i",
		"1": "i",
		"0": "o",
		"$": "s",
		"5": "s",
		"7": "t",
		"+": "t",
		"а": "a", // Cyrillic 'а' looks like Latin 'a'
		"е": "e", // Cyrillic 'е' looks like Latin 'e'
		"і": "i", // Cyrillic 'і' looks like Latin 'i'
		"о": "o", // Cyrillic 'о' looks like Latin 'o'
		"р": "p", // Cyrillic 'р' looks like Latin 'p'
	}
	
	for old, new := range replacements {
		cleaned = strings.ReplaceAll(cleaned, old, new)
	}
	
	// Step 3: Remove all non-letter characters (spaces, punctuation, etc.)
	// Keep only letters
	var builder strings.Builder
	for _, r := range cleaned {
		if unicode.IsLetter(r) {
			builder.WriteRune(r)
		} else {
			// Replace non-letters with space for word separation
			builder.WriteRune(' ')
		}
	}
	cleaned = builder.String()
	
	// Step 4: Collapse repeated characters (rrraaaapeee -> rape)
	cleaned = collapseRepeats(cleaned)
	
	// Step 5: Normalize whitespace (multiple spaces to single space)
	spaceRegex := regexp.MustCompile(`\s+`)
	cleaned = spaceRegex.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)
	
	return cleaned
}

// collapseRepeats reduces repeated LETTER characters to single character
// Preserves spaces and only collapses letter repeats
// Example: "rrraaaapeee" -> "rape", "kill kill" -> "kil kil" (preserves space)
func collapseRepeats(text string) string {
	if len(text) == 0 {
		return text
	}
	
	var result strings.Builder
	lastChar := rune(0)
	lastWasLetter := false
	
	for _, char := range text {
		isLetter := unicode.IsLetter(char)
		
		// Only collapse if it's a letter and the last char was also a letter
		if isLetter && lastWasLetter && char == lastChar {
			// Skip this repeated letter
			continue
		}
		
		// Write the character
		result.WriteRune(char)
		lastChar = char
		lastWasLetter = isLetter
	}
	
	return result.String()
}

// IsWordConfirmed checks if a cleaned input matches a canonical word
// This is the core confirmation function
func IsWordConfirmed(cleanedInput string, canonicalWord string) bool {
	// Exact match
	if cleanedInput == canonicalWord {
		return true
	}
	
	// Contains match (for phrases like "kill myself")
	if strings.Contains(cleanedInput, canonicalWord) {
		return true
	}
	
	return false
}

// ContainsConfirmedWord checks if cleaned text contains any confirmed base word
func ContainsConfirmedWord(cleanedText string, baseWords []string) (bool, []string) {
	var confirmedWords []string
	
	// Split cleaned text into words for word-boundary matching
	words := strings.Fields(cleanedText)
	
	for _, baseWord := range baseWords {
		// Check exact match first (for single words like "kill")
		if cleanedText == baseWord {
			confirmedWords = append(confirmedWords, baseWord)
			continue
		}
		
		// Check if base word is contained in cleaned text
		if strings.Contains(cleanedText, baseWord) {
			// For single words, verify it appears as a whole word (not substring)
			// e.g., "skill" should NOT match "kill"
			if len(strings.Fields(baseWord)) == 1 {
				// Single word - check if it appears as a complete word
				for _, w := range words {
					if w == baseWord {
						confirmedWords = append(confirmedWords, baseWord)
						break
					}
				}
			} else {
				// Multi-word phrase (like "kill myself") - contains is sufficient
				confirmedWords = append(confirmedWords, baseWord)
			}
		}
	}
	
	return len(confirmedWords) > 0, confirmedWords
}

// CheckContent checks if the message contains threats or self-harm content
// Uses the confirmation pattern: Clean → Compare with canonical dictionary
func CheckContent(message string) (hasThreat bool, hasSelfHarm bool, matchedKeywords []string) {
	// Step 1: Clean the input to canonical form
	cleanedText := CleanText(message)
	
	// Step 2: Check against base threat words (canonical dictionary)
	threatConfirmed, threatWords := ContainsConfirmedWord(cleanedText, baseThreatWords)
	if threatConfirmed {
		hasThreat = true
		matchedKeywords = append(matchedKeywords, threatWords...)
	}
	
	// Step 3: Check against base self-harm words (canonical dictionary)
	selfHarmConfirmed, selfHarmWords := ContainsConfirmedWord(cleanedText, baseSelfHarmWords)
	if selfHarmConfirmed {
		hasSelfHarm = true
		matchedKeywords = append(matchedKeywords, selfHarmWords...)
	}
	
	return hasThreat, hasSelfHarm, matchedKeywords
}

// GetIPAddress extracts IP address from request
func GetIPAddress(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies/load balancers)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Take the first IP if there are multiple
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}
	
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Remove port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	
	return ip
}

// RecordViolation records a content violation
func RecordViolation(userID *uuid.UUID, ipAddress string, violationType models.ViolationType, message string, ventID string, actionTaken string) error {
	var err error
	if userID != nil {
		_, err = database.PostgresDB.Exec(`
			INSERT INTO violations (id, created_at, user_id, ip_address, type, message, vent_id, action_taken)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, uuid.New(), time.Now(), userID, ipAddress, string(violationType), message, ventID, actionTaken)
	} else {
		_, err = database.PostgresDB.Exec(`
			INSERT INTO violations (id, created_at, user_id, ip_address, type, message, vent_id, action_taken)
			VALUES ($1, $2, NULL, $3, $4, $5, $6, $7)
		`, uuid.New(), time.Now(), ipAddress, string(violationType), message, ventID, actionTaken)
	}
	return err
}

// GetViolationCount gets the number of violations for an IP address
func GetViolationCount(ipAddress string) (int64, error) {
	var count int64
	err := database.PostgresDB.QueryRow(`
		SELECT COUNT(*) FROM violations
		WHERE ip_address = $1 AND created_at >= $2
	`, ipAddress, time.Now().Add(-24*time.Hour)).Scan(&count)
	return count, err
}

// IsIPBlocked checks if an IP address is currently blocked
func IsIPBlocked(ipAddress string) (bool, *models.BlockedIP, error) {
	var blockedIP models.BlockedIP
	var id uuid.UUID
	err := database.PostgresDB.QueryRow(`
		SELECT id, created_at, expires_at, ip_address, reason, is_active
		FROM blocked_ips
		WHERE ip_address = $1 AND is_active = true AND expires_at > $2
		LIMIT 1
	`, ipAddress, time.Now()).Scan(&id, &blockedIP.CreatedAt, &blockedIP.ExpiresAt, &blockedIP.IPAddress, &blockedIP.Reason, &blockedIP.IsActive)
	
	if err == sql.ErrNoRows {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	
	// Convert UUID to string for ID field
	blockedIP.ID = id.String()
	return true, &blockedIP, nil
}

// BlockIP blocks an IP address for a specified duration
func BlockIP(ipAddress string, reason string, durationDays int) error {
	// First, deactivate any existing blocks for this IP
	_, err := database.PostgresDB.Exec(`
		UPDATE blocked_ips SET is_active = false
		WHERE ip_address = $1 AND is_active = true
	`, ipAddress)
	if err != nil {
		return err
	}
	
	// Create new block
	_, err = database.PostgresDB.Exec(`
		INSERT INTO blocked_ips (id, created_at, expires_at, ip_address, reason, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New(), time.Now(), time.Now().Add(time.Duration(durationDays)*24*time.Hour), ipAddress, reason, true)
	return err
}

// UnblockIP unblocks an IP address (admin function)
func UnblockIP(ipAddress string) error {
	_, err := database.PostgresDB.Exec(`
		UPDATE blocked_ips SET is_active = false
		WHERE ip_address = $1 AND is_active = true
	`, ipAddress)
	return err
}

// CleanupOldViolations removes violations older than specified hours
// This keeps the database clean while preserving blocked IPs
func CleanupOldViolations(hoursOld int) error {
	// Calculate cutoff time
	cutoffTime := time.Now().Add(-time.Duration(hoursOld) * time.Hour)
	
	// Delete violations older than cutoff time
	// Note: This does NOT delete blocked_ips - those are kept separately
	result, err := database.PostgresDB.Exec(`
		DELETE FROM violations WHERE created_at < $1
	`, cutoffTime)
	
	if err != nil {
		return err
	}
	
	// Log cleanup (optional, can be removed if not needed)
	if rowsAffected, err := result.RowsAffected(); err == nil && rowsAffected > 0 {
		// You can add logging here if needed
		// log.Printf("Cleaned up %d old violations (older than %d hours)", rowsAffected, hoursOld)
	}
	
	return nil
}

// StartViolationCleanup starts a background goroutine that periodically cleans up old violations
// Default: cleans up violations older than 6 hours, runs every hour
func StartViolationCleanup(cleanupIntervalHours int, violationsAgeHours int) {
	if cleanupIntervalHours <= 0 {
		cleanupIntervalHours = 1 // Default: run every hour
	}
	if violationsAgeHours <= 0 {
		violationsAgeHours = 6 // Default: delete violations older than 6 hours
	}
	
	go func() {
		ticker := time.NewTicker(time.Duration(cleanupIntervalHours) * time.Hour)
		defer ticker.Stop()
		
		// Run cleanup immediately on startup
		_ = CleanupOldViolations(violationsAgeHours)
		
		// Then run periodically
		for range ticker.C {
			_ = CleanupOldViolations(violationsAgeHours)
		}
	}()
}

