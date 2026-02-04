package sync

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// CompareFiles compares local and remote file metadata and returns the detected change
func CompareFiles(local, remote *FileMetadata, opts *ComparisonOptions) *Change {
	if opts == nil {
		opts = DefaultComparisonOptions()
	}

	// Handle nil cases
	if local == nil && remote == nil {
		return &Change{Type: ChangeNone, Reason: "both files nil"}
	}

	// Local exists, remote doesn't -> create remote
	if local != nil && remote == nil {
		return &Change{
			Type:      ChangeCreate,
			Direction: LocalToRemote,
			LocalMeta: local,
			LocalPath: local.Path,
			Reason:    "local file not found on remote",
			Priority:  calculatePriority(local),
		}
	}

	// Remote exists, local doesn't -> create local
	if local == nil && remote != nil {
		return &Change{
			Type:       ChangeCreate,
			Direction:  RemoteToLocal,
			RemoteMeta: remote,
			RemotePath: remote.Path,
			Reason:     "remote file not found locally",
			Priority:   calculatePriority(remote),
		}
	}

	// Both exist - compare for updates or conflicts
	change := compareExistingFiles(local, remote, opts)
	change.LocalPath = local.Path
	change.RemotePath = remote.Path
	return change
}

// compareExistingFiles compares two existing files and determines the appropriate change
func compareExistingFiles(local, remote *FileMetadata, opts *ComparisonOptions) *Change {
	// Check for conflicts first
	if conflict := detectConflict(local, remote); conflict != nil {
		return &Change{
			Type:       ChangeUpdate,
			Direction:  Bidirectional,
			LocalMeta:  local,
			RemoteMeta: remote,
			Reason:     fmt.Sprintf("conflict detected: %s", conflict.Description),
			Priority:   100, // High priority for conflicts
		}
	}

	// Check if files are equal
	if local.IsEqual(remote, opts) {
		return &Change{Type: ChangeNone, Reason: "files are identical"}
	}

	// Determine update direction based on modification time
	if local.IsNewer(remote) {
		return &Change{
			Type:       ChangeUpdate,
			Direction:  LocalToRemote,
			LocalMeta:  local,
			RemoteMeta: remote,
			Reason:     "local file is newer",
			Priority:   calculatePriority(local),
		}
	}

	if remote.IsNewer(local) {
		return &Change{
			Type:       ChangeUpdate,
			Direction:  RemoteToLocal,
			LocalMeta:  local,
			RemoteMeta: remote,
			Reason:     "remote file is newer",
			Priority:   calculatePriority(remote),
		}
	}

	// Same timestamp but different content (different ETags)
	return &Change{
		Type:       ChangeUpdate,
		Direction:  Bidirectional,
		LocalMeta:  local,
		RemoteMeta: remote,
		Reason:     "same timestamp but different content (ETag mismatch)",
		Priority:   50,
	}
}

// detectConflict detects if there's a conflict between local and remote files
func detectConflict(local, remote *FileMetadata) *Conflict {
	// Type mismatch (file vs directory)
	if local.IsDirectory != remote.IsDirectory {
		return &Conflict{
			Type:        ConflictTypeChanged,
			LocalMeta:   local,
			RemoteMeta:  remote,
			LocalPath:   local.Path,
			RemotePath:  remote.Path,
			Description: fmt.Sprintf("type mismatch: local is %s, remote is %s", getNodeType(local), getNodeType(remote)),
			Timestamp:   time.Now(),
		}
	}

	// Size mismatch with same modification time
	if local.Modified.Equal(remote.Modified) && local.Size != remote.Size {
		return &Conflict{
			Type:        ConflictContentChanged,
			LocalMeta:   local,
			RemoteMeta:  remote,
			LocalPath:   local.Path,
			RemotePath:  remote.Path,
			Description: "same modification time but different sizes",
			Timestamp:   time.Now(),
		}
	}

	// Both modified since last sync (within reasonable time window)
	timeDiff := local.Modified.Sub(remote.Modified)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	// If both files were modified recently and have different content
	recentThreshold := 5 * time.Minute
	if timeDiff < recentThreshold &&
		local.ETag != "" && remote.ETag != "" &&
		local.ETag != remote.ETag {
		return &Conflict{
			Type:        ConflictContentChanged,
			LocalMeta:   local,
			RemoteMeta:  remote,
			LocalPath:   local.Path,
			RemotePath:  remote.Path,
			Description: "both local and remote files modified recently with different content",
			Timestamp:   time.Now(),
		}
	}

	return nil
}

// DetectChanges compares two file trees and returns all detected changes and conflicts
func DetectChanges(localTree, remoteTree *FileTree, opts *ComparisonOptions) ([]*Change, []*Conflict) {
	if opts == nil {
		opts = DefaultComparisonOptions()
	}

	var changes []*Change
	var conflicts []*Conflict

	if localTree == nil && remoteTree == nil {
		return changes, conflicts
	}

	allPaths := make(map[string]bool)

	// Collect all paths
	if localTree != nil {
		for path := range localTree.PathMap {
			allPaths[path] = true
		}
	}
	if remoteTree != nil {
		for path := range remoteTree.PathMap {
			allPaths[path] = true
		}
	}

	// Compare each path
	for path := range allPaths {
		var localNode, remoteNode *FileNode
		if localTree != nil {
			localNode = localTree.PathMap[path]
		}
		if remoteTree != nil {
			remoteNode = remoteTree.PathMap[path]
		}

		var localMeta, remoteMeta *FileMetadata
		if localNode != nil {
			localMeta = localNode.Metadata
		}
		if remoteNode != nil {
			remoteMeta = remoteNode.Metadata
		}

		change := CompareFiles(localMeta, remoteMeta, opts)
		if change.Type != ChangeNone {
			changes = append(changes, change)

			// Extract conflicts from changes (only if both metadata exist)
			if localMeta != nil && remoteMeta != nil {
				if conflict := detectConflict(localMeta, remoteMeta); conflict != nil {
					conflicts = append(conflicts, conflict)
				}
			}
		}
	}

	return changes, conflicts
}

