package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadConfig loads configuration from the specified path
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	if err := ValidateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// SaveConfig saves configuration to the specified path
func SaveConfig(config *Config, path string) error {
	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to temporary file first
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary config file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename config file: %w", err)
	}

	return nil
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
	// Try XDG_CONFIG_HOME first, then fallback to ~/.config
	if xdgHome := os.Getenv("XDG_CONFIG_HOME"); xdgHome != "" {
		return filepath.Join(xdgHome, "nextcloud-sync", "config.json")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".config", "nextcloud-sync", "config.json")
}

// GetLegacyConfigPath returns the legacy configuration file path (~/.nextcloud-sync/config.json)
func GetLegacyConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".nextcloud-sync", "config.json")
}

// NewConfig creates a new default configuration
func NewConfig() *Config {
	return &Config{
		Version:      DefaultVersion,
		Servers:      make(map[string]Server),
		SyncProfiles: make(map[string]SyncProfile),
		GlobalSettings: GlobalSettings{
			MaxRetries:               DefaultMaxRetries,
			TimeoutSeconds:           DefaultTimeoutSeconds,
			ChunkSizeMB:              DefaultChunkSizeMB,
			ProgressUpdateIntervalMS: DefaultProgressUpdateIntervalMS,
			EnableLargeFileSupport:   DefaultEnableLargeFileSupport,
			EnableCompression:        DefaultEnableCompression,
			VerifySSL:                DefaultVerifySSL,
		},
	}
}

// LoadOrCreateConfig loads config from path or creates a default one if it doesn't exist
func LoadOrCreateConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create default config
		config := NewConfig()
		if err := SaveConfig(config, path); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return config, nil
	}

	return LoadConfig(path)
}

// LoadConfigFromPaths attempts to load config from multiple possible paths
func LoadConfigFromPaths(paths []string) (*Config, string, error) {
	for _, path := range paths {
		if path == "" {
			continue
		}

		if _, err := os.Stat(path); err == nil {
			config, err := LoadConfig(path)
			if err != nil {
				return nil, "", fmt.Errorf("failed to load config from %s: %w", path, err)
			}
			return config, path, nil
		}
	}

	return nil, "", fmt.Errorf("no configuration file found in any of the paths")
}

// MigrateConfig migrates configuration from old path to new path
func MigrateConfig(oldPath, newPath string) error {
	// Check if old config exists
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return nil // Nothing to migrate
	}

	// Check if new config already exists
	if _, err := os.Stat(newPath); err == nil {
		return nil // Already migrated
	}

	// Load old config
	config, err := LoadConfig(oldPath)
	if err != nil {
		return fmt.Errorf("failed to load old config: %w", err)
	}

	// Save to new location
	if err := SaveConfig(config, newPath); err != nil {
		return fmt.Errorf("failed to save migrated config: %w", err)
	}

	return nil
}
