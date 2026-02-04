package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// CredentialValidator validates Nextcloud credentials and server connectivity
type CredentialValidator struct {
	httpClient *http.Client
}

// NewCredentialValidator creates a new credential validator
func NewCredentialValidator() *CredentialValidator {
	return &CredentialValidator{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        5,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  false,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}
}

// ValidationResult represents the result of credential validation
type ValidationResult struct {
	Valid           bool        `json:"valid"`
	ServerReachable bool        `json:"server_reachable"`
	WebDAVEnabled   bool        `json:"webdav_enabled"`
	Errors          []string    `json:"errors,omitempty"`
	Warnings        []string    `json:"warnings,omitempty"`
	ServerInfo      *ServerInfo `json:"server_info,omitempty"`
}

// ServerInfo contains information about the Nextcloud server
type ServerInfo struct {
	Version        string `json:"version,omitempty"`
	Product        string `json:"product,omitempty"`
	WebDAVEndpoint string `json:"webdav_endpoint,omitempty"`
	StatusText     string `json:"status_text,omitempty"`
}

// ValidateCredentials performs comprehensive credential validation
func (v *CredentialValidator) ValidateCredentials(ctx context.Context, serverURL, username, appPassword string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:           false,
		ServerReachable: false,
		WebDAVEnabled:   false,
		Errors:          []string{},
		Warnings:        []string{},
	}

	// Normalize server URL
	normalizedURL := strings.TrimSuffix(serverURL, "/")

	// Step 1: Check server reachability
	serverInfo, err := v.checkServerReachability(ctx, normalizedURL)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Server unreachable: %v", err))
		return result, nil
	}

	result.ServerReachable = true
	result.ServerInfo = serverInfo

	// Step 2: Check WebDAV availability
	webdavEndpoint := fmt.Sprintf("%s/remote.php/dav/", normalizedURL)
	if err := v.checkWebDAVEndpoint(ctx, webdavEndpoint); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("WebDAV not available: %v", err))
		return result, nil
	}

	result.WebDAVEnabled = true

	// Step 3: Validate authentication
	if err := v.validateAuthentication(ctx, webdavEndpoint, username, appPassword); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Authentication failed: %v", err))
		return result, nil
	}

	// Step 4: Verify user access
	if err := v.verifyUserAccess(ctx, webdavEndpoint, username); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("User access limited: %v", err))
	}

	result.Valid = true
	return result, nil
}

// checkServerReachability checks if the Nextcloud server is reachable
func (v *CredentialValidator) checkServerReachability(ctx context.Context, serverURL string) (*ServerInfo, error) {
	// Try to access the server root
	req, err := http.NewRequestWithContext(ctx, "GET", serverURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to identify our client
	req.Header.Set("User-Agent", "Nextcloud-Sync-CLI/1.0")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("server is rate limiting requests")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return nil, fmt.Errorf("server returned unexpected status: %d", resp.StatusCode)
	}

	// Extract server information from headers
	serverInfo := &ServerInfo{
		StatusText: resp.Status,
	}

	if product := resp.Header.Get("X-Nextcloud-Version"); product != "" {
		serverInfo.Product = "Nextcloud"
		serverInfo.Version = product
	}

	serverInfo.WebDAVEndpoint = fmt.Sprintf("%s/remote.php/dav/", serverURL)

	return serverInfo, nil
}

// checkWebDAVEndpoint checks if the WebDAV endpoint is available
func (v *CredentialValidator) checkWebDAVEndpoint(ctx context.Context, webdavURL string) error {
	req, err := http.NewRequestWithContext(ctx, "OPTIONS", webdavURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create WebDAV request: %w", err)
	}

	req.Header.Set("User-Agent", "Nextcloud-Sync-CLI/1.0")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to WebDAV endpoint: %w", err)
	}
	defer resp.Body.Close()

	// WebDAV should return 200 OK for OPTIONS requests
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("WebDAV endpoint returned status: %d", resp.StatusCode)
	}

	// Check for DAV headers
	davHeaders := resp.Header.Values("DAV")
	if len(davHeaders) == 0 {
		return fmt.Errorf("WebDAV endpoint missing DAV headers")
	}

	return nil
}

// validateAuthentication validates the app password credentials
func (v *CredentialValidator) validateAuthentication(ctx context.Context, webdavURL, username, appPassword string) error {
	// Create Basic Auth header
	auth, err := NewAppPasswordAuth("", username, appPassword) // Server URL not needed for this check
	if err != nil {
		return fmt.Errorf("failed to create authenticator: %w", err)
	}
	authHeader, err := auth.GetAuthHeader()
	if err != nil {
		return fmt.Errorf("failed to create auth header: %w", err)
	}

	// Make a PROPFIND request to test authentication
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", webdavURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create PROPFIND request: %w", err)
	}

	req.Header.Set("Authorization", authHeader)
	req.Header.Set("User-Agent", "Nextcloud-Sync-CLI/1.0")
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Depth", "0")

	// Minimal PROPFIND body
	req.Body = http.NoBody
	// Alternatively, you could send a minimal XML body:
	// req.Body = strings.NewReader(`<?xml version="1.0" encoding="utf-8"?><D:propfind xmlns:D="DAV:"><D:prop><D:resourcetype/></D:prop></D:propfind>`)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid credentials: authentication failed")
	}

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed with status: %d", resp.StatusCode)
	}

	return nil
}

// verifyUserAccess verifies that the user can access their files
func (v *CredentialValidator) verifyUserAccess(ctx context.Context, webdavURL, username string) error {
	// Try to access the user's files directory
	userFilesURL := fmt.Sprintf("%s/files/%s/", webdavURL, username)

	req, err := http.NewRequestWithContext(ctx, "PROPFIND", userFilesURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create user files request: %w", err)
	}

	req.Header.Set("User-Agent", "Nextcloud-Sync-CLI/1.0")
	req.Header.Set("Depth", "0")
	req.Body = http.NoBody

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to access user files: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("user files directory not found")
	}

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cannot access user files: status %d", resp.StatusCode)
	}

	return nil
}

// Close closes the validator and cleans up resources
func (v *CredentialValidator) Close() {
	if transport, ok := v.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}
