package bot

import (
	"strings"
	"testing"
	"time"

	"chatvault/internal/model"
)

// TestFormatSearchResults_NoResults verifies formatting when no results found.
func TestFormatSearchResults_NoResults(t *testing.T) {
	result := FormatSearchResults("nonexistent", []model.Message{})
	expected := "No messages found for query: \"nonexistent\""

	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

// TestFormatSearchResults_SingleResult verifies formatting with one result.
func TestFormatSearchResults_SingleResult(t *testing.T) {
	mockTime := time.Date(2026, 5, 29, 10, 30, 0, 0, time.UTC)
	messages := []model.Message{
		{
			Text:      "Hello world",
			IsVoice:   false,
			CreatedAt: mockTime,
		},
	}

	result := FormatSearchResults("hello", messages)

	checks := []string{
		"Search results for \"hello\" (1 found)",
		"Hello world",
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Fatalf("expected %q in result:\n%s", check, result)
		}
	}
}

// TestFormatSearchResults_VoiceWithTranscript verifies that transcript
// is used for voice messages.
func TestFormatSearchResults_VoiceWithTranscript(t *testing.T) {
	mockTime := time.Date(2026, 5, 29, 10, 30, 0, 0, time.UTC)
	messages := []model.Message{
		{
			Text:       "Original text",
			Transcript: "Voice transcript content",
			IsVoice:    true,
			CreatedAt:  mockTime,
		},
	}

	result := FormatSearchResults("voice", messages)

	if !strings.Contains(result, "Voice transcript content") {
		t.Fatalf("expected transcript in result:\n%s", result)
	}

	if !strings.Contains(result, "[voice]") {
		t.Fatalf("expected [voice] tag in result:\n%s", result)
	}
}

// TestFormatSearchResults_TruncatesLongContent verifies content truncation.
func TestFormatSearchResults_TruncatesLongContent(t *testing.T) {
	mockTime := time.Date(2026, 5, 29, 10, 30, 0, 0, time.UTC)
	longText := "a"
	for i := 0; i < 200; i++ {
		longText += "b"
	}

	messages := []model.Message{
		{
			Text:      longText,
			IsVoice:   false,
			CreatedAt: mockTime,
		},
	}

	result := FormatSearchResults("query", messages)

	if !strings.Contains(result, "…") {
		t.Fatalf("expected ellipsis for truncated content in result:\n%s", result)
	}
}

// TestFormatSearchResults_ShowsUpTo10Results verifies the limit.
func TestFormatSearchResults_ShowsUpTo10Results(t *testing.T) {
	mockTime := time.Date(2026, 5, 29, 10, 30, 0, 0, time.UTC)
	messages := make([]model.Message, 15)
	for i := 0; i < 15; i++ {
		messages[i] = model.Message{
			Text:      "message",
			IsVoice:   false,
			CreatedAt: mockTime,
		}
	}

	result := FormatSearchResults("query", messages)

	if !strings.Contains(result, "15 found") {
		t.Fatalf("expected result count of 15 in:\n%s", result)
	}

	if !strings.Contains(result, "...and 5 more results") {
		t.Fatalf("expected 'and 5 more results' in:\n%s", result)
	}
}
