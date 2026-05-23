package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"golang.org/x/crypto/curve25519"
)

type DisclosureService struct {
	PrivateKeyX25519 []byte // Server's private key managed in secure memory/KMS enclave
}

var ActiveDisclosureService *DisclosureService

// DecryptReportPayload decrypts an ECIES encrypted report package under audited moderator credentials
func (s *DisclosureService) DecryptReportPayload(ctx context.Context, reportID string, moderatorID string, reason string, ipAddress string) ([]byte, error) {
	if s.PrivateKeyX25519 == nil || len(s.PrivateKeyX25519) != 32 {
		return nil, errors.New("disclosure system encryption key is not initialized in secure enclave")
	}

	// 1. Audit check: Log immediately that decryption was requested
	_, err := database.PostgresDB.ExecContext(ctx, `
		INSERT INTO security_audit_logs (event_type, target_id, actor_id, actor_role, reason, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, "MODERATION_DECRYPTION_REQUEST", reportID, moderatorID, "moderator", "Justification: "+reason, ipAddress)
	if err != nil {
		return nil, fmt.Errorf("audit checkpoint failure: %w", err)
	}

	// 2. Retrieve ECIES report payload from Postgres
	var encryptedB64 string
	err = database.PostgresDB.QueryRowContext(ctx, `
		SELECT encrypted_payload 
		FROM abuse_reports 
		WHERE id = $1
	`, reportID).Scan(&encryptedB64)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("target report ledger entry does not exist")
		}
		return nil, fmt.Errorf("failed to fetch report payload: %w", err)
	}

	encryptedData, err := base64.StdEncoding.DecodeString(encryptedB64)
	if err != nil {
		return nil, errors.New("malformed report base64 packet")
	}

	if len(encryptedData) < 44 {
		return nil, errors.New("payload packet is too short (must contain 32B ephemeral pub, 12B nonce, and ciphertext)")
	}

	// ECIES Packet: [ Ephemeral Pub Key (32B) ] [ AES-GCM Nonce (12B) ] [ Ciphertext (variable) ]
	ephPub := encryptedData[0:32]
	nonce := encryptedData[32:44]
	ciphertext := encryptedData[44:]

	// 3. Compute shared secret via Curve25519 ECDH
	sharedSecret, err := curve25519.X25519(s.PrivateKeyX25519, ephPub)
	if err != nil {
		return nil, fmt.Errorf("ecdh shared secret derivation failed: %w", err)
	}

	// 4. Derive KEK via SHA-256 (simple static KDF matching frontend layout)
	kek := sha256.Sum256(sharedSecret)

	// 5. Decrypt using AES-256-GCM
	block, err := aes.NewCipher(kek[:])
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("failed to decrypt report payload: cryptographic authentication failure")
	}

	// 6. Update report status in Postgres
	_, _ = database.PostgresDB.ExecContext(ctx, `
		UPDATE abuse_reports 
		SET status = 'reviewed' 
		WHERE id = $1
	`, reportID)

	return plaintext, nil
}
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
	}
	return nil
}
