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

// mockWebDAVClient implements webdav.Client for testing
type mockWebDAVClient struct {
	files       map[string]*mockFile
	directories map[string]bool
}

type mockFile struct {
	content []byte
	modTime time.Time
	isDir   bool
}

func newMockWebDAVClient() *mockWebDAVClient {
	return &mockWebDAVClient{
		files:       make(map[string]*mockFile),
		directories: make(map[string]bool),
	}
}

func (m *mockWebDAVClient) ListDirectory(ctx context.Context, path string) ([]*webdav.WebDAVFile, error) {
	return nil, nil
}

func (m *mockWebDAVClient) GetProperties(ctx context.Context, path string) (*webdav.WebDAVProperties, error) {
	if file, exists := m.files[path]; exists {
		return &webdav.WebDAVProperties{
			Path:         path,
			Size:         int64(len(file.content)),
			LastModified: file.modTime,
			IsDirectory:  file.isDir,
		}, nil
	}
	return nil, webdav.NewWebDAVError(404, path, "PROPFIND")
}

func (m *mockWebDAVClient) DownloadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if file, exists := m.files[path]; exists && !file.isDir {
		return io.NopCloser(io.NopCloser(&mockReader{content: file.content})), nil
	}
	return nil, webdav.NewWebDAVError(404, path, "GET")
}

func (m *mockWebDAVClient) UploadFile(ctx context.Context, path string, content io.Reader, size int64) error {
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}

	m.files[path] = &mockFile{
		content: data,
		modTime: time.Now(),
		isDir:   false,
	}
	return nil
}

func (m *mockWebDAVClient) UploadFileChunked(ctx context.Context, path string, content io.Reader, size int64, chunkSize int64) error {
	// For mock purposes, just read all content and store it
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}

	m.files[path] = &mockFile{
		content: data,
		modTime: time.Now(),
		isDir:   false,
	}
	return nil
}

func (m *mockWebDAVClient) ResumeChunkedUpload(ctx context.Context, path string, content io.Reader, size int64, offset int64, chunkSize int64) error {
	// For mock purposes, just read all content and store it
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}

	m.files[path] = &mockFile{
		content: data,
		modTime: time.Now(),
		isDir:   false,
	}
	return nil
}

func (m *mockWebDAVClient) CreateDirectory(ctx context.Context, path string) error {
	m.directories[path] = true
	m.files[path] = &mockFile{
		modTime: time.Now(),
		isDir:   true,
	}
	return nil
}

func (m *mockWebDAVClient) DeleteFile(ctx context.Context, path string) error {
	delete(m.files, path)
	delete(m.directories, path)
	return nil
}

func (m *mockWebDAVClient) MoveFile(ctx context.Context, source, destination string) error {
	if file, exists := m.files[source]; exists {
		m.files[destination] = file
		delete(m.files, source)
	}
	if m.directories[source] {
		m.directories[destination] = true
		delete(m.directories, source)
	}
	return nil
}

func (m *mockWebDAVClient) CopyFile(ctx context.Context, source, destination string) error {
	if file, exists := m.files[source]; exists {
		// Copy the content
		newFile := &mockFile{
			content: make([]byte, len(file.content)),
			modTime: file.modTime,
			isDir:   file.isDir,
		}
		copy(newFile.content, file.content)
		m.files[destination] = newFile
	}
	if m.directories[source] {
		m.directories[destination] = true
	}
	return nil
}

func (m *mockWebDAVClient) Close() error {
	return nil
}

// mockReader implements io.Reader
type mockReader struct {
	content []byte
	pos     int
}

func (r *mockReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.content) {
		return 0, io.EOF
	}

	n = copy(p, r.content[r.pos:])
	r.pos += n
	return n, nil
}

// mockProgressTracker implements ProgressTracker for testing
type mockProgressTracker struct {
	operations  []string
	startCount  int
	updateCount int
	finishCount int
	errorCount  int
}

