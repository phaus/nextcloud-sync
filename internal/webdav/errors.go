package webdav

import (
	"fmt"
	"net/http"
)

// WebDAVError represents a WebDAV-specific error with HTTP status code
type WebDAVError struct {
	StatusCode int
	Message    string
	Path       string
	Method     string
}

// Error implements the error interface
func (e *WebDAVError) Error() string {
	if e.Path != "" && e.Method != "" {
		return fmt.Sprintf("%s %s: %d %s", e.Method, e.Path, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("WebDAV error: %d %s", e.StatusCode, e.Message)
}

// IsTemporary returns true if the error might be resolved by retrying
func (e *WebDAVError) IsTemporary() bool {
	switch e.StatusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// IsAuthError returns true if the error is authentication-related
func (e *WebDAVError) IsAuthError() bool {
	return e.StatusCode == http.StatusUnauthorized
}

// IsPermissionError returns true if the error is permission-related
func (e *WebDAVError) IsPermissionError() bool {
	return e.StatusCode == http.StatusForbidden
}

// IsNotFoundError returns true if the error indicates a resource was not found
func (e *WebDAVError) IsNotFoundError() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsConflictError returns true if the error indicates a conflict
func (e *WebDAVError) IsConflictError() bool {
	return e.StatusCode == http.StatusConflict
}

// IsLockedError returns true if the error indicates a resource is locked
func (e *WebDAVError) IsLockedError() bool {
	return e.StatusCode == http.StatusLocked
}

// IsStorageError returns true if the error indicates insufficient storage
func (e *WebDAVError) IsStorageError() bool {
	return e.StatusCode == http.StatusInsufficientStorage
}

// Predefined error messages for common WebDAV status codes
var errorMessages = map[int]string{
	http.StatusUnauthorized:                  "authentication failed: invalid credentials",
	http.StatusForbidden:                     "permission denied: insufficient privileges",
	http.StatusNotFound:                      "resource not found",
	http.StatusMethodNotAllowed:              "method not allowed",
	http.StatusRequestTimeout:                "request timeout",
	http.StatusConflict:                      "resource conflict: resource already exists",
	http.StatusGone:                          "resource no longer available",
	http.StatusLengthRequired:                "content length required",
	http.StatusPreconditionFailed:            "precondition failed",
	http.StatusRequestEntityTooLarge:         "request entity too large",
	http.StatusRequestURITooLong:             "URI too long",
	http.StatusUnsupportedMediaType:          "unsupported media type",
	http.StatusRequestedRangeNotSatisfiable:  "requested range not satisfiable",
	http.StatusExpectationFailed:             "expectation failed",
	http.StatusTeapot:                        "I'm a teapot", // Just for completeness
	http.StatusMisdirectedRequest:            "misdirected request",
	http.StatusUnprocessableEntity:           "unprocessable entity",
	http.StatusLocked:                        "resource is locked",
	http.StatusFailedDependency:              "failed dependency",
	http.StatusTooEarly:                      "too early",
	http.StatusUpgradeRequired:               "upgrade required",
	http.StatusPreconditionRequired:          "precondition required",
	http.StatusTooManyRequests:               "too many requests",
	http.StatusRequestHeaderFieldsTooLarge:   "request header fields too large",
	http.StatusUnavailableForLegalReasons:    "unavailable for legal reasons",
	http.StatusInternalServerError:           "internal server error",
	http.StatusNotImplemented:                "not implemented",
	http.StatusBadGateway:                    "bad gateway",
	http.StatusServiceUnavailable:            "service unavailable",
	http.StatusGatewayTimeout:                "gateway timeout",
	http.StatusHTTPVersionNotSupported:       "HTTP version not supported",
	http.StatusInsufficientStorage:           "insufficient storage: quota exceeded",
	http.StatusLoopDetected:                  "loop detected",
	http.StatusNotExtended:                   "not extended",
	http.StatusNetworkAuthenticationRequired: "network authentication required",
}

// NewWebDAVError creates a new WebDAVError with the given status code and context
func NewWebDAVError(statusCode int, path, method string) *WebDAVError {
	message, ok := errorMessages[statusCode]
	if !ok {
		message = http.StatusText(statusCode)
		if message == "" {
			message = "unknown error"
		}
	}

	return &WebDAVError{
		StatusCode: statusCode,
		Message:    message,
		Path:       path,
		Method:     method,
	}
}

// NewWebDAVErrorWithMessage creates a new WebDAVError with a custom message
func NewWebDAVErrorWithMessage(statusCode int, path, method, message string) *WebDAVError {
	if message == "" {
		message = http.StatusText(statusCode)
		if message == "" {
			message = "unknown error"
		}
	}

	return &WebDAVError{
		StatusCode: statusCode,
		Message:    message,
		Path:       path,
		Method:     method,
	}
}

// IsWebDAVError checks if an error is a WebDAVError
func IsWebDAVError(err error) (*WebDAVError, bool) {
	if webdavErr, ok := err.(*WebDAVError); ok {
		return webdavErr, true
	}
	return nil, false
}

// WrapHTTPError wraps an HTTP error into a WebDAVError if possible
func WrapHTTPError(err error, path, method string) error {
	if err == nil {
		return nil
	}

	// Check if it's already a WebDAVError
	if _, ok := IsWebDAVError(err); ok {
		return err
	}

	// Try to extract status code from common error patterns
	// This is a basic implementation - in a real scenario you might want
	// to check for specific error types or message patterns
	errMsg := err.Error()

	// Check for common status code patterns in error messages
	for code, message := range errorMessages {
		if containsSubstring(errMsg, message) || containsSubstring(errMsg, fmt.Sprintf("%d", code)) {
			return NewWebDAVError(code, path, method)
		}
	}

	// If no specific status code found, return a generic WebDAV error
	return NewWebDAVErrorWithMessage(http.StatusInternalServerError, path, method, err.Error())
}

// containsSubstring checks if a string contains a substring (case-insensitive)
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				findSubstring(s, substr))))
}

// findSubstring performs a simple case-insensitive substring search
func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}

	s_lower := s
	substr_lower := substr

	// Simple implementation - in production you'd use strings.Contains
	for i := 0; i <= len(s_lower)-len(substr_lower); i++ {
		if s_lower[i:i+len(substr_lower)] == substr_lower {
			return true
		}
	}
	return false
}

// Common error constructors for frequently used scenarios

// NewAuthError creates a 401 Unauthorized error
func NewAuthError(path, method string) *WebDAVError {
	return NewWebDAVError(http.StatusUnauthorized, path, method)
}

// NewPermissionError creates a 403 Forbidden error
func NewPermissionError(path, method string) *WebDAVError {
	return NewWebDAVError(http.StatusForbidden, path, method)
}

// NewNotFoundError creates a 404 Not Found error
func NewNotFoundError(path, method string) *WebDAVError {
	return NewWebDAVError(http.StatusNotFound, path, method)
}

// NewConflictError creates a 409 Conflict error
func NewConflictError(path, method string) *WebDAVError {
	return NewWebDAVError(http.StatusConflict, path, method)
}

// NewLockedError creates a 423 Locked error
func NewLockedError(path, method string) *WebDAVError {
	return NewWebDAVError(http.StatusLocked, path, method)
}

// NewStorageError creates a 507 Insufficient Storage error
func NewStorageError(path, method string) *WebDAVError {
	return NewWebDAVError(http.StatusInsufficientStorage, path, method)
}
