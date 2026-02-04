package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	config := NewConfig()

	assert.Equal(t, DefaultVersion, config.Version)
	assert.NotNil(t, config.Servers)
	assert.NotNil(t, config.SyncProfiles)
	assert.Equal(t, DefaultMaxRetries, config.GlobalSettings.MaxRetries)
	assert.Equal(t, DefaultTimeoutSeconds, config.GlobalSettings.TimeoutSeconds)
	assert.Equal(t, DefaultChunkSizeMB, config.GlobalSettings.ChunkSizeMB)
	assert.Equal(t, DefaultProgressUpdateIntervalMS, config.GlobalSettings.ProgressUpdateIntervalMS)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "config cannot be nil",
		},
		{
			name:    "empty version",
			config:  &Config{Version: ""},
			wantErr: true,
			errMsg:  "invalid version",
		},
		{
			name: "valid minimal config",
			config: &Config{
				Version: "1.0.0",
				Servers: map[string]Server{
					"default": {
						URL:      "https://cloud.example.com",
						Username: "user@example.com",
						AppPassword: EncryptedData{
							Encrypted: "test",
							Salt:      "test",
							Nonce:     "test",
							Algorithm: EncryptionAlgorithm,
						},
					},
				},
				SyncProfiles:   map[string]SyncProfile{},
				GlobalSettings: GlobalSettings{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateServerURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid HTTPS URL",
			url:     "https://cloud.example.com",
			wantErr: false,
		},
		{
			name:    "HTTP URL rejected",
			url:     "http://cloud.example.com",
			wantErr: true,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			url:     "not-a-url",
			wantErr: true,
		},
		{
			name:    "URL without host",
			url:     "https://",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServerURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "valid username",
			username: "user@example.com",
			wantErr:  false,
		},
		{
			name:     "empty username",
			username: "",
			wantErr:  true,
		},
		{
			name:     "username with colon",
			username: "user:name",
			wantErr:  true,
		},
		{
			name:     "too long username",
			username: string(make([]byte, 256)),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSyncProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile SyncProfile
		wantErr bool
	}{
		{
			name: "valid profile",
			profile: SyncProfile{
				Source:        "/home/user/Documents",
				Target:        "https://cloud.example.com/apps/files/files/12345?dir=/Documents",
				Bidirectional: true,
			},
			wantErr: false,
		},
		{
			name: "local to local",
			profile: SyncProfile{
				Source: "/home/user/Documents",
				Target: "/home/user/Backup",
			},
			wantErr: false, // Local-to-local is now allowed
		},
		{
			name: "empty source",
			profile: SyncProfile{
				Target: "https://cloud.example.com/apps/files/files/12345?dir=/Documents",
			},
			wantErr: true,
		},
		{
			name: "empty target",
			profile: SyncProfile{
				Source: "/home/user/Documents",
			},
			wantErr: true,
		},
		{
			name: "dangerous exclude pattern",
			profile: SyncProfile{
				Source:          "/home/user/Documents",
				Target:          "https://cloud.example.com/apps/files/files/12345?dir=/Documents",
				ExcludePatterns: []string{"../../../etc/passwd"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSyncProfile("test", tt.profile)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGlobalSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings GlobalSettings
		wantErr  bool
	}{
		{
			name: "valid settings",
			settings: GlobalSettings{
				MaxRetries:               3,
				TimeoutSeconds:           30,
				ChunkSizeMB:              50,
				ProgressUpdateIntervalMS: 1000,
			},
			wantErr: false,
		},
		{
			name: "invalid max retries",
			settings: GlobalSettings{
				MaxRetries: -1,
			},
			wantErr: true,
		},
		{
			name: "invalid timeout",
			settings: GlobalSettings{
				TimeoutSeconds: -1, // negative value
			},
			wantErr: true,
		},
		{
			name: "invalid chunk size",
			settings: GlobalSettings{
				ChunkSizeMB: -1, // negative value
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGlobalSettings(tt.settings)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateNextcloudURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid Nextcloud URL",
			url:     "https://cloud.example.com/apps/files/files/12345?dir=/Documents",
			wantErr: false,
		},
		{
			name:    "HTTP URL",
			url:     "http://cloud.example.com/apps/files/files/12345?dir=/Documents",
			wantErr: true,
		},
		{
			name:    "missing files path",
			url:     "https://cloud.example.com/apps/files/12345?dir=/Documents",
			wantErr: true,
		},
		{
			name:    "invalid dir parameter",
			url:     "https://cloud.example.com/apps/files/files/12345?dir=Documents",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNextcloudURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create test config
	originalConfig := NewConfig()
	originalConfig.Servers["default"] = Server{
		URL:      "https://cloud.example.com",
		Username: "user@example.com",
		AppPassword: EncryptedData{
			Encrypted: "test_encrypted",
			Salt:      "test_salt",
			Nonce:     "test_nonce",
			Algorithm: EncryptionAlgorithm,
		},
	}

	// Save config
	err := SaveConfig(originalConfig, configPath)
	require.NoError(t, err)

	// Verify file permissions
	info, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode())

	// Load config
	loadedConfig, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Verify loaded config
	assert.Equal(t, originalConfig.Version, loadedConfig.Version)
	assert.Equal(t, len(originalConfig.Servers), len(loadedConfig.Servers))
	assert.Equal(t, originalConfig.Servers["default"].URL, loadedConfig.Servers["default"].URL)
}

func TestLoadOrCreateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Test creating new config
	config, err := LoadOrCreateConfig(configPath)
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, DefaultVersion, config.Version)

	// Verify file was created
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	// Test loading existing config
	config2, err := LoadOrCreateConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, config.Version, config2.Version)
}

func TestGetDefaultConfigPath(t *testing.T) {
	path := GetDefaultConfigPath()
	assert.NotEmpty(t, path)
	assert.Contains(t, path, "config.json")
}

func TestGetLegacyConfigPath(t *testing.T) {
	path := GetLegacyConfigPath()
	assert.NotEmpty(t, path)
	assert.Contains(t, path, "config.json")
}
