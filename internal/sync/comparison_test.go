package sync

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareFiles_NilCases(t *testing.T) {
	opts := DefaultComparisonOptions()

	// Both nil
	change := CompareFiles(nil, nil, opts)
	assert.Equal(t, ChangeNone, change.Type)
	assert.Equal(t, "both files nil", change.Reason)

	// Local exists, remote nil
	local := createTestFile("test.txt", 100, time.Now(), "etag123")
	change = CompareFiles(local, nil, opts)
	assert.Equal(t, ChangeCreate, change.Type)
	assert.Equal(t, LocalToRemote, change.Direction)
	assert.Equal(t, "local file not found on remote", change.Reason)

	// Remote exists, local nil
	remote := createTestFile("test.txt", 100, time.Now(), "etag456")
	change = CompareFiles(nil, remote, opts)
	assert.Equal(t, ChangeCreate, change.Type)
	assert.Equal(t, RemoteToLocal, change.Direction)
	assert.Equal(t, "remote file not found locally", change.Reason)
}

func TestCompareFiles_Identical(t *testing.T) {
	now := time.Now()
	local := createTestFile("test.txt", 100, now, "etag123")
	remote := createTestFile("test.txt", 100, now, "etag123")

	opts := DefaultComparisonOptions()
	change := CompareFiles(local, remote, opts)

	assert.Equal(t, ChangeNone, change.Type)
	assert.Equal(t, "files are identical", change.Reason)
}

func TestCompareFiles_LocalNewer(t *testing.T) {
	now := time.Now()
	local := createTestFile("test.txt", 100, now.Add(time.Hour), "etag123")
	remote := createTestFile("test.txt", 100, now, "etag456")

	opts := DefaultComparisonOptions()
	change := CompareFiles(local, remote, opts)

	assert.Equal(t, ChangeUpdate, change.Type)
	assert.Equal(t, LocalToRemote, change.Direction)
	assert.Equal(t, "local file is newer", change.Reason)
}

func TestCompareFiles_RemoteNewer(t *testing.T) {
	now := time.Now()
	local := createTestFile("test.txt", 100, now, "etag123")
	remote := createTestFile("test.txt", 100, now.Add(time.Hour), "etag456")

	opts := DefaultComparisonOptions()
	change := CompareFiles(local, remote, opts)

	assert.Equal(t, ChangeUpdate, change.Type)
	assert.Equal(t, RemoteToLocal, change.Direction)
	assert.Equal(t, "remote file is newer", change.Reason)
}

func TestDetectConflict_TypeMismatch(t *testing.T) {
	local := createTestFile("test.txt", 100, time.Now(), "etag123")
	local.IsDirectory = false

	remote := createTestFile("test.txt", 0, time.Now(), "")
	remote.IsDirectory = true

	conflict := detectConflict(local, remote)

	require.NotNil(t, conflict)
	assert.Equal(t, ConflictTypeChanged, conflict.Type)
	assert.Contains(t, conflict.Description, "type mismatch")
}

func TestDetectConflict_ContentChanged(t *testing.T) {
	now := time.Now()
	local := createTestFile("test.txt", 100, now.Add(2*time.Minute), "etag123")
	remote := createTestFile("test.txt", 100, now.Add(3*time.Minute), "etag456")

	conflict := detectConflict(local, remote)

	require.NotNil(t, conflict)
	assert.Equal(t, ConflictContentChanged, conflict.Type)
	assert.Contains(t, conflict.Description, "modified recently")
}

func TestDetectConflict_SizeMismatch(t *testing.T) {
	now := time.Now()
	local := createTestFile("test.txt", 100, now, "etag123")
	remote := createTestFile("test.txt", 200, now, "etag456")

	conflict := detectConflict(local, remote)

	require.NotNil(t, conflict)
	assert.Equal(t, ConflictContentChanged, conflict.Type)
	assert.Contains(t, conflict.Description, "same modification time but different sizes")
}

