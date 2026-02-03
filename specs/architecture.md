# System Architecture

## Overview

The Nextcloud Sync CLI tool is designed as a modular, extensible system for synchronizing local folders with Nextcloud instances using WebDAV protocol.

## High-Level Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   CLI Interface │───▶│   Sync Engine    │───▶│  WebDAV Client  │
│   (cmd/sync)    │    │  (internal/sync) │    │ (internal/webdav)│
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Config Mgr    │    │   Progress Track │    │   Auth Manager  │
│ (internal/config)│   │ (internal/progress)│  │  (internal/auth) │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## Component Architecture

### 1. CLI Interface (`cmd/sync/`)

**Responsibilities:**
- Parse command-line arguments
- Validate source and target paths
- Initialize and orchestrate other components
- Handle user interaction and error reporting

**Key Files:**
- `main.go` - Entry point and command orchestration
- `commands.go` - Command definitions and validation
- `output.go` - User-facing output formatting

### 2. Configuration Management (`internal/config/`)

**Responsibilities:**
- Load and parse configuration files
- Manage server profiles and sync profiles
- Handle credential encryption/decryption
- Provide configuration validation

**Key Files:**
- `config.go` - Configuration structure and loading
- `encryption.go` - Password encryption utilities
- `validation.go` - Configuration validation logic

**Data Flow:**
```
Config File (~/.nextcloud-sync/config.json)
        │
        ▼
Parser → Validation → Decryption → Runtime Config
```

### 3. Authentication Manager (`internal/auth/`)

**Responsibilities:**
- Manage app password authentication
- Handle HTTP Basic Auth header generation
- Provide credential validation
- Manage authentication state

**Key Files:**
- `auth.go` - Authentication interface and implementation
- `password.go` - Password encryption and storage
- `validator.go` - Credential validation logic

### 4. WebDAV Client (`internal/webdav/`)

**Responsibilities:**
- Implement WebDAV protocol operations
- Handle HTTP requests to Nextcloud
- Manage connection pooling and retries
- Parse XML responses and error handling

**Key Files:**
- `client.go` - WebDAV client interface
- `requests.go` - HTTP request builders
- `responses.go` - Response parsing utilities
- `properties.go` - WebDAV property handling

**HTTP Client Architecture:**
```
HTTP Client (with timeouts)
        │
        ▼
Request Builder → Auth Header → Send Request
        │
        ▼
Response Parser → Error Handler → Return Results
```

### 5. Sync Engine (`internal/sync/`)

**Responsibilities:**
- Coordinate bidirectional synchronization
- Implement change detection algorithms
- Handle conflict resolution
- Manage file operations (upload/download/delete)

**Key Files:**
- `engine.go` - Main synchronization logic
- `comparison.go` - File comparison algorithms
- `operations.go` - File operation handlers
- `conflict.go` - Conflict resolution logic

**Sync Flow:**
```
Source Scan → Target Scan → Compare Changes → Plan Operations → Execute → Report
```

### 6. Progress Tracking (`internal/progress/`)

**Responsibilities:**
- Display progress bars for file transfers
- Track synchronization statistics
- Handle resume capability for interrupted transfers
- Provide status updates and ETA calculations

**Key Files:**
- `progress.go` - Progress bar implementation
- `statistics.go` - Sync statistics tracking
- `resume.go` - Transfer resume logic

## Data Flow Architecture

### Synchronization Workflow

1. **Initialization Phase**
   ```
   CLI Args → Config Load → Auth Setup → Target Validation
   ```

2. **Discovery Phase**
   ```
   Local File Scan → Remote File Scan → Build File Trees
   ```

3. **Comparison Phase**
   ```
   Local Tree + Remote Tree → Change Detection → Operation Plan
   ```

4. **Execution Phase**
   ```
   Operation Plan → File Operations → Progress Tracking → Status Updates
   ```

5. **Completion Phase**
   ```
   Final Statistics → Status Report → Cleanup
   ```

### File State Management

Each tracked file maintains the following state:
- Local path and metadata
- Remote path and metadata
- Last synchronization timestamp
- Current operation status
- Conflict resolution history

## Security Architecture

### Credential Management
```
App Password → AES-256 Encryption → Config File Storage → Runtime Decryption
```

### Network Security
- HTTPS-only communication with Nextcloud
- Certificate validation
- Timeout and retry limits
- Request/response logging (without sensitive data)

## Performance Architecture

### Concurrency Model
- Single-threaded file operations for stability
- Parallel metadata fetching where possible
- Chunked file uploads for large files
- Progress updates on separate goroutine

### Memory Management
- Streaming file operations (no full file buffering)
- Metadata caching with size limits
- Garbage collection friendly structures
- Memory usage monitoring and limits

## Error Handling Architecture

### Error Hierarchy
```
Error Base
├── NetworkError
├── AuthenticationError
├── FileSystemError
├── ConfigurationError
└── SyncError (with sub-types)
```

### Error Recovery
- Automatic retry with exponential backoff
- Graceful degradation for non-critical errors
- Detailed error reporting with context
- Resume capability for interrupted operations

## Extension Points

### Future Enhancements
- Plugin system for custom file processors
- Additional authentication methods (OAuth2)
- Multiple sync strategies (mirroring, incremental)
- Event-driven synchronization (file watchers)
- Cloud storage provider abstraction

### Integration Points
- Configuration file format extensibility
- WebDAV client interface for other protocols
- Progress tracking interface for GUI implementations
- Authentication interface for alternative methods