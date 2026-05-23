package crypto

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/hkdf"
)

// EncapsulateECIES encrypts the input plaintext (typically a 32-byte symmetric message key)
// to a recipient's public X25519 key.
// The output is format: [32-byte ephemeral public key] + [12-byte nonce] + [AES-GCM ciphertext + 16-byte auth tag].
func EncapsulateECIES(plaintext []byte, recipientPubBytes []byte) ([]byte, error) {
	if len(recipientPubBytes) != 32 {
		return nil, errors.New("recipient public key must be exactly 32 bytes")
	}

	// 1. Reconstruct the X25519 public key
	recipientPub, err := ecdh.X25519().NewPublicKey(recipientPubBytes)
	if err != nil {
		return nil, err
	}

	// 2. Generate an ephemeral X25519 keypair
	ephPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	ephPub := ephPriv.PublicKey().Bytes()

	// 3. Compute the Elliptic Curve Diffie-Hellman (ECDH) shared secret
	sharedSecret, err := ephPriv.ECDH(recipientPub)
	if err != nil {
		return nil, err
	}

	// 4. Derive KEK (Key Encryption Key) using HKDF-SHA256
	hkdfReader := hkdf.New(sha256.New, sharedSecret, nil, []byte("ECIES-X25519-AES-GCM"))
	kek := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, kek); err != nil {
		return nil, err
	}

	// 5. Encrypt plaintext using AES-256-GCM
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext, err := EncryptAES_GCM(plaintext, kek, nonce)
	if err != nil {
		return nil, err
	}

	// 6. Concatenate: ephPub (32) + nonce (12) + ciphertext
	packet := make([]byte, 0, len(ephPub)+len(nonce)+len(ciphertext))
	packet = append(packet, ephPub...)
	packet = append(packet, nonce...)
	packet = append(packet, ciphertext...)

	return packet, nil
}

// DecapsulateECIES decrypts the ECIES packet using the recipient's private X25519 key.
func DecapsulateECIES(packet []byte, recipientPrivBytes []byte) ([]byte, error) {
	if len(recipientPrivBytes) != 32 {
		return nil, errors.New("recipient private key must be exactly 32 bytes")
	}
	if len(packet) < 44 { // 32 (ephPub) + 12 (nonce)
		return nil, errors.New("packet is too short to be a valid ECIES package")
	}

	// 1. Parse the packet
	ephPubBytes := packet[:32]
	nonce := packet[32:44]
	ciphertext := packet[44:]

	// 2. Reconstruct recipient private key and ephemeral public key
	recipientPriv, err := ecdh.X25519().NewPrivateKey(recipientPrivBytes)
	if err != nil {
		return nil, err
	}
	ephPub, err := ecdh.X25519().NewPublicKey(ephPubBytes)
	if err != nil {
		return nil, err
	}

	// 3. Compute the shared secret
	sharedSecret, err := recipientPriv.ECDH(ephPub)
	if err != nil {
		return nil, err
	}

	// 4. Derive the same KEK
	hkdfReader := hkdf.New(sha256.New, sharedSecret, nil, []byte("ECIES-X25519-AES-GCM"))
	kek := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, kek); err != nil {
		return nil, err
	}

	// 5. Decrypt using AES-256-GCM
	plaintext, err := DecryptAES_GCM(ciphertext, kek, nonce)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
