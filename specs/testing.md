# Testing Specification

## Overview

This document outlines the comprehensive testing strategy for the Nextcloud Sync CLI tool to ensure reliability, security, and performance.

## Testing Strategy

### Testing Pyramid

#### Unit Tests (70%)
- **Individual Component Testing**: Test each package/function independently
- **Fast Execution**: Sub-millisecond test execution
- **Mock Dependencies**: Use interfaces and mocks for external dependencies
- **High Coverage**: Target 90%+ code coverage

#### Integration Tests (20%)
- **Component Interaction**: Test integration between major components
- **Real Dependencies**: Use real Nextcloud instances for integration testing
- **Network Testing**: Test WebDAV client with real HTTP endpoints
- **Configuration Testing**: Test configuration management end-to-end

#### End-to-End Tests (10%)
- **Full Workflow**: Test complete synchronization workflows
- **User Scenarios**: Test real-world usage patterns
- **Performance Testing**: Test with large files and directories
- **Error Scenarios**: Test failure modes and recovery

## Test Infrastructure

### Testing Tools

#### Go Testing Framework
- **Testing Package**: Standard `testing` package
- **Assertions**: Use `testify/assert` for readable assertions
- **Mocking**: `testify/mock` for interface mocking
- **Coverage**: `go test -cover` for coverage reporting

#### External Testing Tools
- **HTTP Mocking**: `httptest` package for HTTP server mocking
- **File System**: Temp directories and files for filesystem testing
- **Nextcloud Mock**: Mock Nextcloud WebDAV server for testing
- **Docker**: Containerized Nextcloud for integration testing

### Test Environment Setup

#### Local Development
```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run integration tests
make test-integration

# Run end-to-end tests
make test-e2e
```

#### CI/CD Pipeline
- **GitHub Actions**: Automated testing on pull requests
- **Multiple Platforms**: Linux, macOS, Windows testing
- **Parallel Execution**: Parallel test execution for speed
- **Artifact Collection**: Test results and coverage reports

## Unit Testing

### Package Structure Testing

#### `internal/config` Tests
```go
func TestConfigLoad(t *testing.T) {
    tests := []struct {
        name     string
        config   string
        expected Config
        wantErr  bool
    }{
        // Test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

**Test Coverage Areas:**
- Configuration file parsing
- Validation logic
- Encryption/decryption
- Default value handling
- Error scenarios

#### `internal/auth` Tests
```go
func TestAppPasswordAuthentication(t *testing.T) {
    auth := NewAppPasswordAuth()
    
    // Test successful authentication
    token, err := auth.GetAuthToken("user", "password")
    assert.NoError(t, err)
    assert.Equal(t, "base64(user:password)", token)
    
    // Test empty password
    _, err = auth.GetAuthToken("user", "")
    assert.Error(t, err)
}
```

**Test Coverage Areas:**
- App password encryption/decryption
- HTTP Basic Auth header generation
- Credential validation
- Error handling for invalid credentials

#### `internal/webdav` Tests
```go
func TestPROPFINDRequest(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "PROPFIND", r.Method)
        assert.Contains(t, r.Header.Get("Authorization"), "Basic")
        
        // Mock WebDAV response
        w.Header().Set("Content-Type", "application/xml")
        w.WriteHeader(http.StatusMultiStatus)
        w.Write([]byte(mockWebDAVResponse))
    }))
    defer server.Close()
    
    client := NewWebDAVClient(server.URL, "user", "pass")
    props, err := client.GetProperties("/path")
    assert.NoError(t, err)
    assert.NotNil(t, props)
}
```

**Test Coverage Areas:**
- WebDAV method implementations
- HTTP request building and sending
- Response parsing
- Error handling for HTTP errors
- XML parsing for WebDAV responses

#### `internal/sync` Tests
```go
func TestFileComparison(t *testing.T) {
    local := &FileMetadata{
        Path:     "/test.txt",
        Modified: time.Now(),
        Size:     1024,
        ETag:     "local-etag",
    }
    
    remote := &FileMetadata{
        Path:     "/test.txt",
        Modified: time.Now().Add(-time.Hour),
        Size:     1024,
        ETag:     "remote-etag",
    }
    
    change := CompareFiles(local, remote)
    assert.Equal(t, ChangeTypeUpdate, change.Type)
    assert.Equal(t, DirectionLocalToRemote, change.Direction)
}
```

**Test Coverage Areas:**
- File comparison algorithms
- Change detection logic
- Conflict resolution
- Operation planning
- Error handling in sync operations

### Mock Implementations

#### WebDAV Client Mock
```go
type MockWebDAVClient struct {
    mock.Mock
}

