package ai

import (
	"testing"
)

// TestBuildClassificationPrompt verifies the exact required prompt envelope is preserved.
func TestBuildClassificationPrompt(t *testing.T) {
	prompt := BuildClassificationPrompt("Launch doc update")
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !containsAll(prompt,
		"You are classifying a message from a professional team group chat.",
		"Classify the message into EXACTLY ONE of these types:",
		"Message: \"Launch doc update\"",
		"Respond ONLY in this JSON format, no other text:") {
		t.Fatalf("prompt missing required sections: %s", prompt)
	}
}

// TestBuildSummaryPrompt verifies the summary prompt embeds messages payload correctly.
func TestBuildSummaryPrompt(t *testing.T) {
	messagesJSON := `[{"text":"hi","type":"noise"}]`
	prompt := BuildSummaryPrompt(messagesJSON)
	if !containsAll(prompt,
		"You are summarising a full day of messages from a professional team group chat.",
		"\"summary\": narrative overview of the day's discussion (<= 120 words)",
		"\"decisions\": array of strings, each a clear decision that was made",
		"\"action_items\": array of objects with \"task\" and \"owner\"",
		"Prioritise the most important and recurring points from the day.",
		"Messages:\n"+messagesJSON,
		"Respond ONLY with the JSON object. No markdown, no preamble.") {
		t.Fatalf("summary prompt missing required sections: %s", prompt)
	}
}

// TestSummaryOutputTokensForMessageCount verifies the dynamic output budget scales with chat size.
func TestSummaryOutputTokensForMessageCount(t *testing.T) {
	if got := summaryOutputTokensForMessageCount(1); got != 392 {
		t.Fatalf("unexpected token budget for one message: %d", got)
	}
	if got := summaryOutputTokensForMessageCount(100); got != 1184 {
		t.Fatalf("unexpected token budget for 100 messages: %d", got)
	}
	if got := summaryOutputTokensForMessageCount(400); got != summaryMaxOutputTokens {
		t.Fatalf("expected token budget to cap at %d, got %d", summaryMaxOutputTokens, got)
	}
}

// TestCountSummaryMessages verifies the message counter reads JSON arrays and falls back safely.
func TestCountSummaryMessages(t *testing.T) {
	if got := countSummaryMessages(`[{"text":"a"},{"text":"b"}]`); got != 2 {
		t.Fatalf("unexpected count for array payload: %d", got)
	}
	if got := countSummaryMessages(`not-json`); got != 1 {
		t.Fatalf("unexpected fallback count: %d", got)
	}
}

// TestParseClassificationResult validates accepted and rejected JSON payloads.
func TestParseClassificationResult(t *testing.T) {
	result, err := ParseClassificationResult(`{"type":"idea","topic":"Q3 planning"}`)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if result.Type != "idea" {
		t.Fatalf("unexpected type: %s", result.Type)
	}
	if result.Topic == nil || *result.Topic != "Q3 planning" {
		t.Fatalf("unexpected topic: %+v", result.Topic)
	}

	_, err = ParseClassificationResult(`{"type":"invalid","topic":null}`)
	if err == nil {
		t.Fatal("expected invalid type error")
	}
}

// containsAll checks if all expected substrings exist in a target string.
func containsAll(target string, expected ...string) bool {
	for _, value := range expected {
		if !contains(target, value) {
			return false
		}
	}
	return true
}

// contains checks if needle exists in haystack.
func contains(haystack string, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

// indexOf returns the first index of needle in haystack.
func indexOf(haystack string, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
