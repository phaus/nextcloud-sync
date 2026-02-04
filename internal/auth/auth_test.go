package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/phaus/nextcloud-sync/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAppPasswordAuth(t *testing.T) {
	tests := []struct {
		name        string
		serverURL   string
		username    string
		appPassword string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid credentials",
			serverURL:   "https://cloud.example.com",
			username:    "testuser",
			appPassword: "abcde-fghij-klmno-pqrst-uvwxy-z1234",
			expectError: false,
		},
		{
			name:        "empty server URL",
			serverURL:   "",
			username:    "testuser",
			appPassword: "abcde-fghij-klmno-pqrst-uvwxy-z1234",
			expectError: true,
			errorMsg:    "server URL cannot be empty",
		},
		{
			name:        "empty username",
			serverURL:   "https://cloud.example.com",
			username:    "",
			appPassword: "abcde-fghij-klmno-pqrst-uvwxy-z1234",
			expectError: true,
			errorMsg:    "username cannot be empty",
		},
		{
			name:        "empty app password",
			serverURL:   "https://cloud.example.com",
			username:    "testuser",
			appPassword: "",
			expectError: true,
			errorMsg:    "app password cannot be empty",
		},
		{
			name:        "valid credentials",
			serverURL:   "https://cloud.example.com",
			username:    "testuser",
			appPassword: "abcde-fghij-klmno-pqrstu-vwxyz-12345",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := NewAppPasswordAuth(tt.serverURL, tt.username, tt.appPassword)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, auth)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, auth)
				assert.Equal(t, strings.TrimSuffix(tt.serverURL, "/"), auth.GetServerURL())
				assert.Equal(t, tt.username, auth.GetUsername())
			}
		})
	}
}

func TestAppPasswordAuth_GetAuthHeader(t *testing.T) {
	auth, err := NewAppPasswordAuth("https://cloud.example.com", "testuser", "abcde-fghij-klmno-pqrst-uvwxy-z1234")
	require.NoError(t, err)

	authHeader, err := auth.GetAuthHeader()
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(authHeader, "Basic "))
	assert.NotEmpty(t, authHeader)
}

func TestAppPasswordAuth_IsExpired(t *testing.T) {
	auth, err := NewAppPasswordAuth("https://cloud.example.com", "testuser", "abcde-fghij-klmno-pqrst-uvwxy-z1234")
	require.NoError(t, err)

	// Initially should be expired (not validated yet)
	assert.True(t, auth.IsExpired())

	// After validation (simulated by setting lastValidated)
	auth.lastValidated = time.Now()
	assert.False(t, auth.IsExpired())

	// Should be expired after more than an hour
	auth.lastValidated = time.Now().Add(-2 * time.Hour)
	assert.True(t, auth.IsExpired())
}

func TestAppPasswordAuth_Close(t *testing.T) {
	auth, err := NewAppPasswordAuth("https://cloud.example.com", "testuser", "abcde-fghij-klmno-pqrst-uvwxy-z1234")
	require.NoError(t, err)

	// Verify app password is set initially
	assert.NotEmpty(t, auth.appPassword)

	// Close should zero out sensitive data
	auth.Close()
	assert.Empty(t, auth.appPassword)
}

func TestNewAppPasswordAuthFromConfig(t *testing.T) {
	// Create test encrypted password
	encrypted, err := config.EncryptPassword("test-app-password-12345")
	require.NoError(t, err)

	server := config.Server{
		URL:         "https://cloud.example.com",
		Username:    "testuser",
		AppPassword: encrypted,
	}

	auth, err := NewAppPasswordAuthFromConfig(server)
	require.NoError(t, err)
	assert.NotNil(t, auth)
	assert.Equal(t, "https://cloud.example.com", auth.GetServerURL())
	assert.Equal(t, "testuser", auth.GetUsername())
}

