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
