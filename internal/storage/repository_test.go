package storage

import (
	"encoding/json"
	"testing"

	"chatvault/internal/model"
)

// TestNormalizeDailySummary converts nil slices to empty arrays for safe JSON encoding.
func TestNormalizeDailySummary(t *testing.T) {
	summary := normalizeDailySummary(model.DailySummary{})

	if summary.Decisions == nil {
		t.Fatal("expected decisions to be initialized")
	}
	if summary.ActionItems == nil {
		t.Fatal("expected action_items to be initialized")
	}
	if summary.Ideas == nil {
		t.Fatal("expected ideas to be initialized")
	}
	if summary.OpenQuestions == nil {
		t.Fatal("expected open_questions to be initialized")
	}
	if len(summary.Decisions) != 0 || len(summary.ActionItems) != 0 || len(summary.Ideas) != 0 || len(summary.OpenQuestions) != 0 {
		t.Fatal("expected all normalized collections to be empty")
	}
}

// TestEncodeDecodeByteaRoundTrip verifies encodeBytea/decodeBytea agree with
// each other -- and, implicitly, with Postgres's own "\x<hex>" bytea text
// format, which is what PostgREST actually sends/expects over the REST API.
// Without this, an encrypted Notion OAuth token written via encodeBytea
// would be unreadable (or worse, silently corrupted) on the next read,
// because encoding/json's default base64 encoding for []byte does not match
// what Postgres's bytea input parser expects.
func TestEncodeDecodeByteaRoundTrip(t *testing.T) {
	original := []byte{0x00, 0x01, 0xFF, 0xAB, 0xCD, 0xEF}

	encoded := encodeBytea(original)
	if encoded != "\\x0001ffabcdef" {
		t.Fatalf("expected Postgres hex bytea format, got %q", encoded)
	}

	decoded, err := decodeBytea(encoded)
	if err != nil {
		t.Fatalf("decodeBytea failed: %v", err)
	}
	if string(decoded) != string(original) {
		t.Fatalf("round trip mismatch: got %x, want %x", decoded, original)
	}
}

func TestDecodeBytea_EmptyStringIsNilNotError(t *testing.T) {
	decoded, err := decodeBytea("")
	if err != nil {
		t.Fatalf("expected no error for empty input, got %v", err)
	}
	if decoded != nil {
		t.Fatalf("expected nil slice for empty input, got %v", decoded)
	}
}

// TestHydrateActionItems converts database rows to ActionItem structs.
func TestHydrateActionItems(t *testing.T) {
	rows := []actionItemRow{
		{
			ID:      1,
			ChatID:  12345,
			Task:    "Fix login bug",
			Owner:   stringPtr("alice"),
			Status:  "open",
			DueDate: stringPtr("2026-07-01"),
		},
		{
			ID:     2,
			ChatID: 12345,
			Task:   "Write docs",
			Owner:  nil,
			Status: "in_progress",
		},
	}

	items := hydrateActionItems(rows)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].ID == nil || *items[0].ID != 1 {
		t.Fatalf("item 0 ID mismatch: %v", items[0].ID)
	}
	if items[0].Task != "Fix login bug" {
		t.Fatalf("item 0 task mismatch: %s", items[0].Task)
	}
	if items[0].Owner == nil || *items[0].Owner != "alice" {
		t.Fatalf("item 0 owner mismatch: %v", items[0].Owner)
	}
	if items[0].Status != "open" {
		t.Fatalf("item 0 status mismatch: %s", items[0].Status)
	}

	if items[1].Owner != nil {
		t.Fatalf("item 1 owner should be nil, got %v", items[1].Owner)
	}
}

// TestActionItemJSONMarshaling verifies omitempty tags preserve old JSONB compatibility.
func TestActionItemJSONMarshaling(t *testing.T) {
	// Old-style action item from JSONB (no id, status, due_date, etc.)
	oldStyle := `{"task":"old task","owner":"bob"}`
	var item model.ActionItem
	if err := json.Unmarshal([]byte(oldStyle), &item); err != nil {
		t.Fatalf("failed to unmarshal old-style action item: %v", err)
	}
	if item.Task != "old task" {
		t.Fatalf("expected task 'old task', got %s", item.Task)
	}
	if item.Owner == nil || *item.Owner != "bob" {
		t.Fatalf("expected owner 'bob', got %v", item.Owner)
	}
	if item.ID != nil {
		t.Fatalf("expected nil ID for old-style item, got %v", item.ID)
	}

	// New-style action item with status
	newStyle := `{"id":42,"task":"new task","owner":"alice","status":"done"}`
	if err := json.Unmarshal([]byte(newStyle), &item); err != nil {
		t.Fatalf("failed to unmarshal new-style action item: %v", err)
	}
	if item.ID == nil || *item.ID != 42 {
		t.Fatalf("expected id 42, got %v", item.ID)
	}
	if item.Status != "done" {
		t.Fatalf("expected status 'done', got %s", item.Status)
	}
}

// TestNullablePtr64 handles both nil and valid int64 pointers.
func TestNullablePtr64(t *testing.T) {
	tests := []struct {
		name     string
		input    *int64
		expected interface{}
	}{
		{"nil pointer", nil, nil},
		{"valid value", int64Ptr(42), int64(42)},
		{"zero value", int64Ptr(0), int64(0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nullablePtr64(tt.input)
			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Helper functions for tests
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
