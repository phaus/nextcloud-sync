package utils

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	assert.Equal(t, 3, config.MaxRetries, "Default max retries should be 3")
	assert.Equal(t, 1*time.Second, config.InitialDelay, "Default initial delay should be 1 second")
	assert.Equal(t, 30*time.Second, config.MaxDelay, "Default max delay should be 30 seconds")
	assert.Equal(t, 2.0, config.Multiplier, "Default multiplier should be 2.0")
	assert.Equal(t, 0.1, config.RandomizationFactor, "Default randomization factor should be 0.1")
}

func TestCalculateDelay(t *testing.T) {
	config := &RetryConfig{
		MaxDelay:            30 * time.Second,
		Multiplier:          2.0,
		RandomizationFactor: 0.0, // No randomization for predictable tests
	}

	tests := []struct {
		name          string
		currentDelay  time.Duration
		expectedDelay time.Duration
	}{
		{
			name:          "1 second base",
			currentDelay:  1 * time.Second,
			expectedDelay: 2 * time.Second,
		},
		{
			name:          "10 seconds base",
			currentDelay:  10 * time.Second,
			expectedDelay: 20 * time.Second,
		},
		{
			name:          "capped at max delay",
			currentDelay:  20 * time.Second,
			expectedDelay: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := calculateDelay(tt.currentDelay, config)
			assert.Equal(t, tt.expectedDelay, delay)
		})
	}
}

func TestCalculateDelayWithRandomization(t *testing.T) {
	config := &RetryConfig{
		MaxDelay:            30 * time.Second,
		Multiplier:          2.0,
		RandomizationFactor: 0.5, // 50% randomization
	}

	currentDelay := 1 * time.Second
	expectedBase := float64(currentDelay) * config.Multiplier // 2 seconds

	// Run multiple times to check randomization
	delays := make([]time.Duration, 100)
	for i := 0; i < 100; i++ {
		delays[i] = calculateDelay(currentDelay, config)
	}

	// Check that delays vary (some should be less than base, some more)
	var less, more int
	expectedDuration := time.Duration(expectedBase)
	for _, delay := range delays {
		if delay < expectedDuration {
			less++
		} else if delay > expectedDuration {
			more++
		}
	}

	assert.Greater(t, less, 0, "Some delays should be less than base with randomization")
	assert.Greater(t, more, 0, "Some delays should be more than base with randomization")
}

func TestRetryWithBackoffSuccess(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:          3,
		InitialDelay:        10 * time.Millisecond, // Short for tests
		MaxDelay:            100 * time.Millisecond,
		Multiplier:          2.0,
		RandomizationFactor: 0.0, // No randomization for predictable tests
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 2 {
			return errors.New("temporary error")
		}
		return nil // Success on second call
	}

	isRetryable := func(err error) bool {
		return err.Error() == "temporary error"
	}

	ctx := context.Background()
	err := RetryWithBackoff(ctx, config, isRetryable, fn)

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount, "Function should be called twice")
}

func TestRetryWithBackoffMaxRetriesExceeded(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:          2,
		InitialDelay:        10 * time.Millisecond, // Short for tests
		MaxDelay:            100 * time.Millisecond,
		Multiplier:          2.0,
		RandomizationFactor: 0.0, // No randomization for predictable tests
	}

	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("always fails")
	}

	isRetryable := func(err error) bool {
		return true
	}

	ctx := context.Background()
	err := RetryWithBackoff(ctx, config, isRetryable, fn)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retries (2) exceeded")
	assert.Equal(t, 3, callCount, "Function should be called maxRetries + 1 times")
}

func TestRetryWithBackoffNonRetryableError(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:          3,
		InitialDelay:        10 * time.Millisecond,
		MaxDelay:            100 * time.Millisecond,
		Multiplier:          2.0,
		RandomizationFactor: 0.0,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("non-retryable error")
	}

	isRetryable := func(err error) bool {
		return false // All errors are non-retryable
	}

	ctx := context.Background()
	err := RetryWithBackoff(ctx, config, isRetryable, fn)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-retryable error")
	assert.Equal(t, 1, callCount, "Function should be called only once for non-retryable error")
}

func TestRetryWithBackoffCancelledContext(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:          5,
		InitialDelay:        100 * time.Millisecond, // Longer for this test
		MaxDelay:            1 * time.Second,
		Multiplier:          2.0,
		RandomizationFactor: 0.0,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("temporary error")
	}

	isRetryable := func(err error) bool {
		return true
	}

	// Create a context that will be cancelled after 50ms
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := RetryWithBackoff(ctx, config, isRetryable, fn)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled during retry")
	assert.Less(t, elapsed, 200*time.Millisecond, "Should return quickly due to context cancellation")
	assert.GreaterOrEqual(t, callCount, 1, "Should have made at least one attempt")
}

// Define mockWebDAVError at package level
type mockWebDAVError struct {
	temporary bool
	message   string
}

func (m *mockWebDAVError) Error() string {
	return m.message
}

func (m *mockWebDAVError) IsTemporary() bool {
	return m.temporary
}

func TestIsTemporaryWebDAVError(t *testing.T) {

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "temporary WebDAV error",
			err:      &mockWebDAVError{temporary: true, message: "timeout"},
			expected: true,
		},
		{
			name:     "non-temporary WebDAV error",
			err:      &mockWebDAVError{temporary: false, message: "not found"},
			expected: false,
		},
		{
			name:     "connection refused error",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "timeout error",
			err:      errors.New("request timeout"),
			expected: true,
		},
		{
			name:     "network unreachable error",
			err:      errors.New("network is unreachable"),
			expected: true,
		},
		{
			name:     "service unavailable error",
			err:      errors.New("service unavailable"),
			expected: true,
		},
		{
			name:     "deadline exceeded error",
			err:      errors.New("deadline exceeded"),
			expected: true,
		},
		{
			name:     "non-temporary error",
			err:      errors.New("file not found"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTemporaryWebDAVError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
