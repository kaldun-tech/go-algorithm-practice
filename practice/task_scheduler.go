package practice

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"
)

// Think through:
// 1. What struct holds your heap?
// https://pkg.go.dev/container/heap
// A: Can use the containers/heap package out of the box. Wrap a slice (usually []Task or []*Task) and implement the interface on it.

// 2. How do you implement heap.Interface (Len, Less, Swap, Push, Pop)?
// A: We implement implicitly via Go's duck typing. Implement the 5 declared methods

// 3. What does Less() compare - just time, or time + priority?
// A: Start with time, override to add prio later:
// if t1.ExecuteAt.Equal(t2.ExecuteAt) {
//    return t1.Priority < t2.Priority
// }
// return t1.ExecuteAt.Before(t2.ExecuteAt)

// 4. Thread-safety: where do you need locks?
// A: Assuming multi-threading locks become needed when reading and writing the data. In practice a regular Mutex often suffices here because:
// - heap.Pop and heap.Push both mutate the heap
// - GetNextTask likely needs to pop (write), not just peek
// - The read-heavy pattern doesn't apply well to heaps

// - Run loop timing: How does Run() sleep efficiently? A naive time.Sleep(100ms) polls unnecessarily.
// Consider sleeping until the next task's ExecuteAt time, or using
// a time.Timer that resets when a new sooner task arrives.
// - Cancellation: Should AddTask return a cancel handle? If a task is scheduled for 10 minutes out, can it be removed?
// - What does GetNextTask do when nothing is due? Block? Return nil? This affects your API design significantly.
// - Panic safety: If Callback() panics, does it crash the scheduler? You may want defer recover() in the run loop.

type Task struct {
	ID        string
	ExecuteAt time.Time
	Priority  uint // Priority 0 should be executed first
	Callback  func()
}

type TaskHeap []Task

func (h *TaskHeap) Len() int { return len(*h) }

// Detects whether Task at position i is less than position j
// Ordered first by execution time, then by priority
func (h *TaskHeap) Less(i, j int) bool {
	a := (*h)[i]
	b := (*h)[j]
	return a.ExecuteAt.Before(b.ExecuteAt) ||
		(a.ExecuteAt.Equal(b.ExecuteAt) && a.Priority < b.Priority)
}

func (h *TaskHeap) Swap(i, j int) {
	(*h)[i], (*h)[j] = (*h)[j], (*h)[i]
}

// Appends the incoming element to the end of the heap
func (h *TaskHeap) Push(t any) {
	// Push and Pop use pointer receivers because they modify the slice's length, not just its contents.
	*h = append(*h, t.(Task))
}

// Removes strictly the last element, the heap handles sifting and swapping with the root internally
func (h *TaskHeap) Pop() any {
	oldTH := *h
	n := len(oldTH)
	last := oldTH[n-1]
	*h = oldTH[0 : n-1]
	return last
}

type TaskScheduler struct {
	tasks  TaskHeap // Value type not pointer
	mu     sync.Mutex
	signal chan struct{} // wake Run() when task added
}

func NewTaskScheduler() *TaskScheduler {
	ts := &TaskScheduler{
		tasks:  make(TaskHeap, 0),
		signal: make(chan struct{}),
	}
	heap.Init(&ts.tasks)
	return ts
}

func (ts *TaskScheduler) Len() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.tasks.Len()
}

// Use a non-blocking send outside the lock
func (ts *TaskScheduler) AddTask(t Task) {
	ts.mu.Lock()
	heap.Push(&ts.tasks, t)
	ts.mu.Unlock()

	// Non-blocking send - if Run() is busy, it will re-check the heap anyway
	select {
	case ts.signal <- struct{}{}:
	default:
	}
}

func (ts *TaskScheduler) GetNextTask() (*Task, bool) {
	// Use internal Len rather than public Len to avoid double locking
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if len(ts.tasks) == 0 {
		return nil, false
	}

	task := heap.Pop(&ts.tasks).(Task)
	return &task, true
}

func (ts *TaskScheduler) ExecuteNextTask() {
	// Pop the next task (with lock)
	ts.mu.Lock()
	if ts.tasks.Len() == 0 {
		ts.mu.Unlock()
		return
	}
	task := heap.Pop(&ts.tasks).(Task)
	ts.mu.Unlock()

	// Execute the callback (without lock, so other tasks can be added)
	if task.Callback != nil {
		defer func() {
			// Recover from panics so the scheduler keeps running
			if r := recover(); r != nil {
				fmt.Println("task panicked:", task.ID, r)
			}
		}()
		task.Callback()
	}
}

func (ts *TaskScheduler) Run(ctx context.Context) {
	for {
		// Use internal Len rather than public to avoid double locking
		ts.mu.Lock()
		if ts.tasks.Len() == 0 {
			// Nothing scheduled -> wait for signal or cancellation
			ts.mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-ts.signal:
				// Restart loop and check again
				continue
			}
		}

		// Peek at next task (don't pop yet)
		next := ts.tasks[0]
		delay := time.Until(next.ExecuteAt)
		ts.mu.Unlock()

		if delay <= 0 {
			// Due now -> execute it
			ts.ExecuteNextTask()
			continue
		}

		// Wait until it's due, or something else happens
		// Defer doesn't fit well here because we would leak timers and deferred calls
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-ts.signal:
			timer.Stop()
			continue // new task added, recalculate
		case <-timer.C:
			ts.ExecuteNextTask()
		}
	}
}
