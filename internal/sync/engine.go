package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/phaus/nextcloud-sync/internal/webdav"
	"github.com/phaus/nextcloud-sync/pkg/exclude"
)

// SyncEngine coordinates the overall synchronization process
type SyncEngine struct {
	webdavClient   webdav.Client
	config         *SyncConfig
	excludeMatcher *exclude.Matcher
}

// NewSyncEngine creates a new sync engine
func NewSyncEngine(client webdav.Client, config *SyncConfig) (*SyncEngine, error) {
	// Create exclude matcher from config patterns
	matcher, err := createExcludeMatcher(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create exclude matcher: %w", err)
	}

	return &SyncEngine{
		webdavClient:   client,
		config:         config,
		excludeMatcher: matcher,
	}, nil
}

// createExcludeMatcher creates an exclude matcher from sync configuration
func createExcludeMatcher(config *SyncConfig) (*exclude.Matcher, error) {
	patternSet := exclude.NewPatternSet()

	// Load default patterns
	defaultPatterns := exclude.LoadDefaultPatterns()
	patternSet.Merge(defaultPatterns)

	// Add patterns from config
	for _, pattern := range config.ExcludePatterns {
		if err := patternSet.AddPattern(pattern); err != nil {
			return nil, fmt.Errorf("invalid exclude pattern '%s': %w", pattern, err)
		}
	}

	// Try to load patterns from .nextcloudignore file if source is local
	if strings.HasPrefix(config.Source, "/") || !strings.Contains(config.Source, "://") {
		if localPatterns, err := exclude.LoadFromFile(config.Source); err == nil {
			patternSet.Merge(localPatterns)
		}
		// Ignore errors for .nextcloudignore file - it's optional
	}

	return exclude.NewMatcherWithRoot(patternSet, config.Source), nil
}

