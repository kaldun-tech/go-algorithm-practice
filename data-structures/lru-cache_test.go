package datastructures

import "testing"

func TestLRUCache_BasicOperations(t *testing.T) {
	cache := NewLRUCache(2)

	// Put and get basic operations
	cache.Put(1, 1)
	if got := cache.Get(1); got != 1 {
		t.Errorf("Get(1) = %d, want 1", got)
	}

	cache.Put(2, 2)
	if got := cache.Get(1); got != 1 {
		t.Errorf("Get(1) = %d, want 1", got)
	}
	if got := cache.Get(2); got != 2 {
		t.Errorf("Get(2) = %d, want 2", got)
	}

	// Eviction test - capacity is 2
	cache.Put(3, 3) // Evicts key 1
	if got := cache.Get(1); got != -1 {
		t.Errorf("Get(1) = %d, want -1 (evicted)", got)
	}
	if got := cache.Get(2); got != 2 {
		t.Errorf("Get(2) = %d, want 2", got)
	}
	if got := cache.Get(3); got != 3 {
		t.Errorf("Get(3) = %d, want 3", got)
	}
}

func TestLRUCache_UpdateExisting(t *testing.T) {
	cache := NewLRUCache(2)

	cache.Put(1, 1)
	cache.Put(2, 2)
	cache.Put(1, 10) // Update key 1

	if got := cache.Get(1); got != 10 {
		t.Errorf("Get(1) = %d, want 10", got)
	}

	// Key 1 was updated (moved to front), so key 2 should be evicted next
	cache.Put(3, 3)
	if got := cache.Get(2); got != -1 {
		t.Errorf("Get(2) = %d, want -1 (should be evicted)", got)
	}
	if got := cache.Get(1); got != 10 {
		t.Errorf("Get(1) = %d, want 10", got)
	}
	if got := cache.Get(3); got != 3 {
		t.Errorf("Get(3) = %d, want 3", got)
	}
}

func TestLRUCache_AccessUpdatesRecency(t *testing.T) {
	cache := NewLRUCache(2)

	cache.Put(1, 1)
	cache.Put(2, 2)
	cache.Get(1) // Access key 1, makes it recently used

	// Now key 2 is LRU, so it should be evicted
	cache.Put(3, 3)
	if got := cache.Get(2); got != -1 {
		t.Errorf("Get(2) = %d, want -1 (should be evicted)", got)
	}
	if got := cache.Get(1); got != 1 {
		t.Errorf("Get(1) = %d, want 1", got)
	}
	if got := cache.Get(3); got != 3 {
		t.Errorf("Get(3) = %d, want 3", got)
	}
}

func TestLRUCache_GetNonExistent(t *testing.T) {
	cache := NewLRUCache(2)

	if got := cache.Get(1); got != -1 {
		t.Errorf("Get(1) on empty cache = %d, want -1", got)
	}

	cache.Put(1, 1)
	if got := cache.Get(2); got != -1 {
		t.Errorf("Get(2) for non-existent key = %d, want -1", got)
	}
}

func TestLRUCache_CapacityOne(t *testing.T) {
	cache := NewLRUCache(1)

	cache.Put(1, 1)
	if got := cache.Get(1); got != 1 {
		t.Errorf("Get(1) = %d, want 1", got)
	}

	cache.Put(2, 2) // Evicts key 1
	if got := cache.Get(1); got != -1 {
		t.Errorf("Get(1) = %d, want -1 (evicted)", got)
	}
	if got := cache.Get(2); got != 2 {
		t.Errorf("Get(2) = %d, want 2", got)
	}

	cache.Put(2, 20) // Update same key
	if got := cache.Get(2); got != 20 {
		t.Errorf("Get(2) = %d, want 20", got)
	}
}

func TestLRUCache_LeetCodeExample(t *testing.T) {
	// This is the classic LeetCode example
	cache := NewLRUCache(2)

	cache.Put(1, 1)
	cache.Put(2, 2)
	if got := cache.Get(1); got != 1 {
		t.Errorf("Get(1) = %d, want 1", got)
	}

	cache.Put(3, 3) // Evicts key 2
	if got := cache.Get(2); got != -1 {
		t.Errorf("Get(2) = %d, want -1", got)
	}

	cache.Put(4, 4) // Evicts key 1
	if got := cache.Get(1); got != -1 {
		t.Errorf("Get(1) = %d, want -1", got)
	}
	if got := cache.Get(3); got != 3 {
		t.Errorf("Get(3) = %d, want 3", got)
	}
	if got := cache.Get(4); got != 4 {
		t.Errorf("Get(4) = %d, want 4", got)
	}
}

func TestLRUCache_MultipleUpdates(t *testing.T) {
	cache := NewLRUCache(2)

	cache.Put(2, 1)
	cache.Put(2, 2) // Update same key immediately
	if got := cache.Get(2); got != 2 {
		t.Errorf("Get(2) = %d, want 2", got)
	}

	cache.Put(1, 1)
	cache.Put(4, 1) // Evicts key 2
	if got := cache.Get(2); got != -1 {
		t.Errorf("Get(2) = %d, want -1", got)
	}
}

// Benchmark to verify O(1) operations
func BenchmarkLRUCache_Put(b *testing.B) {
	cache := NewLRUCache(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(i%1000, i)
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	cache := NewLRUCache(1000)
	for i := 0; i < 1000; i++ {
		cache.Put(i, i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(i % 1000)
	}
}

func BenchmarkLRUCache_Mixed(b *testing.B) {
	cache := NewLRUCache(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			cache.Put(i%1000, i)
		} else {
			cache.Get(i % 1000)
		}
	}
}
