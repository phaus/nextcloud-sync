package webdav

import (
	"fmt"
	"strings"
	"time"
)

// WebDAV Property namespaces and XML constants
const (
	WebDAVNamespace = "DAV:"
	XMLNSDav        = "xmlns:d=\"DAV:\""
)

// Depth values for PROPFIND requests
const (
	DepthZero     = "0"        // Only the requested resource
	DepthOne      = "1"        // The resource and its immediate children
	DepthInfinity = "infinity" // The resource and all its descendants
)

// PROPFIND request templates
const (
	// AllPropertiesRequest requests all properties using allprop
	AllPropertiesRequest = `<?xml version="1.0" encoding="utf-8" ?>
<d:propfind xmlns:d="DAV:">
  <d:allprop/>
</d:propfind>`
)

// PropertyRequest represents a WebDAV property request configuration
type PropertyRequest struct {
	Properties []string // Properties to request
	Depth      string   // Depth header value
	AllProp    bool     // Whether to use allprop
}

// NewPropertyRequest creates a new property request with standard properties
func NewPropertyRequest() *PropertyRequest {
	return &PropertyRequest{
		Properties: GetBasicProperties(),
		Depth:      DepthOne,
		AllProp:    false,
	}
}

// NewMinimalPropertyRequest creates a property request with minimal properties
func NewMinimalPropertyRequest() *PropertyRequest {
	return &PropertyRequest{
		Properties: []string{
			PropLastModified,
			PropETag,
			PropResourceType,
		},
		Depth:   DepthZero,
		AllProp: false,
	}
}

// NewAllPropertiesRequest creates a request for all properties
func NewAllPropertiesRequest() *PropertyRequest {
	return &PropertyRequest{
		Properties: []string{},
		Depth:      DepthZero,
		AllProp:    true,
	}
}

// BuildPROPFINDBody constructs the XML body for a PROPFIND request
func (pr *PropertyRequest) BuildPROPFINDBody() string {
	if pr.AllProp {
		return AllPropertiesRequest
	}

	if len(pr.Properties) == 0 {
		// Use basic properties as default
		pr.Properties = GetBasicProperties()
	}

	// Build custom property request
	var propBuilder strings.Builder
	propBuilder.WriteString(`<?xml version="1.0" encoding="utf-8" ?>
<d:propfind xmlns:d="DAV:">
  <d:prop>`)

	for _, prop := range pr.Properties {
		propBuilder.WriteString("\n    <")
		propBuilder.WriteString(strings.TrimPrefix(prop, "d:"))
		propBuilder.WriteString("/>")
	}

	propBuilder.WriteString(`
  </d:prop>
</d:propfind>`)

	return propBuilder.String()
}

// SetDepth sets the depth for the property request
func (pr *PropertyRequest) SetDepth(depth string) error {
	switch depth {
	case DepthZero, DepthOne, DepthInfinity:
		pr.Depth = depth
		return nil
	default:
		return fmt.Errorf("invalid depth value: %s, must be 0, 1, or infinity", depth)
	}
}

// AddProperty adds a property to the request
func (pr *PropertyRequest) AddProperty(property string) {
	if !pr.AllProp {
		pr.Properties = append(pr.Properties, property)
	}
}

// RemoveProperty removes a property from the request
func (pr *PropertyRequest) RemoveProperty(property string) {
	if !pr.AllProp {
		for i, prop := range pr.Properties {
			if prop == property {
				pr.Properties = append(pr.Properties[:i], pr.Properties[i+1:]...)
				break
			}
		}
	}
}

// PropertyValidator validates WebDAV property values
type PropertyValidator struct{}

// NewPropertyValidator creates a new property validator
func NewPropertyValidator() *PropertyValidator {
	return &PropertyValidator{}
}

// ValidateETag validates that an ETag is properly formatted
func (pv *PropertyValidator) ValidateETag(etag string) error {
	if etag == "" {
		return nil // Empty ETag is valid
	}

	// ETags should be enclosed in quotes
	if !strings.HasPrefix(etag, "\"") || !strings.HasSuffix(etag, "\"") {
		return fmt.Errorf("invalid ETag format: %s", etag)
	}

	return nil
}

// ValidateContentLength validates content length
func (pv *PropertyValidator) ValidateContentLength(size int64) error {
	if size < 0 {
		return fmt.Errorf("invalid content length: %d, must be non-negative", size)
	}
	return nil
}

