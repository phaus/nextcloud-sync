# Nextcloud Sync CLI

A powerful command-line tool for synchronizing local folders with Nextcloud instances using WebDAV protocol.

## Overview

The Nextcloud Sync CLI provides bidirectional synchronization between local directories and Nextcloud instances, with support for:
- App password authentication
- Conflict resolution with source-wins policy
- Progress tracking and resume capability
- Gitignore-style file exclusions
- Cross-platform compatibility

## Specs

The specifications can be found in specs/*.md 

## Features

### Core Functionality
- **Bidirectional Sync**: Sync changes both ways between local and remote
- **Source Wins**: Clear conflict resolution policy where source overrides target
- **App Password Authentication**: Secure authentication using Nextcloud app passwords
- **Progress Tracking**: Real-time progress bars with ETA and resume capability
- **File Exclusions**: `.nextcloudignore` with gitignore-style patterns
- **Change Detection**: Efficient sync using Nextcloud WebDAV properties

### Security Features
- **Encrypted Credential Storage**: AES-256-GCM encryption for app passwords
- **Secure Configuration**: 600 file permissions on config files
- **HTTPS Only**: All network communication over encrypted connections
- **Input Validation**: Comprehensive validation to prevent security issues

### Performance Features
- **Large File Support**: Chunked uploads with resume capability
- **Memory Efficient**: Streaming operations to minimize memory usage
- **Optimized Sync**: Uses Nextcloud's built-in change detection
- **Concurrent Operations**: Safe parallel operations where possible
- **Retry with Exponential Backoff**: Automatic retry for temporary network failures with configurable parameters

## Quick Start

### Installation

#### Download Binary (Recommended)
```bash
# Linux
curl -L https://github.com/user/nextcloud-sync/releases/latest/download/agent-linux-amd64.tar.gz | tar xz
sudo mv agent /usr/local/bin/

# macOS
curl -L https://github.com/user/nextcloud-sync/releases/latest/download/agent-darwin-amd64.tar.gz | tar xz
sudo mv agent /usr/local/bin/

# Windows
# Download and extract from releases page
```

#### Build from Source
```bash
git clone https://github.com/user/nextcloud-sync.git
cd nextcloud-sync
make build
sudo make install
```

### Initial Setup

1. **Generate App Password in Nextcloud**:
   - Go to Nextcloud Settings → Security
   - Click "Create new app password"
   - Label it (e.g., "CLI Sync Tool")
   - Copy the generated password

2. **Configure the Tool**:
```bash
agent setup
# Follow prompts to enter:
# - Nextcloud URL
# - Username
# - App password
```

### Basic Usage

#### Sync Local to Nextcloud
```bash
# Upload local folder to Nextcloud
agent ~/Documents https://cloud.example.com/apps/files/files/12345?dir=/Documents
```

#### Sync Nextcloud to Local
```bash
# Download from Nextcloud to local folder
agent https://cloud.example.com/apps/files/files/12345?dir=/Photos ~/Photos
```

#### Bidirectional Sync
```bash
# Sync both directions (default behavior)
agent ~/Projects https://cloud.example.com/apps/files/files/12345?dir=/Projects
```

#### Advanced Options
```bash
# Dry run to see what would be synced
agent --dry-run ~/Documents https://cloud.example.com/apps/files/files/12345?dir=/Documents

# Bidirectional sync (sync changes both ways)
agent --bidirectional ~/Documents https://cloud.example.com/apps/files/files/12345?dir=/Documents

# Exclude certain patterns
agent --exclude="*.tmp" --exclude=".DS_Store" ~/Documents https://cloud.example.com/...

# Use predefined profile
agent --profile=documents

# Verbose output
agent --verbose ~/Documents https://cloud.example.com/...
```

## Configuration

### Configuration File Location
- **Linux**: `~/.config/nextcloud-sync/config.json` or `~/.nextcloud-sync/config.json`
- **macOS**: `~/Library/Application Support/nextcloud-sync/config.json`
- **Windows**: `%APPDATA%\nextcloud-sync\config.json`

### Configuration Structure
```json
{
  "version": "1.0",
  "servers": {
    "default": {
      "url": "https://cloud.example.com",
      "username": "user@example.com",
      "app_password": {
        "encrypted": "base64_encrypted_blob",
        "salt": "base64_salt",
        "nonce": "base64_nonce",
        "algorithm": "aes-256-gcm"
      }
    }
  },
  "sync_profiles": {
    "documents": {
      "source": "~/Documents",
      "target": "https://cloud.example.com/apps/files/files/12345?dir=/Documents",
      "exclude_patterns": ["*.tmp", ".DS_Store"],
      "bidirectional": true
    }
  }
}
```

### File Exclusions

Create a `.nextcloudignore` file in your sync directory:

```
# Comments start with #
*.tmp
.DS_Store
.git/
node_modules/
*.log
temp/
```

## Command Reference

### Main Command
```bash
agent <source> <target> [options]
```

#### Arguments
- `source`: Source path (local folder or Nextcloud URL)
- `target`: Target path (local folder or Nextcloud URL)

#### Options
- `--bidirectional`: Enable bidirectional synchronization
- `--dry-run`: Show what would be synced without making changes
- `--force`: Force overwrite conflicting files
- `--exclude=PATTERN`: Additional exclude patterns
- `--profile=NAME`: Use predefined sync profile
- `--verbose`: Detailed logging output
- `--config=PATH`: Custom config file location

### Other Commands
```bash
# Setup wizard
agent setup

# Check for updates
agent update-check

# Show version
agent --version

# Test configuration
agent --config-test

# Test connectivity
agent --connectivity-test
```

## URL Format

Nextcloud URLs should be in the format:
```
https://cloud.example.com/apps/files/files/USER_ID?dir=/PATH
```

**Example**: `https://cloud.consolving.de/apps/files/files/2743527?dir=/uploads`

## Security

### Credential Storage
- App passwords are encrypted using AES-256-GCM
- Configuration files have 600 permissions (owner read/write only)
- No passwords are stored in plaintext or logged

### Network Security
- All communication uses HTTPS with certificate validation
- Authentication uses HTTP Basic Auth with app passwords
- No sensitive data is included in logs or error messages

### Best Practices
- Use dedicated app passwords for this tool
- Regularly rotate app passwords
- Keep the tool updated to latest version
- Use `.nextcloudignore` to exclude sensitive files

## Troubleshooting

### Common Issues

#### Authentication Failed
```bash
# Check credentials
agent --connectivity-test

# Reset and reconfigure
rm ~/.nextcloud-sync/config.json
agent setup
```

#### Permission Denied
```bash
# Check file permissions
ls -la ~/.nextcloud-sync/config.json
# Should be: -rw-------

# Fix permissions
chmod 600 ~/.nextcloud-sync/config.json
```

#### Connection Timeout
```bash
# Test connectivity
curl -I https://your-nextcloud.com/remote.php/dav/

# Check firewall and proxy settings
```

### Debug Information
```bash
# Enable verbose output
agent --verbose ~/Documents https://cloud.example.com/...

# Test configuration
agent --config-test

# Show version info
agent --version
```

### Log Files
- **Linux/macOS**: `~/.local/share/nextcloud-sync/logs/`
- **Windows**: `%APPDATA%\nextcloud-sync\logs\`

## Development

### Building from Source
```bash
# Clone repository
git clone https://github.com/user/nextcloud-sync.git
cd nextcloud-sync

# Install dependencies
go mod download

# Run tests
make test

# Build
make build

# Install
make install
```

### Testing
```bash
# Run all tests
make test

# Run integration tests (requires Nextcloud instance)
make test-integration

# Run with coverage
make test-coverage
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

- **Documentation**: [Project Wiki](https://github.com/user/nextcloud-sync/wiki)
- **Issues**: [GitHub Issues](https://github.com/user/nextcloud-sync/issues)
- **Discussions**: [GitHub Discussions](https://github.com/user/nextcloud-sync/discussions)

## Development Progress

### Recent Implementations (November 2024 - February 2026)
- ✅ **File Comparison Engine**: Comprehensive algorithms for comparing local and remote files with conflict detection
- ✅ **WebDAV Client Foundation**: Complete HTTP request/response handling with structured error types
- ✅ **Authentication System**: Secure app password handling with AES-256-GCM encryption
- ✅ **Configuration Management**: Encrypted credential storage with validation
- ✅ **File Exclusion System**: Gitignore-style pattern matching for sync filtering
- ✅ **Basic Sync Operations**: Complete file upload, download, directory operations with progress tracking
- ✅ **Conflict Resolution System**: Source-wins conflict resolution with comprehensive logging and reporting
- ✅ **Progress Tracking System**: Real-time progress bars, statistics, and transfer resume capability
- ✅ **Chunked Upload Support**: Large file upload with chunked transfer and resume capability (50MB+ threshold)
- ✅ **Retry Logic with Exponential Backoff**: Robust retry mechanism for temporary network errors with configurable parameters
- ✅ **URL Utilities**: Centralized URL parsing and validation for Nextcloud integration with WebDAV endpoint extraction
- ✅ **Bidirectional Sync CLI Support**: Complete CLI flag support for bidirectional synchronization with proper result flag handling
- ✅ **Test Suite Fixes**: Resolved failing tests in sync package, corrected mock implementations and delete operation logic

### Current Status
The basic sync operations are now implemented with file upload/download, directory management, and operation planning. Large file support with chunked uploads has been implemented to handle files efficiently. All core unit tests are now passing, providing a solid foundation for the sync engine. Next phases include performance optimization and advanced error handling.

### CI/CD Pipeline
The project includes a comprehensive GitHub Actions CI/CD pipeline that provides:
- **Multi-version testing**: Tests on Go 1.19, 1.20, and 1.21
- **Cross-platform builds**: Linux, macOS, and Windows binaries
- **Code quality checks**: Linting with golangci-lint and code formatting
- **Security scanning**: Automated vulnerability scanning with Gosec and Trivy
- **Coverage reporting**: Test coverage uploaded to Codecov
- **Release automation**: Automatic release asset generation and publishing

## Changelog

### Version 1.0.0 (Upcoming)
- Initial release
- Bidirectional synchronization
- App password authentication
- Progress tracking
- File exclusion support
- Cross-platform compatibility

---

**Note**: This is currently in development. See the [implementation plan](implementation-plan.md) for development progress.