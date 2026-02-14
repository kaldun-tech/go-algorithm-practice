package practice

import (
	"testing"
)

func TestNewFileBlockService(t *testing.T) {
	fbs := NewFileBlockService(4)

	if fbs.capacity != 4 {
		t.Errorf("expected capacity 4, got %d", fbs.capacity)
	}
	if fbs.used != 0 {
		t.Errorf("expected used 0, got %d", fbs.used)
	}
	if fbs.head == nil {
		t.Fatal("expected dummy head, got nil")
	}
	if fbs.head.Next == nil {
		t.Fatal("expected head.Next to point to first block")
	}

	// Verify chain is fully linked
	count := 0
	for b := fbs.head.Next; b != nil; b = b.Next {
		count++
	}
	if count != 4 {
		t.Errorf("expected 4 linked blocks, got %d", count)
	}
}

func TestWriteEmptyString(t *testing.T) {
	fbs := NewFileBlockService(4)

	err := fbs.Write("")
	if err != nil {
		t.Errorf("expected no error for empty string, got %v", err)
	}
	if fbs.used != 0 {
		t.Errorf("expected used to remain 0, got %d", fbs.used)
	}
}

func TestWriteSingleBlock(t *testing.T) {
	fbs := NewFileBlockService(4)

	err := fbs.Write("12345678")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fbs.used != 1 {
		t.Errorf("expected used 1, got %d", fbs.used)
	}
	if fbs.blocks[0].Data != "12345678" {
		t.Errorf("expected block data '12345678', got '%s'", fbs.blocks[0].Data)
	}
}

func TestWriteMultipleBlocks(t *testing.T) {
	fbs := NewFileBlockService(4)

	err := fbs.Write("12345678abcdefgh")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fbs.used != 2 {
		t.Errorf("expected used 2, got %d", fbs.used)
	}
	if fbs.blocks[0].Data != "12345678" {
		t.Errorf("expected first block '12345678', got '%s'", fbs.blocks[0].Data)
	}
	if fbs.blocks[1].Data != "abcdefgh" {
		t.Errorf("expected second block 'abcdefgh', got '%s'", fbs.blocks[1].Data)
	}
}

func TestWriteInvalidLengthTooShort(t *testing.T) {
	fbs := NewFileBlockService(4)

	err := fbs.Write("short")
	if err == nil {
		t.Error("expected error for length not multiple of 8")
	}
	if fbs.used != 0 {
		t.Errorf("expected used to remain 0 after error, got %d", fbs.used)
	}
}

func TestWriteInvalidLengthTooLong(t *testing.T) {
	fbs := NewFileBlockService(4)

	err := fbs.Write("123456789") // 9 bytes
	if err == nil {
		t.Error("expected error for length not multiple of 8")
	}
}

func TestWriteExceedsCapacity(t *testing.T) {
	fbs := NewFileBlockService(2)

	// Try to write 3 blocks worth of data
	err := fbs.Write("12345678abcdefghIJKLMNOP")
	if err == nil {
		t.Error("expected error when exceeding capacity")
	}
	if fbs.used != 0 {
		t.Errorf("expected used to remain 0 after error, got %d", fbs.used)
	}
}

func TestWriteFillsToCapacity(t *testing.T) {
	fbs := NewFileBlockService(2)

	err := fbs.Write("12345678abcdefgh")
	if err != nil {
		t.Errorf("should allow filling to exact capacity, got error: %v", err)
	}
	if fbs.used != 2 {
		t.Errorf("expected used 2, got %d", fbs.used)
	}
}

func TestWriteMultipleCalls(t *testing.T) {
	fbs := NewFileBlockService(4)

	err := fbs.Write("12345678")
	if err != nil {
		t.Errorf("first write failed: %v", err)
	}

	err = fbs.Write("abcdefgh")
	if err != nil {
		t.Errorf("second write failed: %v", err)
	}

	if fbs.used != 2 {
		t.Errorf("expected used 2, got %d", fbs.used)
	}
	if fbs.blocks[0].Data != "12345678" {
		t.Errorf("expected first block '12345678', got '%s'", fbs.blocks[0].Data)
	}
	if fbs.blocks[1].Data != "abcdefgh" {
		t.Errorf("expected second block 'abcdefgh', got '%s'", fbs.blocks[1].Data)
	}
}

func TestWriteAfterPartialFillExceedsCapacity(t *testing.T) {
	fbs := NewFileBlockService(2)

	err := fbs.Write("12345678")
	if err != nil {
		t.Errorf("first write failed: %v", err)
	}

	// Try to write 2 more blocks when only 1 slot remains
	err = fbs.Write("abcdefghIJKLMNOP")
	if err == nil {
		t.Error("expected error when second write exceeds remaining capacity")
	}
	if fbs.used != 1 {
		t.Errorf("expected used to remain 1 after failed write, got %d", fbs.used)
	}
}
