package webdav

import (
	"errors"
	"net/http"
	"testing"
)

func TestWebDAVError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *WebDAVError
		expected string
	}{
		{
			name:     "full error with path and method",
			err:      &WebDAVError{StatusCode: 404, Message: "not found", Path: "/test.txt", Method: "GET"},
			expected: "GET /test.txt: 404 not found",
		},
		{
			name:     "error without path and method",
			err:      &WebDAVError{StatusCode: 500, Message: "internal server error"},
			expected: "WebDAV error: 500 internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("WebDAVError.Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestWebDAVError_IsMethods(t *testing.T) {
	tests := []struct {
		name        string
		err         *WebDAVError
		isTemporary bool
		isAuth      bool
		isPerm      bool
		isNotFound  bool
		isConflict  bool
		isLocked    bool
		isStorage   bool
	}{
		{
			name:        "temporary error",
			err:         &WebDAVError{StatusCode: http.StatusRequestTimeout},
			isTemporary: true,
		},
		{
			name:   "auth error",
			err:    &WebDAVError{StatusCode: http.StatusUnauthorized},
			isAuth: true,
		},
		{
			name:   "permission error",
			err:    &WebDAVError{StatusCode: http.StatusForbidden},
			isPerm: true,
		},
		{
			name:       "not found error",
			err:        &WebDAVError{StatusCode: http.StatusNotFound},
			isNotFound: true,
		},
		{
			name:       "conflict error",
			err:        &WebDAVError{StatusCode: http.StatusConflict},
			isConflict: true,
		},
		{
			name:     "locked error",
			err:      &WebDAVError{StatusCode: http.StatusLocked},
			isLocked: true,
		},
		{
			name:      "storage error",
			err:       &WebDAVError{StatusCode: http.StatusInsufficientStorage},
			isStorage: true,
		},
		{
			name: "other error",
			err:  &WebDAVError{StatusCode: http.StatusBadRequest},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.IsTemporary() != tt.isTemporary {
				t.Errorf("WebDAVError.IsTemporary() = %v, want %v", tt.err.IsTemporary(), tt.isTemporary)
			}
			if tt.err.IsAuthError() != tt.isAuth {
				t.Errorf("WebDAVError.IsAuthError() = %v, want %v", tt.err.IsAuthError(), tt.isAuth)
			}
			if tt.err.IsPermissionError() != tt.isPerm {
				t.Errorf("WebDAVError.IsPermissionError() = %v, want %v", tt.err.IsPermissionError(), tt.isPerm)
			}
			if tt.err.IsNotFoundError() != tt.isNotFound {
				t.Errorf("WebDAVError.IsNotFoundError() = %v, want %v", tt.err.IsNotFoundError(), tt.isNotFound)
			}
			if tt.err.IsConflictError() != tt.isConflict {
				t.Errorf("WebDAVError.IsConflictError() = %v, want %v", tt.err.IsConflictError(), tt.isConflict)
			}
			if tt.err.IsLockedError() != tt.isLocked {
				t.Errorf("WebDAVError.IsLockedError() = %v, want %v", tt.err.IsLockedError(), tt.isLocked)
			}
			if tt.err.IsStorageError() != tt.isStorage {
				t.Errorf("WebDAVError.IsStorageError() = %v, want %v", tt.err.IsStorageError(), tt.isStorage)
			}
		})
	}
}

func TestNewWebDAVError(t *testing.T) {
	err := NewWebDAVError(http.StatusNotFound, "/test.txt", "GET")

	if err.StatusCode != http.StatusNotFound {
		t.Errorf("NewWebDAVError() StatusCode = %d, want %d", err.StatusCode, http.StatusNotFound)
	}
	if err.Message != "resource not found" {
		t.Errorf("NewWebDAVError() Message = %q, want %q", err.Message, "resource not found")
	}
	if err.Path != "/test.txt" {
		t.Errorf("NewWebDAVError() Path = %q, want %q", err.Path, "/test.txt")
	}
	if err.Method != "GET" {
		t.Errorf("NewWebDAVError() Method = %q, want %q", err.Method, "GET")
	}
}

func TestNewWebDAVErrorWithMessage(t *testing.T) {
	customMessage := "custom error message"
	err := NewWebDAVErrorWithMessage(http.StatusBadRequest, "/test.txt", "POST", customMessage)

	if err.StatusCode != http.StatusBadRequest {
		t.Errorf("NewWebDAVErrorWithMessage() StatusCode = %d, want %d", err.StatusCode, http.StatusBadRequest)
	}
	if err.Message != customMessage {
		t.Errorf("NewWebDAVErrorWithMessage() Message = %q, want %q", err.Message, customMessage)
	}
}

func TestNewWebDAVErrorUnknownStatus(t *testing.T) {
	unknownStatus := 999
	err := NewWebDAVError(unknownStatus, "/test.txt", "GET")

	if err.StatusCode != unknownStatus {
		t.Errorf("NewWebDAVError() StatusCode = %d, want %d", err.StatusCode, unknownStatus)
	}
	if err.Message != "unknown error" {
		t.Errorf("NewWebDAVError() Message = %q, want %q", err.Message, "unknown error")
	}
}

func TestIsWebDAVError(t *testing.T) {
	webdavErr := NewWebDAVError(http.StatusNotFound, "/test.txt", "GET")
	genericErr := errors.New("generic error")

	// Test WebDAV error
	result, isWebDAV := IsWebDAVError(webdavErr)
	if !isWebDAV {
		t.Errorf("IsWebDAVError() should return true for WebDAVError")
	}
	if result != webdavErr {
		t.Errorf("IsWebDAVError() should return the original error")
	}

	// Test generic error
	result, isWebDAV = IsWebDAVError(genericErr)
	if isWebDAV {
		t.Errorf("IsWebDAVError() should return false for generic error")
	}
	if result != nil {
		t.Errorf("IsWebDAVError() should return nil for generic error")
	}

	// Test nil error
	result, isWebDAV = IsWebDAVError(nil)
	if isWebDAV {
		t.Errorf("IsWebDAVError() should return false for nil error")
	}
	if result != nil {
		t.Errorf("IsWebDAVError() should return nil for nil error")
	}
}

func TestWrapHTTPError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		path       string
		method     string
		wantWebDAV bool
		wantStatus int
	}{
		{
			name:       "nil error",
			err:        nil,
			path:       "/test.txt",
			method:     "GET",
			wantWebDAV: false,
		},
		{
			name:       "already WebDAV error",
			err:        NewWebDAVError(http.StatusNotFound, "/test.txt", "GET"),
			path:       "/test.txt",
			method:     "GET",
			wantWebDAV: true,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "generic error",
			err:        errors.New("some generic error"),
			path:       "/test.txt",
			method:     "GET",
			wantWebDAV: true,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "error with status code in message",
			err:        errors.New("HTTP 404 not found"),
			path:       "/test.txt",
			method:     "GET",
			wantWebDAV: true,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapHTTPError(tt.err, tt.path, tt.method)

			if tt.err == nil {
				if result != nil {
					t.Errorf("WrapHTTPError() should return nil for nil input")
				}
				return
			}

			webdavErr, isWebDAV := IsWebDAVError(result)
			if isWebDAV != tt.wantWebDAV {
				t.Errorf("WrapHTTPError() isWebDAV = %v, want %v", isWebDAV, tt.wantWebDAV)
			}

			if tt.wantWebDAV && webdavErr.StatusCode != tt.wantStatus {
				t.Errorf("WrapHTTPError() StatusCode = %d, want %d", webdavErr.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestCommonErrorConstructors(t *testing.T) {
	tests := []struct {
		name           string
		constructor    func(string, string) *WebDAVError
		expectedStatus int
	}{
		{
			name:           "NewAuthError",
			constructor:    NewAuthError,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "NewPermissionError",
			constructor:    NewPermissionError,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "NewNotFoundError",
			constructor:    NewNotFoundError,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "NewConflictError",
			constructor:    NewConflictError,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "NewLockedError",
			constructor:    NewLockedError,
			expectedStatus: http.StatusLocked,
		},
		{
			name:           "NewStorageError",
			constructor:    NewStorageError,
			expectedStatus: http.StatusInsufficientStorage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor("/test.txt", "GET")
			if err.StatusCode != tt.expectedStatus {
				t.Errorf("%s() StatusCode = %d, want %d", tt.name, err.StatusCode, tt.expectedStatus)
			}
			if err.Path != "/test.txt" {
				t.Errorf("%s() Path = %q, want %q", tt.name, err.Path, "/test.txt")
			}
			if err.Method != "GET" {
				t.Errorf("%s() Method = %q, want %q", tt.name, err.Method, "GET")
			}
		})
	}
}

func TestErrorMessages(t *testing.T) {
	// Test that all important status codes have messages
	importantCodes := []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusConflict,
		http.StatusLocked,
		http.StatusInsufficientStorage,
	}

	for _, code := range importantCodes {
		message, ok := errorMessages[code]
		if !ok {
			t.Errorf("Missing error message for status code %d", code)
		}
		if message == "" {
			t.Errorf("Empty error message for status code %d", code)
		}
	}
}
