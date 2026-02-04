package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ValidateConfig validates the entire configuration structure
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if err := ValidateVersion(config.Version); err != nil {
		return fmt.Errorf("invalid version: %w", err)
	}

	if err := ValidateServers(config.Servers); err != nil {
		return fmt.Errorf("invalid servers: %w", err)
	}

	if err := ValidateSyncProfiles(config.SyncProfiles); err != nil {
		return fmt.Errorf("invalid sync profiles: %w", err)
	}

	if err := ValidateGlobalSettings(config.GlobalSettings); err != nil {
		return fmt.Errorf("invalid global settings: %w", err)
	}

	return nil
}

// ValidateVersion validates the configuration version
func ValidateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Accept simple versions like "1.0" or semver like "1.0.0"
	versionPattern := `^(\d+)(?:\.(\d+))?(?:\.(\d+))?$`
	matched, err := regexp.MatchString(versionPattern, version)
	if err != nil {
		return fmt.Errorf("failed to validate version format: %w", err)
	}

	if !matched {
		return fmt.Errorf("version must be in version format (x.y.z or x.y)")
	}

	return nil
}

// ValidateServers validates the servers configuration
func ValidateServers(servers map[string]Server) error {
	// Servers map can be empty for a minimal config
	for name, server := range servers {
		if err := ValidateServer(name, server); err != nil {
			return fmt.Errorf("server '%s': %w", name, err)
		}
	}

	return nil
}

// ValidateServer validates a single server configuration
func ValidateServer(name string, server Server) error {
	if name == "" {
		return fmt.Errorf("server name cannot be empty")
	}

	if err := ValidateServerURL(server.URL); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if err := ValidateUsername(server.Username); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}

	if err := ValidateEncryptedData(server.AppPassword); err != nil {
		return fmt.Errorf("invalid app password: %w", err)
	}

	return nil
}

// ValidateServerURL validates the Nextcloud server URL
func ValidateServerURL(serverURL string) error {
	if serverURL == "" {
		return fmt.Errorf("server URL cannot be empty")
	}

	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	if parsedURL.Scheme != "https" {
		return fmt.Errorf("server URL must use HTTPS")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("server URL must have a valid host")
	}

	return nil
}

// ValidateUsername validates the username format
func ValidateUsername(username string) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	if strings.Contains(username, ":") {
		return fmt.Errorf("username cannot contain ':' character")
	}

	if len(username) > 255 {
		return fmt.Errorf("username too long (max 255 characters)")
	}

	return nil
}

// ValidateEncryptedData validates the encrypted password structure
func ValidateEncryptedData(data EncryptedData) error {
	if data.Encrypted == "" {
		return fmt.Errorf("encrypted data cannot be empty")
	}

	if data.Salt == "" {
		return fmt.Errorf("salt cannot be empty")
	}

	if data.Nonce == "" {
		return fmt.Errorf("nonce cannot be empty")
	}

	if data.Algorithm != EncryptionAlgorithm {
		return fmt.Errorf("unsupported encryption algorithm: %s", data.Algorithm)
	}

	return nil
}

// ValidateSyncProfiles validates the sync profiles configuration
func ValidateSyncProfiles(profiles map[string]SyncProfile) error {
	// Profiles are optional, but if they exist, they must be valid
	for name, profile := range profiles {
		if err := ValidateSyncProfile(name, profile); err != nil {
			return fmt.Errorf("profile '%s': %w", name, err)
		}
	}

	return nil
}

// ValidateSyncProfile validates a single sync profile
func ValidateSyncProfile(name string, profile SyncProfile) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	if profile.Source == "" {
		return fmt.Errorf("source path cannot be empty")
	}

	if profile.Target == "" {
		return fmt.Errorf("target path cannot be empty")
	}

	// Validate exclude patterns
	for _, pattern := range profile.ExcludePatterns {
		if err := ValidateExcludePattern(pattern); err != nil {
			return fmt.Errorf("invalid exclude pattern '%s': %w", pattern, err)
		}
	}

	return nil
}

// ValidateExcludePattern validates a gitignore-style exclude pattern
func ValidateExcludePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("exclude pattern cannot be empty")
	}

	// Basic validation - more sophisticated pattern validation might be needed
	// Check for dangerous patterns that could cause issues
	dangerousPatterns := []string{
		"../",  // Directory traversal
		"..\\", // Windows directory traversal
		"/../", // Absolute path traversal
	}

	for _, dangerous := range dangerousPatterns {
		if strings.Contains(pattern, dangerous) {
			return fmt.Errorf("exclude pattern contains potentially dangerous traversal: %s", pattern)
		}
	}

	return nil
}

// ValidateGlobalSettings validates the global settings
func ValidateGlobalSettings(settings GlobalSettings) error {
	if settings.MaxRetries < 0 || settings.MaxRetries > 10 {
		return fmt.Errorf("max_retries must be between 0 and 10")
	}

	// Allow zero values and treat them as defaults
	if settings.TimeoutSeconds != 0 && (settings.TimeoutSeconds < 1 || settings.TimeoutSeconds > 600) {
		return fmt.Errorf("timeout_seconds must be between 1 and 600")
	}

	if settings.ChunkSizeMB != 0 && (settings.ChunkSizeMB < 1 || settings.ChunkSizeMB > 1024) {
		return fmt.Errorf("chunk_size_mb must be between 1 and 1024")
	}

	if settings.ProgressUpdateIntervalMS != 0 && (settings.ProgressUpdateIntervalMS < 100 || settings.ProgressUpdateIntervalMS > 60000) {
		return fmt.Errorf("progress_update_interval_ms must be between 100 and 60000")
	}

	return nil
}

// ValidateNextcloudURL validates a Nextcloud-specific URL format
func ValidateNextcloudURL(nextcloudURL string) error {
	if nextcloudURL == "" {
		return fmt.Errorf("Nextcloud URL cannot be empty")
	}

	parsedURL, err := url.Parse(nextcloudURL)
	if err != nil {
		return fmt.Errorf("failed to parse Nextcloud URL: %w", err)
	}

	if parsedURL.Scheme != "https" {
		return fmt.Errorf("Nextcloud URL must use HTTPS")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("Nextcloud URL must have a valid host")
	}

	// Check if it contains the expected Nextcloud files app pattern
	if !strings.Contains(parsedURL.Path, "/apps/files/files/") {
		return fmt.Errorf("Nextcloud URL must contain '/apps/files/files/' path")
	}

	// Validate the dir parameter if present
	if dirParam := parsedURL.Query().Get("dir"); dirParam != "" {
		if !strings.HasPrefix(dirParam, "/") {
			return fmt.Errorf("dir parameter must start with '/'")
		}
	}

	return nil
}
