package utils

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	saltLength    = 16
	keyLength     = 32
	timeCost      = 3
	memoryCost    = 64 * 1024
	parallelism   = 2
)

// HashPassword hashes a password using Argon2id
func HashPassword(password string) (string, error) {
	// Generate a random salt
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Hash the password with Argon2id
	hash := argon2.IDKey([]byte(password), salt, timeCost, memoryCost, parallelism, keyLength)

	// Encode salt and hash to base64
	saltBase64 := base64.RawStdEncoding.EncodeToString(salt)
	hashBase64 := base64.RawStdEncoding.EncodeToString(hash)

	// Return format: $argon2id$v=19$m=65536,t=3,p=2$salt$hash
	return "$argon2id$v=19$m=65536,t=3,p=2$" + saltBase64 + "$" + hashBase64, nil
}

// VerifyPassword verifies a password against a hash
func VerifyPassword(password, hashedPassword string) (bool, error) {
	// Parse the hash format
	parts := strings.Split(hashedPassword, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("invalid hash format")
	}

	// Decode salt and hash
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	// Compute hash of the provided password
	computedHash := argon2.IDKey([]byte(password), salt, timeCost, memoryCost, parallelism, keyLength)

	// Constant-time comparison
	if len(computedHash) != len(hash) {
		return false, nil
	}

	result := 0
	for i := 0; i < len(hash); i++ {
		result |= int(computedHash[i]) ^ int(hash[i])
	}

	return result == 0, nil
}

