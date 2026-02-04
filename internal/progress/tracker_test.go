package progress

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		total   int64
		updates []int64
	}{
		{
			name:    "basic progress",
			width:   10,
			total:   100,
			updates: []int64{25, 50, 75, 100},
		},
		{
			name:    "zero width",
			width:   0,
			total:   100,
			updates: []int64{50, 100},
		},
		{
			name:    "negative width",
			width:   -5,
			total:   100,
			updates: []int64{50, 100},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := NewProgressBar(tt.width)
			assert.NotNil(t, pb)

			pb.SetOperation("Test Operation")
			pb.Start(tt.total)

			for _, update := range tt.updates {
				pb.Update(update)
			}

			pb.Finish()
		})
	}
}

func TestProgressBar_Disabled(t *testing.T) {
	pb := NewProgressBar(50)
	pb.SetEnabled(false)

	pb.Start(100)
	pb.Update(50)
	pb.Finish()
	pb.Error(nil)

	// Should not panic
}

func TestStatistics(t *testing.T) {
	stats := NewStatistics()
	require.NotNil(t, stats)

	// Test initial state
	assert.Equal(t, 0, stats.TotalFiles)
	assert.Equal(t, 0, stats.ProcessedFiles)
	assert.Equal(t, int64(0), stats.TotalBytes)
	assert.Equal(t, int64(0), stats.TransferredBytes)

	// Test setting totals
	stats.SetTotalFiles(10)
	stats.SetTotalBytes(1000)

	assert.Equal(t, 10, stats.TotalFiles)
	assert.Equal(t, int64(1000), stats.TotalBytes)

	// Test recording operations
	stats.RecordUpload(100)
	assert.Equal(t, 1, stats.Uploads)
	assert.Equal(t, 1, stats.ProcessedFiles)
	assert.Equal(t, int64(100), stats.TransferredBytes)

	stats.RecordDownload(200)
	assert.Equal(t, 1, stats.Downloads)
	assert.Equal(t, 2, stats.ProcessedFiles)
	assert.Equal(t, int64(300), stats.TransferredBytes)

	stats.RecordCreate(150)
	assert.Equal(t, 1, stats.Creates)
	assert.Equal(t, 3, stats.ProcessedFiles)
	assert.Equal(t, int64(450), stats.TransferredBytes)

	stats.RecordUpdate(250)
	assert.Equal(t, 1, stats.Updates)
	assert.Equal(t, 4, stats.ProcessedFiles)
	assert.Equal(t, int64(700), stats.TransferredBytes)

	stats.RecordDelete()
	assert.Equal(t, 1, stats.Deletes)
	assert.Equal(t, 5, stats.ProcessedFiles)

	stats.RecordSkip()
	assert.Equal(t, 1, stats.Skips)
	assert.Equal(t, 6, stats.ProcessedFiles)

	stats.RecordConflict()
	assert.Equal(t, 1, stats.Conflicts)

	stats.RecordError()
	assert.Equal(t, 1, stats.Errors)

	// Test progress calculation
	progress := stats.GetProgress()
	assert.Equal(t, float64(6)/float64(10)*100.0, progress)

	// Test finishing
	stats.Finish()
	assert.Greater(t, stats.Duration, time.Duration(0))

	// Test string representation
	str := stats.String()
	assert.Contains(t, str, "Sync Statistics")
	assert.Contains(t, str, "6/10 processed")
}

func TestResumeManager(t *testing.T) {
	tempDir := t.TempDir()

	rm, err := NewResumeManager(tempDir)
	require.NoError(t, err)
	require.NotNil(t, rm)

	// Test initial state
	assert.True(t, rm.enabled)
	assert.Equal(t, 24*time.Hour, rm.maxAge)

	// Use consistent time for tests
	testTime := time.Now()

	// Test starting a transfer
	state, err := rm.StartTransfer("/test/file.txt", "upload", 1000, testTime)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, "/test/file.txt", state.FilePath)
	assert.Equal(t, int64(1000), state.TotalSize)
	assert.Equal(t, "upload", state.Operation)

	// Test updating progress
	err = rm.UpdateProgress("/test/file.txt", 500, "checksum123")
	require.NoError(t, err)

	// Test getting resume state
	foundState, err := rm.GetResumeState("/test/file.txt", "upload", 1000, testTime)
	require.NoError(t, err)
	require.NotNil(t, foundState)
	assert.Equal(t, int64(500), foundState.TransferredSize)
	assert.Equal(t, "checksum123", foundState.Checksum)

	// Test getting state for non-existent file
	foundState, err = rm.GetResumeState("/test/other.txt", "upload", 1000, time.Now())
	require.NoError(t, err)
	assert.Nil(t, foundState)

	// Test completing transfer
	err = rm.CompleteTransfer("/test/file.txt")
	require.NoError(t, err)

	// State should no longer exist
	foundState, err = rm.GetResumeState("/test/file.txt", "upload", 1000, time.Now())
	require.NoError(t, err)
	assert.Nil(t, foundState)
}

