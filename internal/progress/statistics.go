package progress

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Statistics tracks sync operation statistics
type Statistics struct {
	// Overall statistics
	StartTime        time.Time     `json:"start_time"`
	EndTime          time.Time     `json:"end_time"`
	Duration         time.Duration `json:"duration"`
	TotalFiles       int           `json:"total_files"`
	ProcessedFiles   int           `json:"processed_files"`
	TotalBytes       int64         `json:"total_bytes"`
	TransferredBytes int64         `json:"transferred_bytes"`

	// Operation counts
	Uploads   int `json:"uploads"`
	Downloads int `json:"downloads"`
	Creates   int `json:"creates"`
	Updates   int `json:"updates"`
	Deletes   int `json:"deletes"`
	Skips     int `json:"skips"`
	Conflicts int `json:"conflicts"`
	Errors    int `json:"errors"`

	// Performance metrics
	ThroughputBps float64 `json:"throughput_bps"` // bytes per second
	peakBps       float64 `json:"peak_bps"`       // peak bytes per second

	// Current operation tracking
	currentOperation string
	operationStart   time.Time

	mu sync.RWMutex
}

// NewStatistics creates a new statistics tracker
func NewStatistics() *Statistics {
	now := time.Now()
	return &Statistics{
		StartTime: now,
		EndTime:   now,
	}
}

// StartOperation begins tracking a new operation
func (s *Statistics) StartOperation(operation string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentOperation = operation
	s.operationStart = time.Now()
}

// EndOperation completes the current operation
func (s *Statistics) EndOperation() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentOperation = ""
	s.operationStart = time.Time{}
}

// RecordUpload records an upload operation
func (s *Statistics) RecordUpload(bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Uploads++
	s.ProcessedFiles++
	s.TransferredBytes += bytes
	s.updateThroughput()
}

// RecordDownload records a download operation
func (s *Statistics) RecordDownload(bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Downloads++
	s.ProcessedFiles++
	s.TransferredBytes += bytes
	s.updateThroughput()
}

// RecordCreate records a file creation
func (s *Statistics) RecordCreate(bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Creates++
	s.ProcessedFiles++
	s.TransferredBytes += bytes
	s.updateThroughput()
}

// RecordUpdate records a file update
func (s *Statistics) RecordUpdate(bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Updates++
	s.ProcessedFiles++
	s.TransferredBytes += bytes
	s.updateThroughput()
}

// RecordDelete records a file deletion
func (s *Statistics) RecordDelete() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Deletes++
	s.ProcessedFiles++
	s.updateThroughput()
}

// RecordSkip records a skipped file
func (s *Statistics) RecordSkip() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Skips++
	s.ProcessedFiles++
	s.updateThroughput()
}

// RecordConflict records a conflict
func (s *Statistics) RecordConflict() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Conflicts++
}

// RecordError records an error
func (s *Statistics) RecordError() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Errors++
}

// SetTotalFiles sets the expected total number of files
func (s *Statistics) SetTotalFiles(total int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalFiles = total
}

// SetTotalBytes sets the expected total bytes to transfer
func (s *Statistics) SetTotalBytes(bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalBytes = bytes
}

// AddBytesTransferred adds to the total bytes transferred
func (s *Statistics) AddBytesTransferred(bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TransferredBytes += bytes
	s.updateThroughput()
}

// Finish marks the statistics as complete
func (s *Statistics) Finish() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)
	s.updateThroughput()
}

// GetProgress returns the current progress percentage
func (s *Statistics) GetProgress() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.TotalFiles == 0 {
		return 0.0
	}

	return float64(s.ProcessedFiles) / float64(s.TotalFiles) * 100.0
}

// GetEstimatedTimeRemaining estimates time remaining based on current progress
func (s *Statistics) GetEstimatedTimeRemaining() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.ProcessedFiles == 0 || s.StartTime.IsZero() {
		return 0
	}

	elapsed := time.Since(s.StartTime)
	if elapsed <= 0 {
		return 0
	}

	// Calculate based on files processed
	filesPerSecond := float64(s.ProcessedFiles) / elapsed.Seconds()
	if filesPerSecond <= 0 {
		return 0
	}

	remainingFiles := s.TotalFiles - s.ProcessedFiles
	remainingSeconds := float64(remainingFiles) / filesPerSecond

	return time.Duration(remainingSeconds) * time.Second
}

// updateThroughput updates the throughput calculation
func (s *Statistics) updateThroughput() {
	if s.StartTime.IsZero() || s.TransferredBytes == 0 {
		return
	}

	elapsed := time.Since(s.StartTime)
	if elapsed <= 0 {
		return
	}

	currentBps := float64(s.TransferredBytes) / elapsed.Seconds()
	s.ThroughputBps = currentBps

	// Update peak throughput
	if currentBps > s.peakBps {
		s.peakBps = currentBps
	}
}

// String returns a formatted summary of the statistics
func (s *Statistics) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("Sync Statistics:\n"))
	summary.WriteString(fmt.Sprintf("  Duration: %s\n", s.Duration.Round(time.Second)))
	summary.WriteString(fmt.Sprintf("  Files: %d/%d processed\n", s.ProcessedFiles, s.TotalFiles))
	summary.WriteString(fmt.Sprintf("  Bytes: %s/%s transferred\n", formatBytes(s.TransferredBytes), formatBytes(s.TotalBytes)))

	if s.ThroughputBps > 0 {
		summary.WriteString(fmt.Sprintf("  Throughput: %s/s (peak: %s/s)\n",
			formatBytes(int64(s.ThroughputBps)), formatBytes(int64(s.peakBps))))
	}

	// Operation breakdown
	summary.WriteString(fmt.Sprintf("  Operations: %d uploads, %d downloads, %d creates, %d updates, %d deletes\n",
		s.Uploads, s.Downloads, s.Creates, s.Updates, s.Deletes))

	// Issues
	if s.Skips > 0 || s.Conflicts > 0 || s.Errors > 0 {
		summary.WriteString(fmt.Sprintf("  Issues: %d skips, %d conflicts, %d errors\n",
			s.Skips, s.Conflicts, s.Errors))
	}

	return summary.String()
}

// JSONString returns a JSON-formatted string of the statistics
func (s *Statistics) JSONString() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// This is a simple JSON implementation - in production, use encoding/json
	data, _ := json.Marshal(s)
	return string(data)
}

// Copy returns a copy of the current statistics
func (s *Statistics) Copy() *Statistics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := *s
	return &copy
}
