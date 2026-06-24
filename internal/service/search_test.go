package service

import (
	"context"
	"testing"
)

// TestSearchMessages_NilPool verifies that SearchMessages returns an error
// when the Services has no pool configured.
func TestSearchMessages_NilPool(t *testing.T) {
	// Create a minimal Services with nil pool
	s := &Services{
		pool: nil,
	}

	ctx := context.Background()
	messages, err := s.SearchMessages(ctx, 12345, "test query")

	if err == nil {
		t.Fatal("expected error for nil pool, got nil")
	}

	// When there's an error, the result should be nil
	if messages != nil {
		t.Fatal("expected nil slice for nil pool with error")
	}
}
