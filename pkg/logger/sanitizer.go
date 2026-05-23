package logger

import (
	"io"
	"regexp"
)

var (
	// Redacts potential PHI elements from logs/traces
	phiJSONRegex = regexp.MustCompile(`(?i)"(message|ciphertext|phone|email|password|recovery_secret|nonce|signature)"\s*:\s*"[^"]+"`)
	phoneRegex   = regexp.MustCompile(`\+?\d{1,4}?[-.\s]?\(?\d{1,3}?\)?[-.\s]?\d{1,4}[-.\s]?\d{1,4}[-.\s]?\d{1,9}`)
	emailRegex   = regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
)

// RedactPHIPayloads inspects logs and redacts patterns matching emails, phone numbers, or E2EE parameters
func RedactPHIPayloads(input string) string {
	// 1. Redact direct sensitive JSON parameters
	output := phiJSONRegex.ReplaceAllString(input, `"$1":"[REDACTED_PHI_SECURE]"`)

	// 2. Scrub standard email and telephone numbers
	output = emailRegex.ReplaceAllString(output, "[EMAIL_REDACTED]")
	output = phoneRegex.ReplaceAllString(output, "[PHONE_REDACTED]")

	return output
}

// RedactWriter intercepts writes and scrubs sensitive text.
type RedactWriter struct {
	Target io.Writer
}

func (w *RedactWriter) Write(p []byte) (n int, err error) {
	sanitized := RedactPHIPayloads(string(p))
	nOriginal := len(p)
	_, err = w.Target.Write([]byte(sanitized))
	if err != nil {
		return 0, err
	}
	return nOriginal, nil
}
