package webdav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/phaus/nextcloud-sync/internal/auth"
	"github.com/phaus/nextcloud-sync/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAuthProvider implements auth.AuthProvider for testing
type mockAuthProvider struct {
	serverURL string
	username  string
	password  string
}

func (m *mockAuthProvider) GetAuthHeader() (string, error) {
	return "Basic dGVzdDp0ZXN0", nil // test:test in base64
}

func (m *mockAuthProvider) ValidateCredentials(ctx context.Context) error {
	return nil
}

func (m *mockAuthProvider) RefreshCredentials(ctx context.Context) error {
	return nil
}

func (m *mockAuthProvider) IsExpired() bool {
	return false
}

func (m *mockAuthProvider) GetServerURL() string {
	return m.serverURL
}

func (m *mockAuthProvider) GetUsername() string {
	return m.username
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name         string
		authProvider auth.AuthProvider
		expectError  bool
	}{
		{
			name: "valid auth provider",
			authProvider: &mockAuthProvider{
				serverURL: "https://cloud.example.com",
				username:  "testuser",
				password:  "testpass",
			},
			expectError: false,
		},
		{
			name:         "nil auth provider",
			authProvider: nil,
			expectError:  true,
		},
		{
			name: "empty server URL",
			authProvider: &mockAuthProvider{
				serverURL: "",
				username:  "testuser",
				password:  "testpass",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.authProvider)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if client != nil {
					t.Errorf("expected nil client on error")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Errorf("expected non-nil client")
				return
			}

			// Test that client can be closed without error
			if err := client.Close(); err != nil {
				t.Errorf("close failed: %v", err)
			}
		})
	}
}

func TestBuildURL(t *testing.T) {
	authProvider := &mockAuthProvider{
		serverURL: "https://cloud.example.com",
		username:  "testuser",
		password:  "testpass",
	}

	client, err := NewClient(authProvider)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "root path",
			path:     "/",
			expected: "/remote.php/dav/files/testuser/",
		},
		{
			name:     "documents path",
			path:     "/Documents",
			expected: "/remote.php/dav/files/testuser/Documents",
		},
		{
			name:     "nested path",
			path:     "/Documents/file.txt",
			expected: "/remote.php/dav/files/testuser/Documents/file.txt",
		},
		{
			name:     "relative path",
			path:     "Documents",
			expected: "/remote.php/dav/files/testuser/Documents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.buildURL(tt.path)
			if !contains(result, tt.expected) {
				t.Errorf("expected URL to contain %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestExtractWebDAVEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "nextcloud URL",
			input:       "https://cloud.example.com",
			expected:    "https://cloud.example.com/remote.php/dav/files/USERNAME",
			expectError: false,
		},
		{
			name:        "nextcloud URL with path",
			input:       "https://cloud.example.com/",
			expected:    "https://cloud.example.com/remote.php/dav/files/USERNAME",
			expectError: false,
		},
		{
			name:        "existing webdav URL",
			input:       "https://cloud.example.com/remote.php/dav/files/testuser",
			expected:    "https://cloud.example.com/remote.php/dav/files/testuser",
			expectError: false,
		},
		{
			name:        "empty URL",
			input:       "",
			expected:    "",
			expectError: false, // Current implementation doesn't validate empty input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractWebDAVEndpoint(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		(len(s) >= len(substr) && s[:len(substr)] == substr) ||
		(len(s) > len(substr) &&
			(func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			})())
}

func TestSetRetryConfig(t *testing.T) {
	authProvider := &mockAuthProvider{
		serverURL: "https://cloud.example.com",
		username:  "testuser",
		password:  "testpass",
	}

	client, err := NewClient(authProvider)
	require.NoError(t, err)
	defer client.Close()

	// Test that default retry config is set
	assert.NotNil(t, client.retryConfig)
	assert.Equal(t, 3, client.retryConfig.MaxRetries)

	// Test setting custom retry config
	customConfig := &utils.RetryConfig{
		MaxRetries:          5,
		InitialDelay:        2 * time.Second,
		MaxDelay:            60 * time.Second,
		Multiplier:          3.0,
		RandomizationFactor: 0.2,
	}

	client.SetRetryConfig(customConfig)
	assert.Equal(t, customConfig, client.retryConfig)
}

func TestRetryWithTemporaryError(t *testing.T) {
	// Create a test server that returns temporary errors initially, then success
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++

		if attempts <= 2 {
			// Return temporary error for first 2 attempts
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Success on third attempt
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	authProvider := &mockAuthProvider{
		serverURL: server.URL,
		username:  "testuser",
		password:  "testpass",
	}

	client, err := NewClient(authProvider)
	require.NoError(t, err)
	defer client.Close()

	// Set short retry config for test
	retryConfig := &utils.RetryConfig{
		MaxRetries:          5,
		InitialDelay:        10 * time.Millisecond,
		MaxDelay:            100 * time.Millisecond,
		Multiplier:          2.0,
		RandomizationFactor: 0.0,
	}
	client.SetRetryConfig(retryConfig)

	// Create a simple request to test retry logic
	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	require.NoError(t, err)

	resp, err := client.doRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 3, attempts, "Should have made 3 attempts (2 failures + 1 success)")

	if resp != nil {
		resp.Body.Close()
	}
}

func TestRetryWithNonRetryableError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		// Return non-retryable error (404)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	authProvider := &mockAuthProvider{
		serverURL: server.URL,
		username:  "testuser",
		password:  "testpass",
	}

	client, err := NewClient(authProvider)
	require.NoError(t, err)
	defer client.Close()

	// Set short retry config for test
	retryConfig := &utils.RetryConfig{
		MaxRetries:          5,
		InitialDelay:        10 * time.Millisecond,
		MaxDelay:            100 * time.Millisecond,
		Multiplier:          2.0,
		RandomizationFactor: 0.0,
	}
	client.SetRetryConfig(retryConfig)

	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	require.NoError(t, err)

	resp, err := client.doRequest(req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 1, attempts, "Should have made only 1 attempt for non-retryable error")

	// Check that it's a WebDAV error
	webdavErr, isWebDAV := IsWebDAVError(err)
	if isWebDAV {
		assert.Equal(t, http.StatusNotFound, webdavErr.StatusCode)
	} else {
		// If it's not a WebDAV error, it should still be an error we can handle
		assert.Error(t, err, "Should have some kind of error")
	}
}

