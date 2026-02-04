package sync

import (
	"fmt"
	"log"
	"os"
	"time"
)

// ConflictResolver handles the resolution of synchronization conflicts
type ConflictResolver struct {
	config      *SyncConfig
	logger      *log.Logger
	resolutions []*ConflictResolution
}

// NewConflictResolver creates a new conflict resolver
func NewConflictResolver(config *SyncConfig, logger *log.Logger) *ConflictResolver {
	if logger == nil {
		logger = log.New(os.Stdout, "[CONFLICT] ", log.LstdFlags)
	}

	return &ConflictResolver{
		config:      config,
		logger:      logger,
		resolutions: make([]*ConflictResolution, 0),
	}
}

// ResolveConflict resolves a single conflict using the configured policy
func (r *ConflictResolver) ResolveConflict(conflict *Conflict, sourceDirection ChangeDirection) (*ConflictResolution, error) {
	if conflict == nil {
		return nil, fmt.Errorf("conflict is nil")
	}

	var resolution ConflictResolution
	var err error

	// Use configured conflict policy or default to source_wins
	policy := r.config.ConflictPolicy
	if policy == "" {
		policy = "source_wins"
	}

	switch policy {
	case "source_wins":
		resolution, err = r.resolveSourceWins(conflict, sourceDirection)
	case "target_wins":
		resolution, err = r.resolveTargetWins(conflict, sourceDirection)
	case "skip":
		resolution, err = r.resolveSkip(conflict)
	case "manual":
		resolution, err = r.resolveManual(conflict)
	default:
		return nil, fmt.Errorf("unknown conflict policy: %s", policy)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to resolve conflict: %w", err)
	}

	// Log the resolution
	r.logResolution(conflict, &resolution)

	// Store the resolution
	r.resolutions = append(r.resolutions, &resolution)

	return &resolution, nil
}

// resolveSourceWins implements the source-wins conflict resolution policy
func (r *ConflictResolver) resolveSourceWins(conflict *Conflict, sourceDirection ChangeDirection) (ConflictResolution, error) {
	resolution := ConflictResolution{
		Timestamp: time.Now(),
		Reason:    fmt.Sprintf("source wins policy applied for %s conflict", conflict.Type.String()),
	}

	switch conflict.Type {
	case ConflictContentChanged:
		// Source file overwrites target file
		if sourceDirection == LocalToRemote {
			resolution.Action = "local_wins"
			resolution.Path = conflict.LocalPath
		} else {
			resolution.Action = "remote_wins"
			resolution.Path = conflict.RemotePath
		}

	case ConflictDeletedChanged:
		// Source changes win over target deletion
		if sourceDirection == LocalToRemote {
			if conflict.LocalMeta != nil {
				resolution.Action = "local_wins"
				resolution.Path = conflict.LocalPath
			} else {
				// Local deleted, remote changed - keep remote (target wins since source is gone)
				resolution.Action = "remote_wins"
				resolution.Path = conflict.RemotePath
			}
		} else {
			if conflict.RemoteMeta != nil {
				resolution.Action = "remote_wins"
				resolution.Path = conflict.RemotePath
			} else {
				// Remote deleted, local changed - keep local (target wins since source is gone)
				resolution.Action = "local_wins"
				resolution.Path = conflict.LocalPath
			}
		}

	case ConflictTypeChanged:
		// Type mismatch is more complex - prioritize directories over files for safety
		if conflict.LocalMeta != nil && conflict.LocalMeta.IsDirectory {
			resolution.Action = "local_wins"
			resolution.Path = conflict.LocalPath
			resolution.Reason = "directory prioritized over file in type conflict"
		} else if conflict.RemoteMeta != nil && conflict.RemoteMeta.IsDirectory {
			resolution.Action = "remote_wins"
			resolution.Path = conflict.RemotePath
			resolution.Reason = "directory prioritized over file in type conflict"
		} else {
			// Both are files, use direction-based resolution
			if sourceDirection == LocalToRemote {
				resolution.Action = "local_wins"
				resolution.Path = conflict.LocalPath
			} else {
				resolution.Action = "remote_wins"
				resolution.Path = conflict.RemotePath
			}
		}

	case ConflictPermissionDenied, ConflictStorageError:
		// These are error conditions, not true conflicts
		resolution.Action = "skip"
		resolution.Path = conflict.LocalPath
		if conflict.RemotePath != "" {
			resolution.Path = conflict.RemotePath
		}
		resolution.Reason = fmt.Sprintf("conflict skipped due to error: %s", conflict.Type.String())

	default:
		return ConflictResolution{}, fmt.Errorf("unsupported conflict type: %d", conflict.Type)
	}

	return resolution, nil
}

