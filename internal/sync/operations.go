package sync

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/phaus/nextcloud-sync/internal/webdav"
)

// OperationExecutor handles the execution of sync operations
type OperationExecutor struct {
	webdavClient webdav.Client
	config       *SyncConfig
	ctx          context.Context
}

// NewOperationExecutor creates a new operation executor
func NewOperationExecutor(client webdav.Client, config *SyncConfig) *OperationExecutor {
	return &OperationExecutor{
		webdavClient: client,
		config:       config,
		ctx:          context.Background(),
	}
}

// ExecuteOperation executes a single sync operation
func (e *OperationExecutor) ExecuteOperation(op *SyncOperation) error {
	if e.config.ProgressTracker != nil {
		e.config.ProgressTracker.SetOperation(fmt.Sprintf("%s %s", op.Type.String(), op.SourcePath))
	}

	switch op.Type {
	case ChangeCreate:
		if op.Direction == LocalToRemote {
			return e.uploadFile(op.SourcePath, op.TargetPath)
		} else if op.Direction == RemoteToLocal {
			return e.downloadFile(op.SourcePath, op.TargetPath)
		} else {
			return fmt.Errorf("unsupported direction for create operation: %v", op.Direction)
		}
	case ChangeUpdate:
		if op.Direction == LocalToRemote {
			return e.uploadFile(op.SourcePath, op.TargetPath)
		} else if op.Direction == RemoteToLocal {
			return e.downloadFile(op.SourcePath, op.TargetPath)
		} else {
			return fmt.Errorf("unsupported direction for update operation: %v", op.Direction)
		}
	case ChangeDelete:
		if op.Direction == LocalToRemote {
			return e.deleteLocalFile(op.SourcePath)
		} else if op.Direction == RemoteToLocal {
			return e.deleteRemoteFile(op.SourcePath)
		} else {
			return fmt.Errorf("unsupported direction for delete operation: %v", op.Direction)
		}
	case ChangeMove:
		if op.Direction == LocalToRemote {
			return e.moveLocalFile(op.SourcePath, op.TargetPath)
		} else if op.Direction == RemoteToLocal {
			return e.moveRemoteFile(op.SourcePath, op.TargetPath)
		} else {
			return fmt.Errorf("unsupported direction for move operation: %v", op.Direction)
		}
	default:
		return fmt.Errorf("unsupported operation type: %v", op.Type)
	}
}

// uploadFile uploads a local file to the remote WebDAV server
func (e *OperationExecutor) uploadFile(localPath, remotePath string) error {
	// Open local file
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file %s: %w", localPath, err)
	}
	defer file.Close()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file %s: %w", localPath, err)
	}

	// Update progress tracker
	if e.config.ProgressTracker != nil {
		e.config.ProgressTracker.Start(fileInfo.Size())
	}

	// Create remote directory if it doesn't exist
	remoteDir := filepath.Dir(remotePath)
	if remoteDir != "." && remoteDir != "/" {
		if err := e.ensureRemoteDirectory(remoteDir); err != nil {
			return fmt.Errorf("failed to create remote directory %s: %w", remoteDir, err)
		}
	}

	// Determine if we should use chunked upload
	chunkSize := e.config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 1024 * 1024 // Default to 1MB
	}

	largeFileThreshold := e.config.LargeFileThreshold
	if largeFileThreshold <= 0 {
		largeFileThreshold = 50 * 1024 * 1024 // Default to 50MB
	}

	// Upload file with appropriate method
	if fileInfo.Size() > largeFileThreshold {
		// Use chunked upload for large files
		progressReader := &progressReader{
			reader:    file,
			tracker:   e.config.ProgressTracker,
			totalSize: fileInfo.Size(),
		}

		err = e.webdavClient.UploadFileChunked(e.ctx, remotePath, progressReader, fileInfo.Size(), chunkSize)
		if err != nil {
			return fmt.Errorf("failed to upload file (chunked) to %s: %w", remotePath, err)
		}
	} else {
		// Use regular upload for smaller files
		progressReader := &progressReader{
			reader:    file,
			tracker:   e.config.ProgressTracker,
			totalSize: fileInfo.Size(),
		}

		err = e.webdavClient.UploadFile(e.ctx, remotePath, progressReader, fileInfo.Size())
		if err != nil {
			return fmt.Errorf("failed to upload file to %s: %w", remotePath, err)
		}
	}

	// Finish progress tracking
	if e.config.ProgressTracker != nil {
		e.config.ProgressTracker.Finish()
	}

	return nil
}

