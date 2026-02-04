package sync

import (
	"testing"
	"time"
)

func TestFileMetadata_IsNewer(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name     string
		fm       *FileMetadata
		other    *FileMetadata
		expected bool
	}{
		{
			name: "newer file returns true",
			fm: &FileMetadata{
				Path:     "/test/file.txt",
				Modified: later,
			},
			other: &FileMetadata{
				Path:     "/test/file.txt",
				Modified: now,
			},
			expected: true,
		},
		{
			name: "older file returns false",
			fm: &FileMetadata{
				Path:     "/test/file.txt",
				Modified: now,
			},
			other: &FileMetadata{
				Path:     "/test/file.txt",
				Modified: later,
			},
			expected: false,
		},
		{
			name:     "nil file returns false",
			fm:       nil,
			other:    &FileMetadata{Modified: now},
			expected: false,
		},
		{
			name:     "nil other returns false",
			fm:       &FileMetadata{Modified: now},
			other:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fm.IsNewer(tt.other)
			if result != tt.expected {
				t.Errorf("FileMetadata.IsNewer() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFileMetadata_IsEqual(t *testing.T) {
	now := time.Now()
	opts := DefaultComparisonOptions()

	tests := []struct {
		name     string
		fm       *FileMetadata
		other    *FileMetadata
		opts     *ComparisonOptions
		expected bool
	}{
		{
			name: "identical files return true",
			fm: &FileMetadata{
				Path:     "/test/file.txt",
				Size:     1024,
				Modified: now,
				ETag:     "abc123",
			},
			other: &FileMetadata{
				Path:     "/test/file.txt",
				Size:     1024,
				Modified: now,
				ETag:     "abc123",
			},
			opts:     opts,
			expected: true,
		},
		{
			name: "different paths return false",
			fm: &FileMetadata{
				Path:     "/test/file1.txt",
				Size:     1024,
				Modified: now,
			},
			other: &FileMetadata{
				Path:     "/test/file2.txt",
				Size:     1024,
				Modified: now,
			},
			opts:     opts,
			expected: false,
		},
		{
			name: "different sizes return false when comparing size",
			fm: &FileMetadata{
				Path:     "/test/file.txt",
				Size:     1024,
				Modified: now,
			},
			other: &FileMetadata{
				Path:     "/test/file.txt",
				Size:     2048,
				Modified: now,
			},
			opts:     opts,
			expected: false,
		},
		{
			name: "different ETags return false when comparing ETags",
			fm: &FileMetadata{
				Path:     "/test/file.txt",
				Size:     1024,
				Modified: now,
				ETag:     "abc123",
			},
			other: &FileMetadata{
				Path:     "/test/file.txt",
				Size:     1024,
				Modified: now,
				ETag:     "def456",
			},
			opts:     opts,
			expected: false,
		},
		{
			name:     "nil files return false",
			fm:       nil,
			other:    &FileMetadata{Path: "/test/file.txt"},
			opts:     opts,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fm.IsEqual(tt.other, tt.opts)
			if result != tt.expected {
				t.Errorf("FileMetadata.IsEqual() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFileNode_AddChild(t *testing.T) {
	parent := &FileNode{
		Metadata: &FileMetadata{
			Path:        "/parent",
			Name:        "parent",
			IsDirectory: true,
		},
		Path: "/parent",
	}

	child := &FileNode{
		Metadata: &FileMetadata{
			Path:        "/parent/child.txt",
			Name:        "child.txt",
			IsDirectory: false,
		},
		Path: "/parent/child.txt",
	}

	parent.AddChild(child)

	if len(parent.Children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(parent.Children))
	}

	if parent.Children[0] != child {
		t.Errorf("Child not properly added")
	}

	if child.Parent != parent {
		t.Errorf("Child parent not properly set")
	}
}

func TestFileNode_FindChild(t *testing.T) {
	child1 := &FileNode{
		Metadata: &FileMetadata{Name: "child1.txt"},
		Path:     "/parent/child1.txt",
	}
	child2 := &FileNode{
		Metadata: &FileMetadata{Name: "child2.txt"},
		Path:     "/parent/child2.txt",
	}

	parent := &FileNode{
		Metadata: &FileMetadata{Name: "parent"},
		Path:     "/parent",
		Children: []*FileNode{child1, child2},
	}

	tests := []struct {
		name      string
		node      *FileNode
		childName string
		expected  *FileNode
	}{
		{
			name:      "find existing child",
			node:      parent,
			childName: "child1.txt",
			expected:  child1,
		},
		{
			name:      "find non-existing child returns nil",
			node:      parent,
			childName: "nonexistent.txt",
			expected:  nil,
		},
		{
			name:      "find in nil children returns nil",
			node:      &FileNode{Children: nil},
			childName: "anything.txt",
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.FindChild(tt.childName)
			if result != tt.expected {
				t.Errorf("FileNode.FindChild() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestChange_IsConflict(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		change   *Change
		expected bool
	}{
		{
			name: "update with differing modification times is conflict",
			change: &Change{
				Type: ChangeUpdate,
				LocalMeta: &FileMetadata{
					Modified: now,
					Size:     1024,
					ETag:     "abc123",
				},
				RemoteMeta: &FileMetadata{
					Modified: now.Add(time.Hour),
					Size:     1024,
					ETag:     "abc123",
				},
			},
			expected: true,
		},
		{
			name: "update with differing sizes is conflict",
			change: &Change{
				Type: ChangeUpdate,
				LocalMeta: &FileMetadata{
					Modified: now,
					Size:     1024,
					ETag:     "abc123",
				},
				RemoteMeta: &FileMetadata{
					Modified: now,
					Size:     2048,
					ETag:     "abc123",
				},
			},
			expected: true,
		},
		{
			name: "update with differing ETags is conflict",
			change: &Change{
				Type: ChangeUpdate,
				LocalMeta: &FileMetadata{
					Modified: now,
					Size:     1024,
					ETag:     "abc123",
				},
				RemoteMeta: &FileMetadata{
					Modified: now,
					Size:     1024,
					ETag:     "def456",
				},
			},
			expected: true,
		},
		{
			name: "create is not conflict",
			change: &Change{
				Type:       ChangeCreate,
				LocalMeta:  &FileMetadata{Modified: now},
				RemoteMeta: nil,
			},
			expected: false,
		},
		{
			name: "update with same metadata is not conflict",
			change: &Change{
				Type: ChangeUpdate,
				LocalMeta: &FileMetadata{
					Modified: now,
					Size:     1024,
					ETag:     "abc123",
				},
				RemoteMeta: &FileMetadata{
					Modified: now,
					Size:     1024,
					ETag:     "abc123",
				},
			},
			expected: false,
		},
		{
			name: "update with nil metadata is not conflict",
			change: &Change{
				Type:       ChangeUpdate,
				LocalMeta:  nil,
				RemoteMeta: &FileMetadata{Modified: now},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.change.IsConflict()
			if result != tt.expected {
				t.Errorf("Change.IsConflict() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestChange_String(t *testing.T) {
	tests := []struct {
		name     string
		change   Change
		expected string
	}{
		{
			name: "local to remote create",
			change: Change{
				Type:      ChangeCreate,
				Direction: LocalToRemote,
				Reason:    "new file",
			},
			expected: "CREATE → new file",
		},
		{
			name: "remote to local update",
			change: Change{
				Type:      ChangeUpdate,
				Direction: RemoteToLocal,
				Reason:    "content changed",
			},
			expected: "UPDATE ← content changed",
		},
		{
			name: "bidirectional move",
			change: Change{
				Type:      ChangeMove,
				Direction: Bidirectional,
				Reason:    "renamed",
			},
			expected: "MOVE ↔ renamed",
		},
		{
			name: "unknown direction",
			change: Change{
				Type:      ChangeDelete,
				Direction: DirectionNone,
				Reason:    "removed",
			},
			expected: "DELETE ? removed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.change.String()
			if result != tt.expected {
				t.Errorf("Change.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultComparisonOptions(t *testing.T) {
	opts := DefaultComparisonOptions()

	if opts.IgnoreModTimeDiff != time.Second {
		t.Errorf("Expected IgnoreModTimeDiff to be 1 second, got %v", opts.IgnoreModTimeDiff)
	}

	if !opts.CompareETags {
		t.Errorf("Expected CompareETags to be true")
	}

	if !opts.CompareSize {
		t.Errorf("Expected CompareSize to be true")
	}

	if opts.IgnoreEmptyFiles {
		t.Errorf("Expected IgnoreEmptyFiles to be false")
	}
}