func (m *MockWebDAVClient) GetProperties(path string) (*WebDAVProperties, error) {
    args := m.Called(path)
    return args.Get(0).(*WebDAVProperties), args.Error(1)
}

func (m *MockWebDAVClient) DownloadFile(path string) (io.ReadCloser, error) {
    args := m.Called(path)
    return args.Get(0).(io.ReadCloser), args.Error(1)
}
```

#### File System Mock
```go
func TestWithTempDir(t *testing.T) {
    tempDir, err := os.MkdirTemp("", "sync-test")
    require.NoError(t, err)
    defer os.RemoveAll(tempDir)
    
    // Create test files
    testFile := filepath.Join(tempDir, "test.txt")
    err = os.WriteFile(testFile, []byte("test content"), 0644)
    require.NoError(t, err)
    
    // Test file operations
    // ...
}
```

## Integration Testing

### Nextcloud Integration

#### Docker Setup
```dockerfile
# docker-compose.test.yml
version: '3.8'
services:
  nextcloud:
    image: nextcloud:stable
    ports:
      - "8080:80"
    environment:
      - SQLITE_DATABASE=nextcloud
      - NEXTCLOUD_ADMIN_USER=test
      - NEXTCLOUD_ADMIN_PASSWORD=test
    volumes:
      - nextcloud_data:/var/www/html/data
