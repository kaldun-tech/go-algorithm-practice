package practice

import (
	"sync"
	"time"
)

// Think through:
// 1. Fixed-size backing array
// A: A ring buffer is a fixed-size, in-memory circular array where new samples overwrite the oldest ones
// Typically implement it as an array/slice plus moving indices, optionally with sync for concurrent readers/writers

// 2. Head/tail pointers (or single write index if only one writer)
// A: Track a write index and either a count or read index

// 3. When to overwrite vs when buffer not full yet
// A: Each entry is written at wIdx. Wrap around wIdx = (wIdx+1) % capacity
// When buffer is full, overwrite the oldest sample - consider timestamp
// Ideal for last N points or sliding-window stats without unbounded memory growth

// 4. Lock granularity - one lock for whole buffer or try lock-free?
// A: Most metric collection has one writer and multiple readers
// A simple mutex is usually fast enough. Use an RWMutex with a small buffer 60-600 points/metric
// Can also consider a single writer, many readers with copy on read: readers call Snapshot to copy out

// MetricsBuffer is a thread-safe ring buffer for metrics collection
type MetricsBuffer struct {
	mu     sync.RWMutex
	buffer []float64
	// Where to write next
	writeIdx int
	// How many elements are valid? Differs from len when not full
	count int
	// Capacity of the buffer
	size int
}

func NewMetricsBuffer(size int) *MetricsBuffer {
	// Initialize the buffer with fixed capacity
	return &MetricsBuffer{
		buffer:   make([]float64, size),
		writeIdx: 0,
		count:    0,
		size:     size,
	}
}

// Push adds a value to the buffer, overwriting the oldest if full
func (mb *MetricsBuffer) Push(value float64) {
	// Thread-safe write
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.buffer[mb.writeIdx] = value
	// Calculate write position
	mb.writeIdx = (mb.writeIdx + 1) % mb.size
	// Update count (capped at size)
	if mb.count < mb.size {
		mb.count++
	}
}

// GetRecent returns the last n samples (or fewer if buffer not full)
func (mb *MetricsBuffer) GetRecent(n int) []float64 {
	// Thread-safe read
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	// Need to:
	// 1. Clamp n to actual count
	if mb.count < n {
		n = mb.count
	}

	// 2. Calculate where the "last n" elements are in the circular buffer
	// Hint: If writeIdx is at position W and count is C,
	// the most recent element is at (W - 1 + size) % size
	// the nth most recent is at (W - n + size) % size
	end := (mb.writeIdx - 1 + mb.size) % mb.size
	start := (mb.writeIdx - n + mb.size) % mb.size

	// 3. Copy to new slice (don't return internal buffer!)
	dest := make([]float64, n)
	if start < end {
		// Simple case: copy from start to end+1 (inclusive of last element)
		copy(dest, mb.buffer[start:end+1])
	} else {
		// Complex case: start to final element
		m := copy(dest, mb.buffer[start:])
		// Next write position is index m: Copy zero to end+1
		copy(dest[m:], mb.buffer[0:end+1])
	}

	return dest
}

// Average calculates the average of all samples in the buffer
func (mb *MetricsBuffer) Average() float64 {
	// Thread-safe read
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	// Handle empty buffer case
	if mb.count == 0 {
		return 0
	}
	// Sum all valid elements and divide by count
	sum := 0.0
	// Iterate from the last written index down, wrapping around
	for i := range mb.count {
		pos := mb.writeIdx - 1 - i
		if pos < 0 {
			// Wrap around: Use a plus operation because it's already negative
			pos = mb.size + pos
		}
		sum += mb.buffer[pos]
	}
	return sum / (float64(mb.count))
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
	return &TimestampedMetricsBuffer{
		buffer:   make([]TimestampedSample, size),
		writeIdx: 0,
		count:    0,
		size:     size,
	}
}

func (tmb *TimestampedMetricsBuffer) Push(value float64) {
	// Same as MetricsBuffer but include timestamp
	// Thread-safe write
	tmb.mu.Lock()
	defer tmb.mu.Unlock()
	tmb.buffer[tmb.writeIdx] = TimestampedSample{
		Value:     value,
		Timestamp: time.Now(),
	}
	// Calculate write position
	tmb.writeIdx = (tmb.writeIdx + 1) % tmb.size
	// Update count (capped at size)
	if tmb.count < tmb.size {
		tmb.count++
	}
}

// GetWindow returns samples from the last duration
// Example: GetWindow(5 * time.Minute) returns all samples from last 5 minutes
func (tmb *TimestampedMetricsBuffer) GetWindow(duration time.Duration) []TimestampedSample {
	// Thread-safe read
	tmb.mu.RLock()
	defer tmb.mu.RUnlock()
	result := []TimestampedSample{}

	// Handle empty case
	if tmb.count == 0 {
		return result
	}

	// Like Average: iterate from the last written index down, wrapping around
	for i := range tmb.count {
		pos := tmb.writeIdx - 1 - i
		if pos < 0 {
			// Wrap around: Use a plus operation because it's already negative
			pos = tmb.size + pos
		}
		// Return only those where time.Since(sample.Timestamp) <= duration
		// Note: Old samples may be in buffer but outside the window
		sample := tmb.buffer[pos]
		if time.Since(sample.Timestamp) <= duration {
			result = append(result, sample)
		} else {
			// Since we iterate from newest to oldest there's no reason to continue
			// iteration. Previous entries are even older
			break
		}
	}

	return result
}
