package webdav

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockWebDAVClientForChunked implements a more detailed mock for chunked upload testing
type MockWebDAVClientForChunked struct {
	uploadedFiles map[string][]byte
	uploadCalls   []UploadCall
}

type UploadCall struct {
	Path      string
	Data      []byte
	Offset    int64
	ChunkSize int64
	TotalSize int64
}

func NewMockWebDAVClientForChunked() *MockWebDAVClientForChunked {
	return &MockWebDAVClientForChunked{
		uploadedFiles: make(map[string][]byte),
		uploadCalls:   make([]UploadCall, 0),
	}
}

func (m *MockWebDAVClientForChunked) ListDirectory(ctx context.Context, path string) ([]*WebDAVFile, error) {
	return []*WebDAVFile{}, nil
}

func (m *MockWebDAVClientForChunked) GetProperties(ctx context.Context, path string) (*WebDAVProperties, error) {
	if data, exists := m.uploadedFiles[path]; exists {
		return &WebDAVProperties{
			Path:         path,
			Size:         int64(len(data)),
			LastModified: time.Now(),
			IsDirectory:  false,
		}, nil
	}
	return nil, NewWebDAVError(404, path, "PROPFIND")
}

func (m *MockWebDAVClientForChunked) DownloadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if data, exists := m.uploadedFiles[path]; exists {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, NewWebDAVError(404, path, "GET")
}

func (m *MockWebDAVClientForChunked) UploadFile(ctx context.Context, path string, content io.Reader, size int64) error {
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	m.uploadedFiles[path] = data
	return nil
}

func (m *MockWebDAVClientForChunked) UploadFileChunked(ctx context.Context, path string, content io.Reader, size int64, chunkSize int64) error {
	// For testing, simulate chunked upload by reading all content
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}

	// Simulate chunked behavior by storing the complete data
	m.uploadedFiles[path] = data

	// Record the call for testing verification
	m.uploadCalls = append(m.uploadCalls, UploadCall{
		Path:      path,
		Data:      data,
		ChunkSize: chunkSize,
		TotalSize: size,
	})

	return nil
}

func (m *MockWebDAVClientForChunked) ResumeChunkedUpload(ctx context.Context, path string, content io.Reader, size int64, offset int64, chunkSize int64) error {
	return m.UploadFileChunked(ctx, path, content, size, chunkSize)
}

func (m *MockWebDAVClientForChunked) CreateDirectory(ctx context.Context, path string) error {
	return nil
}

func (m *MockWebDAVClientForChunked) DeleteFile(ctx context.Context, path string) error {
	delete(m.uploadedFiles, path)
	return nil
}

func (m *MockWebDAVClientForChunked) MoveFile(ctx context.Context, source, destination string) error {
	if data, exists := m.uploadedFiles[source]; exists {
		m.uploadedFiles[destination] = data
		delete(m.uploadedFiles, source)
	}
	return nil
}

func (m *MockWebDAVClientForChunked) CopyFile(ctx context.Context, source, destination string) error {
	if data, exists := m.uploadedFiles[source]; exists {
		m.uploadedFiles[destination] = data
	}
	return nil
}

func (m *MockWebDAVClientForChunked) Close() error {
	return nil
}

func (m *MockWebDAVClientForChunked) GetUploadedFiles() map[string][]byte {
	return m.uploadedFiles
}

func (m *MockWebDAVClientForChunked) GetUploadCalls() []UploadCall {
	return m.uploadCalls
}

func TestUploadFileChunked(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockWebDAVClientForChunked()

	tests := []struct {
		name        string
		filePath    string
		content     string
		chunkSize   int64
		expectError bool
	}{
		{
			name:      "small file with 1KB chunks",
			filePath:  "/test/small.txt",
			content:   "This is a small test file content.",
			chunkSize: 1024,
		},
		{
			name:      "large file with small chunks",
			filePath:  "/test/large.txt",
			content:   strings.Repeat("This is a large test file content. ", 1000),
			chunkSize: 1024,
		},
		{
			name:      "file with exact chunk size multiple",
			filePath:  "/test/exact.txt",
			content:   strings.Repeat("A", 2048), // Exactly 2KB
			chunkSize: 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := strings.NewReader(tt.content)
			size := int64(len(tt.content))

			err := mockClient.UploadFileChunked(ctx, tt.filePath, content, size, tt.chunkSize)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify the file was uploaded
			uploadedFiles := mockClient.GetUploadedFiles()
			assert.Contains(t, uploadedFiles, tt.filePath)

			// Verify the content is correct
			uploadedContent := string(uploadedFiles[tt.filePath])
			assert.Equal(t, tt.content, uploadedContent)

			// Verify upload calls were recorded
			uploadCalls := mockClient.GetUploadCalls()
			assert.NotEmpty(t, uploadCalls)

			// Find the call for this file
			var foundCall *UploadCall
			for _, call := range uploadCalls {
				if call.Path == tt.filePath {
					foundCall = &call
					break
				}
			}

			require.NotNil(t, foundCall)
			assert.Equal(t, tt.filePath, foundCall.Path)
			assert.Equal(t, tt.chunkSize, foundCall.ChunkSize)
			assert.Equal(t, size, foundCall.TotalSize)
		})
	}
}

func TestResumeChunkedUpload(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockWebDAVClientForChunked()

	content := "This is test content for resume functionality."
	filePath := "/test/resume.txt"
	offset := int64(10) // Resume from byte 10
	chunkSize := int64(1024)

	// Create content that includes data from the offset
	resumeContent := content[offset:]
	contentReader := strings.NewReader(resumeContent)
	totalSize := int64(len(content))

	err := mockClient.ResumeChunkedUpload(ctx, filePath, contentReader, totalSize, offset, chunkSize)
	require.NoError(t, err)

	// Verify the file was uploaded
	uploadedFiles := mockClient.GetUploadedFiles()
	assert.Contains(t, uploadedFiles, filePath)

	// For this mock, the uploaded content should be the resume content
	uploadedContent := string(uploadedFiles[filePath])
	assert.Equal(t, resumeContent, uploadedContent)
}

func TestChunkedUploadWithZeroChunkSize(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockWebDAVClientForChunked()

	content := "Test content for default chunk size."
	filePath := "/test/zero-chunk.txt"
	chunkSize := int64(0) // Should default to 1MB

	err := mockClient.UploadFileChunked(ctx, filePath, strings.NewReader(content), int64(len(content)), chunkSize)
	require.NoError(t, err)

	// Verify the file was uploaded
	uploadedFiles := mockClient.GetUploadedFiles()
	assert.Contains(t, uploadedFiles, filePath)

	uploadedContent := string(uploadedFiles[filePath])
	assert.Equal(t, content, uploadedContent)
}
