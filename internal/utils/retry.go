package utils

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

// RetryConfig contains configuration for retry logic
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
	// InitialDelay is the delay for the first retry
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// Multiplier is the backoff multiplier (default: 2.0)
	Multiplier float64
	// RandomizationFactor adds jitter to prevent thundering herd
	RandomizationFactor float64
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:          3,
		InitialDelay:        1 * time.Second,
		MaxDelay:            30 * time.Second,
		Multiplier:          2.0,
		RandomizationFactor: 0.1,
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// IsRetryableFunc determines if an error should be retried
type IsRetryableFunc func(error) bool

// RetryWithBackoff executes a function with exponential backoff retry logic
func RetryWithBackoff(ctx context.Context, config *RetryConfig, isRetryable IsRetryableFunc, fn RetryableFunc) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Execute the function
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if we should retry this error
		if !isRetryable(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Don't wait after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate next delay with exponential backoff and jitter
		nextDelay := calculateDelay(delay, config)

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(nextDelay):
			// Continue with next attempt
		}

		delay = nextDelay
	}

	return fmt.Errorf("max retries (%d) exceeded, last error: %w", config.MaxRetries, lastErr)
}

// calculateDelay calculates the next delay with exponential backoff and jitter
func calculateDelay(currentDelay time.Duration, config *RetryConfig) time.Duration {
	// Calculate exponential backoff
	exponentialDelay := float64(currentDelay) * config.Multiplier

	// Cap at max delay
	if exponentialDelay > float64(config.MaxDelay) {
		exponentialDelay = float64(config.MaxDelay)
	}

	// Add random jitter to prevent thundering herd
	if config.RandomizationFactor > 0 {
		// Generate random value in [-randomizationFactor, +randomizationFactor]
		randomDeviation := (rand.Float64()*2 - 1) * config.RandomizationFactor
		exponentialDelay *= (1 + randomDeviation)
	}

	// Ensure non-negative delay
	if exponentialDelay < 0 {
		exponentialDelay = 0
	}

	return time.Duration(math.Round(exponentialDelay))
}

// IsTemporaryWebDAVError returns true if the error might be resolved by retrying
func IsTemporaryWebDAVError(err error) bool {
	if err == nil {
		return false
	}

	// Import the webdav package to check WebDAV errors
	type webDAVError interface {
		IsTemporary() bool
	}

	if webdavErr, ok := err.(webDAVError); ok {
		return webdavErr.IsTemporary()
	}

	// For non-WebDAV errors, use some common network-related patterns
	errStr := err.Error()
	temporaryPatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"network is unreachable",
		"temporary failure",
		"service unavailable",
		"deadline exceeded",
	}

	for _, pattern := range temporaryPatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// contains performs a case-insensitive substring search
func contains(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}
