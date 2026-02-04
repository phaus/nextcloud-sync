package webdav

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

// Multistatus represents the WebDAV multistatus response
type Multistatus struct {
	XMLName   xml.Name   `xml:"multistatus"`
	Xmlns     string     `xml:"xmlns,attr"`
	Dav       string     `xml:"d,attr"`
	Responses []Response `xml:"response"`
}

// Response represents a single WebDAV response
type Response struct {
	Href     string   `xml:"href"`
	Propstat Propstat `xml:"propstat"`
	Status   string   `xml:"status,omitempty"`
}

// Propstat represents property statistics
type Propstat struct {
	Prop   Prop   `xml:"prop"`
	Status string `xml:"status"`
}

// Prop represents WebDAV properties
type Prop struct {
	DisplayName   string       `xml:"displayname"`
	ContentLength int64        `xml:"getcontentlength"`
	LastModified  string       `xml:"getlastmodified"`
	ETag          string       `xml:"getetag"`
	ContentType   string       `xml:"getcontenttype"`
	ResourceType  ResourceType `xml:"resourcetype"`
}

// ResourceType represents the type of a WebDAV resource
type ResourceType struct {
	Collection []string `xml:"collection,omitempty"`
}

// parseMultistatusResponse parses a WebDAV multistatus XML response
func parseMultistatusResponse(body io.Reader) (*Multistatus, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var multistatus Multistatus
	if err := xml.Unmarshal(data, &multistatus); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	return &multistatus, nil
}

// parseWebDAVFiles converts multistatus response to WebDAVFile slice
func parseWebDAVFiles(multistatus *Multistatus, basePath string) ([]*WebDAVFile, error) {
	var files []*WebDAVFile

	for _, response := range multistatus.Responses {
		// Skip the base directory itself
		if response.Href == basePath || response.Href == basePath+"/" {
			continue
		}

		// Check if the request was successful
		if !strings.Contains(response.Propstat.Status, "200 OK") &&
			!strings.Contains(response.Status, "200 OK") {
			continue
		}

		file := &WebDAVFile{
			Name:        extractFileName(response.Href),
			Path:        response.Propstat.Prop.DisplayName,
			Size:        response.Propstat.Prop.ContentLength,
			ETag:        response.Propstat.Prop.ETag,
			ContentType: response.Propstat.Prop.ContentType,
			IsDirectory: len(response.Propstat.Prop.ResourceType.Collection) > 0,
		}

		// Parse last modified time
		if response.Propstat.Prop.LastModified != "" {
			if lastMod, err := parseWebDAVTime(response.Propstat.Prop.LastModified); err == nil {
				file.LastModified = lastMod
			}
		}

		// Use display name as path if not available
		if file.Path == "" {
			file.Path = file.Name
		}

		files = append(files, file)
	}

	return files, nil
}

// parseWebDAVProperties extracts properties from multistatus response
func parseWebDAVProperties(multistatus *Multistatus) (*WebDAVProperties, error) {
	if len(multistatus.Responses) == 0 {
		return nil, fmt.Errorf("no responses found in multistatus")
	}

	response := multistatus.Responses[0]
	prop := response.Propstat.Prop

	// Check if the request was successful
	if !strings.Contains(response.Propstat.Status, "200 OK") &&
		!strings.Contains(response.Status, "200 OK") {
		return nil, fmt.Errorf("property request failed: %s", response.Propstat.Status)
	}

	properties := &WebDAVProperties{
		Path:        prop.DisplayName,
		Size:        prop.ContentLength,
		ETag:        prop.ETag,
		ContentType: prop.ContentType,
		IsDirectory: len(prop.ResourceType.Collection) > 0,
	}

	// Parse last modified time
	if prop.LastModified != "" {
		if lastMod, err := parseWebDAVTime(prop.LastModified); err == nil {
			properties.LastModified = lastMod
		}
	}

	// Use display name as path if not available
	if properties.Path == "" {
		properties.Path = extractFileName(multistatus.Responses[0].Href)
	}

	return properties, nil
}

// extractFileName extracts the file name from a WebDAV href
func extractFileName(href string) string {
	// Remove any trailing slashes
	href = strings.TrimSuffix(href, "/")

	// Split on slashes and take the last part
	parts := strings.Split(href, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return href
}

// parseWebDAVTime parses a WebDAV timestamp string
func parseWebDAVTime(timeStr string) (time.Time, error) {
	// Try common WebDAV time formats
	formats := []string{
		time.RFC1123,
		"Mon, 2 Jan 2006 15:04:05 GMT",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

// validateMultistatus validates that the multistatus response is well-formed
func validateMultistatus(multistatus *Multistatus) error {
	if multistatus == nil {
		return fmt.Errorf("multistatus response is nil")
	}

	if len(multistatus.Responses) == 0 {
		return fmt.Errorf("no responses in multistatus")
	}

	for i, response := range multistatus.Responses {
		if response.Href == "" {
			return fmt.Errorf("response %d has empty href", i)
		}
	}

	return nil
}