func (m *mockProgressTracker) Start(total int64) {
	m.startCount++
}

func (m *mockProgressTracker) Update(current int64) {
	m.updateCount++
}

func (m *mockProgressTracker) Finish() {
	m.finishCount++
}

func (m *mockProgressTracker) SetOperation(operation string) {
	m.operations = append(m.operations, operation)
}

func (m *mockProgressTracker) Error(err error) {
	m.errorCount++
}

func TestNewOperationExecutor(t *testing.T) {
	mockClient := newMockWebDAVClient()
	config := &SyncConfig{
		Direction: SyncDirectionLocalToRemote,
	}

	executor := NewOperationExecutor(mockClient, config)

	assert.NotNil(t, executor)
	assert.Equal(t, mockClient, executor.webdavClient)
	assert.Equal(t, config, executor.config)
}

func TestExecuteOperation_Upload(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content for upload")
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	// Setup executor
	mockClient := newMockWebDAVClient()
	progressTracker := &mockProgressTracker{}
	config := &SyncConfig{
		ProgressTracker: progressTracker,
	}
	executor := NewOperationExecutor(mockClient, config)

	// Create upload operation
	op := &SyncOperation{
		ID:         "test-upload",
		Type:       ChangeCreate,
		Direction:  LocalToRemote,
		SourcePath: testFile,
		TargetPath: "/remote/test.txt",
	}

	// Execute operation
	err = executor.ExecuteOperation(op)
	assert.NoError(t, err)

	// Verify file was uploaded
	assert.Contains(t, mockClient.files, "/remote/test.txt")
	assert.Equal(t, content, mockClient.files["/remote/test.txt"].content)

	// Verify progress tracking
	assert.GreaterOrEqual(t, progressTracker.startCount, 1)
	assert.GreaterOrEqual(t, progressTracker.finishCount, 1)
	assert.Contains(t, progressTracker.operations, "CREATE "+testFile)
}

func TestExecuteOperation_Download(t *testing.T) {
	// Setup mock remote file
	content := []byte("test content for download")
	mockClient := newMockWebDAVClient()
	mockClient.files["/remote/test.txt"] = &mockFile{
		content: content,
		modTime: time.Now(),
		isDir:   false,
	}

	// Create temporary local directory
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "test.txt")

	// Setup executor
	progressTracker := &mockProgressTracker{}
	config := &SyncConfig{
		ProgressTracker: progressTracker,
	}
	executor := NewOperationExecutor(mockClient, config)

	// Create download operation
	op := &SyncOperation{
		ID:         "test-download",
		Type:       ChangeCreate,
		Direction:  RemoteToLocal,
		SourcePath: "/remote/test.txt",
		TargetPath: localFile,
	}

	// Execute operation
	err := executor.ExecuteOperation(op)
	assert.NoError(t, err)

	// Verify file was downloaded
	downloadedContent, err := os.ReadFile(localFile)
	require.NoError(t, err)
	assert.Equal(t, content, downloadedContent)

	// Verify progress tracking
	assert.GreaterOrEqual(t, progressTracker.startCount, 1)
	assert.GreaterOrEqual(t, progressTracker.finishCount, 1)
	assert.Contains(t, progressTracker.operations, "CREATE /remote/test.txt")
}

