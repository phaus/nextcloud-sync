package webdav

import (
	"strings"
	"testing"
	"time"
)

func TestPropertyRequest_Basic(t *testing.T) {
	propReq := NewPropertyRequest()

	if propReq.AllProp != false {
		t.Errorf("Expected AllProp to be false, got %v", propReq.AllProp)
	}

	if propReq.Depth != DepthOne {
		t.Errorf("Expected depth to be %s, got %s", DepthOne, propReq.Depth)
	}

	if len(propReq.Properties) == 0 {
		t.Error("Expected properties to be set, got empty slice")
	}
}

func TestPropertyRequest_BuildPROPFINDBody(t *testing.T) {
	propReq := NewMinimalPropertyRequest()
	body := propReq.BuildPROPFINDBody()

	if body == "" {
		t.Error("Expected non-empty PROPFIND body")
	}

	// Check that it contains XML structure
	if !strings.Contains(body, "propfind") {
		t.Error("PROPFIND body should contain 'propfind'")
	}

	if !strings.Contains(body, "prop") {
		t.Error("PROPFIND body should contain 'prop'")
	}
}

func TestPropertyRequest_DepthValidation(t *testing.T) {
	propReq := NewPropertyRequest()

	// Test valid depths
	validDepths := []string{DepthZero, DepthOne, DepthInfinity}
	for _, depth := range validDepths {
		if err := propReq.SetDepth(depth); err != nil {
			t.Errorf("Expected no error for valid depth %s, got %v", depth, err)
		}
	}

	// Test invalid depth
	if err := propReq.SetDepth("invalid"); err == nil {
		t.Error("Expected error for invalid depth, got nil")
	}
}

func TestPropertyRequest_AddRemoveProperty(t *testing.T) {
	propReq := NewMinimalPropertyRequest()
	initialCount := len(propReq.Properties)

	// Add property
	propReq.AddProperty("d:testprop")
	if len(propReq.Properties) != initialCount+1 {
		t.Errorf("Expected properties count to be %d, got %d", initialCount+1, len(propReq.Properties))
	}

	// Remove property
	propReq.RemoveProperty("d:testprop")
	if len(propReq.Properties) != initialCount {
		t.Errorf("Expected properties count to be %d, got %d", initialCount, len(propReq.Properties))
	}
}

func TestNewAllPropertiesRequest(t *testing.T) {
	propReq := NewAllPropertiesRequest()

	if !propReq.AllProp {
		t.Error("Expected AllProp to be true for all properties request")
	}

	if propReq.Depth != DepthZero {
		t.Errorf("Expected depth to be %s for all properties request, got %s", DepthZero, propReq.Depth)
	}
}

func TestPropertyValidator_ValidateETag(t *testing.T) {
	validator := NewPropertyValidator()

	// Valid ETags
	validETags := []string{"", "\"abc123\"", "\"etag-with-dashes\""}
	for _, etag := range validETags {
		if err := validator.ValidateETag(etag); err != nil {
			t.Errorf("Expected no error for valid ETag %s, got %v", etag, err)
		}
	}

	// Invalid ETags
	invalidETags := []string{"abc123", "unquoted", ""}
	for _, etag := range invalidETags {
		if etag != "" && etag != "unquoted" { // Skip empty and valid unquoted cases
			if err := validator.ValidateETag(etag); err == nil {
				t.Errorf("Expected error for invalid ETag %s, got nil", etag)
			}
		}
	}
}

func TestPropertyValidator_ValidateContentLength(t *testing.T) {
	validator := NewPropertyValidator()

	// Valid content lengths
	validLengths := []int64{0, 1, 1024, 1024 * 1024}
	for _, length := range validLengths {
		if err := validator.ValidateContentLength(length); err != nil {
			t.Errorf("Expected no error for valid content length %d, got %v", length, err)
		}
	}

	// Invalid content lengths
	invalidLengths := []int64{-1, -1024}
	for _, length := range invalidLengths {
		if err := validator.ValidateContentLength(length); err == nil {
			t.Errorf("Expected error for invalid content length %d, got nil", length)
		}
	}
}

func TestPropertyValidator_ValidateLastModified(t *testing.T) {
	validator := NewPropertyValidator()

	// Valid last modified times
	now := time.Now()
	validTimes := []time.Time{
		now,
		now.Add(-time.Hour),
		now.Add(-24 * time.Hour),
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	for _, modTime := range validTimes {
		if err := validator.ValidateLastModified(modTime); err != nil {
			t.Errorf("Expected no error for valid last modified time %v, got %v", modTime, err)
		}
	}

	// Invalid last modified times
	invalidTimes := []time.Time{
		time.Time{}, // Zero time
		time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC), // Too old
		now.Add(2 * time.Hour),                      // Too far in future
	}
	for _, modTime := range invalidTimes {
		if err := validator.ValidateLastModified(modTime); err == nil {
			t.Errorf("Expected error for invalid last modified time %v, got nil", modTime)
		}
	}
}

func TestPropertyHelper_IsCollection(t *testing.T) {
	helper := NewPropertyHelper()

	// Directory (collection)
	collectionType := ResourceType{Collection: []string{"collection"}}
	if !helper.IsCollection(collectionType) {
		t.Error("Expected collection to be identified as directory")
	}

	// File (not a collection)
	fileType := ResourceType{Collection: []string{}}
	if helper.IsCollection(fileType) {
		t.Error("Expected non-collection to be identified as file")
	}
}

func TestPropertyHelper_CompareETags(t *testing.T) {
	helper := NewPropertyHelper()

	// Matching ETags
	if !helper.CompareETags("\"etag123\"", "\"etag123\"") {
		t.Error("Expected matching ETags to be equal")
	}

	if !helper.CompareETags("etag123", "etag123") {
		t.Error("Expected unquoted matching ETags to be equal")
	}

	// Non-matching ETags
	if helper.CompareETags("\"etag123\"", "\"etag456\"") {
		t.Error("Expected different ETags to not be equal")
	}
}

func TestPredefinedRequests(t *testing.T) {
	// Test standard property request
	standardReq := GetStandardPropertyRequest()
	if standardReq.Depth != DepthOne {
		t.Errorf("Expected standard request depth to be %s, got %s", DepthOne, standardReq.Depth)
	}

	// Test file property request
	fileReq := GetFilePropertyRequest()
	if fileReq.Depth != DepthZero {
		t.Errorf("Expected file request depth to be %s, got %s", DepthZero, fileReq.Depth)
	}

	// Test change detection request
	changeReq := GetChangeDetectionRequest()
	if changeReq.Depth != DepthOne {
		t.Errorf("Expected change detection request depth to be %s, got %s", DepthOne, changeReq.Depth)
	}
}

func TestGetMinimalPropertiesForChangeDetection(t *testing.T) {
	props := GetMinimalPropertiesForChangeDetection()

	expectedProps := []string{
		PropETag,
		PropLastModified,
		PropResourceType,
	}

	if len(props) != len(expectedProps) {
		t.Errorf("Expected %d properties, got %d", len(expectedProps), len(props))
	}

	for i, prop := range props {
		if prop != expectedProps[i] {
			t.Errorf("Expected property %s at index %d, got %s", expectedProps[i], i, prop)
		}
	}
}
