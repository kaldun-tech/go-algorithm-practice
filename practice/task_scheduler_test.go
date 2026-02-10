package practice

import (
	"context"
	"fmt"
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
	first, ok := ts.GetNextTask()
	if !ok || first.ID != "first" {
		t.Errorf("expected first, got %s", first.ID)
	}

	second, ok := ts.GetNextTask()
	if !ok || second.ID != "second" {
		t.Errorf("expected second, got %s", second.ID)
	}

	third, ok := ts.GetNextTask()
	if !ok || third.ID != "third" {
		t.Errorf("expected third, got %s", third.ID)
	}
}

func TestTaskHeap_TimeTakesPrecedenceOverPriority(t *testing.T) {
	ts := NewTaskScheduler()
	now := time.Now()

	// High priority but later time should still come second
	ts.AddTask(Task{ID: "later-high-pri", ExecuteAt: now.Add(time.Second), Priority: 1})
	ts.AddTask(Task{ID: "sooner-low-pri", ExecuteAt: now, Priority: 100})

	first, _ := ts.GetNextTask()
	if first.ID != "sooner-low-pri" {
		t.Errorf("expected time to take precedence, got %s first", first.ID)
	}
}

func TestTaskHeap_SameExecuteTime_PriorityBreaksTie(t *testing.T) {
	ts := NewTaskScheduler()
	now := time.Now()

	// Add multiple tasks at exact same time with different priorities
	// Lower priority number = higher priority (executes first)
	ts.AddTask(Task{ID: "low", ExecuteAt: now, Priority: 10})
	ts.AddTask(Task{ID: "high", ExecuteAt: now, Priority: 1})
	ts.AddTask(Task{ID: "medium", ExecuteAt: now, Priority: 5})

	// Should come out in priority order (lowest number first)
	expected := []string{"high", "medium", "low"}
	for i, exp := range expected {
		task, ok := ts.GetNextTask()
		if !ok || task == nil {
			t.Fatalf("expected task, got nil on iteration %d", i)
		}
		if task.ID != exp {
			t.Errorf("iteration %d: expected %s, got %s", i, exp, task.ID)
		}
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

func TestGetNextTask_EmptyHeap(t *testing.T) {
	// What should happen when GetNextTask is called on an empty heap?
	// Previous implementation paniced. Decided to return (Task, bool) like heap.go does
	ts := NewTaskScheduler()
	task, ok := ts.GetNextTask()
	if task != nil || ok {
		t.Errorf("Unexpected return for GetNextTask with empty task list: %v, %t", task, ok)
	}
}

func TestRun_NewEarlierTaskPreemptsTimer(t *testing.T) {
	// Verify that adding a sooner task while waiting wakes up Run()
	// Setup:
	// 1. Add a task scheduled for 1 second from now
	// 2. Start Run() in a goroutine
	// 3. After 50ms, add a new task scheduled for 100ms from now
	// 4. Verify the second task executes first (before the 1 second mark)
	// This tests that the signal channel properly interrupts the timer
	ts := NewTaskScheduler()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executed := make(chan string, 1)
	ts.AddTask(Task{
		ID:        "later",
		ExecuteAt: time.Now().Add(time.Second),
		Callback: func() {
			executed <- "later"
		},
	})

	go ts.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	ts.AddTask(Task{
		ID:        "sooner",
		ExecuteAt: time.Now().Add(100 * time.Millisecond),
		Callback: func() {
			executed <- "sooner"
		},
	})

	execSooner, execLater := false, false
	for range 2 {
		select {
		case id := <-executed:
			if !execSooner {
				execSooner = (id == "sooner")
				if !execSooner {
					t.Errorf("expected sooner, got %s", id)
				}
			} else if !execLater {
				execLater = (id == "later")
				if !execLater {
					t.Errorf("expected later, got %s", id)
				}
			} else {
				t.Errorf("unexpected ID, got %s", id)
			}
		case <-time.After(2 * time.Second):
			t.Error("tasks were not executed within expected time")
		}
	}
}

func TestRun_ConcurrentAddTask(t *testing.T) {
	// Verify thread safety under concurrent AddTask calls
	// Setup:
	// 1. Create scheduler and start Run()
	// 2. Spawn N goroutines that each add M tasks
	// 3. Verify all N*M tasks execute exactly once
	// 4. Use atomic counter or sync.WaitGroup to track
	// Hints:
	// - var executed int64; atomic.AddInt64(&executed, 1)
	// - Use t.Errorf if final count != expected
	ts := NewTaskScheduler()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var executed int64 = 0
	var n, m int64 = 5, 7

	for i := range n {
		wg.Add(1)
		go func(workerID int64) {
			defer wg.Done()
			fmt.Println("Worker", workerID)
			for j := range m {
				id := fmt.Sprintf("Task %d %d", i, j)
				ts.AddTask(Task{
					ID:        id,
					ExecuteAt: time.Now().Add(100 * time.Millisecond),
					Callback: func() {
						atomic.AddInt64(&executed, 1)
					},
				})
			}
		}(i)
	}
	wg.Wait()

	go ts.Run(ctx)
	time.Sleep(time.Second)
	if executed != n*m {
		t.Errorf("unexpected n * m = %d executions, got %d", n*m, executed)
	}
}

func TestRun_CallbackPanicDoesNotCrashScheduler(t *testing.T) {
	// Verify that a panicking callback doesn't kill Run()
	// Setup:
	// 1. Add task with Callback that panics
	// 2. Add task with normal Callback after it
	// 3. Verify second task still executes
	ts := NewTaskScheduler()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executed := make(chan string, 1)
	ts.AddTask(Task{
		ID:        "panics",
		ExecuteAt: time.Now().Add(100 * time.Millisecond),
		Callback: func() {
			panic("Test panic")
		},
	})
	ts.AddTask(Task{
		ID:        "normal",
		ExecuteAt: time.Now().Add(100 * time.Millisecond),
		Callback: func() {
			executed <- "normal"
		},
	})

	go ts.Run(ctx)
	select {
	case id := <-executed:
		if id != "normal" {
			t.Errorf("expected task normal, got %s", id)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("past task was not executed")
	}
}

// =============================================================================
// Helpers
// =============================================================================

func TestTaskScheduler_Len(t *testing.T) {
	ts := NewTaskScheduler()

	if ts.Len() != 0 {
		t.Errorf("expected 0, got %d", ts.Len())
	}

	ts.AddTask(Task{ID: "a", ExecuteAt: time.Now()})
	ts.AddTask(Task{ID: "b", ExecuteAt: time.Now()})

	if ts.Len() != 2 {
		t.Errorf("expected 2, got %d", ts.Len())
	}
}
