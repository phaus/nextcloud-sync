package utils

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// NextcloudURL represents a parsed Nextcloud URL with its components
type NextcloudURL struct {
	Original   string
	BaseURL    string // https://cloud.example.com
	UserID     string // 2743527
	Directory  string // /uploads
	WebDAVPath string // /remote.php/dav/files/2743527/uploads
}

// ParseNextcloudURL parses a Nextcloud files app URL and extracts its components
// Expected format: https://cloud.example.com/apps/files/files/USER_ID?dir=/PATH
func ParseNextcloudURL(nextcloudURL string) (*NextcloudURL, error) {
	if nextcloudURL == "" {
		return nil, fmt.Errorf("Nextcloud URL cannot be empty")
	}

	parsedURL, err := url.Parse(nextcloudURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Nextcloud URL: %w", err)
	}

	// Validate HTTPS
	if parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("Nextcloud URL must use HTTPS, got: %s", parsedURL.Scheme)
	}

	// Validate host
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("Nextcloud URL must have a valid host")
	}

	// Check for required Nextcloud files app pattern
	if !strings.Contains(parsedURL.Path, "/apps/files/files/") {
		return nil, fmt.Errorf("Nextcloud URL must contain '/apps/files/files/' path, got: %s", parsedURL.Path)
	}

	// Extract components using regex
	pattern := `^/apps/files/files/(\d+)(?:/)?$`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(parsedURL.Path)
	if len(matches) != 2 {
		return nil, fmt.Errorf("invalid Nextcloud files app URL format, expected: /apps/files/files/USER_ID")
	}

	userID := matches[1]

	// Extract directory from query parameters
	directory := parsedURL.Query().Get("dir")
	if directory == "" {
		directory = "/"
	}

	// Ensure directory starts with /
	if !strings.HasPrefix(directory, "/") {
		directory = "/" + directory
	}

	// Build base URL
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	// Build WebDAV path
	webDAVPath := fmt.Sprintf("/remote.php/dav/files/%s%s", userID, directory)

	return &NextcloudURL{
		Original:   nextcloudURL,
		BaseURL:    baseURL,
		UserID:     userID,
		Directory:  directory,
		WebDAVPath: webDAVPath,
	}, nil
}

// ExtractWebDAVEndpoint converts a Nextcloud files app URL to a WebDAV endpoint
// Input: https://cloud.example.com/apps/files/files/2743527?dir=/uploads
// Output: https://cloud.example.com/remote.php/dav/files/2743527/uploads
func ExtractWebDAVEndpoint(nextcloudURL string) (string, error) {
	parsed, err := ParseNextcloudURL(nextcloudURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse Nextcloud URL: %w", err)
	}

	webDAVEndpoint := parsed.BaseURL + parsed.WebDAVPath
	return webDAVEndpoint, nil
}

// ValidateNextcloudURL checks if a URL is a valid Nextcloud files app URL
func ValidateNextcloudURL(nextcloudURL string) error {
	_, err := ParseNextcloudURL(nextcloudURL)
	return err
}

// IsNextcloudURL checks if a URL appears to be a Nextcloud instance URL
// This is a more permissive check for general Nextcloud URLs, not specifically files app URLs
func IsNextcloudURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Must be HTTPS
	if parsedURL.Scheme != "https" {
		return false
	}

	// Must have a host
	if parsedURL.Host == "" {
		return false
	}

	// Check for common Nextcloud indicators
	host := strings.ToLower(parsedURL.Host)
	path := strings.ToLower(parsedURL.Path)

	// Common Nextcloud path indicators
	nextcloudIndicators := []string{
		"/apps/",
		"/remote.php/",
		"/status.php",
		"/ocs/",
		"/dav/",
		"/files/",
	}

	for _, indicator := range nextcloudIndicators {
		if strings.Contains(path, indicator) {
			return true
		}
	}

	// Check host for common Nextcloud subdomains or patterns
	if strings.Contains(host, "cloud") ||
		strings.Contains(host, "nextcloud") ||
		strings.Contains(host, "nc") {
		return true
	}

	return false
}

// NormalizeURL ensures a URL is properly formatted and consistent
func NormalizeURL(urlStr string) (string, error) {
	if urlStr == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	// Ensure scheme is present and HTTPS for security
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "https"
	} else if parsedURL.Scheme == "http" {
		parsedURL.Scheme = "https"
	}

	// Remove default port
	if parsedURL.Port() == "443" && parsedURL.Scheme == "https" {
		parsedURL.Host = parsedURL.Hostname()
	}

	// Handle empty path
	if parsedURL.Path == "" {
		parsedURL.Path = "/"
	}

	// Remove trailing slash from path unless it's just root
	if len(parsedURL.Path) > 1 && strings.HasSuffix(parsedURL.Path, "/") {
		parsedURL.Path = strings.TrimSuffix(parsedURL.Path, "/")
	}

	return parsedURL.String(), nil
}
