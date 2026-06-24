package service

import (
	"strings"
	"testing"

	"chatvault/internal/model"
)

// TestFormatSummaryMessage verifies the Telegram summary rendering.
func TestFormatSummaryMessage(t *testing.T) {
	owner := "alice"
	summary := model.DailySummary{
		SummaryDateUTC: "2026-05-29",
		Summary:        "Team reviewed roadmap and release timing.",
		Decisions:      []string{"Ship v1 next week"},
		ActionItems:    []model.ActionItem{{Task: "Prepare changelog", Owner: &owner}},
		Ideas:          []string{"Pilot onboarding webinar"},
		OpenQuestions:  []string{"Who owns support rotation?"},
	}

	formatted := FormatSummaryMessage(summary)
	checks := []string{
		"Daily Summary (2026-05-29)",
		"Decisions:",
		"- Ship v1 next week",
		"Action Items:",
		"- Prepare changelog (owner: alice)",
		"Ideas:",
		"Open Questions:",
	}
	for _, check := range checks {
		if !strings.Contains(formatted, check) {
			t.Fatalf("formatted summary missing %q\n%s", check, formatted)
		}
	}
}

// TestFormatActionItemsList verifies action items are formatted for Telegram display.
func TestFormatActionItemsList(t *testing.T) {
	id1 := int64(1)
	id2 := int64(2)
	owner := "alice"
	dueDate := "2026-07-01"

	items := []model.ActionItem{
		{
			ID:      &id1,
			Task:    "Fix login bug",
			Owner:   &owner,
			Status:  "open",
			DueDate: &dueDate,
		},
		{
			ID:     &id2,
			Task:   "Write documentation",
			Owner:  nil,
			Status: "in_progress",
		},
	}

	formatted := FormatActionItemsList(items)
	checks := []string{
		"Open Action Items:",
		"ID: 1",
		"Task: Fix login bug",
		"Owner: alice",
		"Status: open",
		"Due: 2026-07-01",
		"ID: 2",
		"Task: Write documentation",
		"Owner: unassigned",
	}
	for _, check := range checks {
		if !strings.Contains(formatted, check) {
			t.Fatalf("formatted action items missing %q\n%s", check, formatted)
		}
	}
}

// TestFormatActionItemsListEmpty shows appropriate message when no items exist.
func TestFormatActionItemsListEmpty(t *testing.T) {
	items := []model.ActionItem{}
	formatted := FormatActionItemsList(items)
	if formatted != "No open action items." {
		t.Fatalf("expected 'No open action items.', got %q", formatted)
	}
}
