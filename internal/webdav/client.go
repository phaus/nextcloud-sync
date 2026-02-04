package webdav

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/phaus/nextcloud-sync/internal/auth"
	"github.com/phaus/nextcloud-sync/internal/utils"
)

// WebDAVFile represents a file or directory on the WebDAV server
type WebDAVFile struct {
	Name         string    `xml:"href"`
	Path         string    `xml:"displayname"`
	Size         int64     `xml:"getcontentlength"`
	LastModified time.Time `xml:"getlastmodified"`
	ETag         string    `xml:"getetag"`
	ContentType  string    `xml:"getcontenttype"`
	IsDirectory  bool      `xml:"iscollection"`
}

// WebDAVProperties represents WebDAV properties for a file
type WebDAVProperties struct {
	Path         string    `xml:"displayname"`
	Size         int64     `xml:"getcontentlength"`
	LastModified time.Time `xml:"getlastmodified"`
	ETag         string    `xml:"getetag"`
	ContentType  string    `xml:"getcontenttype"`
	IsDirectory  bool      `xml:"iscollection"`
}

// Client defines the interface for WebDAV operations
type Client interface {
	// ListDirectory lists the contents of a directory
	ListDirectory(ctx context.Context, path string) ([]*WebDAVFile, error)

	// GetProperties retrieves properties for a specific file or directory
	GetProperties(ctx context.Context, path string) (*WebDAVProperties, error)

	// DownloadFile downloads a file from the server
	DownloadFile(ctx context.Context, path string) (io.ReadCloser, error)

	// UploadFile uploads a file to the server
	UploadFile(ctx context.Context, path string, content io.Reader, size int64) error

	// UploadFileChunked uploads a file in chunks for large files
	UploadFileChunked(ctx context.Context, path string, content io.Reader, size int64, chunkSize int64) error

	// ResumeChunkedUpload resumes a chunked upload from a specific offset
	ResumeChunkedUpload(ctx context.Context, path string, content io.Reader, size int64, offset int64, chunkSize int64) error

	// CreateDirectory creates a new directory
	CreateDirectory(ctx context.Context, path string) error

	// DeleteFile deletes a file or directory
	DeleteFile(ctx context.Context, path string) error

	// MoveFile moves a file from source to destination
	MoveFile(ctx context.Context, source, destination string) error

	// CopyFile copies a file from source to destination
	CopyFile(ctx context.Context, source, destination string) error

	// Close cleans up resources
	Close() error
}

// WebDAVClient implements the Client interface
type WebDAVClient struct {
	auth        auth.AuthProvider
	baseURL     string
	userAgent   string
	httpClient  *http.Client
	retryConfig *utils.RetryConfig
}

// SetRetryConfig sets custom retry configuration
func (c *WebDAVClient) SetRetryConfig(config *utils.RetryConfig) {
	c.retryConfig = config
}

// NewClient creates a new WebDAV client
func NewClient(authProvider auth.AuthProvider) (*WebDAVClient, error) {
	if authProvider == nil {
		return nil, fmt.Errorf("auth provider cannot be nil")
	}

	serverURL := authProvider.GetServerURL()
	if serverURL == "" {
		return nil, fmt.Errorf("server URL cannot be empty")
	}

	// Extract WebDAV endpoint from Nextcloud URL
	webdavURL, err := extractWebDAVEndpoint(serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract WebDAV endpoint: %w", err)
	}

	// Create HTTP client with reasonable defaults
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		},
	}

	return &WebDAVClient{
		auth:        authProvider,
		baseURL:     webdavURL,
		userAgent:   "nextcloud-sync/1.0",
		httpClient:  client,
		retryConfig: utils.DefaultRetryConfig(),
	}, nil
}

