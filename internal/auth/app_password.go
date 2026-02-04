package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/phaus/nextcloud-sync/internal/config"
)

// AppPasswordManager handles app password operations
type AppPasswordManager struct {
	authProvider AuthProvider
}

// NewAppPasswordManager creates a new app password manager
func NewAppPasswordManager(authProvider AuthProvider) *AppPasswordManager {
	return &AppPasswordManager{
		authProvider: authProvider,
	}
}

// CreateAuthFromServerConfig creates an authenticator from server configuration
func (m *AppPasswordManager) CreateAuthFromServerConfig(server config.Server) (AuthProvider, error) {
	return NewAppPasswordAuthFromConfig(server)
}

// ValidateAppPasswordFormat validates if the app password format is correct
func (m *AppPasswordManager) ValidateAppPasswordFormat(password string) error {
	if password == "" {
		return fmt.Errorf("app password cannot be empty")
	}

	// Nextcloud app passwords are typically 20-40 characters long
	// Format: xxxxx-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx or similar
	if len(password) < 15 || len(password) > 50 {
		return fmt.Errorf("app password appears to be invalid length (expected 15-50 characters, got %d)", len(password))
	}

	// Check for typical Nextcloud app password pattern (groups of alphanumeric characters separated by dashes)
	parts := strings.Split(password, "-")
	if len(parts) >= 3 {
		for i, part := range parts {
			if len(part) < 3 {
				return fmt.Errorf("app password part %d appears to be too short", i+1)
			}
		}
	}

	// Basic character validation - Nextcloud app passwords use alphanumeric characters and dashes
	for _, char := range password {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-') {
			return fmt.Errorf("app password contains invalid character: %c", char)
		}
	}

	return nil
}

// EncryptAndStoreAppPassword encrypts an app password for storage
func (m *AppPasswordManager) EncryptAndStoreAppPassword(password string) (config.EncryptedData, error) {
	if err := m.ValidateAppPasswordFormat(password); err != nil {
		return config.EncryptedData{}, fmt.Errorf("invalid app password format: %w", err)
	}

	encrypted, err := config.EncryptPassword(password)
	if err != nil {
		return config.EncryptedData{}, fmt.Errorf("failed to encrypt app password: %w", err)
	}

	return encrypted, nil
}

// ValidateServerCredentials validates server configuration including credentials
func (m *AppPasswordManager) ValidateServerCredentials(serverURL, username, appPassword string) error {
	if serverURL == "" {
		return fmt.Errorf("server URL cannot be empty")
	}

	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	if appPassword == "" {
		return fmt.Errorf("app password cannot be empty")
	}

	// Validate app password format
	if err := m.ValidateAppPasswordFormat(appPassword); err != nil {
		return fmt.Errorf("invalid app password: %w", err)
	}

	// Validate server URL format
	if !strings.HasPrefix(serverURL, "https://") {
		return fmt.Errorf("server URL must use HTTPS")
	}

	if !strings.Contains(serverURL, ".") {
		return fmt.Errorf("server URL appears to be invalid")
	}

	return nil
}

// CreateServerConfig creates a server configuration with encrypted app password
func (m *AppPasswordManager) CreateServerConfig(serverURL, username, appPassword string) (config.Server, error) {
	if err := m.ValidateServerCredentials(serverURL, username, appPassword); err != nil {
		return config.Server{}, err
	}

	// Encrypt the app password
	encryptedPassword, err := m.EncryptAndStoreAppPassword(appPassword)
	if err != nil {
		return config.Server{}, fmt.Errorf("failed to encrypt app password: %w", err)
	}

	return config.Server{
		URL:         serverURL,
		Username:    username,
		AppPassword: encryptedPassword,
	}, nil
}

// TestConnection tests the connection to Nextcloud with given credentials
func (m *AppPasswordManager) TestConnection(ctx context.Context, serverURL, username, appPassword string) error {
	auth, err := NewAppPasswordAuth(serverURL, username, appPassword)
	if err != nil {
		return fmt.Errorf("failed to create authenticator: %w", err)
	}
	defer auth.Close()

	return auth.ValidateCredentials(ctx)
}

// RotateAppPassword creates a new encrypted version with fresh encryption parameters
func (m *AppPasswordManager) RotateAppPassword(oldEncrypted config.EncryptedData) (config.EncryptedData, error) {
	// Decrypt the old password
	password, err := config.DecryptPassword(oldEncrypted)
	if err != nil {
		return config.EncryptedData{}, fmt.Errorf("failed to decrypt old password: %w", err)
	}
	defer config.ZeroString(&password)

	// Re-encrypt with new parameters
	newEncrypted, err := config.EncryptPassword(password)
	if err != nil {
		return config.EncryptedData{}, fmt.Errorf("failed to re-encrypt password: %w", err)
	}

	return newEncrypted, nil
}