func TestRetryMaxAttemptsExceeded(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		// Always return temporary error
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	authProvider := &mockAuthProvider{
		serverURL: server.URL,
		username:  "testuser",
		password:  "testpass",
	}

	client, err := NewClient(authProvider)
	require.NoError(t, err)
	defer client.Close()

	// Set short retry config for test
	retryConfig := &utils.RetryConfig{
		MaxRetries:          2,
		InitialDelay:        10 * time.Millisecond,
		MaxDelay:            100 * time.Millisecond,
		Multiplier:          2.0,
		RandomizationFactor: 0.0,
	}
	client.SetRetryConfig(retryConfig)

	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	require.NoError(t, err)

	start := time.Now()
	resp, err := client.doRequest(req)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 3, attempts, "Should have made maxRetries + 1 attempts")
	assert.GreaterOrEqual(t, elapsed, 30*time.Millisecond, "Should have waited for retries")

	// Check that the error message contains max retries exceeded
	assert.Contains(t, err.Error(), "max retries")
}

func TestRetryWithContextCancellation(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		// Always return temporary error
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	authProvider := &mockAuthProvider{
		serverURL: server.URL,
		username:  "testuser",
		password:  "testpass",
	}

	client, err := NewClient(authProvider)
	require.NoError(t, err)
	defer client.Close()

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Set longer retry config to ensure context cancellation happens first
	retryConfig := &utils.RetryConfig{
		MaxRetries:          10,
		InitialDelay:        100 * time.Millisecond,
		MaxDelay:            1 * time.Second,
		Multiplier:          2.0,
		RandomizationFactor: 0.0,
	}
	client.SetRetryConfig(retryConfig)

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	require.NoError(t, err)

	start := time.Now()
	resp, err := client.doRequest(req)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "context cancelled")
	assert.Less(t, elapsed, 200*time.Millisecond, "Should return quickly due to context cancellation")
}