// downloadFile downloads a remote file to the local filesystem
func (e *OperationExecutor) downloadFile(remotePath, localPath string) error {
	// Get remote file properties first to get size
	props, err := e.webdavClient.GetProperties(e.ctx, remotePath)
	if err != nil {
		return fmt.Errorf("failed to get remote file properties for %s: %w", remotePath, err)
	}

	// Update progress tracker
	if e.config.ProgressTracker != nil {
		e.config.ProgressTracker.Start(props.Size)
	}

	// Download file
	readCloser, err := e.webdavClient.DownloadFile(e.ctx, remotePath)
	if err != nil {
		return fmt.Errorf("failed to download file from %s: %w", remotePath, err)
	}
	defer readCloser.Close()

	// Create local directory if it doesn't exist
	localDir := filepath.Dir(localPath)
	if localDir != "." {
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return fmt.Errorf("failed to create local directory %s: %w", localDir, err)
		}
	}

	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", localPath, err)
	}
	defer file.Close()

	// Copy with progress tracking
	progressWriter := &progressWriter{
		writer:    file,
		tracker:   e.config.ProgressTracker,
		totalSize: props.Size,
	}

	_, err = io.Copy(progressWriter, readCloser)
	if err != nil {
		return fmt.Errorf("failed to copy downloaded content to %s: %w", localPath, err)
	}

	// Finish progress tracking
	if e.config.ProgressTracker != nil {
		e.config.ProgressTracker.Finish()
	}

	return nil
}

// ensureRemoteDirectory creates a remote directory and all parent directories
func (e *OperationExecutor) ensureRemoteDirectory(remotePath string) error {
	// Clean the path and split by forward slashes (WebDAV always uses /)
	remotePath = path.Clean(remotePath)
	if remotePath == "/" || remotePath == "" {
		return nil
	}

	// Split by forward slash regardless of OS
	parts := strings.Split(remotePath, "/")
	currentPath := ""

	for _, part := range parts {
		if part == "" {
			continue
		}

		if currentPath == "" {
			currentPath = "/" + part
		} else {
			currentPath += "/" + part
		}

		// Check if directory exists
		_, err := e.webdavClient.GetProperties(e.ctx, currentPath)
		if err == nil {
			// Directory exists, continue
			continue
		}

		// Try to create directory
		if err := e.webdavClient.CreateDirectory(e.ctx, currentPath); err != nil {
			// If creation failed, check if it was created by another process
			if _, checkErr := e.webdavClient.GetProperties(e.ctx, currentPath); checkErr == nil {
				continue
			}
			return fmt.Errorf("failed to create remote directory %s: %w", currentPath, err)
		}
	}

	return nil
}

// deleteLocalFile deletes a local file or directory
func (e *OperationExecutor) deleteLocalFile(path string) error {
	// Check if it's a directory
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File already deleted
		}
		return fmt.Errorf("failed to stat local file %s: %w", path, err)
	}

	if fileInfo.IsDir() {
		return os.RemoveAll(path)
	}

	return os.Remove(path)
}

// deleteRemoteFile deletes a remote file or directory
func (e *OperationExecutor) deleteRemoteFile(path string) error {
	// Try to delete file/directory
	err := e.webdavClient.DeleteFile(e.ctx, path)
	if err != nil {
		// Check if it's a WebDAV "not found" error
		if webdavErr, ok := err.(*webdav.WebDAVError); ok && webdavErr.IsNotFoundError() {
			return nil // File already deleted
		}
		return fmt.Errorf("failed to delete remote file %s: %w", path, err)
	}

	return nil
}