// extractWebDAVEndpoint converts Nextcloud URL to WebDAV endpoint
func extractWebDAVEndpoint(nextcloudURL string) (string, error) {
	// Handle empty URL
	if nextcloudURL == "" {
		return "", nil
	}

	// Remove trailing slash
	nextcloudURL = strings.TrimSuffix(nextcloudURL, "/")

	// Check if it's already a WebDAV URL
	if strings.Contains(nextcloudURL, "/remote.php/dav/") {
		return nextcloudURL, nil
	}

	// Convert Nextcloud URL to WebDAV URL
	// Format: https://cloud.example.com/remote.php/dav/files/username/
	return fmt.Sprintf("%s/remote.php/dav/files/%s", nextcloudURL, "USERNAME"), nil
}

// buildURL constructs a full URL for a given path
func (c *WebDAVClient) buildURL(relPath string) string {
	// Clean the path
	relPath = path.Clean("/" + relPath)

	// For now, replace USERNAME placeholder - this should be dynamic
	baseURL := strings.Replace(c.baseURL, "USERNAME", c.auth.GetUsername(), 1)

	return baseURL + relPath
}

// createRequest creates an HTTP request with proper authentication
func (c *WebDAVClient) createRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/octet-stream")
	}

	// Add authentication
	authHeader, err := c.auth.GetAuthHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	return req, nil
}

// doRequest executes an HTTP request and handles common errors with retry logic
func (c *WebDAVClient) doRequest(req *http.Request) (*http.Response, error) {
	var resp *http.Response

	// Use retry logic for the request
	err := utils.RetryWithBackoff(req.Context(), c.retryConfig, utils.IsTemporaryWebDAVError, func() error {
		var err error
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return WrapHTTPError(err, req.URL.Path, req.Method)
		}

		// Check for HTTP errors and convert to WebDAV errors
		if resp.StatusCode >= 400 {
			// Don't consume the body on error as it might be needed by caller
			webdavErr := NewWebDAVError(resp.StatusCode, req.URL.Path, req.Method)
			resp.Body.Close()
			return webdavErr
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// doRequestWithoutRetry executes an HTTP request without retry logic (for internal use)
func (c *WebDAVClient) doRequestWithoutRetry(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, WrapHTTPError(err, req.URL.Path, req.Method)
	}

	// Check for HTTP errors and convert to WebDAV errors
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, NewWebDAVError(resp.StatusCode, req.URL.Path, req.Method)
	}

	return resp, nil
}

// ListDirectory implements Client.ListDirectory
func (c *WebDAVClient) ListDirectory(ctx context.Context, dirPath string) ([]*WebDAVFile, error) {
	url := c.buildURL(dirPath)

	// Create property request for directory listing
	propReq := GetStandardPropertyRequest()
	propReq.SetDepth(DepthOne)
	propfindBody := propReq.BuildPROPFINDBody()

	req, err := c.createRequest(ctx, "PROPFIND", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create PROPFIND request: %w", err)
	}

	// Set headers for directory listing
	req.Header.Set("Depth", DepthOne)
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Body = io.NopCloser(strings.NewReader(propfindBody))
	req.ContentLength = int64(len(propfindBody))

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute PROPFIND request: %w", err)
	}
	defer resp.Body.Close()

	// Parse XML response
	multistatus, err := parseMultistatusResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PROPFIND response: %w", err)
	}

	// Validate response
	if err := validateMultistatus(multistatus); err != nil {
		return nil, fmt.Errorf("invalid PROPFIND response: %w", err)
	}

	// Convert to WebDAVFile slice
	files, err := parseWebDAVFiles(multistatus, url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WebDAV files: %w", err)
	}

	return files, nil
}

// GetProperties implements Client.GetProperties
func (c *WebDAVClient) GetProperties(ctx context.Context, filePath string) (*WebDAVProperties, error) {
	url := c.buildURL(filePath)

	// Create property request for single file
	propReq := GetFilePropertyRequest()
	propReq.SetDepth(DepthZero)
	propfindBody := propReq.BuildPROPFINDBody()

	req, err := c.createRequest(ctx, "PROPFIND", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create PROPFIND request: %w", err)
	}

	// Set headers for single file properties
	req.Header.Set("Depth", DepthZero)
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Body = io.NopCloser(strings.NewReader(propfindBody))
	req.ContentLength = int64(len(propfindBody))

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute PROPFIND request: %w", err)
	}
	defer resp.Body.Close()

	// Parse XML response
	multistatus, err := parseMultistatusResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PROPFIND response: %w", err)
	}

	// Validate response
	if err := validateMultistatus(multistatus); err != nil {
		return nil, fmt.Errorf("invalid PROPFIND response: %w", err)
	}

	// Convert to WebDAVProperties
	properties, err := parseWebDAVProperties(multistatus)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WebDAV properties: %w", err)
	}

	return properties, nil
}

