// Package auth provides TOTP-based multi-factor authentication.
package auth

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// MFASetup contains the information needed to set up MFA.
type MFASetup struct {
	Secret      string   `json:"secret"`
	URL         string   `json:"url"`
	BackupCodes []string `json:"backup_codes"`
}

// MFAConfig holds MFA configuration.
type MFAConfig struct {
	Issuer      string
	BackupCodes int
}

// DefaultMFAConfig returns default MFA configuration.
func DefaultMFAConfig() MFAConfig {
	return MFAConfig{
		Issuer:      "FullstackAIWorkflow",
		BackupCodes: 10,
	}
}

// GenerateTOTPSecret generates a new TOTP secret for a user.
func GenerateTOTPSecret(email string, cfg MFAConfig) (*MFASetup, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      cfg.Issuer,
		AccountName: email,
		Period:      30,
		SecretSize:  32,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	// Generate backup codes
	backupCodes, err := GenerateBackupCodes(cfg.BackupCodes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	return &MFASetup{
		Secret:      key.Secret(),
		URL:         key.URL(),
		BackupCodes: backupCodes,
	}, nil
}

// ValidateTOTP validates a TOTP code against a secret.
func ValidateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// ValidateTOTPWithWindow validates a TOTP code with a time window.
func ValidateTOTPWithWindow(secret, code string, skew uint) bool {
	valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      skew,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && valid
}

// GenerateBackupCodes generates a set of backup codes.
func GenerateBackupCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code, err := generateBackupCode()
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}
	return codes, nil
}

// generateBackupCode generates a single backup code in format XXXX-XXXX.
func generateBackupCode() (string, error) {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.EncodeToString(b)
	return fmt.Sprintf("%s-%s", encoded[:4], encoded[4:8]), nil
}

// HashBackupCode creates a hash of a backup code for storage.
func HashBackupCode(code string) string {
	// Use bcrypt for backup codes since they're like passwords
	hash, err := HashPassword(code)
	if err != nil {
		return ""
	}
	return hash
}

// ValidateBackupCode checks if a backup code matches any in the list.
// Returns the index of the matched code, or -1 if not found.
func ValidateBackupCode(code string, hashedCodes []string) int {
	for i, hashed := range hashedCodes {
		if CheckPassword(code, hashed) {
			return i
		}
	}
	return -1
}

// MFAStatus represents the MFA status of a user.
type MFAStatus struct {
	Enabled         bool `json:"enabled"`
	BackupCodesLeft int  `json:"backup_codes_left"`
}
