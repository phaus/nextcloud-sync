package main

import (
	"flag"
	"fmt"
	"strings"
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

	// TODO: Implement actual sync logic
	fmt.Println("Sync functionality not yet implemented")
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