// resolveTargetWins implements the target-wins conflict resolution policy
func (r *ConflictResolver) resolveTargetWins(conflict *Conflict, sourceDirection ChangeDirection) (ConflictResolution, error) {
	resolution := ConflictResolution{
		Timestamp: time.Now(),
		Reason:    fmt.Sprintf("target wins policy applied for %s conflict", conflict.Type.String()),
	}

	switch conflict.Type {
	case ConflictContentChanged:
		// Target file overwrites source file
		if sourceDirection == LocalToRemote {
			resolution.Action = "remote_wins"
			resolution.Path = conflict.RemotePath
		} else {
			resolution.Action = "local_wins"
			resolution.Path = conflict.LocalPath
		}

	case ConflictDeletedChanged:
		// Target deletion wins over source changes
		if sourceDirection == LocalToRemote {
			if conflict.RemoteMeta != nil {
				resolution.Action = "remote_wins"
				resolution.Path = conflict.RemotePath
			} else {
				// Remote deleted, local changed - keep deletion
				resolution.Action = "skip"
				resolution.Path = conflict.LocalPath
			}
		} else {
			if conflict.LocalMeta != nil {
				resolution.Action = "local_wins"
				resolution.Path = conflict.LocalPath
			} else {
				// Local deleted, remote changed - keep deletion
				resolution.Action = "skip"
				resolution.Path = conflict.RemotePath
			}
		}

	case ConflictTypeChanged:
		// Prioritize directories for safety
		if conflict.LocalMeta != nil && conflict.LocalMeta.IsDirectory {
			resolution.Action = "local_wins"
			resolution.Path = conflict.LocalPath
			resolution.Reason = "directory prioritized over file in type conflict"
		} else if conflict.RemoteMeta != nil && conflict.RemoteMeta.IsDirectory {
			resolution.Action = "remote_wins"
			resolution.Path = conflict.RemotePath
			resolution.Reason = "directory prioritized over file in type conflict"
		} else {
			// Both are files, use opposite direction-based resolution
			if sourceDirection == LocalToRemote {
				resolution.Action = "remote_wins"
				resolution.Path = conflict.RemotePath
			} else {
				resolution.Action = "local_wins"
				resolution.Path = conflict.LocalPath
			}
		}

	default:
		return ConflictResolution{}, fmt.Errorf("unsupported conflict type: %d", conflict.Type)
	}

	return resolution, nil
}

// resolveSkip skips the conflict without making changes
func (r *ConflictResolver) resolveSkip(conflict *Conflict) (ConflictResolution, error) {
	return ConflictResolution{
		Action:    "skip",
		Path:      conflict.LocalPath,
		Timestamp: time.Now(),
		Reason:    fmt.Sprintf("conflict skipped due to policy for %s", conflict.Type.String()),
	}, nil
}

// resolveManual marks the conflict for manual resolution
func (r *ConflictResolver) resolveManual(conflict *Conflict) (ConflictResolution, error) {
	return ConflictResolution{
		Action:    "manual",
		Path:      conflict.LocalPath,
		Timestamp: time.Now(),
		Reason:    fmt.Sprintf("conflict requires manual resolution for %s", conflict.Type.String()),
	}, nil
}

// ResolveConflicts resolves multiple conflicts
func (r *ConflictResolver) ResolveConflicts(conflicts []*Conflict, sourceDirection ChangeDirection) ([]*ConflictResolution, error) {
	resolutions := make([]*ConflictResolution, 0, len(conflicts))

	for _, conflict := range conflicts {
		resolution, err := r.ResolveConflict(conflict, sourceDirection)
		if err != nil {
			r.logger.Printf("Failed to resolve conflict at %s: %v", conflict.LocalPath, err)
			continue
		}
		resolutions = append(resolutions, resolution)
	}

	return resolutions, nil
}

// GetResolutions returns all conflict resolutions made during this session
func (r *ConflictResolver) GetResolutions() []*ConflictResolution {
	return r.resolutions
}

// ClearResolutions clears the resolution history
func (r *ConflictResolver) ClearResolutions() {
	r.resolutions = make([]*ConflictResolution, 0)
}

// logResolution logs a conflict resolution
func (r *ConflictResolver) logResolution(conflict *Conflict, resolution *ConflictResolution) {
	r.logger.Printf("Conflict resolved: %s -> %s (%s) at %s",
		conflict.Type.String(),
		resolution.Action,
		resolution.Reason,
		resolution.Path,
	)
}

// String returns a string representation of the conflict type
func (ct ConflictType) String() string {
	switch ct {
	case ConflictNone:
		return "NONE"
	case ConflictContentChanged:
		return "CONTENT_CHANGED"
	case ConflictDeletedChanged:
		return "DELETED_CHANGED"
	case ConflictTypeChanged:
		return "TYPE_CHANGED"
	case ConflictPermissionDenied:
		return "PERMISSION_DENIED"
	case ConflictStorageError:
		return "STORAGE_ERROR"
	default:
		return "UNKNOWN"
	}
}

// IsResolvable returns true if the conflict can be automatically resolved
func (c *Conflict) IsResolvable() bool {
	switch c.Type {
	case ConflictContentChanged, ConflictDeletedChanged, ConflictTypeChanged:
		return true
	case ConflictPermissionDenied, ConflictStorageError:
		return false // These are errors, not true conflicts
	default:
		return false
	}
}

// GetSeverity returns the severity level of the conflict
func (c *Conflict) GetSeverity() string {
	switch c.Type {
	case ConflictTypeChanged:
		return "high" // Type conflicts are critical
	case ConflictContentChanged:
		return "medium" // Content conflicts need attention
	case ConflictDeletedChanged:
		return "medium" // Delete/change conflicts need attention
	case ConflictPermissionDenied, ConflictStorageError:
		return "high" // Error conditions are severe
	default:
		return "low"
	}
}

// RequiresUserIntervention returns true if the conflict needs user input
func (c *Conflict) RequiresUserIntervention() bool {
	return c.Type == ConflictPermissionDenied ||
		c.Type == ConflictStorageError ||
		(c.Type == ConflictTypeChanged &&
			c.LocalMeta != nil && c.RemoteMeta != nil &&
			c.LocalMeta.IsDirectory != c.RemoteMeta.IsDirectory)
}
