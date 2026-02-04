package sync

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/phaus/nextcloud-sync/internal/webdav"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockWebDAVClient implements a simple mock for testing
type MockWebDAVClient struct {
	files map[string]*webdav.WebDAVFile
}

func NewMockWebDAVClient() *MockWebDAVClient {
	return &MockWebDAVClient{
		files: make(map[string]*webdav.WebDAVFile),
	}
}

func (m *MockWebDAVClient) AddFile(path string, info *webdav.WebDAVFile) {
	m.files[path] = info
}

func (m *MockWebDAVClient) ListDirectory(ctx context.Context, path string) ([]*webdav.WebDAVFile, error) {
	var result []*webdav.WebDAVFile
	for file, info := range m.files {
		if filepath.Dir(file) == path || (path == "/" && filepath.Dir(file) == ".") {
			result = append(result, info)
		}
	}
	return result, nil
}

func (m *MockWebDAVClient) GetProperties(ctx context.Context, path string) (*webdav.WebDAVProperties, error) {
	if info, exists := m.files[path]; exists {
		return &webdav.WebDAVProperties{
			Path:         info.Path,
			Size:         info.Size,
			LastModified: info.LastModified,
			ETag:         info.ETag,
			ContentType:  info.ContentType,
			IsDirectory:  info.IsDirectory,
		}, nil
	}
	return nil, &webdav.WebDAVError{StatusCode: 404}
}

func (m *MockWebDAVClient) DownloadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *MockWebDAVClient) UploadFile(ctx context.Context, path string, reader io.Reader, size int64) error {
	return nil
}

func (m *MockWebDAVClient) UploadFileChunked(ctx context.Context, path string, content io.Reader, size int64, chunkSize int64) error {
	return nil
}

func (m *MockWebDAVClient) ResumeChunkedUpload(ctx context.Context, path string, content io.Reader, size int64, offset int64, chunkSize int64) error {
	return nil
}

func (m *MockWebDAVClient) CreateDirectory(ctx context.Context, path string) error {
	return nil
}

func (m *MockWebDAVClient) DeleteFile(ctx context.Context, path string) error {
	delete(m.files, path)
	return nil
}

func (m *MockWebDAVClient) MoveFile(ctx context.Context, source, destination string) error {
	if info, exists := m.files[source]; exists {
		m.files[destination] = info
		delete(m.files, source)
	}
	return nil
}

func (m *MockWebDAVClient) CopyFile(ctx context.Context, source, destination string) error {
	if info, exists := m.files[source]; exists {
		m.files[destination] = info
	}
	return nil
}

func (m *MockWebDAVClient) Close() error {
	return nil
}

func TestSyncEngine_CreateExcludeMatcher(t *testing.T) {
	tests := []struct {
		name           string
		config         *SyncConfig
		expectError    bool
		expectPatterns []string
	}{
		{
			name: "default patterns only",
			config: &SyncConfig{
				Source:          "/test/source",
				Target:          "https://cloud.example.com/files/test",
				ExcludePatterns: []string{},
			},
			expectError:    false,
			expectPatterns: []string{".DS_Store", "Thumbs.db", "*.tmp"}, // From default patterns
		},
		{
			name: "custom patterns added",
			config: &SyncConfig{
				Source:          "/test/source",
				Target:          "https://cloud.example.com/files/test",
				ExcludePatterns: []string{"*.log", "temp/"},
			},
			expectError:    false,
			expectPatterns: []string{".DS_Store", "Thumbs.db", "*.tmp", "*.log", "temp/"},
		},
		{
			name: "invalid pattern",
			config: &SyncConfig{
				Source:          "/test/source",
				Target:          "https://cloud.example.com/files/test",
				ExcludePatterns: []string{"[invalid"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := createExcludeMatcher(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, matcher)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, matcher)

				patterns := matcher.GetPatternSet().GetPatterns()
				assert.GreaterOrEqual(t, len(patterns), len(tt.expectPatterns))
			}
		})
	}
}

