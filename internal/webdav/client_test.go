package webdav

import (
	"context"
	"testing"

	"github.com/phaus/nextcloud-sync/internal/auth"
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