func TestBase64Encode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "testuser:testpass",
			expected: "dGVzdHVzZXI6dGVzdHBhc3M=",
		},
		{
			input:    "user:password123",
			expected: "dXNlcjpwYXNzd29yZDEyMw==",
		},
		{
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := base64Encode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppPasswordManager_ValidateAppPasswordFormat(t *testing.T) {
	manager := NewAppPasswordManager(nil)

	tests := []struct {
		name        string
		password    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid Nextcloud app password",
			password:    "abcde-fghij-klmno-pqrstu-vwxyz-12345",
			expectError: false,
		},
		{
			name:        "valid app password without dashes",
			password:    "abcdefghijklmnopqrstuvwxyz1234",
			expectError: false,
		},
		{
			name:        "empty password",
			password:    "",
			expectError: true,
			errorMsg:    "cannot be empty",
		},
		{
			name:        "too short password",
			password:    "abc123",
			expectError: true,
			errorMsg:    "appears to be invalid length",
		},
		{
			name:        "password with invalid characters",
			password:    "abcde-fghij-klmno-pqrstu-vwxyz-@123",
			expectError: true,
			errorMsg:    "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateAppPasswordFormat(tt.password)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAppPasswordManager_ValidateServerCredentials(t *testing.T) {
	manager := NewAppPasswordManager(nil)

	tests := []struct {
		name        string
		serverURL   string
		username    string
		appPassword string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid credentials",
			serverURL:   "https://cloud.example.com",
			username:    "testuser",
			appPassword: "abcde-fghij-klmno-pqrst-uvwxy-z1234",
			expectError: false,
		},
		{
			name:        "empty server URL",
			serverURL:   "",
			username:    "testuser",
			appPassword: "abcde-fghij-klmno-pqrstu-vwxyz-12345",
			expectError: true,
			errorMsg:    "server URL cannot be empty",
		},
		{
			name:        "non-HTTPS server URL",
			serverURL:   "http://cloud.example.com",
			username:    "testuser",
			appPassword: "abcde-fghij-klmno-pqrstu-vwxyz-12345",
			expectError: true,
			errorMsg:    "must use HTTPS",
		},
		{
			name:        "empty username",
			serverURL:   "https://cloud.example.com",
			username:    "",
			appPassword: "abcde-fghij-klmno-pqrstu-vwxyz-12345",
			expectError: true,
			errorMsg:    "username cannot be empty",
		},
		{
			name:        "empty app password",
			serverURL:   "https://cloud.example.com",
			username:    "testuser",
			appPassword: "",
			expectError: true,
			errorMsg:    "app password cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateServerCredentials(tt.serverURL, tt.username, tt.appPassword)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAppPasswordManager_CreateServerConfig(t *testing.T) {
	manager := NewAppPasswordManager(nil)

	serverConfig, err := manager.CreateServerConfig(
		"https://cloud.example.com",
		"testuser",
		"abcde-fghij-klmno-pqrstu-vwxyz-12345",
	)

	require.NoError(t, err)
	assert.Equal(t, "https://cloud.example.com", serverConfig.URL)
	assert.Equal(t, "testuser", serverConfig.Username)
	assert.NotEmpty(t, serverConfig.AppPassword.Encrypted)
	assert.NotEmpty(t, serverConfig.AppPassword.Salt)
	assert.NotEmpty(t, serverConfig.AppPassword.Nonce)
	assert.Equal(t, config.EncryptionAlgorithm, serverConfig.AppPassword.Algorithm)
}

func TestAppPasswordManager_RotateAppPassword(t *testing.T) {
	manager := NewAppPasswordManager(nil)

	// Create original encrypted password
	original, err := config.EncryptPassword("test-password-12345")
	require.NoError(t, err)

	// Rotate the encryption
	rotated, err := manager.RotateAppPassword(original)
	require.NoError(t, err)

	// Should have different encryption data
	assert.NotEqual(t, original.Encrypted, rotated.Encrypted)
	assert.NotEqual(t, original.Salt, rotated.Salt)
	assert.NotEqual(t, original.Nonce, rotated.Nonce)
	assert.Equal(t, original.Algorithm, rotated.Algorithm)

	// Both should decrypt to the same password
	originalDecrypted, err := config.DecryptPassword(original)
	require.NoError(t, err)
	defer config.ZeroString(&originalDecrypted)

	rotatedDecrypted, err := config.DecryptPassword(rotated)
	require.NoError(t, err)
	defer config.ZeroString(&rotatedDecrypted)

	assert.Equal(t, originalDecrypted, rotatedDecrypted)
}

func TestNewCredentialValidator(t *testing.T) {
	validator := NewCredentialValidator()
	assert.NotNil(t, validator)
	assert.NotNil(t, validator.httpClient)

	// Test close
	validator.Close()
}

func TestCredentialValidator_ValidateCredentials_InvalidInput(t *testing.T) {
	validator := NewCredentialValidator()
	defer validator.Close()

	ctx := context.Background()

	tests := []struct {
		name        string
		serverURL   string
		username    string
		appPassword string
		expectError bool
		expectValid bool
	}{
		{
			name:        "empty app password",
			serverURL:   "https://cloud.example.com",
			username:    "testuser",
			appPassword: "abcde-fghij-klmno-pqrstu-vwxyz-12345",
			expectError: false,
			expectValid: false,
		},
		{
			name:        "empty username",
			serverURL:   "https://cloud.example.com",
			username:    "",
			appPassword: "abcde-fghij-klmno-pqrst-uvwxy-z1234",
			expectError: false,
			expectValid: false,
		},
		{
			name:        "empty app password",
			serverURL:   "https://cloud.example.com",
			username:    "testuser",
			appPassword: "",
			expectError: false,
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.ValidateCredentials(ctx, tt.serverURL, tt.username, tt.appPassword)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectValid, result.Valid)
		})
	}
}