```

#### Integration Test Setup
```go
func TestNextcloudIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    // Setup test Nextcloud instance
    nextcloudURL := os.Getenv("NEXTCLOUD_URL")
    if nextcloudURL == "" {
        nextcloudURL = "http://localhost:8080"
    }
    
    // Create test user and app password
    auth := SetupTestUser(t, nextcloudURL)
    
    // Test real synchronization
    err := TestSyncWorkflow(t, nextcloudURL, auth)
    assert.NoError(t, err)
}
```

### Network Testing

#### HTTP Client Integration
```go
func TestHTTPClientIntegration(t *testing.T) {
    // Test with real Nextcloud instance
    client := NewHTTPClient(&HTTPConfig{
        Timeout: 30 * time.Second,
        RetryCount: 3,
    })
    
    resp, err := client.Get("https://cloud.example.com/remote.php/dav/files/test")
    assert.NoError(t, err)
    assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
```

### Configuration Integration

#### End-to-End Configuration Testing
```go
func TestConfigurationIntegration(t *testing.T) {
    tempDir := t.TempDir()
    configPath := filepath.Join(tempDir, "config.json")
    
    // Create configuration
    config := &Config{
        Servers: map[string]ServerConfig{
            "test": {
                URL:        "https://cloud.example.com",
                Username:   "test@example.com",
                AppPassword: "test-password",
            },
        },
    }
    
    // Save configuration
    err := SaveConfig(config, configPath)
    assert.NoError(t, err)
    
    // Load configuration
    loaded, err := LoadConfig(configPath)
    assert.NoError(t, err)
    assert.Equal(t, config.Servers["test"].URL, loaded.Servers["test"].URL)
}
```

## End-to-End Testing

### Full Workflow Testing

#### Synchronization Workflow
```go
func TestCompleteSyncWorkflow(t *testing.T) {
    // Setup test environment
    localDir := t.TempDir()
    nextcloudURL := SetupNextcloudInstance(t)
    auth := CreateTestUser(t, nextcloudURL)
    
    // Create test files
    CreateTestFiles(t, localDir, map[string]string{
        "file1.txt": "content1",
        "file2.txt": "content2",
        "subdir/file3.txt": "content3",
    })
    
    // Perform synchronization
    targetURL := fmt.Sprintf("%s/remote.php/dav/files/%s/test-sync", nextcloudURL, auth.Username)
    err := SyncDirectories(localDir, targetURL, auth, &SyncConfig{
        Direction:      DirectionUpload,
        ConflictPolicy: ConflictPolicySourceWins,
    })
    assert.NoError(t, err)
    
    // Verify results
    VerifyNextcloudFiles(t, nextcloudURL, auth, "/test-sync", map[string]string{
        "file1.txt": "content1",
        "file2.txt": "content2",
        "subdir/file3.txt": "content3",
    })
}
```

#### Performance Testing
```go
func TestLargeFileSync(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping performance test in short mode")
    }
    
    // Create large test file (100MB)
    largeFile := createLargeFile(t, 100*1024*1024)
    
    start := time.Now()
    err := SyncSingleFile(largeFile, targetURL, auth)
    duration := time.Since(start)
    
    assert.NoError(t, err)
    assert.Less(t, duration, 5*time.Minute) // Should complete within 5 minutes
}
```

### Error Scenario Testing

#### Network Failure Testing
```go
func TestNetworkFailureRecovery(t *testing.T) {
    // Setup flaky network connection
    flakyServer := setupFlakyServer(t, map[int]int{
        1: http.StatusRequestTimeout,  // First request times out
        2: http.StatusInternalServerError, // Second request fails
        3: http.StatusOK,                // Third request succeeds
    })
    
    client := NewWebDAVClient(flakyServer.URL, "user", "pass")
    client.RetryCount = 3
    
    // Should succeed after retries
    _, err := client.GetProperties("/test")
    assert.NoError(t, err)
}
```

#### Authentication Failure Testing
```go
func TestAuthenticationFailure(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Authorization") == "Basic dXNlcjp3cm9uZw==" { // base64(user:wrong)
            w.WriteHeader(http.StatusUnauthorized)
            return
        }
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()
    
    client := NewWebDAVClient(server.URL, "user", "wrong")
    _, err := client.GetProperties("/test")
    assert.Error(t, err)
    assert.IsType(t, &AuthenticationError{}, err)
}
```

## Performance Testing

### Benchmark Testing

#### File Comparison Benchmark
```go
func BenchmarkFileComparison(b *testing.B) {
    local := generateFileMetadata(1000) // 1000 files
    remote := generateFileMetadata(1000)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        changes := CompareFileTrees(local, remote)
        _ = changes
    }
}
```

#### Memory Usage Testing
```go
func TestMemoryUsage(t *testing.T) {
    // Test memory usage with large directories
    files := generateLargeFileTree(10000) // 10k files
    
    var m runtime.MemStats
    runtime.GC()
    runtime.ReadMemStats(&m)
    
    before := m.Alloc
    
    // Perform operation
    changes := AnalyzeChanges(files)
    _ = changes
    
    runtime.GC()
    runtime.ReadMemStats(&m)
    after := m.Alloc
    
    memoryUsed := after - before
    assert.Less(t, memoryUsed, uint64(50*1024*1024)) // Less than 50MB
}
```

## Security Testing

### Credential Security Testing

#### Password Encryption Testing
```go
func TestPasswordEncryption(t *testing.T) {
    password := "super-secret-password"
    
    // Encrypt password
    encrypted, err := EncryptPassword(password)
    assert.NoError(t, err)
    assert.NotEmpty(t, encrypted)
    
    // Ensure encrypted is different from original
    assert.NotEqual(t, password, encrypted)
    
    // Decrypt and verify
    decrypted, err := DecryptPassword(encrypted)
    assert.NoError(t, err)
    assert.Equal(t, password, decrypted)
}
```

### Path Traversal Testing

#### Path Validation Testing
```go
func TestPathTraversalPrevention(t *testing.T) {
    maliciousPaths := []string{
        "../../../etc/passwd",
        "..\\..\\..\\windows\\system32\\config\\sam",
        "/etc/shadow",
        "C:\\Windows\\System32\\config\\SAM",
    }
    
    for _, path := range maliciousPaths {
        err := ValidatePath(path)
        assert.Error(t, err, "Path should be rejected: %s", path)
    }
}
```

## Test Data Management

### Test Data Generation

#### File Tree Generation
```go
func GenerateTestFileTree(depth, filesPerDir int) map[string]*FileMetadata {
    tree := make(map[string]*FileMetadata)
    
    for i := 0; i < filesPerDir; i++ {
        path := fmt.Sprintf("/dir%d/file%d.txt", i%depth, i)
        tree[path] = &FileMetadata{
            Path:     path,
            Modified: time.Now().Add(-time.Duration(i) * time.Hour),
            Size:     int64(rand.Intn(1024 * 1024)),
            ETag:     fmt.Sprintf("etag-%d", i),
        }
    }
    
    return tree
}
```

### Test Environment Cleanup

#### Automatic Cleanup
```go
func TestWithCleanup(t *testing.T) {
    tempDir := t.TempDir() // Automatically cleaned up
    configFile := filepath.Join(tempDir, "config.json")
    
    // Test with temporary files
    // ...
    
    // No need for manual cleanup - t.TempDir() handles it
}
```

## Continuous Integration

### GitHub Actions Workflow

```yaml
name: Test
on: [push, pull_request]

jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        go: [1.21, 1.22]
    
    runs-on: ${{ matrix.os }}
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: ${{ matrix.go }}
    
    - name: Run tests
      run: make test
    
    - name: Run integration tests
      run: make test-integration
      env:
        NEXTCLOUD_URL: http://localhost:8080
    
    - name: Upload coverage
      uses: codecov/codecov-action@v3
      with:
        file: ./coverage.out
```

This comprehensive testing specification ensures the Nextcloud Sync CLI tool is thoroughly tested across all dimensions: functionality, performance, security, and reliability.