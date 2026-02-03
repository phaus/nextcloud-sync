# Deployment Specification

## Overview

This document defines the deployment strategy, build processes, and distribution methods for the Nextcloud Sync CLI tool.

## Build System

### Build Requirements

#### Go Environment
- **Minimum Version**: Go 1.21
- **Target Platforms**: Linux, macOS, Windows
- **Architectures**: amd64, arm64
- **Static Linking**: Prefer static binaries for portability

#### Build Tools
- **Build System**: Go modules and standard `go build`
- **Version Management**: Git tags and build info embedding
- **Code Generation**: No external code generation required
- **Testing**: Standard `go test` with coverage reporting

### Build Configuration

#### Makefile Structure
```makefile
# Variables
VERSION := $(shell git describe --tags --always)
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)"

# Targets
.PHONY: build clean test lint release

# Main build target
build:
	go build $(LDFLAGS) -o bin/agent cmd/sync/main.go

# Cross-platform builds
build-all:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/agent-linux-amd64 cmd/sync/main.go
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/agent-darwin-amd64 cmd/sync/main.go
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/agent-darwin-arm64 cmd/sync/main.go
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/agent-windows-amd64.exe cmd/sync/main.go

# Development targets
test:
	go test -v -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/ coverage.out
```

#### Build Information Embedding
```go
// Build information embedded at compile time
var (
    version   = "dev"
    buildTime = "unknown"
    gitCommit = "unknown"
)
```

## Distribution Strategy

### Binary Distribution

#### Package Formats
- **Linux**: Tar.gz, Debian (.deb), RPM (.rpm)
- **macOS**: Tar.gz, macOS Package (.pkg)
- **Windows**: Zip installer, MSI installer

#### Distribution Channels
- **GitHub Releases**: Primary distribution channel
- **Package Managers**: Homebrew, apt, yum, chocolatey
- **Direct Downloads**: HTTPS distribution from project website

### Version Management

#### Semantic Versioning
- **Format**: `MAJOR.MINOR.PATCH`
- **Pre-releases**: `MAJOR.MINOR.PATCH-rc.N`
- **Build Metadata**: Optional build information

#### Release Process
1. **Development**: Feature development on main branch
2. **Testing**: Comprehensive testing and QA
3. **Release Candidate**: Pre-release testing
4. **Release**: Tagged stable release
5. **Distribution**: Build and publish packages

## Installation Methods

### Method 1: Direct Binary Download

#### Linux/macOS
```bash
# Download and install
curl -L https://github.com/user/nextcloud-sync/releases/latest/download/agent-linux-amd64.tar.gz | tar xz
sudo mv agent /usr/local/bin/

# Verify installation
agent --version
```

#### Windows
```powershell
# Download and extract
Invoke-WebRequest -Uri "https://github.com/user/nextcloud-sync/releases/latest/download/agent-windows-amd64.zip" -OutFile "agent.zip"
Expand-Archive -Path agent.zip -DestinationPath .
Move-Item agent.exe C:\Program Files\agent\
```

### Method 2: Package Managers

#### Homebrew (macOS)
```bash
# Formula example
class Agent < Formula
  desc "Nextcloud sync CLI tool"
  homepage "https://github.com/user/nextcloud-sync"
  url "https://github.com/user/nextcloud-sync/releases/latest/download/agent-darwin-amd64.tar.gz"
  sha256 "<checksum>"
  
  def install
    bin.install "agent"
  end
end
```

#### APT (Debian/Ubuntu)
```bash
# Install from repository
curl -s https://packagecloud.io/install/repositories/user/nextcloud-sync/script.deb.sh | sudo bash
sudo apt-get install agent
```

#### Chocolatey (Windows)
```xml
<!-- chocolatey package -->
<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://schemas.microsoft.com/packaging/2015/06/nuspec.xsd">
  <metadata>
    <id>agent</id>
    <version>1.0.0</version>
    <packageSourceUrl>https://github.com/user/nextcloud-sync</packageSourceUrl>
    <title>Nextcloud Sync Agent</title>
    <authors>Author Name</authors>
    <description>CLI tool for syncing folders with Nextcloud</description>
  </metadata>
  <files>
    <file src="agent.exe" target="tools" />
  </files>
</package>
```

### Method 3: Build from Source

