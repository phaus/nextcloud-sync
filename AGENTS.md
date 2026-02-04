# AGENTS.md

This file provides guidelines and commands for agentic coding agents working on the Nextcloud Sync CLI project.

## Development Commands

### Building
```bash
# Build the main binary
make build
# or: go build -ldflags "-X main.version=dev" -o bin/agent cmd/sync/*.go

# Cross-platform builds
make build-all
# or:
GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=dev" -o bin/agent-linux-amd64 cmd/sync/*.go
GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=dev" -o bin/agent-darwin-amd64 cmd/sync/*.go
GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=dev" -o bin/agent-darwin-arm64 cmd/sync/*.go
GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=dev" -o bin/agent-windows-amd64.exe cmd/sync/*.go
```

### Testing
```bash
# Run all tests
make test
# or: go test -v -race -coverprofile=coverage.out ./...

# Run a single test file
go test -v ./internal/config
go test -v ./internal/auth
go test -v ./internal/webdav
go test -v ./internal/sync
go test -v ./internal/utils
go test -v ./pkg/exclude

# Run a single test function
go test -v ./internal/config -run TestConfigLoad
go test -v ./internal/auth -run TestAppPasswordAuthentication

# Run integration tests
make test-integration

# Run with coverage
make test-coverage
# or: go test -v -race -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html
```

### Linting and Quality
```bash
# Lint code
make lint
# or: golangci-lint run

# Format code
go fmt ./...
goimports -w .

# Tidy dependencies
go mod tidy
go mod verify
```

## Code Style Guidelines

### File Organization
- Follow the directory structure defined in `implementation-plan.md`
- Keep files focused on single responsibilities
- Use `internal/` for packages not intended for external use
- Use `pkg/` for reusable packages

### Import Organization
```go
import (
    // Standard library first
    "context"
    "crypto/aes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "time"

    // Third-party packages second
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    // Internal packages third
    "github.com/user/nextcloud-sync/internal/auth"
    "github.com/user/nextcloud-sync/internal/config"
)
```

### Naming Conventions
- **Packages**: lowercase, single word when possible (`config`, `auth`, `sync`)
- **Constants**: `UPPER_SNAKE_CASE` for exported constants
- **Variables**: `camelCase` for local variables, `PascalCase` for exported
- **Functions**: `PascalCase` for exported, `camelCase` for unexported
- **Interfaces**: `PascalCase` with `-er` suffix (`Reader`, `Writer`, `Authenticator`)
- **Structs**: `PascalCase`, descriptive names (`Config`, `WebDAVClient`, `SyncEngine`)

### Type Definitions
```go
// Interface definitions should be small and focused
type AuthProvider interface {
    GetAuthToken(username, password string) (string, error)
    ValidateCredentials(url, username, password string) error
}

// Structs should have clear field names
type Config struct {
    Version  string            `json:"version"`
    Servers  map[string]Server `json:"servers"`
    Profiles map[string]Profile `json:"sync_profiles"`
}

// Use constructor functions for complex initialization
func NewConfig() *Config {
    return &Config{
        Version:  "1.0",
        Servers:  make(map[string]Server),
        Profiles: make(map[string]Profile),
    }
}
```

### Error Handling
- Always handle errors returned by functions
- Use `fmt.Errorf` with `%w` for error wrapping
- Define custom error types for domain-specific errors
- Include context in error messages but not sensitive data

```go
// Good error handling
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
    }

    var config Config
    if err := json.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
    }

    return &config, nil
}

// Custom error types
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}
```

### Function Design
- Keep functions small and focused (ideally under 50 lines)
- Use descriptive function names that explain what they do
- Return errors as the last return value
- Use named parameters when function signature is unclear

```go
// Good function design
func CompareFiles(local, remote *FileMetadata) (*Change, error) {
    if local == nil || remote == nil {
        return nil, fmt.Errorf("file metadata cannot be nil")
    }

    if local.Modified.After(remote.Modified) {
        return &Change{Type: ChangeUpdate, Direction: LocalToRemote}, nil
    }

    if remote.Modified.After(local.Modified) {
        return &Change{Type: ChangeUpdate, Direction: RemoteToLocal}, nil
    }

    return &Change{Type: ChangeNone}, nil
}
```

### Testing Guidelines
- Write tests alongside the code they test
- Use table-driven tests for multiple scenarios
- Use `testify/assert` for assertions and `testify/require` for setup
- Mock external dependencies using interfaces
- Test both success and failure scenarios

```go
// Table-driven test example
func TestConfigLoad(t *testing.T) {
    tests := []struct {
        name     string
        config   string
        expected *Config
        wantErr  bool
    }{
        {
            name: "valid config",
            config: `{"version": "1.0", "servers": {}, "profiles": {}}`,
            expected: &Config{Version: "1.0", Servers: map[string]Server{}, Profiles: map[string]Profile{}},
            wantErr: false,
        },
        {
            name:     "invalid json",
            config:   `{"invalid": json}`,
            expected: nil,
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create temp file
            tmp := t.TempDir()
            configFile := filepath.Join(tmp, "config.json")
            require.NoError(t, os.WriteFile(configFile, []byte(tt.config), 0644))

            // Test function
            result, err := LoadConfig(configFile)

            if tt.wantErr {
                assert.Error(t, err)
                assert.Nil(t, result)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expected, result)
            }
        })
    }
}
```

### Documentation
- Add package-level comments explaining the package's purpose
- Document exported functions, types, and constants
- Include usage examples in documentation
- Keep documentation consistent with code

### Security Guidelines
- Never log or expose passwords or other sensitive data
- Validate all external input
- Use constant-time comparison for sensitive data
- Zero out sensitive data when no longer needed
- Follow the security specifications in `specs/security.md`

### Performance Guidelines
- Use streaming I/O for large files to minimize memory usage
- Implement connection pooling for HTTP clients
- Use contexts for cancellation and timeouts
- Avoid unnecessary allocations in hot paths
- Profile and optimize critical paths

### Git Workflow
- Commit code changes and corresponding tests together
- Use descriptive commit messages following the pattern: `feat: add webdav client`
- Update `implementation-plan.md` when completing tasks
- Run tests before committing
- Push only when all tests pass

## Project-Specific Notes

### Configuration Management
- Configuration files should be encrypted with proper permissions (600)
- Use the config structure defined in `specs/api-specification.md`
- Validate all configuration values before use

### WebDAV Implementation
- Follow the WebDAV API specification in `specs/api-specification.md`
- Use proper HTTP status codes and error handling
- Implement proper retry logic with exponential backoff

### Authentication
- Use app passwords only, never store in plaintext
- Implement proper encryption as specified in `specs/security.md`
- Validate credentials before storage

### Testing Integration
- Use the testing strategy outlined in `specs/testing.md`
- Mock Nextcloud endpoints for unit tests
- Use real Nextcloud instances for integration testing

### WebDAV Error Handling
- All WebDAV operations now return structured WebDAVError types
- Use `IsWebDAVError(err)` to check if error is a WebDAVError
- Use type-specific methods: `IsAuthError()`, `IsNotFoundError()`, `IsConflictError()`, etc.
- Use `IsTemporary()` to determine if operation should be retried
- Errors include context: path, method, status code, and descriptive message

When working on this project, always reference the specifications in `specs/` and the `implementation-plan.md` for detailed requirements and architecture decisions.