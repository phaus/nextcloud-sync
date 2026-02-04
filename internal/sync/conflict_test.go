package sync

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConflictResolver(t *testing.T) {
	config := &SyncConfig{
		ConflictPolicy: "source_wins",
	}

	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	resolver := NewConflictResolver(config, logger)

	require.NotNil(t, resolver)
	assert.Equal(t, config, resolver.config)
	assert.Equal(t, logger, resolver.logger)
	assert.Empty(t, resolver.resolutions)
}

func TestNewConflictResolver_NilLogger(t *testing.T) {
	config := &SyncConfig{
		ConflictPolicy: "source_wins",
	}

	resolver := NewConflictResolver(config, nil)

	require.NotNil(t, resolver)
	assert.NotNil(t, resolver.logger)
}

func TestResolveConflict_SourceWins(t *testing.T) {
	tests := []struct {
		name            string
		conflict        *Conflict
		sourceDirection ChangeDirection
		expectedAction  string
		expectedPath    string
	}{
		{
			name: "content conflict local wins",
			conflict: &Conflict{
				Type:       ConflictContentChanged,
				LocalPath:  "/local/file.txt",
				RemotePath: "/remote/file.txt",
				LocalMeta: &FileMetadata{
					Path:     "/local/file.txt",
					Size:     100,
					Modified: time.Now().Add(-time.Hour),
				},
				RemoteMeta: &FileMetadata{
					Path:     "/remote/file.txt",
					Size:     50,
					Modified: time.Now().Add(-2 * time.Hour),
				},
				Description: "content conflict",
				Timestamp:   time.Now(),
			},
			sourceDirection: LocalToRemote,
			expectedAction:  "local_wins",
			expectedPath:    "/local/file.txt",
		},
		{
			name: "content conflict remote wins",
			conflict: &Conflict{
				Type:       ConflictContentChanged,
				LocalPath:  "/local/file.txt",
				RemotePath: "/remote/file.txt",
				LocalMeta: &FileMetadata{
					Path:     "/local/file.txt",
					Size:     100,
					Modified: time.Now().Add(-time.Hour),
				},
				RemoteMeta: &FileMetadata{
					Path:     "/remote/file.txt",
					Size:     50,
					Modified: time.Now().Add(-2 * time.Hour),
				},
				Description: "content conflict",
				Timestamp:   time.Now(),
			},
			sourceDirection: RemoteToLocal,
			expectedAction:  "remote_wins",
			expectedPath:    "/remote/file.txt",
		},
		{
			name: "type conflict directory wins over file",
			conflict: &Conflict{
				Type:       ConflictTypeChanged,
				LocalPath:  "/local/item",
				RemotePath: "/remote/item",
				LocalMeta: &FileMetadata{
					Path:        "/local/item",
					IsDirectory: false,
				},
				RemoteMeta: &FileMetadata{
					Path:        "/remote/item",
					IsDirectory: true,
				},
				Description: "type mismatch: local is file, remote is directory",
				Timestamp:   time.Now(),
			},
			sourceDirection: LocalToRemote,
			expectedAction:  "remote_wins",
			expectedPath:    "/remote/item",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SyncConfig{
				ConflictPolicy: "source_wins",
			}
			resolver := NewConflictResolver(config, nil)

			resolution, err := resolver.ResolveConflict(tt.conflict, tt.sourceDirection)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedAction, resolution.Action)
			assert.Equal(t, tt.expectedPath, resolution.Path)
			assert.NotZero(t, resolution.Timestamp)
			assert.NotEmpty(t, resolution.Reason)
		})
	}
}

func TestResolveConflict_TargetWins(t *testing.T) {
	config := &SyncConfig{
		ConflictPolicy: "target_wins",
	}
	resolver := NewConflictResolver(config, nil)

	conflict := &Conflict{
		Type:       ConflictContentChanged,
		LocalPath:  "/local/file.txt",
		RemotePath: "/remote/file.txt",
		LocalMeta: &FileMetadata{
			Path:     "/local/file.txt",
			Size:     100,
			Modified: time.Now().Add(-time.Hour),
		},
		RemoteMeta: &FileMetadata{
			Path:     "/remote/file.txt",
			Size:     50,
			Modified: time.Now().Add(-2 * time.Hour),
		},
		Description: "content conflict",
		Timestamp:   time.Now(),
	}

	// When syncing from local to remote, target (remote) should win
	resolution, err := resolver.ResolveConflict(conflict, LocalToRemote)
	require.NoError(t, err)
	assert.Equal(t, "remote_wins", resolution.Action)
	assert.Equal(t, "/remote/file.txt", resolution.Path)
}

func TestResolveConflict_Skip(t *testing.T) {
	config := &SyncConfig{
		ConflictPolicy: "skip",
	}
	resolver := NewConflictResolver(config, nil)

	conflict := &Conflict{
		Type:        ConflictContentChanged,
		LocalPath:   "/local/file.txt",
		RemotePath:  "/remote/file.txt",
		Description: "content conflict",
		Timestamp:   time.Now(),
	}

	resolution, err := resolver.ResolveConflict(conflict, LocalToRemote)
	require.NoError(t, err)
	assert.Equal(t, "skip", resolution.Action)
	assert.Equal(t, "/local/file.txt", resolution.Path)
}

func TestResolveConflict_Manual(t *testing.T) {
	config := &SyncConfig{
		ConflictPolicy: "manual",
	}
	resolver := NewConflictResolver(config, nil)

	conflict := &Conflict{
		Type:        ConflictContentChanged,
		LocalPath:   "/local/file.txt",
		RemotePath:  "/remote/file.txt",
		Description: "content conflict",
		Timestamp:   time.Now(),
	}

	resolution, err := resolver.ResolveConflict(conflict, LocalToRemote)
	require.NoError(t, err)
	assert.Equal(t, "manual", resolution.Action)
	assert.Equal(t, "/local/file.txt", resolution.Path)
}

