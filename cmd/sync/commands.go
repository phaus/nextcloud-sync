package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/phaus/nextcloud-sync/internal/auth"
	"github.com/phaus/nextcloud-sync/internal/config"
	"github.com/phaus/nextcloud-sync/internal/sync"
	"github.com/phaus/nextcloud-sync/internal/webdav"
)

// Command represents a CLI command
type Command struct {
	Name        string
	Description string
	Handler     func(args []string) error
}

// CommandRegistry holds all available commands
var commands = []Command{
	{
		Name:        "setup",
		Description: "Interactive setup wizard for configuration",
		Handler:     handleSetup,
	},
	{
		Name:        "update-check",
		Description: "Check for updates",
		Handler:     handleUpdateCheck,
	},
}

// Global flags
var (
	dryRun           = flag.Bool("dry-run", false, "Show what would be synced without making changes")
	force            = flag.Bool("force", false, "Force overwrite conflicting files")
	excludePatterns  = multiFlag{}
	profile          = flag.String("profile", "", "Use predefined sync profile")
	verbose          = flag.Bool("verbose", false, "Detailed logging output")
	configPath       = flag.String("config", "", "Custom config file location")
	configTest       = flag.Bool("config-test", false, "Test configuration")
	connectivityTest = flag.Bool("connectivity-test", false, "Test connectivity")
	showHelp         = flag.Bool("help", false, "Show help information")
	showVersion      = flag.Bool("version", false, "Show version information")
)

// multiFlag allows multiple values for a flag
type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

// registerFlags configures all command-line flags
func registerFlags() {
	flag.Var(&excludePatterns, "exclude", "Additional exclude patterns (can be used multiple times)")
}

// parseCommand checks if the first argument is a registered command
func parseCommand(args []string) (*Command, []string) {
	if len(args) == 0 {
		return nil, args
	}

	for _, cmd := range commands {
		if args[0] == cmd.Name {
			return &cmd, args[1:]
		}
	}

	return nil, args
}

// showUsage displays help information
func showUsage() {
	fmt.Printf("Nextcloud Sync CLI - Version %s\n\n", version)
	fmt.Println("Usage:")
	fmt.Printf("  agent <source> <target> [options]   Sync between source and target\n")
	fmt.Printf("  agent <command> [options]           Run a command\n\n")

	fmt.Println("Commands:")
	for _, cmd := range commands {
		fmt.Printf("  %-15s %s\n", cmd.Name, cmd.Description)
	}
	fmt.Println()

	fmt.Println("Sync Options:")
	flag.PrintDefaults()
	fmt.Println()

	fmt.Println("Examples:")
	fmt.Println("  agent ~/Documents https://cloud.example.com/apps/files/files/12345?dir=/Documents")
	fmt.Println("  agent --dry-run --verbose ~/Photos https://cloud.example.com/...")
	fmt.Println("  agent --profile=documents")
	fmt.Println("  agent setup")
	fmt.Println()

	fmt.Println("For more information, see the documentation at:")
	fmt.Println("  https://github.com/user/nextcloud-sync/wiki")
}

// handleSync processes the main sync command
func handleSync(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("sync command requires source and target arguments")
	}

	source := args[0]
	target := args[1]

	if *verbose {
		fmt.Printf("Source: %s\n", source)
		fmt.Printf("Target: %s\n", target)
		fmt.Printf("Dry run: %t\n", *dryRun)
		fmt.Printf("Force: %t\n", *force)
		if len(excludePatterns) > 0 {
			fmt.Printf("Exclude patterns: %s\n", strings.Join(excludePatterns, ", "))
		}
		if *profile != "" {
			fmt.Printf("Profile: %s\n", *profile)
		}
	}

	// Determine sync direction
	direction := sync.SyncDirectionLocalToRemote
	if strings.Contains(source, "://") && !strings.Contains(target, "://") {
		direction = sync.SyncDirectionRemoteToLocal
	} else if strings.Contains(source, "://") && strings.Contains(target, "://") {
		return fmt.Errorf("both source and target cannot be remote URLs")
	}

	// Load configuration
	configPath := *configPath
	if configPath == "" {
		configPath = getDefaultConfigPath()
	}

	var appConfig *config.Config
	if _, err := os.Stat(configPath); err == nil {
		appConfig, err = config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	} else {
		// Create default config if none exists
		appConfig = config.NewConfig()
	}

	// Create sync configuration
	syncConfig := &sync.SyncConfig{
		Source:          source,
		Target:          target,
		Direction:       direction,
		DryRun:          *dryRun,
		Force:           *force,
		ExcludePatterns: []string(excludePatterns),
		MaxRetries:      3,
		Timeout:         30 * time.Second,
		ChunkSize:       1024 * 1024, // 1MB
		ConflictPolicy:  "source_wins",
	}

	// Create authentication provider
	var authProvider auth.AuthProvider
	var err error

	if strings.Contains(target, "://") || strings.Contains(source, "://") {
		// Determine server URL from remote target/source
		serverURL := extractServerURL(source, target)
		username, password, err := getCredentials(appConfig, serverURL)
		if err != nil {
			return fmt.Errorf("failed to get credentials: %w", err)
		}

		authProvider, err = auth.NewAppPasswordAuth(serverURL, username, password)
		if err != nil {
			return fmt.Errorf("failed to create auth provider: %w", err)
		}
	}

	// Create WebDAV client
	var webdavClient webdav.Client
	if authProvider != nil {
		webdavClient, err = webdav.NewClient(authProvider)
		if err != nil {
			return fmt.Errorf("failed to create WebDAV client: %w", err)
		}
		defer webdavClient.Close()
	}

	// Create sync engine
	engine, err := sync.NewSyncEngine(webdavClient, syncConfig)
	if err != nil {
		return fmt.Errorf("failed to create sync engine: %w", err)
	}

	// Execute sync
	ctx := context.Background()
	result, err := engine.Sync(ctx)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	// Display results
	displaySyncResult(result)

	return nil
}