func TestExecuteOperation_Delete(t *testing.T) {
	// Setup mock remote file
	mockClient := newMockWebDAVClient()
	mockClient.files["/remote/test.txt"] = &mockFile{
		content: []byte("test content"),
		modTime: time.Now(),
		isDir:   false,
	}

	// Create temporary local file
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(localFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Setup executor
	config := &SyncConfig{}
	executor := NewOperationExecutor(mockClient, config)

	// Test local deletion
	localDeleteOp := &SyncOperation{
		ID:         "test-delete-local",
		Type:       ChangeDelete,
		Direction:  LocalToRemote,
		SourcePath: localFile,
		TargetPath: "/remote/test.txt",
	}

	err = executor.ExecuteOperation(localDeleteOp)
	assert.NoError(t, err)
	_, err = os.Stat(localFile)
	assert.True(t, os.IsNotExist(err))

	// Test remote deletion
	remoteDeleteOp := &SyncOperation{
		ID:         "test-delete-remote",
		Type:       ChangeDelete,
		Direction:  RemoteToLocal,
		SourcePath: localFile,
		TargetPath: "/remote/test.txt",
	}

	err = executor.ExecuteOperation(remoteDeleteOp)
	assert.NoError(t, err)
	assert.NotContains(t, mockClient.files, "/remote/test.txt")
}

func TestExecuteOperation_Move(t *testing.T) {
	// Setup mock remote file
	mockClient := newMockWebDAVClient()
	mockClient.files["/remote/source.txt"] = &mockFile{
		content: []byte("test content"),
		modTime: time.Now(),
		isDir:   false,
	}

	// Create temporary local file
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	destFile := filepath.Join(tmpDir, "dest.txt")
	err := os.WriteFile(sourceFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Setup executor
	config := &SyncConfig{}
	executor := NewOperationExecutor(mockClient, config)

	// Test local move
	localMoveOp := &SyncOperation{
		ID:         "test-move-local",
		Type:       ChangeMove,
		Direction:  LocalToRemote,
		SourcePath: sourceFile,
		TargetPath: destFile,
	}

	err = executor.ExecuteOperation(localMoveOp)
	assert.NoError(t, err)
	_, err = os.Stat(sourceFile)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(destFile)
	assert.NoError(t, err)

	// Test remote move
	remoteMoveOp := &SyncOperation{
		ID:         "test-move-remote",
		Type:       ChangeMove,
		Direction:  RemoteToLocal,
		SourcePath: "/remote/source.txt",
		TargetPath: "/remote/dest.txt",
	}

	err = executor.ExecuteOperation(remoteMoveOp)
	assert.NoError(t, err)
	assert.NotContains(t, mockClient.files, "/remote/source.txt")
	assert.Contains(t, mockClient.files, "/remote/dest.txt")
}

func TestEnsureRemoteDirectory(t *testing.T) {
	mockClient := newMockWebDAVClient()
	config := &SyncConfig{}
	executor := NewOperationExecutor(mockClient, config)

	// Test nested directory creation
	err := executor.ensureRemoteDirectory("/a/b/c/d")
	assert.NoError(t, err)

	// Verify all directories were created
	assert.True(t, mockClient.directories["/a"])
	assert.True(t, mockClient.directories["/a/b"])
	assert.True(t, mockClient.directories["/a/b/c"])
	assert.True(t, mockClient.directories["/a/b/c/d"])
}

func TestPlanOperations(t *testing.T) {
	mockClient := newMockWebDAVClient()
	config := &SyncConfig{}
	executor := NewOperationExecutor(mockClient, config)

	// Create test changes
	changes := []*Change{
		{
			Type:       ChangeCreate,
			Direction:  LocalToRemote,
			LocalPath:  "/local/file1.txt",
			RemotePath: "/remote/file1.txt",
			LocalMeta: &FileMetadata{
				Path:     "/local/file1.txt",
				Size:     100,
				Modified: time.Now(),
			},
			Priority: 5,
			Reason:   "New file",
		},
		{
			Type:       ChangeUpdate,
			Direction:  RemoteToLocal,
			LocalPath:  "/local/file2.txt",
			RemotePath: "/remote/file2.txt",
			RemoteMeta: &FileMetadata{
				Path:     "/remote/file2.txt",
				Size:     200,
				Modified: time.Now(),
			},
			Priority: 3,
			Reason:   "File updated",
		},
		{
			Type:       ChangeUpdate,
			Direction:  LocalToRemote,
			LocalPath:  "/local/file3.txt",
			RemotePath: "/remote/file3.txt",
			LocalMeta: &FileMetadata{
				Path:     "/local/file3.txt",
				Size:     150,
				Modified: time.Now(),
			},
			RemoteMeta: &FileMetadata{
				Path:     "/remote/file3.txt",
				Size:     140,
				Modified: time.Now().Add(-time.Hour),
			},
			Priority: 4,
			Reason:   "Conflict detected",
		},
	}

	// Create plan
	plan, err := executor.PlanOperations(changes)
	require.NoError(t, err)

	// Verify plan
	assert.Equal(t, 3, len(plan.Operations)) // 2 file ops + 1 auto directory creation
	assert.Equal(t, 1, len(plan.Conflicts))
	assert.Equal(t, int64(300), plan.TotalSize) // 100 + 200 (directory ops have size 0)
	assert.Equal(t, 2, plan.TotalFiles)         // Only 2 non-conflicting files

	// Verify conflict
	conflict := plan.Conflicts[0]
	assert.Equal(t, ConflictContentChanged, conflict.Type)
	assert.Equal(t, "/local/file3.txt", conflict.LocalPath)
	assert.Equal(t, "/remote/file3.txt", conflict.RemotePath)
}

func TestExecutePlan(t *testing.T) {
	// Setup temporary directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	// Setup mock client and executor
	mockClient := newMockWebDAVClient()
	progressTracker := &mockProgressTracker{}
	config := &SyncConfig{
		ProgressTracker: progressTracker,
	}
	executor := NewOperationExecutor(mockClient, config)

	// Create plan with upload operation
	plan := &SyncPlan{
		Operations: []*SyncOperation{
			{
				ID:         "upload-test",
				Type:       ChangeCreate,
				Direction:  LocalToRemote,
				SourcePath: testFile,
				TargetPath: "/remote/test.txt",
				Size:       int64(len(content)),
				Priority:   5,
			},
		},
		TotalFiles: 1,
		TotalSize:  int64(len(content)),
		CreatedAt:  time.Now(),
	}

	// Execute plan
	result, err := executor.ExecutePlan(plan)
	require.NoError(t, err)

	// Verify result
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.TotalFiles)
	assert.Equal(t, 1, result.ProcessedFiles)
	assert.Equal(t, int64(len(content)), result.TotalSize)
	assert.Equal(t, int64(len(content)), result.TransferredSize)
	assert.Equal(t, 1, len(result.CreatedFiles))
	assert.Contains(t, result.CreatedFiles, testFile)
	assert.Empty(t, result.Errors)
	assert.GreaterOrEqual(t, result.Duration, time.Duration(0))

	// Verify file was uploaded
	assert.Contains(t, mockClient.files, "/remote/test.txt")
	assert.Equal(t, content, mockClient.files["/remote/test.txt"].content)
}

func TestProgressReader(t *testing.T) {
	content := []byte("test content for progress reader")
	tracker := &mockProgressTracker{}

	reader := &progressReader{
		reader:    &mockReader{content: content},
		tracker:   tracker,
		totalSize: int64(len(content)),
	}

	// Read all content
	result, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, content, result)

	// Verify progress was tracked
	assert.Greater(t, tracker.updateCount, 0)
}

func TestProgressWriter(t *testing.T) {
	content := []byte("test content for progress writer")
	tracker := &mockProgressTracker{}

	var writtenData []byte
	writer := &progressWriter{
		writer:    &mockDataWriter{data: &writtenData},
		tracker:   tracker,
		totalSize: int64(len(content)),
	}

	// Write all content
	n, err := writer.Write(content)
	require.NoError(t, err)
	assert.Equal(t, len(content), n)

	// Verify data was written
	assert.Equal(t, content, writtenData)

	// Verify progress was tracked
	assert.Greater(t, tracker.updateCount, 0)
}

// mockDataWriter captures written data for testing
type mockDataWriter struct {
	data *[]byte
}

func (w *mockDataWriter) Write(p []byte) (n int, err error) {
	*w.data = append(*w.data, p...)
	return len(p), nil
}
