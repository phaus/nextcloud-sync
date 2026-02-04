package progress

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ResumeState represents the state needed to resume a transfer
type ResumeState struct {
	FilePath        string    `json:"file_path"`
	TotalSize       int64     `json:"total_size"`
	TransferredSize int64     `json:"transferred_size"`
	Checksum        string    `json:"checksum,omitempty"`
	LastModified    time.Time `json:"last_modified"`
	Operation       string    `json:"operation"` // "upload" or "download"
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ResumeManager handles transfer resume functionality
type ResumeManager struct {
	resumeDir string
	states    map[string]*ResumeState // key: file path
	mu        sync.RWMutex
	enabled   bool
	maxAge    time.Duration // Maximum age for resume files
}

// NewResumeManager creates a new resume manager
func NewResumeManager(resumeDir string) (*ResumeManager, error) {
	if resumeDir == "" {
		// Default to user's cache directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		resumeDir = filepath.Join(homeDir, ".nextcloud-sync", "resume")
	}

	// Create resume directory if it doesn't exist
	if err := os.MkdirAll(resumeDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create resume directory: %w", err)
	}

	rm := &ResumeManager{
		resumeDir: resumeDir,
		states:    make(map[string]*ResumeState),
		enabled:   true,
		maxAge:    24 * time.Hour, // Default: 24 hours
	}

	// Load existing resume states
	if err := rm.loadStates(); err != nil {
		return nil, fmt.Errorf("failed to load resume states: %w", err)
	}

	// Clean up old resume states
	if err := rm.cleanupOldStates(); err != nil {
		// Log error but don't fail initialization
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup old resume states: %v\n", err)
	}

	return rm, nil
}

// SetEnabled enables or disables resume functionality
func (rm *ResumeManager) SetEnabled(enabled bool) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.enabled = enabled
}

// SetMaxAge sets the maximum age for resume files
func (rm *ResumeManager) SetMaxAge(maxAge time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.maxAge = maxAge
}

