package progress

import (
	"fmt"
	"runtime"
	"strings"
	stdsync "sync"
	"time"
)

// CombinedProgressTracker implements a progress tracking interface
// and combines progress bar, statistics, and resume functionality
type CombinedProgressTracker struct {
	progressBar  *ProgressBar
	statistics   *Statistics
	resumeMgr    *ResumeManager
	currentState *ResumeState

	mu      stdsync.RWMutex
	enabled bool
	verbose bool

	// Configuration
	updateInterval time.Duration
}

// Config holds configuration for the progress tracker
type Config struct {
	ProgressBarWidth int           `json:"progress_bar_width"`
	UpdateInterval   time.Duration `json:"update_interval"`
	ResumeEnabled    bool          `json:"resume_enabled"`
	ResumeMaxAge     time.Duration `json:"resume_max_age"`
	Verbose          bool          `json:"verbose"`
	ShowStatistics   bool          `json:"show_statistics"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		ProgressBarWidth: 50,
		UpdateInterval:   100 * time.Millisecond,
		ResumeEnabled:    true,
		ResumeMaxAge:     24 * time.Hour,
		Verbose:          false,
		ShowStatistics:   true,
	}
}

// NewCombinedProgressTracker creates a new combined progress tracker
func NewCombinedProgressTracker(config *Config, resumeDir string) (*CombinedProgressTracker, error) {
	if config == nil {
		config = DefaultConfig()
	}

	pt := &CombinedProgressTracker{
		enabled:        true,
		verbose:        config.Verbose,
		updateInterval: config.UpdateInterval,
	}

	// Initialize progress bar
	pt.progressBar = NewProgressBar(config.ProgressBarWidth)
	pt.progressBar.SetEnabled(config.ShowStatistics)

	// Initialize statistics
	pt.statistics = NewStatistics()

	// Initialize resume manager
	if config.ResumeEnabled {
		var err error
		pt.resumeMgr, err = NewResumeManager(resumeDir)
		if err != nil {
			return nil, fmt.Errorf("failed to create resume manager: %w", err)
		}
		pt.resumeMgr.SetMaxAge(config.ResumeMaxAge)
	}

	return pt, nil
}

// Start implements sync.ProgressTracker interface
func (pt *CombinedProgressTracker) Start(total int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if !pt.enabled {
		return
	}

	// Start progress bar
	pt.progressBar.Start(total)

	// Start statistics tracking
	pt.statistics.SetTotalBytes(total)
	pt.statistics.StartOperation(pt.currentState.FilePath)

	// Update resume state if it exists
	if pt.currentState != nil && pt.resumeMgr != nil {
		pt.currentState.TotalSize = total
		pt.currentState.TransferredSize = 0
		pt.currentState.CreatedAt = time.Now()
		pt.currentState.UpdatedAt = time.Now()
	}
}

// Update implements sync.ProgressTracker interface
func (pt *CombinedProgressTracker) Update(current int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if !pt.enabled {
		return
	}

	// Update progress bar
	pt.progressBar.Update(current)

	// Update statistics
	pt.statistics.AddBytesTransferred(current - pt.statistics.TransferredBytes)

	// Update resume state
	if pt.currentState != nil && pt.resumeMgr != nil {
		pt.currentState.TransferredSize = current
		pt.currentState.UpdatedAt = time.Now()
		_ = pt.resumeMgr.UpdateProgress(pt.currentState.FilePath, current, "")
	}
}

// Finish implements sync.ProgressTracker interface
func (pt *CombinedProgressTracker) Finish() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if !pt.enabled {
		return
	}

	// Finish progress bar
	pt.progressBar.Finish()

	// Update statistics
	pt.statistics.EndOperation()
	if pt.currentState != nil {
		switch pt.currentState.Operation {
		case "upload":
			pt.statistics.RecordUpload(pt.currentState.TransferredSize)
		case "download":
			pt.statistics.RecordDownload(pt.currentState.TransferredSize)
		default:
			pt.statistics.RecordUpdate(pt.currentState.TransferredSize)
		}
	}

	// Complete transfer in resume manager
	if pt.currentState != nil && pt.resumeMgr != nil {
		_ = pt.resumeMgr.CompleteTransfer(pt.currentState.FilePath)
		pt.currentState = nil
	}
}

// SetOperation implements sync.ProgressTracker interface
func (pt *CombinedProgressTracker) SetOperation(operation string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if !pt.enabled {
		return
	}

	pt.progressBar.SetOperation(operation)

	// Extract file path from operation if possible
	// Operation format is typically "TYPE path"
	parts := splitOperation(operation)
	if len(parts) >= 2 {
		filePath := parts[1]
		operationType := parts[0]

		// Normalize operation type to lowercase
		operationType = strings.ToLower(operationType)

		// Check for resume state
		if pt.resumeMgr != nil {
			if state, err := pt.resumeMgr.GetResumeState(filePath, operationType, 0, time.Time{}); err == nil && state != nil {
				pt.currentState = state
				if pt.verbose {
					fmt.Printf("Resuming transfer for %s: %d/%d bytes\n",
						filePath, state.TransferredSize, state.TotalSize)
				}
			} else {
				// Create new resume state
				pt.currentState = &ResumeState{
					FilePath:  filePath,
					Operation: operationType,
				}
			}
		}
	}
}

// Error implements sync.ProgressTracker interface
func (pt *CombinedProgressTracker) Error(err error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if !pt.enabled {
		return
	}

	pt.progressBar.Error(err)
	pt.statistics.RecordError()

	// Update resume state with error
	if pt.currentState != nil && pt.resumeMgr != nil {
		pt.currentState.UpdatedAt = time.Now()
		_ = pt.resumeMgr.UpdateProgress(pt.currentState.FilePath, pt.currentState.TransferredSize, "")
	}
}

// GetStatistics returns a copy of the current statistics
func (pt *CombinedProgressTracker) GetStatistics() *Statistics {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.statistics.Copy()
}

// GetResumeManager returns the resume manager
func (pt *CombinedProgressTracker) GetResumeManager() *ResumeManager {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.resumeMgr
}

// SetEnabled enables or disables the progress tracker
func (pt *CombinedProgressTracker) SetEnabled(enabled bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.enabled = enabled
	pt.progressBar.SetEnabled(enabled)
}

// SetVerbose enables or disables verbose output
func (pt *CombinedProgressTracker) SetVerbose(verbose bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.verbose = verbose
}

// Cleanup performs cleanup operations
func (pt *CombinedProgressTracker) Cleanup() error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Finish statistics
	pt.statistics.Finish()

	// Print statistics if verbose
	if pt.verbose && pt.statistics.String() != "" {
		fmt.Print(pt.statistics.String())
	}

	// Cleanup resume manager
	if pt.resumeMgr != nil {
		if err := pt.resumeMgr.Cleanup(); err != nil {
			return fmt.Errorf("failed to cleanup resume manager: %w", err)
		}
	}

	return nil
}

// PrintSummary prints a summary of the sync operation
func (pt *CombinedProgressTracker) PrintSummary() {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	stats := pt.statistics.Copy()

	fmt.Printf("\n" + stats.String())

	// Print resume states if any exist
	if pt.resumeMgr != nil {
		transfers := pt.resumeMgr.GetActiveTransfers()
		if len(transfers) > 0 {
			fmt.Printf("\nActive Transfers (can be resumed):\n")
			for _, transfer := range transfers {
				progress := float64(transfer.TransferredSize) / float64(transfer.TotalSize) * 100
				fmt.Printf("  %s: %.1f%% (%s/%s)\n",
					transfer.FilePath, progress,
					formatBytes(transfer.TransferredSize), formatBytes(transfer.TotalSize))
			}
		}
	}
}

// IsTerminal returns true if running in a terminal
func (pt *CombinedProgressTracker) IsTerminal() bool {
	return runtime.GOOS != "windows" // Simple check, could be enhanced
}

// GetEstimatedTimeRemaining returns the estimated time remaining
func (pt *CombinedProgressTracker) GetEstimatedTimeRemaining() time.Duration {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.statistics.GetEstimatedTimeRemaining()
}

// GetProgress returns the current progress percentage
func (pt *CombinedProgressTracker) GetProgress() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.statistics.GetProgress()
}

// Helper functions

// splitOperation splits an operation string into type and path
func splitOperation(operation string) []string {
	if operation == "" {
		return []string{"", ""}
	}

	// Find the first space to separate operation type from path
	parts := strings.SplitN(operation, " ", 2)

	// Ensure we have exactly 2 parts
	for len(parts) < 2 {
		parts = append(parts, "")
	}

	return parts
}
