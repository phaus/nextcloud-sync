package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ProgressBar implements a terminal progress bar
type ProgressBar struct {
	total      int64
	current    int64
	width      int
	startTime  time.Time
	lastUpdate time.Time
	mu         sync.RWMutex
	operation  string
	eta        time.Duration
	enabled    bool
	writer     io.Writer
}

// NewProgressBar creates a new progress bar
func NewProgressBar(width int) *ProgressBar {
	if width <= 0 {
		width = 50
	}

	return &ProgressBar{
		width:      width,
		startTime:  time.Now(),
		lastUpdate: time.Now(),
		enabled:    true,
		writer:     os.Stderr,
	}
}

// SetOperation sets the current operation description
func (pb *ProgressBar) SetOperation(operation string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.operation = operation
}

// Start initializes the progress bar with total bytes
func (pb *ProgressBar) Start(total int64) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.total = total
	pb.current = 0
	pb.startTime = time.Now()
	pb.lastUpdate = time.Now()
	pb.eta = 0

	if pb.enabled && total > 0 {
		pb.render()
	}
}

// Update updates the current progress
func (pb *ProgressBar) Update(current int64) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if !pb.enabled || pb.total <= 0 {
		return
	}

	pb.current = current
	now := time.Now()

	// Update ETA every 100ms to avoid too frequent calculations
	if now.Sub(pb.lastUpdate) >= 100*time.Millisecond {
		pb.updateETA()
		pb.lastUpdate = now
		pb.render()
	}
}

// Finish marks the progress as complete
func (pb *ProgressBar) Finish() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.current = pb.total
	if pb.enabled && pb.total > 0 {
		pb.render()
		fmt.Fprintln(pb.writer) // New line after completion
	}
}

// Error displays an error message
func (pb *ProgressBar) Error(err error) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.enabled {
		// Clear current line and show error
		fmt.Fprintf(pb.writer, "\r\033[K") // Clear line
		fmt.Fprintf(pb.writer, "❌ Error in %s: %v\n", pb.operation, err)
	}
}

// SetEnabled enables or disables the progress bar
func (pb *ProgressBar) SetEnabled(enabled bool) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.enabled = enabled
}

// SetWriter sets the output writer
func (pb *ProgressBar) SetWriter(writer io.Writer) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.writer = writer
}

// updateETA calculates the estimated time remaining
func (pb *ProgressBar) updateETA() {
	if pb.current <= 0 || pb.startTime.IsZero() {
		return
	}

	elapsed := time.Since(pb.startTime)
	if pb.current > pb.total {
		pb.total = pb.current
	}

	// Simple linear extrapolation
	bytesPerSecond := float64(pb.current) / elapsed.Seconds()
	if bytesPerSecond > 0 {
		remainingBytes := pb.total - pb.current
		remainingSeconds := float64(remainingBytes) / bytesPerSecond
		pb.eta = time.Duration(remainingSeconds) * time.Second
	}
}

// render renders the progress bar to the terminal
func (pb *ProgressBar) render() {
	if !pb.enabled || pb.writer == nil {
		return
	}

	percentage := float64(pb.current) / float64(pb.total)
	if percentage > 1.0 {
		percentage = 1.0
	}

	// Calculate filled width
	filledWidth := int(percentage * float64(pb.width))
	if filledWidth > pb.width {
		filledWidth = pb.width
	}

	// Build the progress bar
	var bar strings.Builder
	bar.Grow(pb.width + 50) // Pre-allocate approximate capacity

	// Progress bar
	bar.WriteString("\r[")
	if filledWidth > 0 {
		bar.WriteString(strings.Repeat("=", filledWidth))
	}
	if filledWidth < pb.width {
		bar.WriteString(strings.Repeat(" ", pb.width-filledWidth))
	}
	bar.WriteString("] ")

	// Percentage
	bar.WriteString(fmt.Sprintf("%3.0f%%", percentage*100))

	// Bytes transferred
	bar.WriteString(fmt.Sprintf(" %s/%s", formatBytes(pb.current), formatBytes(pb.total)))

	// ETA
	if pb.eta > 0 {
		bar.WriteString(fmt.Sprintf(" ETA: %s", formatDuration(pb.eta)))
	}

	// Operation name (truncate if too long)
	if pb.operation != "" {
		maxOpLen := 30
		op := pb.operation
		if len(op) > maxOpLen {
			op = op[:maxOpLen-3] + "..."
		}
		bar.WriteString(fmt.Sprintf(" | %s", op))
	}

	// Write to output
	fmt.Fprint(pb.writer, bar.String())
}

// formatBytes formats bytes in human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	units := []string{"KB", "MB", "GB", "TB"}
	exp := 0
	value := float64(bytes)
	for value >= unit && exp < len(units)-1 {
		value /= unit
		exp++
	}

	// Handle the case where we've done the maximum number of divisions
	if exp == len(units)-1 && value >= unit {
		value /= unit
		exp++
	}

	return fmt.Sprintf("%.1f %s", value, units[exp-1])
}

// formatDuration formats duration in human readable format
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "< 1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, minutes)
}
