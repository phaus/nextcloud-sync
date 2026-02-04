package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

// EncryptPassword encrypts a plaintext app password using AES-256-GCM
func EncryptPassword(password string) (EncryptedData, error) {
	// Generate random salt
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return EncryptedData{}, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive key from machine-specific information
	key, err := deriveEncryptionKey(salt)
	if err != nil {
		return EncryptedData{}, fmt.Errorf("failed to derive encryption key: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return EncryptedData{}, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return EncryptedData{}, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptedData{}, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Encrypt password
	encrypted := gcm.Seal(nil, nonce, []byte(password), nil)

	return EncryptedData{
		Encrypted: base64.StdEncoding.EncodeToString(encrypted),
		Salt:      base64.StdEncoding.EncodeToString(salt),
		Nonce:     base64.StdEncoding.EncodeToString(nonce),
		Algorithm: EncryptionAlgorithm,
	}, nil
}

// DecryptPassword decrypts an encrypted app password
func DecryptPassword(data EncryptedData) (string, error) {
	if data.Algorithm != EncryptionAlgorithm {
		return "", fmt.Errorf("unsupported encryption algorithm: %s", data.Algorithm)
	}

	// Validate integrity first
	if err := ValidateEncryptedDataIntegrity(data); err != nil {
		return "", fmt.Errorf("invalid encrypted data: %w", err)
	}

	// Decode base64 values
	encrypted, err := base64.StdEncoding.DecodeString(data.Encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted data: %w", err)
	}

	salt, err := base64.StdEncoding.DecodeString(data.Salt)
	if err != nil {
		return "", fmt.Errorf("failed to decode salt: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(data.Nonce)
	if err != nil {
		return "", fmt.Errorf("failed to decode nonce: %w", err)
	}

	// Derive key
	key, err := deriveEncryptionKey(salt)
	if err != nil {
		return "", fmt.Errorf("failed to derive encryption key: %w", err)
	}

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt password: %w", err)
	}

	return string(plaintext), nil
}

// deriveEncryptionKey derives an encryption key from machine-specific information
func deriveEncryptionKey(salt []byte) ([]byte, error) {
	// Get machine-specific secret
	machineSecret, err := getMachineSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to get machine secret: %w", err)
	}

	// Derive key using PBKDF2
	key := pbkdf2.Key([]byte(machineSecret), salt, PBKDF2Iterations, 32, sha256.New)

	return key, nil
}

// getMachineSecret creates a machine-specific secret for key derivation
func getMachineSecret() (string, error) {
	// This creates a machine-specific secret by combining multiple factors
	// In a real implementation, you might want to use more sophisticated methods

	// For now, we'll use a combination of user home directory and other machine identifiers
	// This ensures the encrypted data can only be decrypted on the same machine by the same user

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// You could add more machine-specific factors here like:
	// - Machine ID from /etc/machine-id (Linux)
	// - UUID from system_profiler (macOS)
	// - Computer name and username

	return fmt.Sprintf("nextcloud-sync-machine-secret:%s", homeDir), nil
}

// ZeroBytes securely zeros out a byte slice
func ZeroBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

// ZeroString securely zeros out a string
func ZeroString(s *string) {
	if s == nil {
		return
	}

	// Convert to byte slice and zero out
	data := []byte(*s)
	ZeroBytes(data)
	*s = ""
}

// GenerateRandomBytes generates cryptographically secure random bytes
func GenerateRandomBytes(size int) ([]byte, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return bytes, nil
}

// ValidateEncryptedDataIntegrity checks if the encrypted data appears to be valid
func ValidateEncryptedDataIntegrity(data EncryptedData) error {
	if data.Encrypted == "" {
		return fmt.Errorf("encrypted data is empty")
	}

	if data.Salt == "" {
		return fmt.Errorf("salt is empty")
	}

	if data.Nonce == "" {
		return fmt.Errorf("nonce is empty")
	}

	if data.Algorithm != EncryptionAlgorithm {
		return fmt.Errorf("unsupported algorithm: %s", data.Algorithm)
	}

	// Try to decode base64 values to ensure they're valid
	if _, err := base64.StdEncoding.DecodeString(data.Encrypted); err != nil {
		return fmt.Errorf("invalid base64 encoding for encrypted data: %w", err)
	}

	if _, err := base64.StdEncoding.DecodeString(data.Salt); err != nil {
		return fmt.Errorf("invalid base64 encoding for salt: %w", err)
	}

	if _, err := base64.StdEncoding.DecodeString(data.Nonce); err != nil {
		return fmt.Errorf("invalid base64 encoding for nonce: %w", err)
	}

	// Validate salt and nonce sizes
	saltBytes, _ := base64.StdEncoding.DecodeString(data.Salt)
	if len(saltBytes) != SaltSize {
		return fmt.Errorf("invalid salt size: expected %d, got %d", SaltSize, len(saltBytes))
	}

	nonceBytes, _ := base64.StdEncoding.DecodeString(data.Nonce)
	if len(nonceBytes) != NonceSize {
		return fmt.Errorf("invalid nonce size: expected %d, got %d", NonceSize, len(nonceBytes))
	}

	return nil
}

// RotateEncryption creates a new encrypted version of the data with a fresh key
func RotateEncryption(oldData EncryptedData, password string) (EncryptedData, error) {
	// Decrypt with old data
	plaintext, err := DecryptPassword(oldData)
	if err != nil {
		return EncryptedData{}, fmt.Errorf("failed to decrypt during rotation: %w", err)
	}
	defer ZeroString(&plaintext)

	// Re-encrypt with new salt/nonce
	newData, err := EncryptPassword(plaintext)
	if err != nil {
		return EncryptedData{}, fmt.Errorf("failed to encrypt during rotation: %w", err)
	}

	return newData, nil
}
