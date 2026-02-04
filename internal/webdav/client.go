package webdav

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/phaus/nextcloud-sync/internal/auth"
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
	auth       auth.AuthProvider
	baseURL    string
	userAgent  string
	httpClient *http.Client
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
		auth:       authProvider,
		baseURL:    webdavURL,
		userAgent:  "nextcloud-sync/1.0",
		httpClient: client,
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

// doRequest executes an HTTP request and handles common errors
func (c *WebDAVClient) doRequest(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	// Check for authentication errors
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, fmt.Errorf("authentication failed: invalid credentials")
	}

	// Check for other HTTP errors
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
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
		return fmt.Errorf("unexpected upload status: %d", resp.StatusCode)
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
		return fmt.Errorf("unexpected directory creation status: %d", resp.StatusCode)
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
		return fmt.Errorf("unexpected delete status: %d", resp.StatusCode)
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
		return fmt.Errorf("unexpected move status: %d", resp.StatusCode)
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
		return fmt.Errorf("unexpected copy status: %d", resp.StatusCode)
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