func TestDetectChanges(t *testing.T) {
	// Create local tree
	localTree := &FileTree{
		PathMap: make(map[string]*FileNode),
	}
	localFile1 := createTestFile("file1.txt", 100, time.Now(), "etag1")
	localFile2 := createTestFile("file2.txt", 200, time.Now(), "etag2")
	localTree.PathMap["file1.txt"] = &FileNode{Metadata: localFile1, Path: "file1.txt"}
	localTree.PathMap["file2.txt"] = &FileNode{Metadata: localFile2, Path: "file2.txt"}

	// Create remote tree
	remoteTree := &FileTree{
		PathMap: make(map[string]*FileNode),
	}
	remoteFile1 := createTestFile("file1.txt", 100, time.Now(), "etag1") // Same
	remoteFile3 := createTestFile("file3.txt", 300, time.Now(), "etag3") // Different
	remoteTree.PathMap["file1.txt"] = &FileNode{Metadata: remoteFile1, Path: "file1.txt"}
	remoteTree.PathMap["file3.txt"] = &FileNode{Metadata: remoteFile3, Path: "file3.txt"}

	opts := DefaultComparisonOptions()
	changes, conflicts := DetectChanges(localTree, remoteTree, opts)

	// Should detect changes for file2.txt (local only) and file3.txt (remote only)
	assert.Len(t, changes, 2)
	assert.Len(t, conflicts, 0)

	// Check file2.txt change (local to remote)
	var file2Change *Change
	for _, change := range changes {
		if change.LocalMeta != nil && change.LocalMeta.Name == "file2.txt" {
			file2Change = change
			break
		}
	}
	require.NotNil(t, file2Change)
	assert.Equal(t, ChangeCreate, file2Change.Type)
	assert.Equal(t, LocalToRemote, file2Change.Direction)

	// Check file3.txt change (remote to local)
	var file3Change *Change
	for _, change := range changes {
		if change.RemoteMeta != nil && change.RemoteMeta.Name == "file3.txt" {
			file3Change = change
			break
		}
	}
	require.NotNil(t, file3Change)
	assert.Equal(t, ChangeCreate, file3Change.Type)
	assert.Equal(t, RemoteToLocal, file3Change.Direction)
}

func TestCalculatePriority(t *testing.T) {
	baseTime := time.Now().Add(-48 * time.Hour) // 48 hours ago to avoid "recent" bonus

	// Small file
	smallFile := &FileMetadata{
		Path:        "subdir/small.txt",
		Name:        "small.txt",
		Size:        100,
		Modified:    baseTime,
		ETag:        "etag",
		IsDirectory: false,
	}
	priority := calculatePriority(smallFile)
	assert.Equal(t, 10, priority) // Base priority only

	// Large file
	largeFile := &FileMetadata{
		Path:        "subdir/large.txt",
		Name:        "large.txt",
		Size:        200 * 1024 * 1024, // 200MB
		Modified:    baseTime,
		ETag:        "etag",
		IsDirectory: false,
	}
	priority = calculatePriority(largeFile)
	assert.Equal(t, 40, priority) // Base + 30 for >100MB

	// Recently modified file
	recentFile := &FileMetadata{
		Path:        "subdir/recent.txt",
		Name:        "recent.txt",
		Size:        100,
		Modified:    time.Now().Add(-30 * time.Minute),
		ETag:        "etag",
		IsDirectory: false,
	}
	priority = calculatePriority(recentFile)
	assert.Equal(t, 25, priority) // Base + 15 for recent

	// Directory
	dir := &FileMetadata{
		Path:        "subdir/mydir",
		Name:        "mydir",
		Size:        0,
		Modified:    baseTime,
		IsDirectory: true,
	}
	priority = calculatePriority(dir)
	assert.Equal(t, 5, priority) // Base - 5 for directory

	// Root file
	rootFile := &FileMetadata{
		Path:        "root.txt",
		Name:        "root.txt",
		Size:        100,
		Modified:    baseTime,
		ETag:        "etag",
		IsDirectory: false,
	}
	priority = calculatePriority(rootFile)
	assert.Equal(t, 15, priority) // Base + 5 for root
}

func TestCompareETagsSafely(t *testing.T) {
	// Identical ETags
	assert.True(t, CompareETagsSafely("etag123", "etag123"))
	assert.True(t, CompareETagsSafely("\"etag123\"", "etag123"))
	assert.True(t, CompareETagsSafely(" etag123 ", "  etag123  "))

	// Different ETags
	assert.False(t, CompareETagsSafely("etag123", "etag456"))

	// Empty ETags
	assert.False(t, CompareETagsSafely("", "etag123"))
	assert.False(t, CompareETagsSafely("etag123", ""))
	assert.False(t, CompareETagsSafely("", ""))
}

