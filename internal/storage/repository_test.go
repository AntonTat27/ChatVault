package storage

import (
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
