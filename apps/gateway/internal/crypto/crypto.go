// Package crypto provides encryption utilities for sensitive data at rest.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// TokenEncryptor handles encryption and decryption of OAuth tokens.
type TokenEncryptor struct {
	gcm cipher.AEAD
}

// NewTokenEncryptor creates a new encryptor with the given 32-byte key.
// If key is nil or empty, encryption is disabled (tokens stored in plaintext).
func NewTokenEncryptor(key []byte) (*TokenEncryptor, error) {
	if len(key) == 0 {
		return &TokenEncryptor{gcm: nil}, nil // Encryption disabled
	}

	if len(key) != 32 {
		return nil, errors.New("encryption key must be exactly 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &TokenEncryptor{gcm: gcm}, nil
}

// Encrypt encrypts a plaintext token and returns a base64-encoded ciphertext.
// Returns the plaintext if encryption is disabled.
func (e *TokenEncryptor) Encrypt(plaintext string) (string, error) {
	if e.gcm == nil {
		return plaintext, nil // Encryption disabled
	}

	if plaintext == "" {
		return "", nil
	}

	// Create random nonce
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt (nonce is prepended to ciphertext)
	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode as base64 with prefix to identify encrypted tokens
	return "enc:" + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext and returns the plaintext.
// If the token is not encrypted (no "enc:" prefix), returns as-is.
func (e *TokenEncryptor) Decrypt(encoded string) (string, error) {
	// Check for encryption prefix
	if len(encoded) < 4 || encoded[:4] != "enc:" {
		return encoded, nil // Not encrypted, return as-is
	}

	if e.gcm == nil {
		return "", errors.New("token is encrypted but encryption is disabled")
	}

	// Decode base64
	ciphertext, err := base64.StdEncoding.DecodeString(encoded[4:])
	if err != nil {
		return "", err
	}

	// Extract nonce and decrypt
	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// IsEnabled returns true if encryption is enabled.
func (e *TokenEncryptor) IsEnabled() bool {
	return e.gcm != nil
}
