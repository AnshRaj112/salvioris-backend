package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/AnshRaj112/serenify-backend/pkg/crypto"
	"github.com/google/uuid"
	"golang.org/x/crypto/curve25519"
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

	if len(packet) < 44 {
		return nil, errors.New("report packet is too short")
	}

	// Try Method A: HKDF ECIES-X25519 decryption (via pkg/crypto)
	decryptedBytes, err := crypto.DecapsulateECIES(packet, c.PrivateKey)
	if err == nil {
		return decryptedBytes, nil
	}

	// Try Method B: Static SHA-256 KDF (from previous disclosure.go)
	ephPub := packet[0:32]
	nonce := packet[32:44]
	ciphertext := packet[44:]

	sharedSecret, err := curve25519.X25519(c.PrivateKey, ephPub)
	if err == nil {
		// Try SHA-256 KDF
		hashKek := sha256.Sum256(sharedSecret)
		block, err := aes.NewCipher(hashKek[:])
		if err == nil {
			aesgcm, err := cipher.NewGCM(block)
			if err == nil {
				decryptedBytes, err = aesgcm.Open(nil, nonce, ciphertext, nil)
				if err == nil {
					return decryptedBytes, nil
				}
			}
		}

		// Try Method C: Direct ECDH (no SHA-256, no HKDF)
		block, err = aes.NewCipher(sharedSecret)
		if err == nil {
			aesgcm, err := cipher.NewGCM(block)
			if err == nil {
				decryptedBytes, err = aesgcm.Open(nil, nonce, ciphertext, nil)
				if err == nil {
					return decryptedBytes, nil
				}
			}
		}
	}

	return nil, errors.New("failed to decrypt report payload: cryptographic authentication failure in all standard/fallback formats")
}

// DisclosureService manages the governed report disclosure workflows.
type DisclosureService struct {
	PrivateKeyX25519 []byte // Server's private key managed in secure memory/KMS enclave
	KMS              KMSClientInterface
}

var ActiveDisclosureService *DisclosureService

func (s *DisclosureService) ensureKMSInitialized() {
	if s.KMS == nil {
		if len(s.PrivateKeyX25519) == 32 {
			s.KMS = &LocalKMSEscrowClient{PrivateKey: s.PrivateKeyX25519}
		} else {
			encKey := os.Getenv("ENCRYPTION_KEY")
			if encKey != "" {
				if keyBytes, err := base64.StdEncoding.DecodeString(encKey); err == nil && len(keyBytes) == 32 {
					s.PrivateKeyX25519 = keyBytes
					s.KMS = &LocalKMSEscrowClient{PrivateKey: keyBytes}
				}
			}
		}
	}
}

// InitDisclosureService registers the global service with the HSM private key.
func InitDisclosureService(kmsPrivateKey []byte) {
	ActiveDisclosureService = &DisclosureService{
		PrivateKeyX25519: kmsPrivateKey,
		KMS:              &LocalKMSEscrowClient{PrivateKey: kmsPrivateKey},
	}
}

// InitActiveDisclosureService initializes the global service using a base64-encoded key.
func InitActiveDisclosureService(privateKeyB64 string) error {
	privKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return fmt.Errorf("invalid disclosure key base64 formatting: %w", err)
	}
	if len(privKeyBytes) != 32 {
		return errors.New("disclosure key must be exactly 32 bytes X25519 secret key")
	}
	ActiveDisclosureService = &DisclosureService{
		PrivateKeyX25519: privKeyBytes,
		KMS:              &LocalKMSEscrowClient{PrivateKey: privKeyBytes},
	}
	return nil
}

// DecryptReportPayload verifies the moderator request, logs the event in PostgreSQL, and decrypts the user-disclosed package.
func (s *DisclosureService) DecryptReportPayload(ctx context.Context, reportID string, moderatorID string, reason string, ipAddress string) (string, error) {
	s.ensureKMSInitialized()
	if s.KMS == nil {
		return "", errors.New("disclosure KMS client is not initialized")
	}

	// 1. Strict Validation: Ensure that the report exists in the abuse_reports ledger.
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
		INSERT INTO security_audit_logs (id, event_type, target_id, actor_id, actor_role, reason, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, uuid.New(), "GOVERNED_DISCLOSURE_DECRYPTION", reportID, moderatorID, "moderator", reason, ipAddress, time.Now().UTC())
	if err != nil {
		// Fallback to inserting without ID and actor_role if schema lacks it (just in case)
		_, err = database.PostgresDB.ExecContext(ctx, `
			INSERT INTO security_audit_logs (event_type, target_id, actor_id, reason, ip_address, created_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
		`, "MODERATION_DECRYPTION_REQUEST", reportID, moderatorID, "Justification: "+reason, ipAddress)
		if err != nil {
			return "", fmt.Errorf("audit logging failed; decryption aborted: %w", err)
		}
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

func init() {
	// Automatically initialize the active disclosure service with a fallback key
	// for development and out-of-the-box operation.
	encKey := os.Getenv("ENCRYPTION_KEY")
	var keyBytes []byte
	if encKey != "" {
		if decoded, err := base64.StdEncoding.DecodeString(encKey); err == nil && len(decoded) == 32 {
			keyBytes = decoded
		}
	}
	if len(keyBytes) != 32 {
		keyBytes = make([]byte, 32)
		_, _ = rand.Read(keyBytes)
	}

	ActiveDisclosureService = &DisclosureService{
		PrivateKeyX25519: keyBytes,
		KMS:              &LocalKMSEscrowClient{PrivateKey: keyBytes},
	}
}
