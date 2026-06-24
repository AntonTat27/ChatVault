package db

import (
	"context"
	"testing"
)

// TestUpsertMessageEmbedding_NilPool verifies that UpsertMessageEmbedding
// returns an error when the pool is nil.
func TestUpsertMessageEmbedding_NilPool(t *testing.T) {
	ctx := context.Background()
	err := UpsertMessageEmbedding(ctx, nil, 1, 12345, []float32{0.1, 0.2, 0.3}, "text-embedding-004")

	if err == nil {
		t.Fatal("expected error for nil pool, got nil")
	}
}

// TestListMessagesMissingEmbeddings_NilPool verifies that
// ListMessagesMissingEmbeddings returns an error when the pool is nil.
func TestListMessagesMissingEmbeddings_NilPool(t *testing.T) {
	ctx := context.Background()
	messages, err := ListMessagesMissingEmbeddings(ctx, nil, 0, 50)

	if err == nil {
		t.Fatal("expected error for nil pool, got nil")
	}
	if messages != nil {
		t.Fatal("expected nil slice for nil pool with error")
	}
}

// TestSemanticSearchMessages_NilPool verifies that SemanticSearchMessages
// returns an error when the pool is nil.
func TestSemanticSearchMessages_NilPool(t *testing.T) {
	ctx := context.Background()
	messages, err := SemanticSearchMessages(ctx, nil, 12345, []float32{0.1, 0.2, 0.3}, 10)

	if err == nil {
		t.Fatal("expected error for nil pool, got nil")
	}
	if messages != nil {
		t.Fatal("expected nil slice for nil pool with error")
	}
}
