package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
	fmt.Println("🔧 Nextcloud Sync CLI Setup Wizard")
	fmt.Println("================================")
	fmt.Println("This wizard will help you configure your Nextcloud sync settings.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Load existing config or create new one
	configPath := getDefaultConfigPath()
	var appConfig *config.Config

	if _, err := os.Stat(configPath); err == nil {
		appConfig, err = config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load existing config: %w", err)
		}
		fmt.Printf("Found existing configuration at: %s\n", configPath)
		fmt.Println("Press Enter to continue, or Ctrl+C to exit...")
		reader.ReadString('\n')
	} else {
		appConfig = config.NewConfig()
		fmt.Printf("Creating new configuration at: %s\n", configPath)
	}

	// Setup servers
	fmt.Println("\n📡 Server Configuration")
	fmt.Println("-----------------------")

	if appConfig.Servers == nil {
		appConfig.Servers = make(map[string]config.Server)
	}

	for {
		serverName, err := promptString(reader, "Enter a name for this server (or press Enter to finish): ", false)
		if err != nil {
			return err
		}
		if strings.TrimSpace(serverName) == "" {
			break
		}

		// Check if server already exists
		if _, exists := appConfig.Servers[serverName]; exists {
			fmt.Printf("Server '%s' already exists. Choose a different name.\n", serverName)
			continue
		}

		server, err := setupServer(reader)
		if err != nil {
			return fmt.Errorf("failed to setup server: %w", err)
		}

		appConfig.Servers[serverName] = server
		fmt.Printf("✅ Server '%s' configured successfully.\n", serverName)

		cont, err := promptYesNo(reader, "Add another server? (y/n): ", false)
		if err != nil || !cont {
			break
		}
	}

	// Setup sync profiles
	fmt.Println("\n📁 Sync Profile Configuration")
	fmt.Println("------------------------------")

	if appConfig.SyncProfiles == nil {
		appConfig.SyncProfiles = make(map[string]config.SyncProfile)
	}

	for {
		profileName, err := promptString(reader, "Enter a name for this sync profile (or press Enter to finish): ", false)
		if err != nil {
			return err
		}
		if strings.TrimSpace(profileName) == "" {
			break
		}

		// Check if profile already exists
		if _, exists := appConfig.SyncProfiles[profileName]; exists {
			fmt.Printf("Profile '%s' already exists. Choose a different name.\n", profileName)
			continue
		}

		profile, err := setupSyncProfile(reader, appConfig.Servers)
		if err != nil {
			return fmt.Errorf("failed to setup sync profile: %w", err)
		}

		appConfig.SyncProfiles[profileName] = profile
		fmt.Printf("✅ Sync profile '%s' configured successfully.\n", profileName)

		cont, err := promptYesNo(reader, "Add another sync profile? (y/n): ", false)
		if err != nil || !cont {
			break
		}
	}

	// Save configuration
	fmt.Printf("\n💾 Saving configuration to: %s\n", configPath)

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := config.SaveConfig(appConfig, configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Set secure permissions
	if err := os.Chmod(configPath, 0600); err != nil {
		fmt.Printf("⚠️  Warning: Could not set secure permissions on config file: %v\n", err)
	}

	fmt.Println("✅ Configuration saved successfully!")
	fmt.Printf("\n🎉 Setup complete! You can now use:\n")
	fmt.Printf("   agent --profile=%s\n", "your_profile_name")
	fmt.Printf("\nOr sync directly:\n")
	fmt.Printf("   agent ~/Documents https://your-nextcloud.com/apps/files/files/12345?dir=/Documents\n")

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

// setupServer configures a new server with user input
func setupServer(reader *bufio.Reader) (config.Server, error) {
	var server config.Server

	// Server URL
	for {
		url, err := promptString(reader, "Enter Nextcloud server URL (e.g., https://cloud.example.com): ", true)
		if err != nil {
			return server, err
		}

		// Validate URL
		if !strings.HasPrefix(url, "https://") {
			fmt.Println("⚠️  Warning: Server should use HTTPS for security.")
			cont, err := promptYesNo(reader, "Continue anyway? (y/n): ", false)
			if err != nil || !cont {
				continue
			}
		}

		server.URL = strings.TrimSuffix(url, "/")
		break
	}

	// Username
	username, err := promptString(reader, "Enter your Nextcloud username: ", true)
	if err != nil {
		return server, err
	}
	server.Username = username

	// App Password
	fmt.Println("\n📝 App Password Setup:")
	fmt.Println("You need to generate an app password in Nextcloud:")
	fmt.Println("1. Log into your Nextcloud instance")
	fmt.Println("2. Go to Settings → Security")
	fmt.Println("3. Click 'Create new app password'")
	fmt.Println("4. Enter a name (e.g., 'CLI Sync Tool')")
	fmt.Println("5. Copy the generated password")

	for {
		password, err := promptPassword(reader, "Enter app password: ")
		if err != nil {
			return server, err
		}

		if password == "" {
			fmt.Println("Password cannot be empty.")
			continue
		}

		// Encrypt password
		encrypted, err := config.EncryptPassword(password)
		if err != nil {
			return server, fmt.Errorf("failed to encrypt password: %w", err)
		}

		server.AppPassword = encrypted
		// Clear password from memory (best effort)
		if len(password) > 0 {
			password = "" // overwrite string reference
		}
		break
	}

	// Root path (optional)
	rootPath, err := promptString(reader, "Enter root path (optional, press Enter for default): ", false)
	if err != nil {
		return server, err
	}
	if rootPath != "" {
		server.RootPath = strings.TrimPrefix(rootPath, "/")
	}

	return server, nil
}

// setupSyncProfile configures a new sync profile with user input
func setupSyncProfile(reader *bufio.Reader, servers map[string]config.Server) (config.SyncProfile, error) {
	var profile config.SyncProfile

	// Source path
	source, err := promptString(reader, "Enter local source path (e.g., ~/Documents): ", true)
	if err != nil {
		return profile, err
	}
	profile.Source = source

	// Target path
	target, err := promptString(reader, "Enter remote target path (e.g., /Documents): ", true)
	if err != nil {
		return profile, err
	}

	// Construct full Nextcloud URL if we have servers configured
	if len(servers) > 0 {
		fmt.Println("\nAvailable servers:")
		serverNames := make([]string, 0, len(servers))
		for name := range servers {
			serverNames = append(serverNames, name)
		}
		for i, name := range serverNames {
			fmt.Printf("%d. %s (%s)\n", i+1, name, servers[name].URL)
		}

		for {
			choice, err := promptString(reader, "Select server (enter number or URL): ", false)
			if err != nil {
				return profile, err
			}

			if choice == "" {
				// Use direct URL input
				url, err := promptString(reader, "Enter full Nextcloud URL: ", true)
				if err != nil {
					return profile, err
				}
				profile.Target = url
				break
			}

			// Try to parse as number
			if num, err := strconv.Atoi(choice); err == nil {
				// User entered a number
				if num >= 1 && num <= len(serverNames) {
					serverName := serverNames[num-1]
					server := servers[serverName]
					// Extract user ID from URL or ask for it
					userID, err := promptString(reader, "Enter your user ID or leave empty to detect: ", false)
					if err != nil {
						return profile, err
					}
					if userID == "" {
						userID = server.Username
					}
					profile.Target = fmt.Sprintf("%s/apps/files/files/%s?dir=%s",
						server.URL, userID, strings.TrimPrefix(target, "/"))
					break
				}
				fmt.Printf("Invalid selection. Please enter 1-%d.\n", len(serverNames))
			} else {
				// User entered a server name
				if server, exists := servers[choice]; exists {
					userID, err := promptString(reader, "Enter your user ID or leave empty to detect: ", false)
					if err != nil {
						return profile, err
					}
					if userID == "" {
						userID = server.Username
					}
					profile.Target = fmt.Sprintf("%s/apps/files/files/%s?dir=%s",
						server.URL, userID, strings.TrimPrefix(target, "/"))
					break
				}
				fmt.Printf("Server '%s' not found.\n", choice)
			}
		}
	} else {
		// No servers configured, ask for full URL
		url, err := promptString(reader, "Enter full Nextcloud URL: ", true)
		if err != nil {
			return profile, err
		}
		profile.Target = url
	}

	// Exclude patterns
	excludeList, err := promptString(reader, "Enter exclude patterns (comma-separated, e.g., *.tmp,.DS_Store): ", false)
	if err != nil {
		return profile, err
	}
	if excludeList != "" {
		patterns := strings.Split(excludeList, ",")
		for i, pattern := range patterns {
			patterns[i] = strings.TrimSpace(pattern)
		}
		profile.ExcludePatterns = patterns
	}

	// Bidirectional sync
	bidirectional, err := promptYesNo(reader, "Enable bidirectional sync? (y/n): ", false)
	if err != nil {
		return profile, err
	}
	profile.Bidirectional = bidirectional

	// Force overwrite
	force, err := promptYesNo(reader, "Force overwrite on conflicts? (y/n): ", false)
	if err != nil {
		return profile, err
	}
	profile.ForceOverwrite = force

	return profile, nil
}

// promptString prompts the user for string input
func promptString(reader *bufio.Reader, prompt string, required bool) (string, error) {
	for {
		fmt.Print(prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		input = strings.TrimSpace(input)
		if required && input == "" {
			fmt.Println("This field is required.")
			continue
		}
		return input, nil
	}
}

// promptPassword prompts the user for password input (visible for now due to Go version constraints)
func promptPassword(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// promptYesNo prompts the user for a yes/no response
func promptYesNo(reader *bufio.Reader, prompt string, required bool) (bool, error) {
	for {
		fmt.Print(prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}

		input = strings.ToLower(strings.TrimSpace(input))
		if input == "" && !required {
			return false, nil
		}

		switch input {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Println("Please enter 'y' or 'n'.")
		}
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
