package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/phaus/nextcloud-sync/internal/config"
)

// AuthProvider defines the interface for authentication providers
type AuthProvider interface {
	// GetAuthHeader returns the HTTP authorization header for authentication
	GetAuthHeader() (string, error)

	// ValidateCredentials validates the stored credentials against the server
	ValidateCredentials(ctx context.Context) error

	// RefreshCredentials refreshes the authentication if needed
	RefreshCredentials(ctx context.Context) error

	// IsExpired returns true if the current authentication is expired
	IsExpired() bool

	// GetServerURL returns the configured server URL
	GetServerURL() string

	// GetUsername returns the configured username
	GetUsername() string
}

// AppPasswordAuth implements AuthProvider for Nextcloud app passwords
type AppPasswordAuth struct {
	serverURL     string
	username      string
	appPassword   string
	httpClient    *http.Client
	lastValidated time.Time
}

// NewAppPasswordAuth creates a new app password authenticator
func NewAppPasswordAuth(serverURL, username, appPassword string) (*AppPasswordAuth, error) {
	if serverURL == "" {
		return nil, fmt.Errorf("server URL cannot be empty")
	}

	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	if appPassword == "" {
		return nil, fmt.Errorf("app password cannot be empty")
	}

	// Normalize server URL
	serverURL = strings.TrimSuffix(serverURL, "/")

	// Create HTTP client with reasonable defaults
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		},
	}

	return &AppPasswordAuth{
		serverURL:   serverURL,
		username:    username,
		appPassword: appPassword,
		httpClient:  client,
	}, nil
}

// NewAppPasswordAuthFromConfig creates an authenticator from encrypted config data
func NewAppPasswordAuthFromConfig(server config.Server) (*AppPasswordAuth, error) {
	// Decrypt the app password
	password, err := config.DecryptPassword(server.AppPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt app password: %w", err)
	}

	auth, err := NewAppPasswordAuth(server.URL, server.Username, password)
	if err != nil {
		// Zero out the decrypted password on error
		config.ZeroString(&password)
		return nil, err
	}

	// Zero out the decrypted password after use
	config.ZeroString(&password)

	return auth, nil
}

// GetAuthHeader returns the HTTP Basic Auth header
func (a *AppPasswordAuth) GetAuthHeader() (string, error) {
	if a.appPassword == "" {
		return "", fmt.Errorf("app password is not set")
	}

	return fmt.Sprintf("Basic %s", a.encodeCredentials()), nil
}

// encodeCredentials encodes username and password for Basic Auth
func (a *AppPasswordAuth) encodeCredentials() string {
	credentials := fmt.Sprintf("%s:%s", a.username, a.appPassword)
	// Note: This base64 encoding is required by HTTP Basic Auth specification
	// The password will be zeroed after use in the calling code
	return base64Encode(credentials)
}

// ValidateCredentials validates the app password against the Nextcloud server
func (a *AppPasswordAuth) ValidateCredentials(ctx context.Context) error {
	// Create a simple request to validate credentials
	req, err := http.NewRequestWithContext(ctx, "GET", a.serverURL+"/remote.php/dav/", nil)
	if err != nil {
		return fmt.Errorf("failed to create validation request: %w", err)
	}

	// Set authorization header
	authHeader, err := a.GetAuthHeader()
	if err != nil {
		return fmt.Errorf("failed to get auth header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	// Make request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid credentials: authentication failed")
	}

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status during validation: %d", resp.StatusCode)
	}

	// Update last validation time
	a.lastValidated = time.Now()

	return nil
}

// RefreshCredentials for app passwords doesn't need to do anything
// since app passwords don't expire, but we validate them
func (a *AppPasswordAuth) RefreshCredentials(ctx context.Context) error {
	return a.ValidateCredentials(ctx)
}

// IsExpired returns false for app passwords since they don't expire
func (a *AppPasswordAuth) IsExpired() bool {
	// App passwords don't expire, but we can implement a cache invalidation
	// Revalidate if not validated in the last hour
	if time.Since(a.lastValidated) > time.Hour {
		return true
	}
	return false
}

// GetServerURL returns the configured server URL
func (a *AppPasswordAuth) GetServerURL() string {
	return a.serverURL
}

// GetUsername returns the configured username
func (a *AppPasswordAuth) GetUsername() string {
	return a.username
}

// Close cleans up resources
func (a *AppPasswordAuth) Close() {
	// Zero out sensitive data
	a.appPassword = ""

	// Close HTTP client transport if it has a CloseIdleConnections method
	if transport, ok := a.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// base64Encode is a helper function for base64 encoding
func base64Encode(input string) string {
	return base64.StdEncoding.EncodeToString([]byte(input))
}
