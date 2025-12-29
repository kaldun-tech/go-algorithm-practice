package datastructures

// LRUCache implements a Least Recently Used cache with O(1) get and put operations.
// This is a common interview question that tests understanding of hash maps and doubly linked lists.
//
// Typical interview requirements:
// - Get(key) - Get value from cache, return -1 if not exists
// - Put(key, value) - Insert or update key-value pair
// - When cache reaches capacity, evict least recently used item before inserting new item
// - Both operations should be O(1)
//
// Implementation approach:
// - HashMap for O(1) lookup
// - Doubly linked list to track recency (most recent at head, least recent at tail)
// - Move nodes to head on access
// - Evict from tail when full
type LRUCache struct {
	capacity int
	cache    map[int]*listNode
	// Most recently used
	head *listNode
	// Least recently used
	tail *listNode
}

type listNode struct {
	key   int
	value int
	prev  *listNode
	next  *listNode
}

// NewLRUCache creates a new LRU cache with given capacity
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[int]*listNode),
		// prev and next are nil
	}
}

// Get retrieves a value from the cache
// Returns the value if key exists, -1 otherwise
// Marks the key as recently used
func (c *LRUCache) Get(key int) int {
	n, exists := c.cache[key]
	if !exists {
		return -1
	}
	c.moveToHead(n)
	return n.value
}

// Put inserts or updates a key-value pair
// Evicts least recently used item if at capacity
func (c *LRUCache) Put(key int, value int) {
	n, exists := c.cache[key]
	if exists {
		// Already here, just update value and move to head
		n.value = value
		c.moveToHead(n)
	} else {
		// Add new node as head and to cache
		n = &listNode{
			key:   key,
			value: value,
		}
		c.cache[key] = n
		c.addHead(n)
		// Check for overflow
		if len(c.cache) > c.capacity {
			// Evict previous tail
			tail := c.popTail()
			delete(c.cache, tail.key)
		}
	}
}

// Adds a node as head
func (c *LRUCache) addHead(n *listNode) {
	// Link to existing head as next if any
	n.prev = nil
	n.next = c.head
	if c.head != nil {
		c.head.prev = n
	}
	c.head = n
	// Set tail if needed
	if c.tail == nil {
		c.tail = n
	}
}

// Move an existing node to head
func (c *LRUCache) moveToHead(n *listNode) {
	if n == c.head {
		// Nothing to do
		return
	}
	if n.prev != nil {
		// Remove previous link to this
		n.prev.next = n.next
	}
	if n.next != nil {
		// Remove next link to this
		n.next.prev = n.prev
	}
	if n == c.tail {
		// Move tail back
		c.tail = n.prev
	}

	n.prev = nil
	n.next = c.head
	if c.head != nil {
		// Shift existing head forward
		c.head.prev = n
	}
	c.head = n
}

// Removes current tail
func (c *LRUCache) popTail() *listNode {
	if c.tail == nil {
		// Nothing to do
		return nil
	}
	n := c.tail
	if c.tail.prev != nil {
		// Previous node becomes tail
		c.tail.prev.next = nil
	} else {
		// Empty
		c.head = nil
	}
	c.tail = c.tail.prev
	return n
}
