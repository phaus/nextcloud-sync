package config

import (
	"time"
)

// Config represents the main configuration structure
type Config struct {
	Version        string                 `json:"version"`
	Servers        map[string]Server      `json:"servers"`
	SyncProfiles   map[string]SyncProfile `json:"sync_profiles"`
	GlobalSettings GlobalSettings         `json:"global_settings"`
}

// Server represents a Nextcloud server configuration
type Server struct {
	URL         string        `json:"url"`
	Username    string        `json:"username"`
	AppPassword EncryptedData `json:"app_password"`
	RootPath    string        `json:"root_path,omitempty"`
}

// EncryptedData represents encrypted app password with metadata
type EncryptedData struct {
	Encrypted string `json:"encrypted"`
	Salt      string `json:"salt"`
	Nonce     string `json:"nonce"`
	Algorithm string `json:"algorithm"`
}

// SyncProfile represents a synchronization profile
type SyncProfile struct {
	Source          string     `json:"source"`
	Target          string     `json:"target"`
	ExcludePatterns []string   `json:"exclude_patterns,omitempty"`
	Bidirectional   bool       `json:"bidirectional"`
	LastSync        *time.Time `json:"last_sync,omitempty"`
	ForceOverwrite  bool       `json:"force_overwrite,omitempty"`
}

// GlobalSettings represents application-wide settings
type GlobalSettings struct {
	MaxRetries               int  `json:"max_retries"`
	TimeoutSeconds           int  `json:"timeout_seconds"`
	ChunkSizeMB              int  `json:"chunk_size_mb"`
	ProgressUpdateIntervalMS int  `json:"progress_update_interval_ms"`
	EnableLargeFileSupport   bool `json:"enable_large_file_support,omitempty"`
	EnableCompression        bool `json:"enable_compression,omitempty"`
	VerifySSL                bool `json:"verify_ssl,omitempty"`
}

// Constants for default configuration values
const (
	DefaultVersion                  = "1.0"
	DefaultMaxRetries               = 3
	DefaultTimeoutSeconds           = 30
	DefaultChunkSizeMB              = 50
	DefaultProgressUpdateIntervalMS = 1000
	DefaultEnableLargeFileSupport   = true
	DefaultEnableCompression        = false
	DefaultVerifySSL                = true
	EncryptionAlgorithm             = "aes-256-gcm"
	PBKDF2Iterations                = 100000
	SaltSize                        = 32
	NonceSize                       = 12
)
