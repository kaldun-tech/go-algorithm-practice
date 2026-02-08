package practice

import (
	"sync"
	"time"
)

// Think through:
// 1. Fixed-size backing array
// 2. Head/tail pointers (or single write index if only one writer)
// 3. When to overwrite vs when buffer not full yet
// 4. Lock granularity - one lock for whole buffer or try lock-free?

// MetricsBuffer is a thread-safe ring buffer for metrics collection
type MetricsBuffer struct {
	mu     sync.RWMutex
	buffer []float64
	// writeIdx - where to write next?
	// count - how many elements are valid? (differs from len when not full)
	// size - capacity of the buffer
}

func NewMetricsBuffer(size int) *MetricsBuffer {
	// Initialize the buffer with fixed capacity
	return nil
}

// Push adds a value to the buffer, overwriting the oldest if full
func (mb *MetricsBuffer) Push(value float64) {
	// Thread-safe write
	// Calculate write position: writeIdx % size
	// Increment writeIdx
	// Update count (capped at size)
}

// GetRecent returns the last n samples (or fewer if buffer not full)
func (mb *MetricsBuffer) GetRecent(n int) []float64 {
	// Thread-safe read
	// Need to:
	// 1. Clamp n to actual count
	// 2. Calculate where the "last n" elements are in the circular buffer
	// 3. Copy to new slice (don't return internal buffer!)
	//
	// Hint: If writeIdx is at position W and count is C,
	// the most recent element is at (W - 1 + size) % size
	// the nth most recent is at (W - n + size) % size
	return nil
}

// Average calculates the average of all samples in the buffer
func (mb *MetricsBuffer) Average() float64 {
	// Thread-safe read
	// Sum all valid elements and divide by count
	// Handle empty buffer case
	return 0
}

// =============================================================================
// Challenge: Timestamped samples
// =============================================================================

type TimestampedSample struct {
	Value     float64
	Timestamp time.Time
}

type TimestampedMetricsBuffer struct {
	mu       sync.RWMutex
	buffer   []TimestampedSample
	writeIdx int
	count    int
	size     int
}

func NewTimestampedMetricsBuffer(size int) *TimestampedMetricsBuffer {
	return nil
}

func (tmb *TimestampedMetricsBuffer) Push(value float64) {
	// Same as MetricsBuffer but include timestamp
}

// GetWindow returns samples from the last duration
// Example: GetWindow(5 * time.Minute) returns all samples from last 5 minutes
func (tmb *TimestampedMetricsBuffer) GetWindow(duration time.Duration) []TimestampedSample {
	// Thread-safe read
	// Iterate through valid samples
	// Return only those where time.Since(sample.Timestamp) <= duration
	//
	// Note: Old samples may still be in buffer but outside the window
	return nil
}
