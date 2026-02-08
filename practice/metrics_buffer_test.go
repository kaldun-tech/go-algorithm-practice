package practice

import (
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Basic functionality tests
// =============================================================================

func TestMetricsBuffer_PushToNonFullBuffer(t *testing.T) {
	mb := NewMetricsBuffer(5)

	mb.Push(1.0)
	mb.Push(2.0)
	mb.Push(3.0)

	recent := mb.GetRecent(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(recent))
	}

	// Should be in order: oldest to newest or newest to oldest?
	// Define your expected behavior and test for it
}

func TestMetricsBuffer_PushWhenFullOverwrites(t *testing.T) {
	mb := NewMetricsBuffer(3)

	mb.Push(1.0)
	mb.Push(2.0)
	mb.Push(3.0)
	mb.Push(4.0) // Should overwrite 1.0

	recent := mb.GetRecent(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(recent))
	}

	// Verify 1.0 is gone and 4.0 is present
	found1, found4 := false, false
	for _, v := range recent {
		if v == 1.0 {
			found1 = true
		}
		if v == 4.0 {
			found4 = true
		}
	}

	if found1 {
		t.Error("1.0 should have been overwritten")
	}
	if !found4 {
		t.Error("4.0 should be in buffer")
	}
}

// =============================================================================
// GetRecent tests
// =============================================================================

func TestMetricsBuffer_GetRecentMoreThanSize(t *testing.T) {
	mb := NewMetricsBuffer(3)

	mb.Push(1.0)
	mb.Push(2.0)

	// Request more than available
	recent := mb.GetRecent(10)
	if len(recent) != 2 {
		t.Errorf("expected 2 elements (all available), got %d", len(recent))
	}
}

func TestMetricsBuffer_GetRecentEmpty(t *testing.T) {
	mb := NewMetricsBuffer(3)

	recent := mb.GetRecent(5)
	if len(recent) != 0 {
		t.Errorf("expected 0 elements for empty buffer, got %d", len(recent))
	}
}

func TestMetricsBuffer_GetRecentReturnsCorrectOrder(t *testing.T) {
	mb := NewMetricsBuffer(5)

	mb.Push(1.0)
	mb.Push(2.0)
	mb.Push(3.0)
	mb.Push(4.0)
	mb.Push(5.0)

	recent := mb.GetRecent(3)

	// Define expected order: should be [3, 4, 5] or [5, 4, 3]?
	// Test for your chosen behavior
	_ = recent
}

// =============================================================================
// Average tests
// =============================================================================

func TestMetricsBuffer_Average(t *testing.T) {
	mb := NewMetricsBuffer(5)

	mb.Push(10.0)
	mb.Push(20.0)
	mb.Push(30.0)

	avg := mb.Average()
	expected := 20.0
	if avg != expected {
		t.Errorf("expected average %f, got %f", expected, avg)
	}
}

func TestMetricsBuffer_AverageEmpty(t *testing.T) {
	mb := NewMetricsBuffer(5)

	avg := mb.Average()
	// What should empty buffer return? 0? NaN?
	if avg != 0 {
		t.Errorf("expected 0 for empty buffer, got %f", avg)
	}
}

func TestMetricsBuffer_AverageAfterOverwrite(t *testing.T) {
	mb := NewMetricsBuffer(3)

	mb.Push(100.0)
	mb.Push(100.0)
	mb.Push(100.0)
	mb.Push(10.0) // Overwrites first 100.0

	avg := mb.Average()
	expected := 70.0 // (100 + 100 + 10) / 3
	if avg != expected {
		t.Errorf("expected average %f, got %f", expected, avg)
	}
}

// =============================================================================
// Concurrency tests
// =============================================================================

func TestMetricsBuffer_ConcurrentPushAndRead(t *testing.T) {
	mb := NewMetricsBuffer(100)
	var wg sync.WaitGroup

	// Multiple writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				mb.Push(float64(id*100 + j))
			}
		}(i)
	}

	// Concurrent reader
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = mb.GetRecent(10)
			_ = mb.Average()
		}
	}()

	wg.Wait()

	// If we get here without race detector complaints, concurrency is handled
	recent := mb.GetRecent(100)
	if len(recent) != 100 {
		t.Errorf("expected 100 elements after concurrent writes, got %d", len(recent))
	}
}

// =============================================================================
// Challenge: Timestamped buffer tests
// =============================================================================

func TestTimestampedBuffer_GetWindow(t *testing.T) {
	tmb := NewTimestampedMetricsBuffer(100)

	// Push some samples
	tmb.Push(1.0)
	time.Sleep(10 * time.Millisecond)
	tmb.Push(2.0)
	time.Sleep(10 * time.Millisecond)
	tmb.Push(3.0)

	// Get samples from last 50ms (should include all)
	window := tmb.GetWindow(50 * time.Millisecond)
	if len(window) != 3 {
		t.Errorf("expected 3 samples in 50ms window, got %d", len(window))
	}

	// Get samples from last 5ms (should include only most recent)
	window = tmb.GetWindow(5 * time.Millisecond)
	if len(window) != 1 {
		t.Errorf("expected 1 sample in 5ms window, got %d", len(window))
	}
}

func TestTimestampedBuffer_OldSamplesExpire(t *testing.T) {
	tmb := NewTimestampedMetricsBuffer(10)

	tmb.Push(1.0)
	time.Sleep(30 * time.Millisecond)
	tmb.Push(2.0)

	// Only recent sample should be in window
	window := tmb.GetWindow(20 * time.Millisecond)
	if len(window) != 1 {
		t.Errorf("expected 1 sample (old one expired), got %d", len(window))
	}
}