// moveLocalFile moves a local file from source to destination
func (e *OperationExecutor) moveLocalFile(source, destination string) error {
	// Create destination directory if needed
	destDir := filepath.Dir(destination)
	if destDir != "." {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
		}
	}

	// Move the file
	return os.Rename(source, destination)
}

// moveRemoteFile moves a remote file from source to destination
func (e *OperationExecutor) moveRemoteFile(source, destination string) error {
	// Ensure destination directory exists
	destDir := filepath.Dir(destination)
	if destDir != "." && destDir != "/" {
		if err := e.ensureRemoteDirectory(destDir); err != nil {
			return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
		}
	}

	// Move the file
	return e.webdavClient.MoveFile(e.ctx, source, destination)
}

// PlanOperations creates a sync plan from detected changes
func (e *OperationExecutor) PlanOperations(changes []*Change) (*SyncPlan, error) {
	plan := &SyncPlan{
		Operations: make([]*SyncOperation, 0),
		CreatedAt:  time.Now(),
		Conflicts:  make([]*Conflict, 0),
		Warnings:   make([]string, 0),
	}

	var totalSize int64
	var totalFiles int

	for _, change := range changes {
		// Check for conflicts
		if change.IsConflict() {
			conflict := &Conflict{
				Type:        ConflictContentChanged,
				LocalPath:   change.LocalPath,
				RemotePath:  change.RemotePath,
				LocalMeta:   change.LocalMeta,
				RemoteMeta:  change.RemoteMeta,
				Description: fmt.Sprintf("File changed on both sides: %s", change.Reason),
				Timestamp:   time.Now(),
			}
			plan.Conflicts = append(plan.Conflicts, conflict)
			continue
		}

		// Create operation
		op := &SyncOperation{
			ID:           fmt.Sprintf("%s_%d", change.Type.String(), time.Now().UnixNano()),
			Type:         change.Type,
			Direction:    change.Direction,
			Priority:     change.Priority,
			Dependencies: make([]string, 0),
		}

		// Set source and target paths based on direction
		if change.Direction == LocalToRemote {
			op.SourcePath = change.LocalPath
			op.TargetPath = change.RemotePath
		} else if change.Direction == RemoteToLocal {
			op.SourcePath = change.RemotePath
			op.TargetPath = change.LocalPath
		} else {
			// Default for other cases
			op.SourcePath = change.LocalPath
			op.TargetPath = change.RemotePath
		}

		// Set size and update totals
		if change.Direction == LocalToRemote && change.LocalMeta != nil {
			op.Size = change.LocalMeta.Size
		} else if change.Direction == RemoteToLocal && change.RemoteMeta != nil {
			op.Size = change.RemoteMeta.Size
		}

		totalSize += op.Size
		totalFiles++

		// Add dependencies for directory creation
		if change.Type == ChangeCreate || change.Type == ChangeUpdate {
			if change.Direction == LocalToRemote && change.RemotePath != "" {
				parentDir := filepath.Dir(change.RemotePath)
				if parentDir != "." && parentDir != "/" {
					// Find or create dependency for parent directory
					parentOpID := e.findOrCreateDirectoryOp(plan, parentDir)
					if parentOpID != "" {
						op.Dependencies = append(op.Dependencies, parentOpID)
					}
				}
			} else if change.Direction == RemoteToLocal && change.LocalPath != "" {
				parentDir := filepath.Dir(change.LocalPath)
				if parentDir != "." {
					// Local directories are created on-demand, no explicit dependency needed
				}
			}
		}

		plan.Operations = append(plan.Operations, op)
	}

	plan.TotalFiles = totalFiles
	plan.TotalSize = totalSize

	// Estimate time (rough calculation: 1MB per second)
	if plan.TotalSize > 0 {
		plan.EstimatedTime = time.Duration(plan.TotalSize/1024/1024) * time.Second
		if plan.EstimatedTime < time.Second {
			plan.EstimatedTime = time.Second
		}
	}

	return plan, nil
}

