package webdav

import (
	"strings"
	"testing"
)

func TestParseMultistatusResponse(t *testing.T) {
	// Sample WebDAV PROPFIND response
	xmlResponse := `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:">
    <d:response>
        <d:href>/remote.php/dav/files/user/documents/</d:href>
        <d:propstat>
            <d:prop>
                <d:displayname>documents</d:displayname>
                <d:getcontentlength>0</d:getcontentlength>
                <d:getlastmodified>Mon, 04 Feb 2026 10:00:00 GMT</d:getlastmodified>
                <d:getetag>&quot;abc123&quot;</d:getetag>
                <d:getcontenttype></d:getcontenttype>
                <d:resourcetype><d:collection/></d:resourcetype>
            </d:prop>
            <d:status>HTTP/1.1 200 OK</d:status>
        </d:propstat>
    </d:response>
    <d:response>
        <d:href>/remote.php/dav/files/user/documents/test.txt</d:href>
        <d:propstat>
            <d:prop>
                <d:displayname>test.txt</d:displayname>
                <d:getcontentlength>1024</d:getcontentlength>
                <d:getlastmodified>Mon, 04 Feb 2026 09:30:00 GMT</d:getlastmodified>
                <d:getetag>&quot;def456&quot;</d:getetag>
                <d:getcontenttype>text/plain</d:getcontenttype>
                <d:resourcetype></d:resourcetype>
            </d:prop>
            <d:status>HTTP/1.1 200 OK</d:status>
        </d:propstat>
    </d:response>
</d:multistatus>`

	reader := strings.NewReader(xmlResponse)
	multistatus, err := parseMultistatusResponse(reader)
	if err != nil {
		t.Fatalf("Failed to parse multistatus response: %v", err)
	}

	if len(multistatus.Responses) != 2 {
		t.Errorf("Expected 2 responses, got %d", len(multistatus.Responses))
	}

	// Test first response (directory)
	resp1 := multistatus.Responses[0]
	if !strings.Contains(resp1.Href, "documents/") {
		t.Errorf("Expected documents directory href, got %s", resp1.Href)
	}
	if resp1.Propstat.Prop.DisplayName != "documents" {
		t.Errorf("Expected displayname 'documents', got %s", resp1.Propstat.Prop.DisplayName)
	}

	if len(resp1.Propstat.Prop.ResourceType.Collection) == 0 {
		t.Error("Expected directory to have collection resource type")
	}

	// Test second response (file)
	resp2 := multistatus.Responses[1]
	if !strings.Contains(resp2.Href, "test.txt") {
		t.Errorf("Expected test.txt href, got %s", resp2.Href)
	}
	if resp2.Propstat.Prop.DisplayName != "test.txt" {
		t.Errorf("Expected displayname 'test.txt', got %s", resp2.Propstat.Prop.DisplayName)
	}
	if resp2.Propstat.Prop.ContentLength != 1024 {
		t.Errorf("Expected content length 1024, got %d", resp2.Propstat.Prop.ContentLength)
	}
	if len(resp2.Propstat.Prop.ResourceType.Collection) > 0 {
		t.Error("Expected file to not have collection resource type")
	}
}

func TestParseWebDAVFiles(t *testing.T) {
	multistatus := &Multistatus{
		Responses: []Response{
			{
				Href: "/remote.php/dav/files/user/documents/",
				Propstat: Propstat{
					Prop: Prop{
						DisplayName:  "documents",
						ResourceType: ResourceType{Collection: []string{""}},
					},
					Status: "HTTP/1.1 200 OK",
				},
			},
			{
				Href: "/remote.php/dav/files/user/documents/test.txt",
				Propstat: Propstat{
					Prop: Prop{
						DisplayName:   "test.txt",
						ContentLength: 1024,
						LastModified:  "Mon, 04 Feb 2026 09:30:00 GMT",
						ETag:          "\"def456\"",
						ContentType:   "text/plain",
					},
					Status: "HTTP/1.1 200 OK",
				},
			},
		},
	}

	files, err := parseWebDAVFiles(multistatus, "/remote.php/dav/files/user/documents/")
	if err != nil {
		t.Fatalf("Failed to parse WebDAV files: %v", err)
	}

	// Should skip the base directory and return only the file
	if len(files) != 1 {
		t.Errorf("Expected 1 file (skipping base directory), got %d", len(files))
	}

	file := files[0]
	if file.Name != "test.txt" {
		t.Errorf("Expected file name 'test.txt', got %s", file.Name)
	}
	if file.Size != 1024 {
		t.Errorf("Expected file size 1024, got %d", file.Size)
	}
	if file.ETag != "\"def456\"" {
		t.Errorf("Expected ETag '\"def456\"', got %s", file.ETag)
	}
	if file.IsDirectory {
		t.Error("Expected file to not be a directory")
	}
}