// ValidateLastModified validates last modified timestamp
func (pv *PropertyValidator) ValidateLastModified(modTime time.Time) error {
	// Check if the time is reasonable (not zero, not too far in the future)
	if modTime.IsZero() {
		return fmt.Errorf("last modified time cannot be zero")
	}

	// Check if time is more than 1 hour in the future (allow some clock skew)
	if modTime.After(time.Now().Add(time.Hour)) {
		return fmt.Errorf("last modified time is too far in the future: %s", modTime)
	}

	// Check if time is before 1990 (unlikely for valid files)
	if modTime.Before(time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return fmt.Errorf("last modified time is too far in the past: %s", modTime)
	}

	return nil
}

// ValidateWebDAVFile validates a WebDAVFile structure
func (pv *PropertyValidator) ValidateWebDAVFile(file *WebDAVFile) error {
	if file == nil {
		return fmt.Errorf("WebDAVFile cannot be nil")
	}

	if file.Name == "" {
		return fmt.Errorf("file name cannot be empty")
	}

	// Validate ETag if present
	if err := pv.ValidateETag(file.ETag); err != nil {
		return fmt.Errorf("invalid ETag: %w", err)
	}

	// Validate content length
	if err := pv.ValidateContentLength(file.Size); err != nil {
		return fmt.Errorf("invalid content length: %w", err)
	}

	// Validate last modified time
	if err := pv.ValidateLastModified(file.LastModified); err != nil {
		return fmt.Errorf("invalid last modified: %w", err)
	}

	return nil
}

// ValidateWebDAVProperties validates a WebDAVProperties structure
func (pv *PropertyValidator) ValidateWebDAVProperties(props *WebDAVProperties) error {
	if props == nil {
		return fmt.Errorf("WebDAVProperties cannot be nil")
	}

	// Validate ETag if present
	if err := pv.ValidateETag(props.ETag); err != nil {
		return fmt.Errorf("invalid ETag: %w", err)
	}

	// Validate content length
	if err := pv.ValidateContentLength(props.Size); err != nil {
		return fmt.Errorf("invalid content length: %w", err)
	}

	// Validate last modified time
	if err := pv.ValidateLastModified(props.LastModified); err != nil {
		return fmt.Errorf("invalid last modified: %w", err)
	}

	return nil
}

// PropertyHelper provides utility functions for working with WebDAV properties
type PropertyHelper struct{}

// NewPropertyHelper creates a new property helper
func NewPropertyHelper() *PropertyHelper {
	return &PropertyHelper{}
}

// IsCollection checks if a resource type indicates a collection (directory)
func (ph *PropertyHelper) IsCollection(resourceType ResourceType) bool {
	return len(resourceType.Collection) > 0
}

// FormatSize formats a file size in human-readable format
func (ph *PropertyHelper) FormatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

// IsRecent checks if a file has been modified recently (within specified duration)
func (ph *PropertyHelper) IsRecent(modTime time.Time, within time.Duration) bool {
	return time.Since(modTime) <= within
}

// CompareETags compares two ETags for equality
func (ph *PropertyHelper) CompareETags(etag1, etag2 string) bool {
	// Normalize ETags by removing quotes for comparison
	norm1 := strings.Trim(etag1, "\"")
	norm2 := strings.Trim(etag2, "\"")
	return norm1 == norm2
}

// GetStandardPropertyRequest returns a pre-configured request for directory listings
func GetStandardPropertyRequest() *PropertyRequest {
	return &PropertyRequest{
		Properties: GetBasicProperties(),
		Depth:      DepthOne,
		AllProp:    false,
	}
}

// GetFilePropertyRequest returns a pre-configured request for single file properties
func GetFilePropertyRequest() *PropertyRequest {
	return &PropertyRequest{
		Properties: GetBasicProperties(),
		Depth:      DepthZero,
		AllProp:    false,
	}
}

// GetChangeDetectionRequest returns a minimal request for change detection
func GetChangeDetectionRequest() *PropertyRequest {
	return &PropertyRequest{
		Properties: []string{
			PropETag,
			PropLastModified,
		},
		Depth:   DepthOne,
		AllProp: false,
	}
}

// GetMinimalPropertiesForChangeDetection returns properties needed for change detection
func GetMinimalPropertiesForChangeDetection() []string {
	return []string{
		PropETag,
		PropLastModified,
		PropResourceType,
	}
}
