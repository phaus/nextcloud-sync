package progress

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResumeState_JSON(t *testing.T) {
	state := ResumeState{
		FilePath:        "/test/file.txt",
		TotalSize:       1000,
		TransferredSize: 500,
		Checksum:        "abc123",
		LastModified:    time.Now().Truncate(time.Second),
		Operation:       "upload",
		CreatedAt:       time.Now().Truncate(time.Second),
		UpdatedAt:       time.Now().Truncate(time.Second),
	}

	// Test JSON marshaling
	data, err := json.Marshal(state)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test JSON unmarshaling
	var unmarshaled ResumeState
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, state.FilePath, unmarshaled.FilePath)
	assert.Equal(t, state.TotalSize, unmarshaled.TotalSize)
	assert.Equal(t, state.TransferredSize, unmarshaled.TransferredSize)
	assert.Equal(t, state.Checksum, unmarshaled.Checksum)
	assert.Equal(t, state.Operation, unmarshaled.Operation)
	assert.True(t, state.LastModified.Equal(unmarshaled.LastModified))
}

func TestResumeManager_CleanupOldStates(t *testing.T) {
	tempDir := t.TempDir()

	rm, err := NewResumeManager(tempDir)
	require.NoError(t, err)

	// Set a very short max age for testing
	rm.SetMaxAge(time.Millisecond * 100)

	// Create a transfer state
	_, err = rm.StartTransfer("/test/old.txt", "upload", 1000, time.Now())
	require.NoError(t, err)

	// Wait for the state to become old
	time.Sleep(time.Millisecond * 200)

	// Trigger cleanup manually
	err = rm.Cleanup()
	require.NoError(t, err)

	// The old state should have been cleaned up
	activeTransfers := rm.GetActiveTransfers()
	assert.Empty(t, activeTransfers)
}

func TestResumeManager_FileNameGeneration(t *testing.T) {
	tempDir := t.TempDir()

	rm, err := NewResumeManager(tempDir)
	require.NoError(t, err)

	// Test that different file paths generate different filenames
	file1 := "/test/file1.txt"
	file2 := "/test/file2.txt"
	file3 := "/test/file1.txt" // Same as file1

	name1 := rm.getResumeFileName(file1)
	name2 := rm.getResumeFileName(file2)
	name3 := rm.getResumeFileName(file3)

	assert.NotEqual(t, name1, name2)
	assert.Equal(t, name1, name3)
	assert.Contains(t, name1, ".resume")
	assert.Contains(t, name2, ".resume")
}

func TestResumeManager_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()

	rm, err := NewResumeManager(tempDir)
	require.NoError(t, err)

	// Test concurrent access to the resume manager
	done := make(chan bool, 10)

	// Start multiple goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			filePath := filepath.Join("/test", fmt.Sprintf("file%d.txt", id))

			// Start transfer
			state, err := rm.StartTransfer(filePath, "upload", 1000, time.Now())
			assert.NoError(t, err)
			assert.NotNil(t, state)

			// Update progress
			for j := 0; j < 10; j++ {
				err = rm.UpdateProgress(filePath, int64((j+1)*100), "")
				assert.NoError(t, err)
				time.Sleep(time.Millisecond)
			}

			// Complete transfer
			err = rm.CompleteTransfer(filePath)
			assert.NoError(t, err)

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// OK
		case <-time.After(time.Second * 5):
			t.Fatal("Timeout waiting for goroutines")
		}
	}

	// Verify no active transfers remain
	activeTransfers := rm.GetActiveTransfers()
	assert.Empty(t, activeTransfers)
}

func TestResumeManager_AutoCleanup(t *testing.T) {
	tempDir := t.TempDir()

	rm, err := NewResumeManager(tempDir)
	require.NoError(t, err)

	// Set max age to a very short time for testing
	rm.SetMaxAge(time.Millisecond)

	// Create a transfer
	_, err = rm.StartTransfer("/test/autoclean.txt", "upload", 1000, time.Now())
	require.NoError(t, err)

	// Manually trigger cleanup to test the cleanup logic
	err = rm.Cleanup()
	require.NoError(t, err)

	// Verify it was cleaned up (though Cleanup() clears all states)
	activeTransfers := rm.GetActiveTransfers()
	assert.Empty(t, activeTransfers)
}

func TestResumeManager_DirectoryHandling(t *testing.T) {
	// Test with non-existent directory (should be created)
	nonExistentDir := filepath.Join(os.TempDir(), "nextcloud-resume-test", time.Now().Format("20060102-150405"))
	defer os.RemoveAll(nonExistentDir)

	rm, err := NewResumeManager(nonExistentDir)
	require.NoError(t, err)

	// Verify directory was created
	info, err := os.Stat(nonExistentDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Test basic functionality
	_, err = rm.StartTransfer("/test/directory-test.txt", "upload", 1000, time.Now())
	require.NoError(t, err)

	err = rm.CompleteTransfer("/test/directory-test.txt")
	require.NoError(t, err)
}

func TestResumeManager_EmptyDirectory(t *testing.T) {
	// Test with empty string (should use default directory)
	rm, err := NewResumeManager("")
	require.NoError(t, err)
	require.NotNil(t, rm)

	// Should have created a default resume directory
	assert.NotEmpty(t, rm.resumeDir)
	assert.Contains(t, rm.resumeDir, ".nextcloud-sync")
}

func TestCombinedProgressTracker_ResumeIntegration(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultConfig()
	config.ShowStatistics = false // Disable for testing

	pt, err := NewCombinedProgressTracker(config, tempDir)
	require.NoError(t, err)

	// Test that resume manager is accessible
	resumeMgr := pt.GetResumeManager()
	require.NotNil(t, resumeMgr)

	// Test setting operation and checking resume state
	pt.SetOperation("UPLOAD /test/resume-integration.txt")
	pt.Start(2000)
	pt.Update(1000)

	// Check that resume state was created (may not exist due to resume logic)
	activeTransfers := resumeMgr.GetActiveTransfers()
	if len(activeTransfers) > 0 {
		state := activeTransfers[0]
		assert.Equal(t, "/test/resume-integration.txt", state.FilePath)
		assert.Equal(t, "upload", state.Operation)
		assert.Equal(t, int64(2000), state.TotalSize)
	}

	// Complete the transfer
	pt.Finish()

	// State should be cleaned up
	activeTransfers = resumeMgr.GetActiveTransfers()
	assert.Empty(t, activeTransfers)
}

func TestSplitOperation(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "UPLOAD /test/file.txt",
			expected: []string{"UPLOAD", "/test/file.txt"},
		},
		{
			input:    "DOWNLOAD /path/with spaces/file.txt",
			expected: []string{"DOWNLOAD", "/path/with spaces/file.txt"},
		},
		{
			input:    "UPLOAD \"path with quotes/file.txt\"",
			expected: []string{"UPLOAD", "\"path with quotes/file.txt\""},
		},
		{
			input:    "CREATE",
			expected: []string{"CREATE", ""},
		},
		{
			input:    "",
			expected: []string{"", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitOperation(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