// StartTransfer begins tracking a new transfer for potential resume
func (rm *ResumeManager) StartTransfer(filePath, operation string, totalSize int64, lastModified time.Time) (*ResumeState, error) {
	if !rm.enabled {
		return nil, nil
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Create new resume state
	state := &ResumeState{
		FilePath:        filePath,
		TotalSize:       totalSize,
		TransferredSize: 0,
		LastModified:    lastModified,
		Operation:       operation,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Check if there's an existing state to resume from
	if existingState, exists := rm.states[filePath]; exists {
		// Validate that the file hasn't changed
		if existingState.TotalSize == totalSize &&
			existingState.LastModified.Equal(lastModified) &&
			existingState.Operation == operation {
			state.TransferredSize = existingState.TransferredSize
			state.CreatedAt = existingState.CreatedAt
		}
	}

	rm.states[filePath] = state

	// Save to disk
	if err := rm.saveState(state); err != nil {
		delete(rm.states, filePath) // Remove from memory if save fails
		return nil, fmt.Errorf("failed to save resume state: %w", err)
	}

	return state, nil
}

// UpdateProgress updates the transferred bytes for a transfer
func (rm *ResumeManager) UpdateProgress(filePath string, transferredSize int64, checksum string) error {
	if !rm.enabled {
		return nil
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, exists := rm.states[filePath]
	if !exists {
		return fmt.Errorf("no resume state found for file: %s", filePath)
	}

	state.TransferredSize = transferredSize
	state.UpdatedAt = time.Now()

	// Update checksum if provided
	if checksum != "" {
		state.Checksum = checksum
	}

	// Save to disk
	return rm.saveState(state)
}

// CompleteTransfer marks a transfer as complete and removes the resume state
func (rm *ResumeManager) CompleteTransfer(filePath string) error {
	if !rm.enabled {
		return nil
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, exists := rm.states[filePath]
	if !exists {
		return nil
	}

	// Remove from memory
	delete(rm.states, filePath)

	// Remove resume file
	resumeFile := rm.getResumeFileName(state.FilePath)
	if err := os.Remove(resumeFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove resume file: %w", err)
	}

	return nil
}

// GetResumeState returns the resume state for a file if it exists and is valid
func (rm *ResumeManager) GetResumeState(filePath, operation string, totalSize int64, lastModified time.Time) (*ResumeState, error) {
	if !rm.enabled {
		return nil, nil
	}

	rm.mu.RLock()
	defer rm.mu.RUnlock()

	state, exists := rm.states[filePath]
	if !exists {
		return nil, nil
	}

	// Validate that the state is still valid
	if state.Operation != operation {
		return nil, nil
	}

	if state.TotalSize != totalSize {
		return nil, nil
	}

	if !state.LastModified.Equal(lastModified) {
		return nil, nil
	}

	// Check if the state is too old
	if time.Since(state.UpdatedAt) > rm.maxAge {
		return nil, nil
	}

	return state, nil
}

// Cleanup removes all resume states
func (rm *ResumeManager) Cleanup() error {
	if !rm.enabled {
		return nil
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Remove all resume files
	for filePath := range rm.states {
		resumeFile := rm.getResumeFileName(filePath)
		if err := os.Remove(resumeFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove resume file for %s: %w", filePath, err)
		}
	}

	// Clear memory
	rm.states = make(map[string]*ResumeState)

	return nil
}

// GetActiveTransfers returns a list of all currently tracked transfers
func (rm *ResumeManager) GetActiveTransfers() []*ResumeState {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	transfers := make([]*ResumeState, 0, len(rm.states))
	for _, state := range rm.states {
		transfers = append(transfers, state)
	}

	return transfers
}

// loadStates loads all resume states from disk
func (rm *ResumeManager) loadStates() error {
	entries, err := os.ReadDir(rm.resumeDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read resume directory: %w", err)
	}

	if os.IsNotExist(err) {
		return nil // No resume directory yet
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".resume" {
			continue
		}

		resumeFile := filepath.Join(rm.resumeDir, entry.Name())
		state, err := rm.loadStateFromFile(resumeFile)
		if err != nil {
			// Log error but continue loading other files
			fmt.Fprintf(os.Stderr, "Warning: failed to load resume state from %s: %v\n", resumeFile, err)
			continue
		}

		rm.states[state.FilePath] = state
	}

	return nil
}

// cleanupOldStates removes resume states that are too old
func (rm *ResumeManager) cleanupOldStates() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now()
	var toRemove []string

	for filePath, state := range rm.states {
		if now.Sub(state.UpdatedAt) > rm.maxAge {
			toRemove = append(toRemove, filePath)
		}
	}

	// Remove old states
	for _, filePath := range toRemove {
		delete(rm.states, filePath)
		resumeFile := rm.getResumeFileName(filePath)
		if err := os.Remove(resumeFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove old resume file for %s: %w", filePath, err)
		}
	}

	return nil
}

// saveState saves a resume state to disk
func (rm *ResumeManager) saveState(state *ResumeState) error {
	resumeFile := rm.getResumeFileName(state.FilePath)

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal resume state: %w", err)
	}

	// Write to temporary file first, then rename to atomic write
	tempFile := resumeFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary resume file: %w", err)
	}

	if err := os.Rename(tempFile, resumeFile); err != nil {
		os.Remove(tempFile) // Clean up temp file
		return fmt.Errorf("failed to rename resume file: %w", err)
	}

	return nil
}

// loadStateFromFile loads a resume state from a file
func (rm *ResumeManager) loadStateFromFile(filename string) (*ResumeState, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read resume file: %w", err)
	}

	var state ResumeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resume state: %w", err)
	}

	return &state, nil
}

// getResumeFileName generates the resume file name for a given file path
func (rm *ResumeManager) getResumeFileName(filePath string) string {
	// Use hash of file path to avoid issues with special characters
	hash := sha256.Sum256([]byte(filePath))
	hashStr := hex.EncodeToString(hash[:])
	return filepath.Join(rm.resumeDir, hashStr+".resume")
}

// CalculateChecksum calculates SHA256 checksum for a file
func CalculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