// DownloadFile implements Client.DownloadFile
func (c *WebDAVClient) DownloadFile(ctx context.Context, filePath string) (io.ReadCloser, error) {
	url := c.buildURL(filePath)

	req, err := c.createRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GET request: %w", err)
	}

	return resp.Body, nil
}

// UploadFile implements Client.UploadFile
func (c *WebDAVClient) UploadFile(ctx context.Context, filePath string, content io.Reader, size int64) error {
	url := c.buildURL(filePath)

	req, err := c.createRequest(ctx, "PUT", url, content)
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %w", err)
	}

	// Set Content-Length if known
	if size > 0 {
		req.ContentLength = size
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to execute PUT request: %w", err)
	}
	defer resp.Body.Close()

	// Check for successful upload
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return NewWebDAVError(resp.StatusCode, filePath, "PUT")
	}

	return nil
}

// UploadFileChunked implements Client.UploadFileChunked
func (c *WebDAVClient) UploadFileChunked(ctx context.Context, filePath string, content io.Reader, size int64, chunkSize int64) error {
	// Validate inputs
	if chunkSize <= 0 {
		chunkSize = 1024 * 1024 // Default to 1MB
	}

	// For small files, use regular upload
	if size <= chunkSize {
		return c.UploadFile(ctx, filePath, content, size)
	}

	// Create a buffered reader for chunking
	buffer := make([]byte, chunkSize)
	var offset int64 = 0

	for offset < size {
		// Read a chunk
		bytesRead, err := io.ReadFull(content, buffer)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("failed to read chunk at offset %d: %w", offset, err)
		}

		// Handle last chunk which might be smaller
		chunkData := buffer[:bytesRead]
		if bytesRead == 0 {
			break
		}

		// Upload this chunk
		err = c.uploadChunk(ctx, filePath, chunkData, offset, size)
		if err != nil {
			return fmt.Errorf("failed to upload chunk at offset %d: %w", offset, err)
		}

		offset += int64(bytesRead)

		// If we read less than requested, we're done
		if int64(bytesRead) < chunkSize {
			break
		}
	}

	return nil
}

// ResumeChunkedUpload implements Client.ResumeChunkedUpload
func (c *WebDAVClient) ResumeChunkedUpload(ctx context.Context, filePath string, content io.Reader, size int64, offset int64, chunkSize int64) error {
	// Validate inputs
	if chunkSize <= 0 {
		chunkSize = 1024 * 1024 // Default to 1MB
	}

	// Seek to the resume position if the content supports seeking
	if seeker, ok := content.(io.Seeker); ok {
		_, err := seeker.Seek(offset, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek to offset %d: %w", offset, err)
		}
	} else if offset > 0 {
		// If we can't seek, we need to read and discard bytes to get to the offset
		discarded := int64(0)
		buffer := make([]byte, 4096)
		for discarded < offset {
			toRead := int64(len(buffer))
			if discarded+toRead > offset {
				toRead = offset - discarded
			}

			bytesRead, err := io.ReadFull(content, buffer[:toRead])
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				return fmt.Errorf("failed to skip to offset %d: %w", offset, err)
			}

			discarded += int64(bytesRead)
			if int64(bytesRead) < toRead {
				break // EOF reached
			}
		}
	}

	// Continue upload from the offset
	buffer := make([]byte, chunkSize)
	currentOffset := offset

	for currentOffset < size {
		// Read a chunk
		bytesRead, err := io.ReadFull(content, buffer)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("failed to read chunk at offset %d: %w", currentOffset, err)
		}

		// Handle last chunk which might be smaller
		chunkData := buffer[:bytesRead]
		if bytesRead == 0 {
			break
		}

		// Upload this chunk
		err = c.uploadChunk(ctx, filePath, chunkData, currentOffset, size)
		if err != nil {
			return fmt.Errorf("failed to upload chunk at offset %d: %w", currentOffset, err)
		}

		currentOffset += int64(bytesRead)

		// If we read less than requested, we're done
		if int64(bytesRead) < chunkSize {
			break
		}
	}

	return nil
}