func TestParseWebDAVProperties(t *testing.T) {
	multistatus := &Multistatus{
		Responses: []Response{
			{
				Href: "/remote.php/dav/files/user/documents/test.txt",
				Propstat: Propstat{
					Prop: Prop{
						DisplayName:   "test.txt",
						ContentLength: 2048,
						LastModified:  "Mon, 04 Feb 2026 10:30:00 GMT",
						ETag:          "\"xyz789\"",
						ContentType:   "application/json",
					},
					Status: "HTTP/1.1 200 OK",
				},
			},
		},
	}

	properties, err := parseWebDAVProperties(multistatus)
	if err != nil {
		t.Fatalf("Failed to parse WebDAV properties: %v", err)
	}

	if properties.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got %s", properties.Path)
	}
	if properties.Size != 2048 {
		t.Errorf("Expected size 2048, got %d", properties.Size)
	}
	if properties.ETag != "\"xyz789\"" {
		t.Errorf("Expected ETag '\"xyz789\"', got %s", properties.ETag)
	}
	if properties.ContentType != "application/json" {
		t.Errorf("Expected content type 'application/json', got %s", properties.ContentType)
	}
	if properties.IsDirectory {
		t.Error("Expected properties to not be a directory")
	}
}

func TestExtractFileName(t *testing.T) {
	tests := []struct {
		href     string
		expected string
	}{
		{
			href:     "/remote.php/dav/files/user/documents/test.txt",
			expected: "test.txt",
		},
		{
			href:     "/remote.php/dav/files/user/documents/",
			expected: "documents",
		},
		{
			href:     "test.txt",
			expected: "test.txt",
		},
		{
			href:     "/remote.php/dav/files/user/documents/subfolder/",
			expected: "subfolder",
		},
	}

	for _, test := range tests {
		result := extractFileName(test.href)
		if result != test.expected {
			t.Errorf("extractFileName(%s) = %s, expected %s", test.href, result, test.expected)
		}
	}
}

func TestParseWebDAVTime(t *testing.T) {
	tests := []struct {
		timeStr   string
		expectErr bool
	}{
		{
			timeStr:   "Mon, 04 Feb 2026 10:30:00 GMT",
			expectErr: false,
		},
		{
			timeStr:   "Mon, 4 Feb 2026 10:30:00 GMT",
			expectErr: false,
		},
		{
			timeStr:   "2026-02-04T10:30:00Z",
			expectErr: false,
		},
		{
			timeStr:   "invalid-time",
			expectErr: true,
		},
	}

	for _, test := range tests {
		result, err := parseWebDAVTime(test.timeStr)
		if test.expectErr {
			if err == nil {
				t.Errorf("parseWebDAVTime(%s) expected error but got none", test.timeStr)
			}
		} else {
			if err != nil {
				t.Errorf("parseWebDAVTime(%s) unexpected error: %v", test.timeStr, err)
			}
			if result.IsZero() {
				t.Errorf("parseWebDAVTime(%s) returned zero time", test.timeStr)
			}
		}
	}
}

func TestValidateMultistatus(t *testing.T) {
	tests := []struct {
		name        string
		multistatus *Multistatus
		expectErr   bool
	}{
		{
			name:        "nil multistatus",
			multistatus: nil,
			expectErr:   true,
		},
		{
			name: "empty responses",
			multistatus: &Multistatus{
				Responses: []Response{},
			},
			expectErr: true,
		},
		{
			name: "valid response",
			multistatus: &Multistatus{
				Responses: []Response{
					{Href: "/test.txt"},
				},
			},
			expectErr: false,
		},
		{
			name: "response with empty href",
			multistatus: &Multistatus{
				Responses: []Response{
					{Href: ""},
				},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateMultistatus(test.multistatus)
			if test.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !test.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
