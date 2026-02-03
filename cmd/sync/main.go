package main

import (
	"fmt"
	"os"
)

var (
	version = "dev"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("nextcloud-sync %s\n", version)
		os.Exit(0)
	}

	fmt.Println("Nextcloud Sync CLI - Development Version")
	fmt.Println("Use --version for version information")
}