// uploadChunk uploads a single chunk using Content-Range header
func (c *WebDAVClient) uploadChunk(ctx context.Context, filePath string, chunkData []byte, offset, totalSize int64) error {
	url := c.buildURL(filePath)

	// Create a reader for the chunk data
	chunkReader := bytes.NewReader(chunkData)

	// Create the request
	req, err := c.createRequest(ctx, "PUT", url, chunkReader)
	if err != nil {
		return fmt.Errorf("failed to create PUT request for chunk: %w", err)
	}

	// Set Content-Length for this chunk
	req.ContentLength = int64(len(chunkData))

	// Set Content-Range header for chunked upload
	endRange := offset + int64(len(chunkData)) - 1
	req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, endRange, totalSize))

	// For the first chunk, don't send Content-Range to create the file
	if offset == 0 {
		req.Header.Del("Content-Range")
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to execute chunk PUT request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	// For chunked uploads, we accept 200 (OK) or 201 (Created) or 206 (Partial Content)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusPartialContent {
		return NewWebDAVError(resp.StatusCode, filePath, "PUT (chunk)")
	}

	return nil
}

// CreateDirectory implements Client.CreateDirectory
func (c *WebDAVClient) CreateDirectory(ctx context.Context, dirPath string) error {
	url := c.buildURL(dirPath)

	req, err := c.createRequest(ctx, "MKCOL", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create MKCOL request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to execute MKCOL request: %w", err)
	}
	defer resp.Body.Close()

	// Check for successful creation
	if resp.StatusCode != http.StatusCreated {
		return NewWebDAVError(resp.StatusCode, dirPath, "MKCOL")
	}

	return nil
}

// DeleteFile implements Client.DeleteFile
func (c *WebDAVClient) DeleteFile(ctx context.Context, filePath string) error {
	url := c.buildURL(filePath)

	req, err := c.createRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to execute DELETE request: %w", err)
	}
	defer resp.Body.Close()

	// Check for successful deletion
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return NewWebDAVError(resp.StatusCode, filePath, "DELETE")
	}

	return nil
}

// MoveFile implements Client.MoveFile
func (c *WebDAVClient) MoveFile(ctx context.Context, source, destination string) error {
	sourceURL := c.buildURL(source)
	destURL := c.buildURL(destination)

	req, err := c.createRequest(ctx, "MOVE", sourceURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create MOVE request: %w", err)
	}

	// Set Destination header for MOVE
	req.Header.Set("Destination", destURL)
	req.Header.Set("Overwrite", "T") // Allow overwrite

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to execute MOVE request: %w", err)
	}
	defer resp.Body.Close()

	// Check for successful move
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return NewWebDAVError(resp.StatusCode, source, "MOVE")
	}

	return nil
}

// CopyFile implements Client.CopyFile
func (c *WebDAVClient) CopyFile(ctx context.Context, source, destination string) error {
	sourceURL := c.buildURL(source)
	destURL := c.buildURL(destination)

	req, err := c.createRequest(ctx, "COPY", sourceURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create COPY request: %w", err)
	}

	// Set Destination header for COPY
	req.Header.Set("Destination", destURL)
	req.Header.Set("Overwrite", "T") // Allow overwrite

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to execute COPY request: %w", err)
	}
	defer resp.Body.Close()

	// Check for successful copy
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return NewWebDAVError(resp.StatusCode, source, "COPY")
	}

	return nil
}

// Close implements Client.Close
func (c *WebDAVClient) Close() error {
	// Close HTTP client transport if it has a CloseIdleConnections method
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}

	// Close auth provider if it has a Close method
	if closer, ok := c.auth.(interface{ Close() }); ok {
		closer.Close()
	}

	return nil
}
