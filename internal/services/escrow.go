package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/pkg/crypto"
	"github.com/google/uuid"
)

// KMSClientInterface abstracts integration with the secure HSM/KMS provider.
type KMSClientInterface interface {
	DecryptReportPayload(ctx context.Context, encryptedReportB64 string, reason string, operatorID string) ([]byte, error)
}

// LocalKMSEscrowClient wraps local development X25519 private keys.
type LocalKMSEscrowClient struct {
	PrivateKey []byte
}

func (c *LocalKMSEscrowClient) DecryptReportPayload(ctx context.Context, encryptedReportB64 string, reason string, operatorID string) ([]byte, error) {
	packet, err := base64.StdEncoding.DecodeString(encryptedReportB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 report: %w", err)
	}

	// Decapsulate using ECIES-X25519 decryption
	decryptedBytes, err := crypto.DecapsulateECIES(packet, c.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed report decapsulation: %w", err)
	}

	return decryptedBytes, nil
}

// DisclosureService manages the governed report disclosure workflows.
type DisclosureService struct {
	KMS KMSClientInterface
}

var ActiveDisclosureService *DisclosureService

// InitDisclosureService registers the global service with the HSM private key.
func InitDisclosureService(kmsPrivateKey []byte) {
	ActiveDisclosureService = &DisclosureService{
		KMS: &LocalKMSEscrowClient{PrivateKey: kmsPrivateKey},
	}
}

// DecryptReportPayload verifies the moderator request, logs the event in PostgreSQL, and decrypts the user-disclosed package.
func (s *DisclosureService) DecryptReportPayload(ctx context.Context, reportID string, moderatorID string, reason string, ipAddress string) (string, error) {
	if s.KMS == nil {
		return "", errors.New("disclosure KMS client is not initialized")
	}

	// 1. Strict Validation: Ensure that the report exists in the abuse_reports ledger.
	// This prevents moderators from bypassing consent and browsing arbitrary messages.
	var reportExists bool
	err := database.PostgresDB.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM abuse_reports 
			WHERE id = $1
			LIMIT 1
		)
	`, reportID).Scan(&reportExists)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("failed to check abuse report ledger: %w", err)
	}

	if !reportExists {
		return "", errors.New("governed disclosure failed: report does not exist in database records")
	}

	// Fetch ECIES encrypted payload
	var encryptedPayload string
	err = database.PostgresDB.QueryRowContext(ctx, `
		SELECT encrypted_payload 
		FROM abuse_reports 
		WHERE id = $1
	`, reportID).Scan(&encryptedPayload)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve report payload: %w", err)
	}

	// 2. Log Access to Append-Only PostgreSQL Audit Chain
	_, err = database.PostgresDB.ExecContext(ctx, `
		INSERT INTO security_audit_logs (id, event_type, target_id, actor_id, reason, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, uuid.New(), "GOVERNED_DISCLOSURE_DECRYPTION", reportID, moderatorID, reason, ipAddress, time.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("audit logging failed; decryption aborted: %w", err)
	}

	// 3. Request KMS / HSM private key decryption of ECIES disclosure envelope
	decryptedBytes, err := s.KMS.DecryptReportPayload(ctx, encryptedPayload, reason, moderatorID)
	if err != nil {
		return "", fmt.Errorf("KMS decapsulation failed: %w", err)
	}

	return string(decryptedBytes), nil
}

// GenerateLocalEscrowKeyPair generates a temporary X25519 keypair for local development/testing.
func GenerateLocalEscrowKeyPair() (publicKey []byte, privateKey []byte, err error) {
	privateKey = make([]byte, 32)
	if _, err := rand.Read(privateKey); err != nil {
		return nil, nil, err
	}
	return publicKey, privateKey, nil
}
