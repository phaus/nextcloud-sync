package sync

import (
	"time"
)

// FileMetadata represents the metadata for a file or directory
type FileMetadata struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	Modified    time.Time `json:"modified"`
	ETag        string    `json:"etag"`
	IsDirectory bool      `json:"is_directory"`
	Permissions string    `json:"permissions,omitempty"`
	ContentType string    `json:"content_type,omitempty"`
}

// ChangeType represents the type of change detected
type ChangeType int

const (
	ChangeNone ChangeType = iota
	ChangeCreate
	ChangeUpdate
	ChangeDelete
	ChangeMove
)

// ChangeDirection represents the direction of synchronization
type ChangeDirection int

const (
	DirectionNone ChangeDirection = iota
	LocalToRemote
	RemoteToLocal
	Bidirectional
)

// Change represents a detected change between local and remote files
type Change struct {
	Type       ChangeType      `json:"type"`
	Direction  ChangeDirection `json:"direction"`
	LocalPath  string          `json:"local_path,omitempty"`
	RemotePath string          `json:"remote_path,omitempty"`
	LocalMeta  *FileMetadata   `json:"local_meta,omitempty"`
	RemoteMeta *FileMetadata   `json:"remote_meta,omitempty"`
	Reason     string          `json:"reason"`
	Priority   int             `json:"priority"` // Higher numbers = higher priority
}

// ConflictType represents the type of conflict detected
type ConflictType int

const (
	ConflictNone             ConflictType = iota
	ConflictContentChanged                // Both local and remote changed
	ConflictDeletedChanged                // One side deleted, other side modified
	ConflictTypeChanged                   // File vs directory type mismatch
	ConflictPermissionDenied              // Cannot access due to permissions
	ConflictStorageError                  // Insufficient storage or other storage error
)

// Conflict represents a synchronization conflict
type Conflict struct {
	Type        ConflictType       `json:"type"`
	LocalPath   string             `json:"local_path,omitempty"`
	RemotePath  string             `json:"remote_path,omitempty"`
	LocalMeta   *FileMetadata      `json:"local_meta,omitempty"`
	RemoteMeta  *FileMetadata      `json:"remote_meta,omitempty"`
	Description string             `json:"description"`
	Timestamp   time.Time          `json:"timestamp"`
	Resolution  ConflictResolution `json:"resolution,omitempty"`
}

// ConflictResolution represents how a conflict was resolved
type ConflictResolution struct {
	Action    string    `json:"action"` // "local_wins", "remote_wins", "skip", "manual"
	Path      string    `json:"path"`   // Final path that was kept
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
}

// SyncDirection represents the overall sync direction
type SyncDirection int

const (
	SyncDirectionLocalToRemote SyncDirection = iota
	SyncDirectionRemoteToLocal
	SyncDirectionBidirectional
)

// SyncConfig represents the configuration for a sync operation
type SyncConfig struct {
	Source          string          `json:"source"`
	Target          string          `json:"target"`
	Direction       SyncDirection   `json:"direction"`
	DryRun          bool            `json:"dry_run"`
	Force           bool            `json:"force"`
	ExcludePatterns []string        `json:"exclude_patterns,omitempty"`
	MaxRetries      int             `json:"max_retries"`
	Timeout         time.Duration   `json:"timeout"`
	ChunkSize       int64           `json:"chunk_size"`
	ConflictPolicy  string          `json:"conflict_policy"` // "source_wins", "target_wins", "skip"
	ProgressTracker ProgressTracker `json:"-"`
}

// ProgressTracker interface for tracking sync progress
type ProgressTracker interface {
	Start(total int64)
	Update(current int64)
	Finish()
	SetOperation(operation string)
	Error(err error)
}

// SyncOperation represents a single sync operation to be performed
type SyncOperation struct {
	ID           string          `json:"id"`
	Type         ChangeType      `json:"type"`
	Direction    ChangeDirection `json:"direction"`
	SourcePath   string          `json:"source_path"`
	TargetPath   string          `json:"target_path"`
	Size         int64           `json:"size"`
	Priority     int             `json:"priority"`
	Dependencies []string        `json:"dependencies,omitempty"` // IDs of operations that must complete first
}

// SyncPlan represents the complete plan for a sync operation
type SyncPlan struct {
	Operations    []*SyncOperation `json:"operations"`
	TotalFiles    int              `json:"total_files"`
	TotalSize     int64            `json:"total_size"`
	EstimatedTime time.Duration    `json:"estimated_time"`
	CreatedAt     time.Time        `json:"created_at"`
	Conflicts     []*Conflict      `json:"conflicts,omitempty"`
	Warnings      []string         `json:"warnings,omitempty"`
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Success         bool          `json:"success"`
	TotalFiles      int           `json:"total_files"`
	ProcessedFiles  int           `json:"processed_files"`
	TotalSize       int64         `json:"total_size"`
	TransferredSize int64         `json:"transferred_size"`
	Duration        time.Duration `json:"duration"`
	Errors          []string      `json:"errors,omitempty"`
	Warnings        []string      `json:"warnings,omitempty"`
	SkippedFiles    []string      `json:"skipped_files,omitempty"`
	Conflicts       []*Conflict   `json:"conflicts,omitempty"`
	CreatedFiles    []string      `json:"created_files,omitempty"`
	UpdatedFiles    []string      `json:"updated_files,omitempty"`
	DeletedFiles    []string      `json:"deleted_files,omitempty"`
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
}

