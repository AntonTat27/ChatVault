package model

import "time"

const (
	// TagIdea identifies suggestion-like messages.
	TagIdea = "idea"
	// TagDecision identifies resolved outcomes.
	TagDecision = "decision"
	// TagActionItem identifies tasks and follow-ups.
	TagActionItem = "action-item"
	// TagQuestion identifies unresolved asks.
	TagQuestion = "question"
	// TagDocument identifies references and files.
	TagDocument = "document"
	// TagNoise identifies off-topic chatter.
	TagNoise = "noise"
	// VoiceTranscriptFallback is used when voice transcription is unavailable.
	VoiceTranscriptFallback = "[transcription unavailable]"
)

// AllowedAITags lists valid classification types.
var AllowedAITags = map[string]struct{}{
	TagIdea:       {},
	TagDecision:   {},
	TagActionItem: {},
	TagQuestion:   {},
	TagDocument:   {},
	TagNoise:      {},
}

// Message represents a stored Telegram message.
type Message struct {
	ID         int64
	ChatID     int64
	MessageID  int
	SenderID   int64
	ChatTitle  string
	Text       string
	Transcript string
	AIType     string
	Topic      *string
	IsVoice    bool
	CreatedAt  time.Time
}

// ClassificationResult is the AI classification payload.
type ClassificationResult struct {
	Type  string  `json:"type"`
	Topic *string `json:"topic"`
}

// DailySummary stores structured summary fields.
type DailySummary struct {
	ChatID         int64        `json:"chat_id"`
	SummaryDateUTC string       `json:"summary_date_utc"`
	Summary        string       `json:"summary"`
	Decisions      []string     `json:"decisions"`
	ActionItems    []ActionItem `json:"action_items"`
	Ideas          []string     `json:"ideas"`
	OpenQuestions  []string     `json:"open_questions"`
	MessageCount   int          `json:"message_count"`
	CreatedAt      time.Time    `json:"created_at"`
}

// ActionItem defines a summary action item.
type ActionItem struct {
	Task  string  `json:"task"`
	Owner *string `json:"owner"`
}

// NotionConfig stores per-chat Notion integration settings.
type NotionConfig struct {
	ChatID       int64
	Token        string
	DatabaseID   string
	UpdatedAt    time.Time
	Configured   bool
	ChatName     string
	MessageCount int
}