func TestIsContentChanged(t *testing.T) {
	opts := DefaultComparisonOptions()

	// Identical files
	local := createTestFile("test.txt", 100, time.Now(), "etag123")
	remote := createTestFile("test.txt", 100, time.Now(), "etag123")
	assert.False(t, IsContentChanged(local, remote, opts))

	// Different ETags
	remote.ETag = "etag456"
	assert.True(t, IsContentChanged(local, remote, opts))

	// Different sizes
	remote.ETag = ""
	remote.Size = 200
	assert.True(t, IsContentChanged(local, remote, opts))

	// Different modification times
	remote.Size = 100
	remote.Modified = time.Now().Add(-2 * time.Hour) // Outside tolerance
	assert.True(t, IsContentChanged(local, remote, opts))

	// Within tolerance
	remote.Modified = time.Now().Add(-500 * time.Millisecond) // Within 1 second tolerance
	opts.IgnoreModTimeDiff = time.Second
	assert.False(t, IsContentChanged(local, remote, opts))
}

func TestShouldSkip(t *testing.T) {
	opts := &ComparisonOptions{IgnoreEmptyFiles: true}

	// Empty file should be skipped
	emptyFile := createTestFile("empty.txt", 0, time.Now(), "")
	assert.True(t, ShouldSkip(emptyFile, opts))

	// Non-empty file should not be skipped
	normalFile := createTestFile("normal.txt", 100, time.Now(), "etag")
	assert.False(t, ShouldSkip(normalFile, opts))

	// Directory should not be skipped
	dir := createTestDir("mydir", time.Now())
	assert.False(t, ShouldSkip(dir, opts))
}

func TestFilterChanges(t *testing.T) {
	changes := []*Change{
		{Type: ChangeCreate, Direction: LocalToRemote},
		{Type: ChangeUpdate, Direction: RemoteToLocal},
		{Type: ChangeCreate, Direction: LocalToRemote},
	}

	// Filter only create changes
	creates := FilterChanges(changes, func(c *Change) bool {
		return c.Type == ChangeCreate
	})
	assert.Len(t, creates, 2)

	// Filter only remote to local changes
	remoteToLocal := FilterChanges(changes, func(c *Change) bool {
		return c.Direction == RemoteToLocal
	})
	assert.Len(t, remoteToLocal, 1)
}

func TestGroupChangesByDirection(t *testing.T) {
	changes := []*Change{
		{Type: ChangeCreate, Direction: LocalToRemote},
		{Type: ChangeUpdate, Direction: RemoteToLocal},
		{Type: ChangeCreate, Direction: LocalToRemote},
		{Type: ChangeNone, Direction: DirectionNone},
	}

	grouped := GroupChangesByDirection(changes)
	assert.Len(t, grouped[LocalToRemote], 2)
	assert.Len(t, grouped[RemoteToLocal], 1)
	assert.Len(t, grouped[DirectionNone], 0) // Should not include ChangeNone
}

func TestSummarizeChanges(t *testing.T) {
	changes := []*Change{
		{Type: ChangeCreate},
		{Type: ChangeUpdate},
		{Type: ChangeCreate},
		{Type: ChangeDelete},
	}

	summary := SummarizeChanges(changes)
	assert.Equal(t, 2, summary[ChangeCreate])
	assert.Equal(t, 1, summary[ChangeUpdate])
	assert.Equal(t, 1, summary[ChangeDelete])
	assert.Equal(t, 0, summary[ChangeMove])
}

// Helper functions for creating test data
func createTestFile(path string, size int64, modTime time.Time, etag string) *FileMetadata {
	return &FileMetadata{
		Path:        path,
		Name:        filepath.Base(path),
		Size:        size,
		Modified:    modTime,
		ETag:        etag,
		IsDirectory: false,
	}
}

func createTestDir(path string, modTime time.Time) *FileMetadata {
	return &FileMetadata{
		Path:        path,
		Name:        filepath.Base(path),
		Size:        0,
		Modified:    modTime,
		ETag:        "",
		IsDirectory: true,
	}
}
