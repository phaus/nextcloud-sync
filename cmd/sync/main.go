package main

import (
	"fmt"
	"log"
	"os"
)

var (
	version = "dev"
)

func main() {
	// Parse and validate command-line arguments
	if err := ParseAndValidate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Get sync arguments
	source, target, _ := GetSyncArgs()

	// Setup logging based on verbose flag
	setupLogging()

	// Handle sync command
	if err := handleSync([]string{source, target}); err != nil {
		log.Fatalf("Sync failed: %v", err)
	}
}

// setupLogging configures logging based on flags
func setupLogging() {
	if IsVerbose() {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetFlags(log.LstdFlags)
	}
}
