package webdav

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// RequestBuilder provides helper methods for creating WebDAV HTTP requests
type RequestBuilder struct {
	baseURL   string
	userAgent string
}

// NewRequestBuilder creates a new request builder
func NewRequestBuilder(baseURL, userAgent string) *RequestBuilder {
	return &RequestBuilder{
		baseURL:   baseURL,
		userAgent: userAgent,
	}
}

// buildBaseURL constructs the full URL for a given path
func (rb *RequestBuilder) buildBaseURL(relPath string) string {
	// Clean the path and ensure it starts with /
	if !strings.HasPrefix(relPath, "/") {
		relPath = "/" + relPath
	}
	return rb.baseURL + relPath
}

// addCommonHeaders adds common headers to all requests
func (rb *RequestBuilder) addCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", rb.userAgent)
	req.Header.Set("Accept", "*/*")
}

// addAuthHeaders adds authentication headers to the request
func (rb *RequestBuilder) addAuthHeaders(req *http.Request, authHeader string) {
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
}

// CreatePROPFINDRequest creates a PROPFIND request for directory listings or properties
func (rb *RequestBuilder) CreatePROPFINDRequest(ctx context.Context, path string, depth string, properties []string, authHeader string) (*http.Request, error) {
	url := rb.buildBaseURL(path)

	body := rb.buildPROPFINDBody(properties)
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", url, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create PROPFIND request: %w", err)
	}

	// Set headers
	rb.addCommonHeaders(req)
	rb.addAuthHeaders(req, authHeader)
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Depth", depth)
	req.ContentLength = int64(len(body))

	return req, nil
}

// CreateGETRequest creates a GET request for file downloads
func (rb *RequestBuilder) CreateGETRequest(ctx context.Context, path string, authHeader string) (*http.Request, error) {
	url := rb.buildBaseURL(path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}

	rb.addCommonHeaders(req)
	rb.addAuthHeaders(req, authHeader)
	req.Header.Set("Accept", "*/*")

	return req, nil
}

// CreatePUTRequest creates a PUT request for file uploads
func (rb *RequestBuilder) CreatePUTRequest(ctx context.Context, path string, content io.Reader, size int64, authHeader string) (*http.Request, error) {
	url := rb.buildBaseURL(path)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, content)
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}

	rb.addCommonHeaders(req)
	rb.addAuthHeaders(req, authHeader)
	req.Header.Set("Content-Type", "application/octet-stream")

	// Set Content-Length if known
	if size > 0 {
		req.ContentLength = size
	}

	return req, nil
}

// CreateMKCOLRequest creates a MKCOL request for directory creation
func (rb *RequestBuilder) CreateMKCOLRequest(ctx context.Context, path string, authHeader string) (*http.Request, error) {
	url := rb.buildBaseURL(path)

	req, err := http.NewRequestWithContext(ctx, "MKCOL", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create MKCOL request: %w", err)
	}

	rb.addCommonHeaders(req)
	rb.addAuthHeaders(req, authHeader)
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")

	return req, nil
}

// CreateDELETERequest creates a DELETE request for file/directory removal
func (rb *RequestBuilder) CreateDELETERequest(ctx context.Context, path string, authHeader string) (*http.Request, error) {
	url := rb.buildBaseURL(path)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create DELETE request: %w", err)
	}

	rb.addCommonHeaders(req)
	rb.addAuthHeaders(req, authHeader)

	return req, nil
}

// CreateMOVERequest creates a MOVE request for file/directory relocation
func (rb *RequestBuilder) CreateMOVERequest(ctx context.Context, sourcePath, destPath string, overwrite bool, authHeader string) (*http.Request, error) {
	sourceURL := rb.buildBaseURL(sourcePath)
	destURL := rb.buildBaseURL(destPath)

	req, err := http.NewRequestWithContext(ctx, "MOVE", sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create MOVE request: %w", err)
	}

	rb.addCommonHeaders(req)
	rb.addAuthHeaders(req, authHeader)
	req.Header.Set("Destination", destURL)

	if overwrite {
		req.Header.Set("Overwrite", "T")
	} else {
		req.Header.Set("Overwrite", "F")
	}

	return req, nil
}

// CreateCOPYRequest creates a COPY request for file/directory duplication
func (rb *RequestBuilder) CreateCOPYRequest(ctx context.Context, sourcePath, destPath string, overwrite bool, authHeader string) (*http.Request, error) {
	sourceURL := rb.buildBaseURL(sourcePath)
	destURL := rb.buildBaseURL(destPath)

	req, err := http.NewRequestWithContext(ctx, "COPY", sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create COPY request: %w", err)
	}

	rb.addCommonHeaders(req)
	rb.addAuthHeaders(req, authHeader)
	req.Header.Set("Destination", destURL)

	if overwrite {
		req.Header.Set("Overwrite", "T")
	} else {
		req.Header.Set("Overwrite", "F")
	}

	return req, nil
}

// buildPROPFINDBody creates the XML body for PROPFIND requests
func (rb *RequestBuilder) buildPROPFINDBody(properties []string) string {
	if len(properties) == 0 {
		// Default properties if none specified
		properties = []string{
			"d:displayname",
			"d:getcontentlength",
			"d:getlastmodified",
			"d:getetag",
			"d:getcontenttype",
			"d:resourcetype",
		}
	}

	var propBuilder strings.Builder
	propBuilder.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\" ?>\n")
	propBuilder.WriteString("<d:propfind xmlns:d=\"DAV:\">\n")
	propBuilder.WriteString("  <d:prop>\n")

	for _, prop := range properties {
		propBuilder.WriteString(fmt.Sprintf("    <%s/>\n", prop))
	}

	propBuilder.WriteString("  </d:prop>\n")
	propBuilder.WriteString("</d:propfind>")

	return propBuilder.String()
}

// Common property constants for PROPFIND requests
const (
	PropDisplayName    = "d:displayname"
	PropContentLength  = "d:getcontentlength"
	PropLastModified   = "d:getlastmodified"
	PropETag           = "d:getetag"
	PropContentType    = "d:getcontenttype"
	PropResourceType   = "d:resourcetype"
	PropCreationDate   = "d:creationdate"
	PropGetContentLang = "d:getcontentlanguage"
)

// GetAllProperties returns a slice of all common WebDAV properties
func GetAllProperties() []string {
	return []string{
		PropDisplayName,
		PropContentLength,
		PropLastModified,
		PropETag,
		PropContentType,
		PropResourceType,
		PropCreationDate,
		PropGetContentLang,
	}
}

// GetBasicProperties returns a slice of essential WebDAV properties
func GetBasicProperties() []string {
	return []string{
		PropDisplayName,
		PropContentLength,
		PropLastModified,
		PropETag,
		PropResourceType,
	}
}

// RequestConfig holds configuration for request creation
type RequestConfig struct {
	Timeout           int
	UserAgent         string
	MaxRetries        int
	RetryDelay        int
	ChunkSize         int64
	EnableCompression bool
}

// DefaultRequestConfig returns a default configuration for requests
func DefaultRequestConfig() *RequestConfig {
	return &RequestConfig{
		Timeout:           30,
		UserAgent:         "nextcloud-sync/1.0",
		MaxRetries:        3,
		RetryDelay:        5,
		ChunkSize:         1024 * 1024, // 1MB
		EnableCompression: true,
	}
}