func TestSyncEngine_BuildLocalFileTree(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	dirs := []string{"src", "temp", "logs"}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		require.NoError(t, err)
	}

	// Create test files
	files := map[string]string{
		"main.go":          "package main",
		"test.txt":         "test content",
		"error.log":        "error log",
		"src/app.go":       "package app",
		"temp/cache.tmp":   "cache data",
		"logs/app.log":     "app log",
		".hidden":          "hidden file",
		".nextcloudignore": "*.tmp\n*.log\n.hidden",
	}

	for file, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, file), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Test with exclude patterns
	config := &SyncConfig{
		Source:          tmpDir,
		Target:          "https://cloud.example.com/files/test",
		ExcludePatterns: []string{"*.txt"}, // Additional pattern
	}

	engine, err := NewSyncEngine(NewMockWebDAVClient(), config)
	require.NoError(t, err)

	ctx := context.Background()
	tree, err := engine.BuildLocalFileTree(ctx)
	require.NoError(t, err)

	// Verify tree structure
	assert.NotNil(t, tree)
	assert.Greater(t, len(tree.PathMap), 0)

	// Check that excluded files are not in the tree
	assert.NotContains(t, tree.PathMap, "test.txt")       // Excluded by pattern
	assert.NotContains(t, tree.PathMap, "temp/cache.tmp") // Excluded by .nextcloudignore
	assert.NotContains(t, tree.PathMap, "error.log")      // Excluded by .nextcloudignore
	assert.NotContains(t, tree.PathMap, "logs/app.log")   // Excluded by .nextcloudignore
	assert.NotContains(t, tree.PathMap, ".hidden")        // Excluded by .nextcloudignore

	// Check that included files are in the tree
	assert.Contains(t, tree.PathMap, "")           // Root directory
	assert.Contains(t, tree.PathMap, "main.go")    // Should be included
	assert.Contains(t, tree.PathMap, "src/app.go") // Should be included
	assert.Contains(t, tree.PathMap, "src")        // Directory should be included
	assert.Contains(t, tree.PathMap, "temp")       // Directory should be included
	assert.Contains(t, tree.PathMap, "logs")       // Directory should be included

	// Verify file metadata
	if mainNode, exists := tree.PathMap["main.go"]; exists {
		assert.False(t, mainNode.Metadata.IsDirectory)
		assert.Equal(t, "main.go", mainNode.Metadata.Name)
		assert.Greater(t, mainNode.Metadata.Size, int64(0))
	}

	if srcNode, exists := tree.PathMap["src"]; exists {
		assert.True(t, srcNode.Metadata.IsDirectory)
		assert.Equal(t, "src", srcNode.Metadata.Name)
	}
}

func TestSyncEngine_BuildRemoteFileTree(t *testing.T) {
	mockClient := NewMockWebDAVClient()

	// Add remote files
	now := time.Now()
	mockClient.AddFile("/", &webdav.WebDAVFile{
		Name:         "test",
		IsDirectory:  true,
		LastModified: now,
	})

	mockClient.AddFile("/test", &webdav.WebDAVFile{
		Name:         "documents.txt",
		IsDirectory:  false,
		Size:         1024,
		LastModified: now,
		ETag:         "\"test123\"",
	})

	mockClient.AddFile("/test", &webdav.WebDAVFile{
		Name:         "temp.tmp",
		IsDirectory:  false,
		Size:         512,
		LastModified: now,
		ETag:         "\"temp456\"",
	})

	config := &SyncConfig{
		Source:          "/local/source",
		Target:          "https://cloud.example.com/files/test?dir=/test",
		ExcludePatterns: []string{"*.tmp"}, // Should exclude temp.tmp
	}

	engine, err := NewSyncEngine(mockClient, config)
	require.NoError(t, err)

	ctx := context.Background()
	tree, err := engine.BuildRemoteFileTree(ctx)
	require.NoError(t, err)

	// Verify tree structure
	assert.NotNil(t, tree)
	assert.Greater(t, len(tree.PathMap), 0)

	// Check that excluded files are not in the tree
	assert.NotContains(t, tree.PathMap, "temp.tmp")

	// Check that included files are in the tree
	assert.Contains(t, tree.PathMap, "documents.txt")
}

func TestSyncEngine_FilterExcludedChanges(t *testing.T) {
	config := &SyncConfig{
		Source:          "/test/source",
		Target:          "https://cloud.example.com/files/test",
		ExcludePatterns: []string{"*.log", "*.tmp"},
	}

	engine, err := NewSyncEngine(NewMockWebDAVClient(), config)
	require.NoError(t, err)

	// Create test changes
	changes := []*Change{
		{
			Type:       ChangeCreate,
			Direction:  LocalToRemote,
			LocalPath:  "include.txt",
			RemotePath: "include.txt",
		},
		{
			Type:       ChangeCreate,
			Direction:  LocalToRemote,
			LocalPath:  "exclude.log",
			RemotePath: "exclude.log",
		},
		{
			Type:       ChangeCreate,
			Direction:  LocalToRemote,
			LocalPath:  "exclude.tmp",
			RemotePath: "exclude.tmp",
		},
		{
			Type:       ChangeCreate,
			Direction:  LocalToRemote,
			LocalPath:  "another.txt",
			RemotePath: "another.txt",
		},
	}

	// Filter changes
	filtered := engine.filterExcludedChanges(changes)

	// Verify excluded changes were removed
	assert.Len(t, filtered, 2)
	assert.Equal(t, "include.txt", filtered[0].LocalPath)
	assert.Equal(t, "another.txt", filtered[1].LocalPath)
}

func TestSyncEngine_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create local test files
	err := os.WriteFile(filepath.Join(tmpDir, "include.txt"), []byte("include"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "exclude.log"), []byte("exclude"), 0644)
	require.NoError(t, err)

	// Create mock remote client
	mockClient := NewMockWebDAVClient()

	config := &SyncConfig{
		Source:          tmpDir,
		Target:          "https://cloud.example.com/files/test?dir=/test",
		Direction:       SyncDirectionLocalToRemote,
		ExcludePatterns: []string{"*.log"},
		DryRun:          true, // Use dry run to avoid actual file operations
	}

	engine, err := NewSyncEngine(mockClient, config)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := engine.Sync(ctx)
	require.NoError(t, err)

	// Verify sync result
	assert.NotNil(t, result)
	assert.True(t, result.Success)

	// Should only include .txt file, not .log file
	assert.GreaterOrEqual(t, result.TotalFiles, 1)
}