// calculatePriority calculates the sync priority for a file based on its metadata
func calculatePriority(meta *FileMetadata) int {
	if meta == nil {
		return 0
	}

	priority := 10 // Base priority

	// Larger files get higher priority (sync them first)
	if meta.Size > 0 {
		if meta.Size > 100*1024*1024 { // > 100MB
			priority += 30
		} else if meta.Size > 10*1024*1024 { // > 10MB
			priority += 20
		} else if meta.Size > 1024*1024 { // > 1MB
			priority += 10
		}
	}

	// Recently modified files get higher priority
	if time.Since(meta.Modified) < time.Hour {
		priority += 15
	} else if time.Since(meta.Modified) < 24*time.Hour {
		priority += 10
	}

	// Directories get lower priority (sync files first)
	if meta.IsDirectory {
		priority -= 5
	}

	// Root files get higher priority
	if filepath.Dir(meta.Path) == "." || filepath.Dir(meta.Path) == "/" {
		priority += 5
	}

	return priority
}

// CompareETagsSafely compares ETags with proper normalization
func CompareETagsSafely(localETag, remoteETag string) bool {
	// Handle empty ETags
	if localETag == "" || remoteETag == "" {
		return false // Cannot determine equality without both ETags
	}

	// Normalize ETags by removing quotes and whitespace
	normalize := func(etag string) string {
		return strings.TrimSpace(strings.Trim(etag, `"`))
	}

	return normalize(localETag) == normalize(remoteETag)
}

// IsContentChanged determines if file content has changed based on available metadata
func IsContentChanged(local, remote *FileMetadata, opts *ComparisonOptions) bool {
	if local == nil || remote == nil {
		return true // Consider as change if one doesn't exist
	}

	if opts == nil {
		opts = DefaultComparisonOptions()
	}

	// Primary check: ETags if available and enabled
	if opts.CompareETags && local.ETag != "" && remote.ETag != "" {
		return !CompareETagsSafely(local.ETag, remote.ETag)
	}

	// Secondary check: Size if enabled
	if opts.CompareSize && local.Size != remote.Size {
		return true
	}

	// Tertiary check: Modification time outside tolerance
	modDiff := local.Modified.Sub(remote.Modified)
	if modDiff < -opts.IgnoreModTimeDiff || modDiff > opts.IgnoreModTimeDiff {
		return true
	}

	return false
}

// getNodeType returns a string representation of the node type
func getNodeType(fm *FileMetadata) string {
	if fm.IsDirectory {
		return "directory"
	}
	return "file"
}

// ShouldSkip determines if a file should be skipped based on comparison options
func ShouldSkip(meta *FileMetadata, opts *ComparisonOptions) bool {
	if meta == nil || opts == nil {
		return false
	}

	// Skip empty files if option is enabled
	if opts.IgnoreEmptyFiles && meta.Size == 0 && !meta.IsDirectory {
		return true
	}

	return false
}

// FilterChanges filters changes based on various criteria
func FilterChanges(changes []*Change, filterFunc func(*Change) bool) []*Change {
	var filtered []*Change
	for _, change := range changes {
		if filterFunc(change) {
			filtered = append(filtered, change)
		}
	}
	return filtered
}

// GetConflictsFromChanges extracts conflicts from a list of changes
func GetConflictsFromChanges(changes []*Change) []*Conflict {
	var conflicts []*Conflict
	seen := make(map[string]bool) // Avoid duplicate conflicts

	for _, change := range changes {
		if change.IsConflict() && change.LocalMeta != nil && change.RemoteMeta != nil {
			key := change.LocalMeta.Path + ":" + change.RemoteMeta.Path
			if !seen[key] {
				conflict := detectConflict(change.LocalMeta, change.RemoteMeta)
				if conflict != nil {
					conflicts = append(conflicts, conflict)
					seen[key] = true
				}
			}
		}
	}

	return conflicts
}

// ChangesByPriority sorts changes by priority (highest first)
type ChangesByPriority []*Change

func (c ChangesByPriority) Len() int           { return len(c) }
func (c ChangesByPriority) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c ChangesByPriority) Less(i, j int) bool { return c[i].Priority > c[j].Priority }

// GroupChangesByDirection groups changes by their sync direction
func GroupChangesByDirection(changes []*Change) map[ChangeDirection][]*Change {
	grouped := make(map[ChangeDirection][]*Change)
	for _, change := range changes {
		if change.Type != ChangeNone {
			grouped[change.Direction] = append(grouped[change.Direction], change)
		}
	}
	return grouped
}

// SummarizeChanges provides a summary of detected changes
func SummarizeChanges(changes []*Change) map[ChangeType]int {
	summary := make(map[ChangeType]int)
	for _, change := range changes {
		summary[change.Type]++
	}
	return summary
}
