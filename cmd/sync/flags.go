package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ParseAndValidate handles command-line argument parsing and validation
func ParseAndValidate() error {
	// Register all flags
	registerFlags()

	// Parse flags
	flag.Parse()

	// Handle special flags that exit immediately
	if *showVersion {
		fmt.Printf("nextcloud-sync %s\n", version)
		os.Exit(0)
	}

	if *showHelp {
		showUsage()
		os.Exit(0)
	}

	// Handle test commands
	if *configTest {
		if err := handleConfigTest(); err != nil {
			return fmt.Errorf("config test failed: %w", err)
		}
		os.Exit(0)
	}

	if *connectivityTest {
		if err := handleConnectivityTest(); err != nil {
			return fmt.Errorf("connectivity test failed: %w", err)
		}
		os.Exit(0)
	}

	// Get remaining arguments after flag parsing
	args := flag.Args()

	// Check if this is a command (setup, update-check, etc.)
	if cmd, cmdArgs := parseCommand(args); cmd != nil {
		if err := cmd.Handler(cmdArgs); err != nil {
			return fmt.Errorf("command '%s' failed: %w", cmd.Name, err)
		}
		os.Exit(0)
	}

	// Otherwise, treat as sync command
	if len(args) == 0 {
		showUsage()
		os.Exit(1)
	}

	// Validate sync arguments
	if err := validateSyncArgs(args); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	// Validate flags
	if err := validateFlags(); err != nil {
		return fmt.Errorf("invalid flags: %w", err)
	}

	return nil
}

// validateSyncArgs validates the main sync arguments
func validateSyncArgs(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("sync command requires at least source and target arguments")
	}

	source := args[0]
	target := args[1]

	// Validate source path
	if !isValidPath(source) {
		return fmt.Errorf("invalid source path: %s", source)
	}

	// Validate target path
	if !isValidPath(target) {
		return fmt.Errorf("invalid target path: %s", target)
	}

	// Check if both are local paths (invalid)
	if isLocalPath(source) && isLocalPath(target) {
		return fmt.Errorf("at least one of source or target must be a remote URL")
	}

	return nil
}

// validateFlags validates the flag values
func validateFlags() error {
	// Validate exclude patterns
	for _, pattern := range excludePatterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("empty exclude pattern not allowed")
		}
	}

	// Validate profile name
	if *profile != "" {
		if !isValidProfileName(*profile) {
			return fmt.Errorf("invalid profile name: %s", *profile)
		}
	}

	// Validate config path
	if *configPath != "" {
		if !filepath.IsAbs(*configPath) {
			return fmt.Errorf("config path must be absolute: %s", *configPath)
		}

		// Check if directory exists
		dir := filepath.Dir(*configPath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("config directory does not exist: %s", dir)
		}
	}

	return nil
}

// isValidPath checks if a path is valid (either local path or URL)
func isValidPath(path string) bool {
	return isLocalPath(path) || isURL(path)
}

// isLocalPath checks if a path is a valid local path
func isLocalPath(path string) bool {
	// Check for common URL patterns
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return false
	}

	// Check for valid local path
	// Allow relative paths, absolute paths, and ~ expansion
	if path == "" {
		return false
	}

	// Basic check for invalid characters in filenames
	invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*"}
	for _, char := range invalidChars {
		if strings.Contains(path, char) {
			return false
		}
	}

	return true
}

// isURL checks if a string is a valid URL
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// isValidProfileName checks if a profile name is valid
func isValidProfileName(name string) bool {
	if name == "" {
		return false
	}

	// Profile names should be alphanumeric with hyphens and underscores
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return false
		}
	}

	return true
}

// GetSyncArgs returns the validated sync arguments
func GetSyncArgs() (source, target string, remainingArgs []string) {
	args := flag.Args()
	if len(args) >= 2 {
		source = args[0]
		target = args[1]
		if len(args) > 2 {
			remainingArgs = args[2:]
		}
	}
	return
}

// GetConfigPath returns the resolved config file path
func GetConfigPath() string {
	if *configPath != "" {
		return *configPath
	}

	// Default config locations based on OS
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Try ~/.config/nextcloud-sync/config.json first, then ~/.nextcloud-sync/config.json
	configPath := filepath.Join(home, ".config", "nextcloud-sync", "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = filepath.Join(home, ".nextcloud-sync", "config.json")
	}

	return configPath
}

// IsVerbose returns whether verbose logging is enabled
func IsVerbose() bool {
	return *verbose
}

// IsDryRun returns whether dry-run mode is enabled
func IsDryRun() bool {
	return *dryRun
}

// IsForce returns whether force mode is enabled
func IsForce() bool {
	return *force
}

// GetExcludePatterns returns the exclude patterns
func GetExcludePatterns() []string {
	return []string(excludePatterns)
}

// GetProfile returns the profile name
func GetProfile() string {
	return *profile
}