func TestResumeManager_Disabled(t *testing.T) {
	tempDir := t.TempDir()

	rm, err := NewResumeManager(tempDir)
	require.NoError(t, err)

	rm.SetEnabled(false)

	// Should return nil for all operations when disabled
	state, err := rm.StartTransfer("/test/file.txt", "upload", 1000, time.Now())
	require.NoError(t, err)
	assert.Nil(t, state)

	err = rm.UpdateProgress("/test/file.txt", 500, "checksum")
	require.NoError(t, err)

	err = rm.CompleteTransfer("/test/file.txt")
	require.NoError(t, err)

	activeTransfers := rm.GetActiveTransfers()
	assert.Empty(t, activeTransfers)
}

func TestResumeManager_Persistence(t *testing.T) {
	tempDir := t.TempDir()

	// Create first resume manager and add a state
	rm1, err := NewResumeManager(tempDir)
	require.NoError(t, err)

	testTime := time.Now().Add(-time.Hour)
	state, err := rm1.StartTransfer("/test/persistent.txt", "download", 2000, testTime)
	require.NoError(t, err)
	require.NotNil(t, state)

	err = rm1.UpdateProgress("/test/persistent.txt", 1000, "")
	require.NoError(t, err)

	// Create second resume manager and verify state is loaded
	rm2, err := NewResumeManager(tempDir)
	require.NoError(t, err)

	foundState, err := rm2.GetResumeState("/test/persistent.txt", "download", 2000, testTime)
	require.NoError(t, err)
	require.NotNil(t, foundState)
	assert.Equal(t, int64(1000), foundState.TransferredSize)

	// Cleanup
	err = rm2.CompleteTransfer("/test/persistent.txt")
	require.NoError(t, err)
}

func TestResumeManager_InvalidState(t *testing.T) {
	tempDir := t.TempDir()

	rm, err := NewResumeManager(tempDir)
	require.NoError(t, err)

	// Create a state
	_, err = rm.StartTransfer("/test/file.txt", "upload", 1000, time.Now())
	require.NoError(t, err)

	// Try to get state with different parameters (should return nil)
	foundState, err := rm.GetResumeState("/test/file.txt", "download", 1000, time.Now())
	require.NoError(t, err)
	assert.Nil(t, foundState)

	foundState, err = rm.GetResumeState("/test/file.txt", "upload", 2000, time.Now())
	require.NoError(t, err)
	assert.Nil(t, foundState)

	foundState, err = rm.GetResumeState("/test/file.txt", "upload", 1000, time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Nil(t, foundState)
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBytes(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{time.Millisecond * 500, "< 1s"},
		{time.Second * 30, "30s"},
		{time.Minute*2 + time.Second*30, "2m30s"},
		{time.Hour*1 + time.Minute*30, "1h30m"},
		{time.Hour*2 + time.Minute*45, "2h45m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateChecksum(t *testing.T) {
	// Create a temporary file for testing
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	// Write test content
	content := "Hello, World!"
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Calculate checksum
	checksum, err := CalculateChecksum(testFile)
	require.NoError(t, err)
	assert.NotEmpty(t, checksum)
	assert.Equal(t, 64, len(checksum)) // SHA256 hex string length

	// Verify checksum is consistent
	checksum2, err := CalculateChecksum(testFile)
	require.NoError(t, err)
	assert.Equal(t, checksum, checksum2)
}

func TestCalculateChecksum_NonExistentFile(t *testing.T) {
	_, err := CalculateChecksum("/non/existent/file.txt")
	assert.Error(t, err)
}

func TestCombinedProgressTracker(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultConfig()
	config.ShowStatistics = false // Disable actual progress bar for testing

	pt, err := NewCombinedProgressTracker(config, tempDir)
	require.NoError(t, err)
	require.NotNil(t, pt)

	// Test that the tracker implements the required methods
	pt.SetOperation("test")
	pt.Start(100)
	pt.Update(50)
	pt.Finish()
	pt.Error(nil)

	// Test initial state
	stats := pt.GetStatistics()
	assert.NotNil(t, stats)

	// Test basic operations
	pt.SetOperation("UPLOAD /test/file.txt")
	pt.Start(1000)
	pt.Update(500)
	pt.Finish()

	stats = pt.GetStatistics()
	assert.Greater(t, stats.Uploads, 0)

	// Test error handling
	pt.SetOperation("DOWNLOAD /test/error.txt")
	pt.Start(100)
	pt.Error(assert.AnError)
	pt.Finish()

	stats = pt.GetStatistics()
	assert.Greater(t, stats.Errors, 0)

	// Test cleanup
	err = pt.Cleanup()
	require.NoError(t, err)
}
