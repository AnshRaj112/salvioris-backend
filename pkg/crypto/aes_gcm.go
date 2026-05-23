package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

// EncryptAES_GCM encrypts plaintext using AES-256-GCM with the provided key and 96-bit nonce.
func EncryptAES_GCM(plaintext, key, nonce []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be exactly 32 bytes for AES-256")
	}
	if len(nonce) != 12 {
		return nil, errors.New("nonce must be exactly 12 bytes (96-bit) for GCM")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptAES_GCM decrypts ciphertext using AES-256-GCM with the provided key and 96-bit nonce.
func DecryptAES_GCM(ciphertext, key, nonce []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be exactly 32 bytes for AES-256")
	}
	if len(nonce) != 12 {
		return nil, errors.New("nonce must be exactly 12 bytes (96-bit) for GCM")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
