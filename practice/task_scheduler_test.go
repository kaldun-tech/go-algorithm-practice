package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Heap ordering tests
// =============================================================================

func TestTaskHeap_OrdersByExecuteTime(t *testing.T) {
	ts := NewTaskScheduler()
	now := time.Now()

	// Add tasks out of order
	ts.AddTask(Task{ID: "third", ExecuteAt: now.Add(30 * time.Second)})
	ts.AddTask(Task{ID: "first", ExecuteAt: now.Add(10 * time.Second)})
	ts.AddTask(Task{ID: "second", ExecuteAt: now.Add(20 * time.Second)})

	// Should come out in time order
	first := ts.GetNextTask()
	if first.ID != "first" {
		t.Errorf("expected first, got %s", first.ID)
	}

	second := ts.GetNextTask()
	if second.ID != "second" {
		t.Errorf("expected second, got %s", second.ID)
	}

	third := ts.GetNextTask()
	if third.ID != "third" {
		t.Errorf("expected third, got %s", third.ID)
	}
}

func TestTaskHeap_SameExecuteTime(t *testing.T) {
	ts := NewTaskScheduler()
	now := time.Now()

	// Add multiple tasks at exact same time
	ts.AddTask(Task{ID: "a", ExecuteAt: now})
	ts.AddTask(Task{ID: "b", ExecuteAt: now})
	ts.AddTask(Task{ID: "c", ExecuteAt: now})

	// All three should be retrievable (order among them is unspecified)
	seen := make(map[string]bool)
	for i := 0; i < 3; i++ {
		task := ts.GetNextTask()
		if task == nil {
			t.Fatalf("expected task, got nil on iteration %d", i)
		}
		seen[task.ID] = true
	}

	if len(seen) != 3 {
		t.Errorf("expected 3 unique tasks, got %d", len(seen))
	}
}

// =============================================================================
// Run() lifecycle tests
// =============================================================================

func TestRun_ExecutesTaskAtCorrectTime(t *testing.T) {
	ts := NewTaskScheduler()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executed := make(chan string, 1)
	ts.AddTask(Task{
		ID:        "delayed",
		ExecuteAt: time.Now().Add(100 * time.Millisecond),
		Callback: func() {
			executed <- "delayed"
		},
	})

	go ts.Run(ctx)

	select {
	case id := <-executed:
		if id != "delayed" {
			t.Errorf("expected delayed, got %s", id)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("task was not executed within expected time")
	}
}

func TestRun_ContextCancellationStopsLoop(t *testing.T) {
	ts := NewTaskScheduler()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		ts.Run(ctx)
		close(done)
	}()

	// Give Run() time to start and block
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success - Run() exited
	case <-time.After(500 * time.Millisecond):
		t.Error("Run() did not exit after context cancellation")
	}
}

func TestRun_PastTaskExecutesImmediately(t *testing.T) {
	ts := NewTaskScheduler()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	executed := make(chan time.Time, 1)
	before := time.Now()

	ts.AddTask(Task{
		ID:        "past",
		ExecuteAt: time.Now().Add(-1 * time.Hour), // In the past
		Callback: func() {
			executed <- time.Now()
		},
	})

	go ts.Run(ctx)

	select {
	case execTime := <-executed:
		elapsed := execTime.Sub(before)
		if elapsed > 100*time.Millisecond {
			t.Errorf("past task took too long to execute: %v", elapsed)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("past task was not executed")
	}
}

// =============================================================================
// TODO: Implement these tests tomorrow
// =============================================================================

func TestGetNextTask_EmptyHeap(t *testing.T) {
	// TODO: What should happen when GetNextTask is called on an empty heap?
	// Current implementation will panic. Decide on behavior:
	// Option A: Return nil
	// Option B: Return (Task, bool) like your heap.go does
	// Option C: Block until a task is available
	t.Skip("TODO: Implement - decide on empty heap behavior")
}

func TestRun_NewEarlierTaskPreemptsTimer(t *testing.T) {
	// TODO: Verify that adding a sooner task while waiting wakes up Run()
	//
	// Setup:
	// 1. Add a task scheduled for 1 second from now
	// 2. Start Run() in a goroutine
	// 3. After 50ms, add a new task scheduled for 100ms from now
	// 4. Verify the second task executes first (before the 1 second mark)
	//
	// This tests that the signal channel properly interrupts the timer
	t.Skip("TODO: Implement - test timer preemption")
}

func TestRun_ConcurrentAddTask(t *testing.T) {
	// TODO: Verify thread safety under concurrent AddTask calls
	//
	// Setup:
	// 1. Create scheduler and start Run()
	// 2. Spawn N goroutines that each add M tasks
	// 3. Verify all N*M tasks execute exactly once
	// 4. Use atomic counter or sync.WaitGroup to track
	//
	// Hints:
	// - var executed int64; atomic.AddInt64(&executed, 1)
	// - Use t.Errorf if final count != expected
	t.Skip("TODO: Implement - test concurrent safety")
}

func TestRun_CallbackPanicDoesNotCrashScheduler(t *testing.T) {
	// TODO: Verify that a panicking callback doesn't kill Run()
	//
	// Setup:
	// 1. Add task with Callback that panics
	// 2. Add task with normal Callback after it
	// 3. Verify second task still executes
	//
	// Note: This will fail until you add recover() to executeNext()
	t.Skip("TODO: Implement - test panic recovery")
}

// =============================================================================
// Helpers
// =============================================================================

func TestTaskScheduler_Len(t *testing.T) {
	ts := NewTaskScheduler()

	if ts.tasks.Len() != 0 {
		t.Errorf("expected 0, got %d", ts.tasks.Len())
	}

	ts.AddTask(Task{ID: "a", ExecuteAt: time.Now()})
	ts.AddTask(Task{ID: "b", ExecuteAt: time.Now()})

	if ts.tasks.Len() != 2 {
		t.Errorf("expected 2, got %d", ts.tasks.Len())
	}
}

// Silence unused import errors for atomic (used in TODO tests)
var _ = atomic.AddInt64
var _ = sync.WaitGroup{}