// FileTree represents a tree structure for file metadata
type FileTree struct {
	Root    *FileNode            `json:"root"`
	PathMap map[string]*FileNode `json:"path_map"`
}

// FileNode represents a node in the file tree
type FileNode struct {
	Metadata *FileMetadata `json:"metadata"`
	Parent   *FileNode     `json:"-"`
	Children []*FileNode   `json:"children,omitempty"`
	Path     string        `json:"path"`
}

// ComparisonOptions controls how files are compared
type ComparisonOptions struct {
	IgnoreModTimeDiff time.Duration `json:"ignore_mod_time_diff"`
	CompareETags      bool          `json:"compare_etags"`
	CompareSize       bool          `json:"compare_size"`
	IgnoreEmptyFiles  bool          `json:"ignore_empty_files"`
}

// DefaultComparisonOptions returns sensible defaults for file comparison
func DefaultComparisonOptions() *ComparisonOptions {
	return &ComparisonOptions{
		IgnoreModTimeDiff: time.Second, // 1 second tolerance
		CompareETags:      true,
		CompareSize:       true,
		IgnoreEmptyFiles:  false,
	}
}

// IsNewer returns true if file a is newer than file b
func (fm *FileMetadata) IsNewer(other *FileMetadata) bool {
	if fm == nil || other == nil {
		return false
	}
	return fm.Modified.After(other.Modified)
}

// IsEqual returns true if two files have identical metadata
func (fm *FileMetadata) IsEqual(other *FileMetadata, opts *ComparisonOptions) bool {
	if fm == nil || other == nil {
		return false
	}

	if fm.Path != other.Path || fm.IsDirectory != other.IsDirectory {
		return false
	}

	// Check size difference
	if opts.CompareSize && fm.Size != other.Size {
		return false
	}

	// Check modification time within tolerance
	modDiff := fm.Modified.Sub(other.Modified)
	if modDiff < -opts.IgnoreModTimeDiff || modDiff > opts.IgnoreModTimeDiff {
		return false
	}

	// Check ETags if enabled
	if opts.CompareETags && fm.ETag != "" && other.ETag != "" {
		return fm.ETag == other.ETag
	}

	return true
}

// AddChild adds a child node to a file tree node
func (fn *FileNode) AddChild(child *FileNode) {
	if fn.Children == nil {
		fn.Children = make([]*FileNode, 0)
	}
	fn.Children = append(fn.Children, child)
	child.Parent = fn
}

// FindChild finds a child node by name
func (fn *FileNode) FindChild(name string) *FileNode {
	if fn.Children == nil {
		return nil
	}
	for _, child := range fn.Children {
		if child.Metadata.Name == name {
			return child
		}
	}
	return nil
}

// IsConflict returns true if the change represents a conflict
func (c *Change) IsConflict() bool {
	return c.Type == ChangeUpdate &&
		((c.LocalMeta != nil && c.RemoteMeta != nil) &&
			(c.LocalMeta.Modified != c.RemoteMeta.Modified ||
				c.LocalMeta.Size != c.RemoteMeta.Size ||
				c.LocalMeta.ETag != c.RemoteMeta.ETag))
}

// String returns a string representation of the change
func (c Change) String() string {
	var dirStr string
	switch c.Direction {
	case LocalToRemote:
		dirStr = "→"
	case RemoteToLocal:
		dirStr = "←"
	case Bidirectional:
		dirStr = "↔"
	default:
		dirStr = "?"
	}

	var typeStr string
	switch c.Type {
	case ChangeCreate:
		typeStr = "CREATE"
	case ChangeUpdate:
		typeStr = "UPDATE"
	case ChangeDelete:
		typeStr = "DELETE"
	case ChangeMove:
		typeStr = "MOVE"
	default:
		typeStr = "UNKNOWN"
	}

	return typeStr + " " + dirStr + " " + c.Reason
}

// String returns a string representation of the ChangeType
func (ct ChangeType) String() string {
	switch ct {
	case ChangeNone:
		return "NONE"
	case ChangeCreate:
		return "CREATE"
	case ChangeUpdate:
		return "UPDATE"
	case ChangeDelete:
		return "DELETE"
	case ChangeMove:
		return "MOVE"
	default:
		return "UNKNOWN"
	}
}
