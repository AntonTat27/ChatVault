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
	ID         int64     `json:"id"`
	ChatID     int64     `json:"chat_id"`
	MessageID  int       `json:"message_id"`
	SenderID   int64     `json:"sender_id"`
	ChatTitle  string    `json:"chat_title"`
	Text       string    `json:"text"`
	Transcript string    `json:"transcript"`
	AIType     string    `json:"ai_type"`
	Topic      *string   `json:"topic"`
	IsVoice    bool      `json:"is_voice"`
	CreatedAt  time.Time `json:"created_at"`
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

// ActionItem defines a summary action item and durable action item row.
type ActionItem struct {
	ID              *int64  `json:"id,omitempty"`
	ChatID          int64   `json:"chat_id,omitempty"`
	SourceMessageID *int64  `json:"source_message_id,omitempty"`
	SummaryID       *int64  `json:"summary_id,omitempty"`
	Task            string  `json:"task"`
	Owner           *string `json:"owner"`
	Status          string  `json:"status,omitempty"`
	DueDate         *string `json:"due_date,omitempty"`
	AssigneeUserID  *int64  `json:"assignee_user_id,omitempty"`
}

// NotionConfig stores per-chat Notion integration settings. Token is always
// the plaintext value to use for API calls; for OAuth-connected chats it is
// populated by decrypting TokenEncrypted at call time (see
// Services.exportSummaryToNotion), never persisted or logged decrypted.
type NotionConfig struct {
	ChatID             int64
	Token              string
	TokenEncrypted     []byte
	DatabaseID         string
	OAuthWorkspaceID   string
	OAuthWorkspaceName string
	UpdatedAt          time.Time
	Configured         bool
	ChatName           string
	MessageCount       int
}

// DashboardUser is a Telegram identity that has logged into the web dashboard.
type DashboardUser struct {
	TelegramUserID int64
	FirstName      string
	LastName       string
	Username       string
	PhotoURL       string
}

// ChatSummaryRef is a lightweight chat reference for the dashboard's chat list.
type ChatSummaryRef struct {
	ChatID    int64  `json:"chat_id"`
	ChatTitle string `json:"chat_title"`
	Role      string `json:"role"`
}