// BuildLocalFileTree builds a file tree from the local source directory
func (se *SyncEngine) BuildLocalFileTree(ctx context.Context) (*FileTree, error) {
	tree := &FileTree{
		PathMap: make(map[string]*FileNode),
	}

	// Use exclude matcher to walk the directory
	err := se.excludeMatcher.Walk(se.config.Source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from source
		relPath, err := filepath.Rel(se.config.Source, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}
		if relPath == "." {
			relPath = ""
		}

		// Create metadata
		metadata := &FileMetadata{
			Path:        relPath,
			Name:        info.Name(),
			Size:        info.Size(),
			Modified:    info.ModTime(),
			IsDirectory: info.IsDir(),
		}

		// Create node
		node := &FileNode{
			Metadata: metadata,
			Path:     relPath,
		}

		// Add to tree
		tree.PathMap[relPath] = node

		// Set as root if this is the source directory
		if relPath == "" {
			tree.Root = node
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to build local file tree: %w", err)
	}

	// Build parent-child relationships
	se.buildTreeRelationships(tree)

	return tree, nil
}

// BuildRemoteFileTree builds a file tree from the remote target
func (se *SyncEngine) BuildRemoteFileTree(ctx context.Context) (*FileTree, error) {
	tree := &FileTree{
		PathMap: make(map[string]*FileNode),
	}

	// Extract directory path from remote URL
	remotePath := se.extractRemotePath(se.config.Target)

	// List remote directory recursively
	err := se.listRemoteDirectory(ctx, remotePath, "", tree)
	if err != nil {
		return nil, fmt.Errorf("failed to build remote file tree: %w", err)
	}

	// Build parent-child relationships
	se.buildTreeRelationships(tree)

	return tree, nil
}

// listRemoteDirectory recursively lists remote directory and builds file tree
func (se *SyncEngine) listRemoteDirectory(ctx context.Context, basePath, currentPath string, tree *FileTree) error {
	// Construct full path
	fullPath := basePath
	if currentPath != "" {
		fullPath = basePath + "/" + currentPath
	}

	// List directory contents
	files, err := se.webdavClient.ListDirectory(ctx, fullPath)
	if err != nil {
		return fmt.Errorf("failed to list remote directory %s: %w", fullPath, err)
	}

	for _, file := range files {
		// Check if file should be excluded
		relPath := currentPath
		if relPath != "" {
			relPath += "/" + file.Name
		} else {
			relPath = file.Name
		}

		if se.excludeMatcher.ShouldExclude(relPath, file.IsDirectory) {
			continue // Skip excluded files/directories
		}

		// Create metadata
		metadata := &FileMetadata{
			Path:        relPath,
			Name:        file.Name,
			Size:        file.Size,
			Modified:    file.LastModified,
			ETag:        file.ETag,
			IsDirectory: file.IsDirectory,
		}

		// Create node
		node := &FileNode{
			Metadata: metadata,
			Path:     relPath,
		}

		// Add to tree
		tree.PathMap[relPath] = node

		// Set as root if this is the base directory
		if currentPath == "" && file.Name == filepath.Base(basePath) {
			tree.Root = node
		}

		// Recursively list subdirectories
		if file.IsDirectory {
			err := se.listRemoteDirectory(ctx, basePath, relPath, tree)
			if err != nil {
				return fmt.Errorf("failed to list subdirectory %s: %w", relPath, err)
			}
		}
	}

	return nil
}

// buildTreeRelationships builds parent-child relationships in the file tree
func (se *SyncEngine) buildTreeRelationships(tree *FileTree) {
	for path, node := range tree.PathMap {
		if path == "" {
			continue // Skip root
		}

		parentPath := filepath.Dir(path)
		if parentPath == "." {
			parentPath = ""
		}

		if parentNode, exists := tree.PathMap[parentPath]; exists {
			parentNode.AddChild(node)
		}
	}
}

// extractRemotePath extracts directory path from remote URL
func (se *SyncEngine) extractRemotePath(remoteURL string) string {
	// This is a simplified implementation
	// In a full implementation, this would parse the Nextcloud URL properly
	if strings.Contains(remoteURL, "?dir=") {
		parts := strings.Split(remoteURL, "?dir=")
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return "/"
}

// Sync performs the synchronization between source and target
func (se *SyncEngine) Sync(ctx context.Context) (*SyncResult, error) {
	startTime := time.Now()

	// Build file trees
	var localTree, remoteTree *FileTree
	var err error

	if se.isLocalSource() {
		localTree, err = se.BuildLocalFileTree(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to build local file tree: %w", err)
		}
	}

	if se.isRemoteTarget() {
		remoteTree, err = se.BuildRemoteFileTree(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to build remote file tree: %w", err)
		}
	}

	// Perform bidirectional sync if configured
	isBidirectional := se.config.Bidirectional || se.config.Direction == SyncDirectionBidirectional
	if isBidirectional {
		return se.performBidirectionalSync(ctx, localTree, remoteTree, startTime)
	}

	// Perform unidirectional sync (original logic)
	return se.performUnidirectionalSync(ctx, localTree, remoteTree, startTime)
}

// filterExcludedChanges removes changes for excluded files
func (se *SyncEngine) filterExcludedChanges(changes []*Change) []*Change {
	var filtered []*Change
	for _, change := range changes {
		// Check if local file should be excluded
		if change.LocalPath != "" && se.excludeMatcher.ShouldExcludeFile(change.LocalPath) {
			continue
		}

		// Check if remote file should be excluded
		if change.RemotePath != "" && se.excludeMatcher.ShouldExcludeFile(change.RemotePath) {
			continue
		}

		filtered = append(filtered, change)
	}
	return filtered
}

// isLocalSource checks if source is a local path
func (se *SyncEngine) isLocalSource() bool {
	return !strings.Contains(se.config.Source, "://")
}

// isRemoteTarget checks if target is a remote URL
func (se *SyncEngine) isRemoteTarget() bool {
	return strings.Contains(se.config.Target, "://")
}

// GetExcludeMatcher returns the exclude matcher for testing
func (se *SyncEngine) GetExcludeMatcher() *exclude.Matcher {
	return se.excludeMatcher
}

// performBidirectionalSync handles two-way synchronization between local and remote
func (se *SyncEngine) performBidirectionalSync(ctx context.Context, localTree, remoteTree *FileTree, startTime time.Time) (*SyncResult, error) {
	// Detect changes in both directions
	localChanges, localConflicts := DetectChanges(localTree, remoteTree, DefaultComparisonOptions())
	remoteChanges, remoteConflicts := DetectChanges(remoteTree, localTree, DefaultComparisonOptions())

	// Combine all changes and conflicts
	allChanges := append(localChanges, remoteChanges...)
	allConflicts := append(localConflicts, remoteConflicts...)

	// Filter out excluded files from changes
	filteredChanges := se.filterExcludedChanges(allChanges)

	// Create operation plan for bidirectional sync
	executor := NewOperationExecutor(se.webdavClient, se.config)
	plan, err := se.createBidirectionalPlan(executor, filteredChanges)
	if err != nil {
		return nil, fmt.Errorf("failed to create bidirectional sync plan: %w", err)
	}

	// Add conflicts to plan
	plan.Conflicts = append(plan.Conflicts, allConflicts...)

	// Execute plan if not dry run
	if se.config.DryRun {
		return &SyncResult{
			Success:       true,
			TotalFiles:    plan.TotalFiles,
			TotalSize:     plan.TotalSize,
			Conflicts:     plan.Conflicts,
			Warnings:      plan.Warnings,
			StartTime:     startTime,
			EndTime:       time.Now(),
			Duration:      0,
			DryRun:        true,
			Bidirectional: false,
		}, nil
	}

	result, err := executor.ExecutePlan(plan)
	if err != nil {
		return nil, fmt.Errorf("failed to execute sync plan: %w", err)
	}

	// Update result with timing info
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)
	result.DryRun = se.config.DryRun
	result.Bidirectional = false

	return result, nil
}

// performUnidirectionalSync handles one-way synchronization (original logic)
func (se *SyncEngine) performUnidirectionalSync(ctx context.Context, localTree, remoteTree *FileTree, startTime time.Time) (*SyncResult, error) {
	// Detect changes
	changes, conflicts := DetectChanges(localTree, remoteTree, DefaultComparisonOptions())

	// Filter out excluded files from changes
	filteredChanges := se.filterExcludedChanges(changes)

	// Create operation plan
	executor := NewOperationExecutor(se.webdavClient, se.config)
	plan, err := executor.PlanOperations(filteredChanges)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync plan: %w", err)
	}

	// Add conflicts to plan
	plan.Conflicts = append(plan.Conflicts, conflicts...)

	// Execute plan if not dry run
	if se.config.DryRun {
		return &SyncResult{
			Success:       true,
			TotalFiles:    plan.TotalFiles,
			TotalSize:     plan.TotalSize,
			Conflicts:     plan.Conflicts,
			Warnings:      plan.Warnings,
			StartTime:     startTime,
			EndTime:       time.Now(),
			Duration:      0,
			DryRun:        true,
			Bidirectional: true,
		}, nil
	}

	result, err := executor.ExecutePlan(plan)
	if err != nil {
		return nil, fmt.Errorf("failed to execute sync plan: %w", err)
	}

	// Update result with timing info
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)
	result.Bidirectional = false

	return result, nil
}

