package crypto

import (
	"crypto/ed25519"
	"errors"
)

// VerifyEd25519Signature verifies a digital signature made with an Ed25519 private key.
func VerifyEd25519Signature(pubKeyBytes, message, signatureBytes []byte) (bool, error) {
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return false, errors.New("invalid Ed25519 public key size")
	}
	if len(signatureBytes) != ed25519.SignatureSize {
		return false, errors.New("invalid Ed25519 signature size")
	}

	isValid := ed25519.Verify(pubKeyBytes, message, signatureBytes)
	return isValid, nil
}
