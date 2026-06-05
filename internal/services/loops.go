package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const loopsTransactionalURL = "https://app.loops.so/api/v1/transactional"

// OnboardingEmailData contains template variables for the patient onboarding email.
type OnboardingEmailData struct {
	Email             string
	PatientEmail      string
	PatientName       string
	TherapistName     string
	Username          string
	TemporaryPassword string
}

// SendOnboardingEmail sends the therapist-initiated patient onboarding email via Loops.
func SendOnboardingEmail(data OnboardingEmailData) error {
	apiKey := os.Getenv("LOOPS_API_KEY")
	transactionalID := os.Getenv("LOOPS_TRANSACTIONAL_ID")
	if apiKey == "" || transactionalID == "" {
		return fmt.Errorf("loops email is not configured")
	}

	payload := map[string]interface{}{
		"transactionalId": transactionalID,
		"email":           data.Email,
		"addToAudience":   true,
		"dataVariables": map[string]string{
			"email":             data.Email,
			"patientEmail":      data.PatientEmail,
			"patientName":       data.PatientName,
			"therapistName":     data.TherapistName,
			"username":          data.Username,
			"temporaryPassword": data.TemporaryPassword,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, loopsTransactionalURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("loops api returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