// createBidirectionalPlan creates an operation plan optimized for bidirectional sync
func (se *SyncEngine) createBidirectionalPlan(executor *OperationExecutor, changes []*Change) (*SyncPlan, error) {
	plan := &SyncPlan{
		Operations: make([]*SyncOperation, 0),
		Conflicts:  make([]*Conflict, 0),
		Warnings:   make([]string, 0),
	}

	// Group changes by direction for optimal execution
	localToRemote := make([]*Change, 0)
	remoteToLocal := make([]*Change, 0)

	for _, change := range changes {
		switch change.Direction {
		case LocalToRemote:
			localToRemote = append(localToRemote, change)
		case RemoteToLocal:
			remoteToLocal = append(remoteToLocal, change)
		case DirectionNone:
			// No action needed
			continue
		}
	}

	// Plan local to remote operations first (typically uploads are faster to start)
	for _, change := range localToRemote {
		ops, err := executor.planChange(change)
		if err != nil {
			return nil, fmt.Errorf("failed to plan local to remote change %s: %w", change.Path(), err)
		}
		plan.Operations = append(plan.Operations, ops...)
	}

	// Plan remote to local operations (downloads)
	for _, change := range remoteToLocal {
		ops, err := executor.planChange(change)
		if err != nil {
			return nil, fmt.Errorf("failed to plan remote to local change %s: %w", change.Path(), err)
		}
		plan.Operations = append(plan.Operations, ops...)
	}

	// Calculate total files and size
	for _, op := range plan.Operations {
		plan.TotalFiles++
		if op.Size > 0 {
			plan.TotalSize += op.Size
		}
	}

	return plan, nil
}

// SetExcludeMatcher sets a custom exclude matcher for testing
func (se *SyncEngine) SetExcludeMatcher(matcher *exclude.Matcher) {
	se.excludeMatcher = matcher
}
