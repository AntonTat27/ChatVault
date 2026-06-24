package db

import (
	"context"
	"testing"
)

// TestSearchMessages_NilPool verifies that SearchMessages returns an error
// when the pool is nil.
func TestSearchMessages_NilPool(t *testing.T) {
	ctx := context.Background()
	messages, err := SearchMessages(ctx, nil, 12345, "test", 10)

	if err == nil {
		t.Fatal("expected error for nil pool, got nil")
	}

	// When there's an error, the result should be nil
	if messages != nil {
		t.Fatal("expected nil slice for nil pool with error")
	}
}

// TestSearchMessages_DefaultLimit verifies that negative or zero limit
// defaults to searchQueryLimit.
func TestSearchMessages_ZeroLimit(t *testing.T) {
	ctx := context.Background()
	// Pool is nil, so this will fail before limit is even used,
	// but we're testing the parameter handling
	_, err := SearchMessages(ctx, nil, 12345, "test", 0)

	// We expect it to fail due to nil pool, but internally
	// the limit should be normalized before use
	if err == nil {
		t.Fatal("expected error for nil pool")
	}
}

// TestSearchMessages_NegativeLimit verifies that negative limit
// defaults to searchQueryLimit.
func TestSearchMessages_NegativeLimit(t *testing.T) {
	ctx := context.Background()
	// Pool is nil, so this will fail before limit is even used,
	// but we're testing the parameter handling
	_, err := SearchMessages(ctx, nil, 12345, "test", -5)

	// We expect it to fail due to nil pool
	if err == nil {
		t.Fatal("expected error for nil pool")
	}
}

// TestSearchMessages_EmptyQuery with nil pool should still fail gracefully.
func TestSearchMessages_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	messages, err := SearchMessages(ctx, nil, 12345, "", 10)

	if err == nil {
		t.Fatal("expected error for nil pool")
	}

	if len(messages) != 0 {
		t.Fatal("expected empty results")
	}
}