// findOrCreateDirectoryOp finds or creates an operation for directory creation
func (e *OperationExecutor) findOrCreateDirectoryOp(plan *SyncPlan, dirPath string) string {
	// Check if we already have an operation for this directory
	for _, op := range plan.Operations {
		if op.Type == ChangeCreate && op.TargetPath == dirPath {
			return op.ID
		}
	}

	// Create a directory creation operation
	dirOp := &SyncOperation{
		ID:         fmt.Sprintf("mkdir_%s_%d", dirPath, time.Now().UnixNano()),
		Type:       ChangeCreate,
		Direction:  LocalToRemote,
		SourcePath: "",
		TargetPath: dirPath,
		Size:       0,
		Priority:   100, // High priority for directories
	}

	plan.Operations = append(plan.Operations, dirOp)
	return dirOp.ID
}

// ExecutePlan executes a complete sync plan
func (e *OperationExecutor) ExecutePlan(plan *SyncPlan) (*SyncResult, error) {
	result := &SyncResult{
		StartTime:    time.Now(),
		CreatedFiles: make([]string, 0),
		UpdatedFiles: make([]string, 0),
		DeletedFiles: make([]string, 0),
		SkippedFiles: make([]string, 0),
		Errors:       make([]string, 0),
		Warnings:     make([]string, 0),
		Conflicts:    make([]*Conflict, 0),
	}

	// Copy conflicts from plan
	result.Conflicts = plan.Conflicts

	// Sort operations by priority (higher first)
	sortedOps := make([]*SyncOperation, len(plan.Operations))
	copy(sortedOps, plan.Operations)

	// Simple sort by priority
	for i := 0; i < len(sortedOps)-1; i++ {
		for j := i + 1; j < len(sortedOps); j++ {
			if sortedOps[j].Priority > sortedOps[i].Priority {
				sortedOps[i], sortedOps[j] = sortedOps[j], sortedOps[i]
			}
		}
	}

	// Execute operations in order
	for _, op := range sortedOps {
		// Check dependencies
		dependenciesSatisfied := true
		for _, depID := range op.Dependencies {
			if !e.isOperationCompleted(depID, result) {
				dependenciesSatisfied = false
				break
			}
		}

		if !dependenciesSatisfied {
			result.SkippedFiles = append(result.SkippedFiles, fmt.Sprintf("%s (dependencies not satisfied)", op.SourcePath))
			continue
		}

		// Execute operation
		err := e.ExecuteOperation(op)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to execute %s on %s: %v", op.Type.String(), op.SourcePath, err)
			result.Errors = append(result.Errors, errorMsg)

			if e.config.ProgressTracker != nil {
				e.config.ProgressTracker.Error(err)
			}
			continue
		}

		// Update result
		result.ProcessedFiles++
		result.TransferredSize += op.Size

		switch op.Type {
		case ChangeCreate:
			result.CreatedFiles = append(result.CreatedFiles, op.SourcePath)
		case ChangeUpdate:
			result.UpdatedFiles = append(result.UpdatedFiles, op.SourcePath)
		case ChangeDelete:
			result.DeletedFiles = append(result.DeletedFiles, op.SourcePath)
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.TotalFiles = plan.TotalFiles
	result.TotalSize = plan.TotalSize
	result.Success = len(result.Errors) == 0

	return result, nil
}

// planChange creates operations for a single change
func (e *OperationExecutor) planChange(change *Change) ([]*SyncOperation, error) {
	var operations []*SyncOperation

	// Skip if this is a conflict
	if change.IsConflict() {
		return operations, nil
	}

	// Create operation based on change type and direction
	op := &SyncOperation{
		ID:           fmt.Sprintf("%s_%d", change.Type.String(), time.Now().UnixNano()),
		Type:         change.Type,
		Direction:    change.Direction,
		SourcePath:   change.LocalPath,
		TargetPath:   change.RemotePath,
		Priority:     change.Priority,
		Dependencies: make([]string, 0),
	}

	// Set size based on direction and available metadata
	if change.Direction == LocalToRemote && change.LocalMeta != nil {
		op.Size = change.LocalMeta.Size
	} else if change.Direction == RemoteToLocal && change.RemoteMeta != nil {
		op.Size = change.RemoteMeta.Size
	}

	// Handle bidirectional operations
	if change.Direction == Bidirectional {
		// For bidirectional sync, we need to determine the actual direction
		// based on which side is newer (source-wins policy)
		if change.LocalMeta != nil && change.RemoteMeta != nil {
			if change.LocalMeta.IsNewer(change.RemoteMeta) {
				op.Direction = LocalToRemote
				op.SourcePath = change.LocalPath
				op.TargetPath = change.RemotePath
				op.Size = change.LocalMeta.Size
			} else {
				op.Direction = RemoteToLocal
				op.SourcePath = change.RemotePath
				op.TargetPath = change.LocalPath
				op.Size = change.RemoteMeta.Size
			}
		} else if change.LocalMeta != nil {
			// Only local exists, upload to remote
			op.Direction = LocalToRemote
			op.Size = change.LocalMeta.Size
		} else if change.RemoteMeta != nil {
			// Only remote exists, download to local
			op.Direction = RemoteToLocal
			op.Size = change.RemoteMeta.Size
		}
	}

	// Add directory creation dependencies if needed
	if change.Type == ChangeCreate || change.Type == ChangeUpdate {
		if op.Direction == LocalToRemote && op.TargetPath != "" {
			parentDir := filepath.Dir(op.TargetPath)
			if parentDir != "." && parentDir != "/" {
				parentOpID := e.findOrCreateDirectoryOpForPlan(nil, parentDir)
				if parentOpID != "" {
					op.Dependencies = append(op.Dependencies, parentOpID)
				}
			}
		}
	}

	operations = append(operations, op)
	return operations, nil
}

// findOrCreateDirectoryOpForPlan finds or creates a directory operation for a plan
func (e *OperationExecutor) findOrCreateDirectoryOpForPlan(plan *SyncPlan, dirPath string) string {
	// If we have a plan, check existing operations
	if plan != nil {
		for _, op := range plan.Operations {
			if op.Type == ChangeCreate && op.TargetPath == dirPath {
				return op.ID
			}
		}
	}

	// Generate a unique ID for directory creation
	dirOpID := fmt.Sprintf("mkdir_%s_%d", dirPath, time.Now().UnixNano())

	// If we have a plan, add the operation to it
	if plan != nil {
		dirOp := &SyncOperation{
			ID:         dirOpID,
			Type:       ChangeCreate,
			Direction:  LocalToRemote,
			SourcePath: "",
			TargetPath: dirPath,
			Size:       0,
			Priority:   100, // High priority for directories
		}
		plan.Operations = append(plan.Operations, dirOp)
	}

	return dirOpID
}

// isOperationCompleted checks if an operation (by ID) has been completed
func (e *OperationExecutor) isOperationCompleted(opID string, result *SyncResult) bool {
	// This is a simplified check - in a real implementation,
	// we'd track operation IDs more carefully
	// For now, we assume all operations are completed if we reach them
	return true
}

// progressReader wraps an io.Reader to track progress
type progressReader struct {
	reader    io.Reader
	tracker   ProgressTracker
	totalSize int64
	readBytes int64
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	pr.readBytes += int64(n)

	if pr.tracker != nil {
		pr.tracker.Update(pr.readBytes)
	}

	return n, err
}

// progressWriter wraps an io.Writer to track progress
type progressWriter struct {
	writer     io.Writer
	tracker    ProgressTracker
	totalSize  int64
	wroteBytes int64
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	pw.wroteBytes += int64(n)

	if pw.tracker != nil {
		pw.tracker.Update(pw.wroteBytes)
	}

	return n, err
}