// Command handlers

func handleSetup(args []string) error {
	fmt.Println("Setup wizard not yet implemented")
	return nil
}

func handleUpdateCheck(args []string) error {
	fmt.Println("Update check not yet implemented")
	return nil
}

func handleConfigTest() error {
	fmt.Println("Configuration test not yet implemented")
	return nil
}

func handleConnectivityTest() error {
	fmt.Println("Connectivity test not yet implemented")
	return nil
}

// getDefaultConfigPath returns the default configuration file path
func getDefaultConfigPath() string {
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "nextcloud-sync", "config.json")
		}
		return filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming", "nextcloud-sync", "config.json")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "nextcloud-sync", "config.json")
	default: // Linux and others
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			return filepath.Join(xdgConfig, "nextcloud-sync", "config.json")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "nextcloud-sync", "config.json")
	}
}

// extractServerURL extracts the server URL from source or target
func extractServerURL(source, target string) string {
	if strings.Contains(target, "://") {
		return extractBaseURL(target)
	}
	if strings.Contains(source, "://") {
		return extractBaseURL(source)
	}
	return ""
}

// extractBaseURL extracts base URL from a Nextcloud URL
func extractBaseURL(url string) string {
	if idx := strings.Index(url, "/apps/"); idx != -1 {
		return url[:idx]
	}
	if idx := strings.Index(url, "/remote.php/"); idx != -1 {
		return url[:idx]
	}
	return strings.TrimSuffix(url, "/")
}

// getCredentials retrieves credentials for the given server URL
func getCredentials(appConfig *config.Config, serverURL string) (string, string, error) {
	// Try to find matching server in config
	if appConfig != nil && appConfig.Servers != nil {
		for name, server := range appConfig.Servers {
			if strings.Contains(server.URL, extractBaseURL(serverURL)) || extractBaseURL(serverURL) == server.URL {
				// Decrypt password
				password, err := config.DecryptPassword(server.AppPassword)
				if err != nil {
					return "", "", fmt.Errorf("failed to decrypt password for server %s: %w", name, err)
				}
				return server.Username, password, nil
			}
		}
	}

	// Fallback to environment variables or prompt user
	username := os.Getenv("NEXTCLOUD_USERNAME")
	password := os.Getenv("NEXTCLOUD_PASSWORD")

	if username == "" || password == "" {
		return "", "", fmt.Errorf("no credentials found for server %s. Please run 'agent setup' or set NEXTCLOUD_USERNAME and NEXTCLOUD_PASSWORD environment variables", serverURL)
	}

	return username, password, nil
}

// displaySyncResult displays the result of a sync operation
func displaySyncResult(result *sync.SyncResult) {
	fmt.Printf("\nSync completed in %v\n", result.Duration)
	fmt.Printf("Total files: %d\n", result.TotalFiles)
	fmt.Printf("Processed files: %d\n", result.ProcessedFiles)

	if result.TotalSize > 0 {
		fmt.Printf("Total size: %s\n", formatBytes(result.TotalSize))
	}

	if result.TransferredSize > 0 {
		fmt.Printf("Transferred: %s\n", formatBytes(result.TransferredSize))
	}

	fmt.Printf("Created: %d files\n", len(result.CreatedFiles))
	fmt.Printf("Updated: %d files\n", len(result.UpdatedFiles))
	fmt.Printf("Deleted: %d files\n", len(result.DeletedFiles))

	if len(result.SkippedFiles) > 0 {
		fmt.Printf("Skipped: %d files\n", len(result.SkippedFiles))
		for _, skipped := range result.SkippedFiles {
			if *verbose {
				fmt.Printf("  - %s\n", skipped)
			}
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("Warnings: %d\n", len(result.Warnings))
		for _, warning := range result.Warnings {
			fmt.Printf("  - %s\n", warning)
		}
	}

	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(result.Errors))
		for _, error := range result.Errors {
			fmt.Printf("  - %s\n", error)
		}
	}

	if len(result.Conflicts) > 0 {
		fmt.Printf("Conflicts: %d\n", len(result.Conflicts))
		for _, conflict := range result.Conflicts {
			fmt.Printf("  - %s\n", conflict.Description)
		}
	}

	if result.DryRun {
		fmt.Println("\nNote: This was a dry run. No actual changes were made.")
	}

	if result.Success {
		if len(result.Errors) == 0 {
			fmt.Println("\n✓ Sync completed successfully!")
		} else {
			fmt.Println("\n⚠ Sync completed with some errors.")
		}
	} else {
		fmt.Println("\n✗ Sync failed.")
	}
}

// formatBytes formats a byte count as human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