func TestResolveConflict_UnknownPolicy(t *testing.T) {
	config := &SyncConfig{
		ConflictPolicy: "unknown",
	}
	resolver := NewConflictResolver(config, nil)

	conflict := &Conflict{
		Type:        ConflictContentChanged,
		LocalPath:   "/local/file.txt",
		RemotePath:  "/remote/file.txt",
		Description: "content conflict",
		Timestamp:   time.Now(),
	}

	_, err := resolver.ResolveConflict(conflict, LocalToRemote)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown conflict policy")
}

func TestResolveConflict_NilConflict(t *testing.T) {
	config := &SyncConfig{
		ConflictPolicy: "source_wins",
	}
	resolver := NewConflictResolver(config, nil)

	_, err := resolver.ResolveConflict(nil, LocalToRemote)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conflict is nil")
}

func TestResolveConflicts_Multiple(t *testing.T) {
	config := &SyncConfig{
		ConflictPolicy: "source_wins",
	}
	resolver := NewConflictResolver(config, nil)

	conflicts := []*Conflict{
		{
			Type:        ConflictContentChanged,
			LocalPath:   "/local/file1.txt",
			RemotePath:  "/remote/file1.txt",
			Description: "content conflict 1",
			Timestamp:   time.Now(),
		},
		{
			Type:        ConflictTypeChanged,
			LocalPath:   "/local/file2.txt",
			RemotePath:  "/remote/file2.txt",
			Description: "type conflict",
			Timestamp:   time.Now(),
		},
	}

	resolutions, err := resolver.ResolveConflicts(conflicts, LocalToRemote)
	require.NoError(t, err)
	assert.Len(t, resolutions, 2)

	// Check that all resolutions were stored
	assert.Len(t, resolver.GetResolutions(), 2)
}

func TestGetResolutions_And_ClearResolutions(t *testing.T) {
	config := &SyncConfig{
		ConflictPolicy: "source_wins",
	}
	resolver := NewConflictResolver(config, nil)

	conflict := &Conflict{
		Type:        ConflictContentChanged,
		LocalPath:   "/local/file.txt",
		RemotePath:  "/remote/file.txt",
		Description: "content conflict",
		Timestamp:   time.Now(),
	}

	// Initially empty
	assert.Empty(t, resolver.GetResolutions())

	// Add a resolution
	_, err := resolver.ResolveConflict(conflict, LocalToRemote)
	require.NoError(t, err)
	assert.Len(t, resolver.GetResolutions(), 1)

	// Clear resolutions
	resolver.ClearResolutions()
	assert.Empty(t, resolver.GetResolutions())
}

func TestConflictType_String(t *testing.T) {
	tests := []struct {
		conflictType ConflictType
		expected     string
	}{
		{ConflictNone, "NONE"},
		{ConflictContentChanged, "CONTENT_CHANGED"},
		{ConflictDeletedChanged, "DELETED_CHANGED"},
		{ConflictTypeChanged, "TYPE_CHANGED"},
		{ConflictPermissionDenied, "PERMISSION_DENIED"},
		{ConflictStorageError, "STORAGE_ERROR"},
		{ConflictType(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.conflictType.String())
		})
	}
}

func TestConflict_IsResolvable(t *testing.T) {
	tests := []struct {
		conflictType ConflictType
		expected     bool
	}{
		{ConflictNone, false},
		{ConflictContentChanged, true},
		{ConflictDeletedChanged, true},
		{ConflictTypeChanged, true},
		{ConflictPermissionDenied, false},
		{ConflictStorageError, false},
	}

	for _, tt := range tests {
		t.Run(tt.conflictType.String(), func(t *testing.T) {
			conflict := &Conflict{Type: tt.conflictType}
			assert.Equal(t, tt.expected, conflict.IsResolvable())
		})
	}
}

func TestConflict_GetSeverity(t *testing.T) {
	tests := []struct {
		conflictType ConflictType
		expected     string
	}{
		{ConflictNone, "low"},
		{ConflictContentChanged, "medium"},
		{ConflictDeletedChanged, "medium"},
		{ConflictTypeChanged, "high"},
		{ConflictPermissionDenied, "high"},
		{ConflictStorageError, "high"},
	}

	for _, tt := range tests {
		t.Run(tt.conflictType.String(), func(t *testing.T) {
			conflict := &Conflict{Type: tt.conflictType}
			assert.Equal(t, tt.expected, conflict.GetSeverity())
		})
	}
}

func TestConflict_RequiresUserIntervention(t *testing.T) {
	tests := []struct {
		name     string
		conflict *Conflict
		expected bool
	}{
		{
			name: "permission denied requires intervention",
			conflict: &Conflict{
				Type: ConflictPermissionDenied,
			},
			expected: true,
		},
		{
			name: "storage error requires intervention",
			conflict: &Conflict{
				Type: ConflictStorageError,
			},
			expected: true,
		},
		{
			name: "type conflict with directory vs file requires intervention",
			conflict: &Conflict{
				Type: ConflictTypeChanged,
				LocalMeta: &FileMetadata{
					IsDirectory: false,
				},
				RemoteMeta: &FileMetadata{
					IsDirectory: true,
				},
			},
			expected: true,
		},
		{
			name: "content conflict does not require intervention",
			conflict: &Conflict{
				Type: ConflictContentChanged,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.conflict.RequiresUserIntervention())
		})
	}
}