#### Prerequisites
```bash
# Install Go 1.21+
# Install Git
# Install build tools (make, gcc for CGO if needed)
```

#### Build Commands
```bash
# Clone repository
git clone https://github.com/user/nextcloud-sync.git
cd nextcloud-sync

# Build
make build

# Install
make install
```

## Configuration Management

### Default Configuration Locations

#### Platform-Specific Paths
- **Linux**: `~/.config/nextcloud-sync/` or `~/.nextcloud-sync/`
- **macOS**: `~/Library/Application Support/nextcloud-sync/`
- **Windows**: `%APPDATA%\nextcloud-sync\`

#### Configuration File Migration
- **Backward Compatibility**: Support old configuration locations
- **Migration**: Automatic migration from old to new locations
- **Fallback**: Graceful fallback if preferred location unavailable

### First-Time Setup

#### Interactive Setup
```bash
agent setup
# Interactive prompts for:
# - Nextcloud URL
# - Username
# - App password
# - Default sync profiles
```

#### Configuration File Generation
- **Initial Setup**: Create default configuration structure
- **Validation**: Validate all configuration values
- **Security**: Apply secure file permissions
- **Testing**: Verify connectivity and authentication

## System Integration

### Shell Integration

#### Command Completion
- **Bash**: Dynamic completion script
- **Zsh**: Completion function
- **PowerShell**: Completion module
- **Fish**: Universal completion script

#### PATH Integration
- **System PATH**: Install to standard binary location
- **User PATH**: Install to user's local bin directory
- **Validation**: Verify binary is in PATH after installation

### Service Integration (Optional)

#### systemd Service (Linux)
```ini
[Unit]
Description=Nextcloud Sync Agent
After=network.target

[Service]
Type=oneshot
User=%i
ExecStart=/usr/local/bin/agent sync %h/Documents %h/Nextcloud/Documents
```

#### LaunchAgent (macOS)
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.user.nextcloud-sync</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/agent</string>
        <string>sync</string>
        <string>/Users/user/Documents</string>
        <string>https://cloud.example.com/apps/files/files/12345?dir=/Documents</string>
    </array>
</dict>
</plist>
```

## Maintenance and Updates

### Update Mechanism

#### Built-in Update Check
```bash
agent update-check
# Check for new version
# Prompt for update
# Download and install new version
```

#### Package Manager Updates
- **APT**: `sudo apt-get update && sudo apt-get upgrade agent`
- **Homebrew**: `brew upgrade agent`
- **Chocolatey**: `choco upgrade agent`

### Migration Between Versions

#### Configuration Migration
- **Schema Updates**: Automatic configuration format updates
- **Data Migration**: Preserve user settings across versions
- **Compatibility**: Maintain backward compatibility where possible

#### Database Migration (if applicable)
- **Version Tracking**: Track installed version and migration state
- **Rollback**: Support for rolling back problematic migrations
- **Testing**: Comprehensive migration testing

## Security Considerations

### Binary Signing

#### Code Signing
- **macOS**: Apple Developer ID signing
- **Windows**: Authenticode signing
- **Linux**: GPG signature verification

#### Checksum Verification
```bash
# Verify download integrity
sha256sum agent-linux-amd64.tar.gz
# Compare with published checksum
```

### Secure Distribution
- **HTTPS Only**: All downloads over HTTPS
- **Mirror Verification**: Verify package repository integrity
- **Signature Verification**: Cryptographic verification of packages

## Troubleshooting

### Common Issues

#### Permission Problems
- **Binary Permissions**: Ensure executable permissions
- **Configuration Permissions**: Verify config file access
- **Network Permissions**: Check firewall settings

#### Configuration Issues
- **Path Problems**: Verify file paths exist
- **Authentication**: Check app password validity
- **Network**: Verify Nextcloud connectivity

#### Platform-Specific Issues
- **Windows**: Antivirus interference
- **macOS**: Gatekeeper restrictions
- **Linux**: SELinux/AppArmor policies

### Debug Information

#### Diagnostic Commands
```bash
agent --version          # Version and build info
agent --config-test      # Configuration validation
agent --connectivity-test # Network connectivity test
agent --debug            # Detailed debug output
```

#### Log Collection
- **Location**: Platform-specific log directory
- **Format**: Structured logging with timestamps
- **Levels**: DEBUG, INFO, WARN, ERROR
- **Rotation**: Automatic log rotation and cleanup