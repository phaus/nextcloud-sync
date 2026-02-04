package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNextcloudURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *NextcloudURL
		expectError bool
		errorMsg    string
	}{
		{
			name:  "valid Nextcloud URL with directory",
			input: "https://cloud.example.com/apps/files/files/2743527?dir=/uploads",
			expected: &NextcloudURL{
				Original:   "https://cloud.example.com/apps/files/files/2743527?dir=/uploads",
				BaseURL:    "https://cloud.example.com",
				UserID:     "2743527",
				Directory:  "/uploads",
				WebDAVPath: "/remote.php/dav/files/2743527/uploads",
			},
			expectError: false,
		},
		{
			name:  "valid Nextcloud URL with root directory",
			input: "https://cloud.consolving.de/apps/files/files/1234567",
			expected: &NextcloudURL{
				Original:   "https://cloud.consolving.de/apps/files/files/1234567",
				BaseURL:    "https://cloud.consolving.de",
				UserID:     "1234567",
				Directory:  "/",
				WebDAVPath: "/remote.php/dav/files/1234567/",
			},
			expectError: false,
		},
		{
			name:  "valid Nextcloud URL with nested directory",
			input: "https://cloud.example.com/apps/files/files/2743527?dir=/documents/work",
			expected: &NextcloudURL{
				Original:   "https://cloud.example.com/apps/files/files/2743527?dir=/documents/work",
				BaseURL:    "https://cloud.example.com",
				UserID:     "2743527",
				Directory:  "/documents/work",
				WebDAVPath: "/remote.php/dav/files/2743527/documents/work",
			},
			expectError: false,
		},
		{
			name:        "empty URL",
			input:       "",
			expectError: true,
			errorMsg:    "Nextcloud URL cannot be empty",
		},
		{
			name:        "invalid URL format",
			input:       "not-a-url",
			expectError: true,
			errorMsg:    "Nextcloud URL must use HTTPS",
		},
		{
			name:        "HTTP URL instead of HTTPS",
			input:       "http://cloud.example.com/apps/files/files/2743527?dir=/uploads",
			expectError: true,
			errorMsg:    "Nextcloud URL must use HTTPS",
		},
		{
			name:        "missing host",
			input:       "https:///apps/files/files/2743527?dir=/uploads",
			expectError: true,
			errorMsg:    "Nextcloud URL must have a valid host",
		},
		{
			name:        "wrong path pattern",
			input:       "https://cloud.example.com/apps/files/2743527?dir=/uploads",
			expectError: true,
			errorMsg:    "must contain '/apps/files/files/' path",
		},
		{
			name:        "missing user ID",
			input:       "https://cloud.example.com/apps/files/files/?dir=/uploads",
			expectError: true,
			errorMsg:    "invalid Nextcloud files app URL format",
		},
		{
			name:  "directory without leading slash",
			input: "https://cloud.example.com/apps/files/files/2743527?dir=uploads",
			expected: &NextcloudURL{
				Original:   "https://cloud.example.com/apps/files/files/2743527?dir=uploads",
				BaseURL:    "https://cloud.example.com",
				UserID:     "2743527",
				Directory:  "/uploads",
				WebDAVPath: "/remote.php/dav/files/2743527/uploads",
			},
			expectError: false,
		},
		{
			name:  "URL with trailing slash",
			input: "https://cloud.example.com/apps/files/files/2743527/?dir=/uploads",
			expected: &NextcloudURL{
				Original:   "https://cloud.example.com/apps/files/files/2743527/?dir=/uploads",
				BaseURL:    "https://cloud.example.com",
				UserID:     "2743527",
				Directory:  "/uploads",
				WebDAVPath: "/remote.php/dav/files/2743527/uploads",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseNextcloudURL(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
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
		errorMsg    string
	}{
		{
			name:     "valid URL with directory",
			input:    "https://cloud.example.com/apps/files/files/2743527?dir=/uploads",
			expected: "https://cloud.example.com/remote.php/dav/files/2743527/uploads",
		},
		{
			name:     "valid URL with root directory",
			input:    "https://cloud.consolving.de/apps/files/files/1234567",
			expected: "https://cloud.consolving.de/remote.php/dav/files/1234567/",
		},
		{
			name:     "valid URL with nested directory",
			input:    "https://cloud.example.com/apps/files/files/2743527?dir=/documents/work",
			expected: "https://cloud.example.com/remote.php/dav/files/2743527/documents/work",
		},
		{
			name:        "invalid URL",
			input:       "not-a-url",
			expectError: true,
			errorMsg:    "failed to parse Nextcloud URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractWebDAVEndpoint(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestValidateNextcloudURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:  "valid URL",
			input: "https://cloud.example.com/apps/files/files/2743527?dir=/uploads",
		},
		{
			name:        "empty URL",
			input:       "",
			expectError: true,
		},
		{
			name:        "invalid format",
			input:       "https://example.com/wrong/path",
			expectError: true,
		},
		{
			name:        "HTTP URL",
			input:       "http://cloud.example.com/apps/files/files/2743527?dir=/uploads",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNextcloudURL(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsNextcloudURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid Nextcloud files app URL",
			input:    "https://cloud.example.com/apps/files/files/2743527?dir=/uploads",
			expected: true,
		},
		{
			name:     "Nextcloud WebDAV URL",
			input:    "https://cloud.example.com/remote.php/dav/files/2743527/uploads",
			expected: true,
		},
		{
			name:     "Nextcloud status URL",
			input:    "https://cloud.example.com/status.php",
			expected: true,
		},
		{
			name:     "Nextcloud OCS URL",
			input:    "https://cloud.example.com/ocs/v1.php/apps/files_sharing/api/v1/shares",
			expected: true,
		},
		{
			name:     "URL with cloud in hostname",
			input:    "https://mycloud.example.com/some/path",
			expected: true,
		},
		{
			name:     "URL with nextcloud in hostname",
			input:    "https://nextcloud.example.com/some/path",
			expected: true,
		},
		{
			name:     "URL with nc in hostname",
			input:    "https://nc.example.com/some/path",
			expected: true,
		},
		{
			name:     "HTTP URL (should be false)",
			input:    "http://cloud.example.com/apps/files/files/2743527",
			expected: false,
		},
		{
			name:     "empty URL",
			input:    "",
			expected: false,
		},
		{
			name:     "invalid URL",
			input:    "not-a-url",
			expected: false,
		},
		{
			name:     "generic website",
			input:    "https://example.com/page",
			expected: false,
		},
		{
			name:     "missing host",
			input:    "https:///some/path",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNextcloudURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
		errorMsg    string
	}{
		{
			name:     "already normalized HTTPS URL",
			input:    "https://example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "URL without scheme",
			input:    "example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "HTTP URL converted to HTTPS",
			input:    "http://example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "URL with default HTTPS port removed",
			input:    "https://example.com:443/path",
			expected: "https://example.com/path",
		},
		{
			name:     "URL with custom port preserved",
			input:    "https://example.com:8443/path",
			expected: "https://example.com:8443/path",
		},
		{
			name:     "trailing slash removed (except root)",
			input:    "https://example.com/path/",
			expected: "https://example.com/path",
		},
		{
			name:     "root path preserved",
			input:    "https://example.com/",
			expected: "https://example.com/",
		},
		{
			name:     "empty path",
			input:    "https://example.com",
			expected: "https://example.com/",
		},
		{
			name:     "URL with query parameters",
			input:    "https://example.com/path?param=value",
			expected: "https://example.com/path?param=value",
		},
		{
			name:        "empty URL",
			input:       "",
			expectError: true,
			errorMsg:    "URL cannot be empty",
		},
		{
			name:     "invalid URL normalized to HTTPS",
			input:    "not-a-url",
			expected: "https://not-a-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeURL(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// Benchmark tests
func BenchmarkParseNextcloudURL(b *testing.B) {
	url := "https://cloud.example.com/apps/files/files/2743527?dir=/uploads"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseNextcloudURL(url)
	}
}

func BenchmarkExtractWebDAVEndpoint(b *testing.B) {
	url := "https://cloud.example.com/apps/files/files/2743527?dir=/uploads"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractWebDAVEndpoint(url)
	}
}

func BenchmarkValidateNextcloudURL(b *testing.B) {
	url := "https://cloud.example.com/apps/files/files/2743527?dir=/uploads"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateNextcloudURL(url)
	}
}

func BenchmarkIsNextcloudURL(b *testing.B) {
	url := "https://cloud.example.com/apps/files/files/2743527?dir=/uploads"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsNextcloudURL(url)
	}
}

func BenchmarkNormalizeURL(b *testing.B) {
	url := "https://example.com/path"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NormalizeURL(url)
	}
}
