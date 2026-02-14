package practice

import "fmt"

// Think through:
// 1. How do blocks link together?
// A next pointer is sufficient. If we needed to iterate backwards a previous pointer would help also.
// 2. How do you track which blocks are available vs used?
// The used count represents a write index of the next empty block to write to
// 3. What happens when Write() is called with data longer than 8 bytes?
// We return an error if the data has remainder when its length is divided by 8, or would overfill the capacity we want to return an error.
// Can treat an empty string as a no operation case
// 4. How do you find the next available block efficiently?
// The used count gives us the next index to write to

const BlockDataSize = 8

type Block struct {
	Data string
	Next *Block
}

type FileBlockService struct {
	blocks   []Block
	capacity int
	used     int
	head     *Block
}

func NewFileBlockService(capacity int) *FileBlockService {
	fbs := &FileBlockService{
		blocks:   make([]Block, capacity),
		capacity: capacity,
		used:     0,
	}

	// Set up the block linking by iterating backwards
	for i := capacity - 1; 0 <= i; i-- {
		if i < capacity-1 {
			// Link to next
			fbs.blocks[i] = Block{Next: &fbs.blocks[i+1]}
		} else {
			fbs.blocks[i] = Block{}
		}
	}
	// Use a dummy head to simplify writing
	fbs.head = &Block{Next: &fbs.blocks[0]}

	return fbs
}

// Print outputs a string representation of the block chain to the console
// In format [data1] -> [data2] -> [data3] where data items may be empty
func (fbs *FileBlockService) Print() {
	// Walk the chain from dummy head and print each block's data
	for b := fbs.head.Next; b != nil; b = b.Next {
		fmt.Print("[", b.Data, "]")
		if b.Next != nil {
			fmt.Print(" -> ")
		}
	}
	fmt.Println()
}

// Write adds non-empty data items in 8 byte increments to available blocks
// Returns error if capacity would be exceeded, nil on success
func (fbs *FileBlockService) Write(data string) error {
	if data == "" {
		// Nothing to do
		return nil
	}
	n := len(data)
	if n%8 != 0 {
		return fmt.Errorf("Invalid length %d", n)
	}

	toAdd := n / 8
	if fbs.capacity < toAdd+fbs.used {
		return fmt.Errorf("Cannot add %d blocks with %d used and capacity %d", toAdd, fbs.used, fbs.capacity)
	}

	// Find next available block
	for i := range toAdd {
		// Create next block from data slice
		start := i * 8
		end := (i + 1) * 8
		fbs.blocks[fbs.used].Data = data[start:end]
		// Link to end of chain and update used count
		fbs.used++
	}
	return nil
}
