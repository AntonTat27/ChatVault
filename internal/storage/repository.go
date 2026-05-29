package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"chatvault/internal/model"
)

const (
	tableChats        = "chats"
	tableMessages     = "messages"
	tableSummaries    = "daily_summaries"
	tableNotionConfig = "notion_configs"
)

// Repository provides data access methods for ChatVault.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a repository backed by sql.DB.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// UpsertChat ensures chat metadata and schedule configuration exist.
func (r *Repository) UpsertChat(ctx context.Context, chatID int64, chatTitle string, hour int, minute int) error {
	query := fmt.Sprintf(`
INSERT INTO %s (chat_id, chat_title, summary_hour_utc, summary_minute_utc, is_active)
VALUES ($1, $2, $3, $4, true)
ON CONFLICT (chat_id) DO UPDATE SET
chat_title = EXCLUDED.chat_title,
is_active = true`, tableChats)
	_, err := r.db.ExecContext(ctx, query, chatID, chatTitle, hour, minute)
	return err
}

// InsertMessage stores a Telegram message and returns the stored record ID.
func (r *Repository) InsertMessage(ctx context.Context, msg model.Message) (int64, error) {
	query := fmt.Sprintf(`
INSERT INTO %s (chat_id, message_id, sender_id, chat_title, message_text, is_voice, transcript)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (chat_id, message_id) DO UPDATE SET
sender_id = EXCLUDED.sender_id,
chat_title = EXCLUDED.chat_title,
message_text = EXCLUDED.message_text,
is_voice = EXCLUDED.is_voice,
transcript = EXCLUDED.transcript
RETURNING id`, tableMessages)

	var id int64
	err := r.db.QueryRowContext(ctx, query, msg.ChatID, msg.MessageID, msg.SenderID, msg.ChatTitle, msg.Text, msg.IsVoice, nullableString(msg.Transcript)).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateClassification updates ai_tag and topic_tag for a message.
func (r *Repository) UpdateClassification(ctx context.Context, chatID int64, messageID int, result model.ClassificationResult) error {
	query := fmt.Sprintf(`UPDATE %s SET ai_tag = $1, topic_tag = $2, updated_at = now() WHERE chat_id = $3 AND message_id = $4`, tableMessages)
	_, err := r.db.ExecContext(ctx, query, result.Type, nullablePtr(result.Topic), chatID, messageID)
	return err
}

// SaveVoiceProcessing updates transcript and tags as a transaction.
func (r *Repository) SaveVoiceProcessing(ctx context.Context, chatID int64, messageID int, transcript string, result model.ClassificationResult) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	queryTranscript := fmt.Sprintf(`UPDATE %s SET transcript = $1, updated_at = now() WHERE chat_id = $2 AND message_id = $3`, tableMessages)
	if _, err = tx.ExecContext(ctx, queryTranscript, transcript, chatID, messageID); err != nil {
		return err
	}

	queryTag := fmt.Sprintf(`UPDATE %s SET ai_tag = $1, topic_tag = $2, updated_at = now() WHERE chat_id = $3 AND message_id = $4`, tableMessages)
	if _, err = tx.ExecContext(ctx, queryTag, result.Type, nullablePtr(result.Topic), chatID, messageID); err != nil {
		return err
	}

	return tx.Commit()
}

// ListMessagesForDate returns messages for a UTC date for summary generation.
func (r *Repository) ListMessagesForDate(ctx context.Context, chatID int64, day time.Time) ([]model.Message, error) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	query := fmt.Sprintf(`
SELECT id, chat_id, message_id, sender_id, chat_title,
COALESCE(NULLIF(transcript, ''), message_text) AS effective_text,
COALESCE(transcript, ''), COALESCE(ai_tag, ''), topic_tag,
is_voice, created_at
FROM %s
WHERE chat_id = $1 AND created_at >= $2 AND created_at < $3
ORDER BY created_at ASC`, tableMessages)

	rows, err := r.db.QueryContext(ctx, query, chatID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]model.Message, 0)
	for rows.Next() {
		var m model.Message
		var topic sql.NullString
		if err := rows.Scan(&m.ID, &m.ChatID, &m.MessageID, &m.SenderID, &m.ChatTitle, &m.Text, &m.Transcript, &m.AIType, &topic, &m.IsVoice, &m.CreatedAt); err != nil {
			return nil, err
		}
		if topic.Valid {
			m.Topic = &topic.String
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// SaveSummary persists a generated summary payload.
func (r *Repository) SaveSummary(ctx context.Context, summary model.DailySummary) error {
	decisions, err := json.Marshal(summary.Decisions)
	if err != nil {
		return err
	}
	actions, err := json.Marshal(summary.ActionItems)
	if err != nil {
		return err
	}
	ideas, err := json.Marshal(summary.Ideas)
	if err != nil {
		return err
	}
	questions, err := json.Marshal(summary.OpenQuestions)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
INSERT INTO %s
(chat_id, summary_date_utc, summary_text, decisions, action_items, ideas, open_questions, message_count)
VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8)
ON CONFLICT (chat_id, summary_date_utc) DO UPDATE SET
summary_text = EXCLUDED.summary_text,
decisions = EXCLUDED.decisions,
action_items = EXCLUDED.action_items,
ideas = EXCLUDED.ideas,
open_questions = EXCLUDED.open_questions,
message_count = EXCLUDED.message_count,
created_at = now()`, tableSummaries)

	_, err = r.db.ExecContext(ctx, query, summary.ChatID, summary.SummaryDateUTC, summary.Summary, decisions, actions, ideas, questions, summary.MessageCount)
	return err
}

// GetSummary loads a daily summary by chat and date.
func (r *Repository) GetSummary(ctx context.Context, chatID int64, dateUTC string) (model.DailySummary, error) {
	query := fmt.Sprintf(`SELECT summary_text, decisions, action_items, ideas, open_questions, message_count, created_at FROM %s WHERE chat_id = $1 AND summary_date_utc = $2`, tableSummaries)
	var summary model.DailySummary
	var decisionsRaw, actionsRaw, ideasRaw, questionsRaw []byte
	err := r.db.QueryRowContext(ctx, query, chatID, dateUTC).Scan(&summary.Summary, &decisionsRaw, &actionsRaw, &ideasRaw, &questionsRaw, &summary.MessageCount, &summary.CreatedAt)
	if err != nil {
		return summary, err
	}
	summary.ChatID = chatID
	summary.SummaryDateUTC = dateUTC
	if err := json.Unmarshal(decisionsRaw, &summary.Decisions); err != nil {
		return summary, err
	}
	if err := json.Unmarshal(actionsRaw, &summary.ActionItems); err != nil {
		return summary, err
	}
	if err := json.Unmarshal(ideasRaw, &summary.Ideas); err != nil {
		return summary, err
	}
	if err := json.Unmarshal(questionsRaw, &summary.OpenQuestions); err != nil {
		return summary, err
	}
	return summary, nil
}

// ListMessagesByTagSince returns tagged messages for a time window.
func (r *Repository) ListMessagesByTagSince(ctx context.Context, chatID int64, aiTag string, since time.Time) ([]model.Message, error) {
	query := fmt.Sprintf(`
SELECT id, chat_id, message_id, sender_id, chat_title,
COALESCE(NULLIF(transcript, ''), message_text),
COALESCE(transcript, ''), COALESCE(ai_tag, ''), topic_tag,
is_voice, created_at
FROM %s
WHERE chat_id = $1 AND ai_tag = $2 AND created_at >= $3
ORDER BY created_at DESC`, tableMessages)

	rows, err := r.db.QueryContext(ctx, query, chatID, aiTag, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]model.Message, 0)
	for rows.Next() {
		var m model.Message
		var topic sql.NullString
		if err := rows.Scan(&m.ID, &m.ChatID, &m.MessageID, &m.SenderID, &m.ChatTitle, &m.Text, &m.Transcript, &m.AIType, &topic, &m.IsVoice, &m.CreatedAt); err != nil {
			return nil, err
		}
		if topic.Valid {
			m.Topic = &topic.String
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// ListActiveChatsForSchedule returns active chats due for summary at UTC hour/minute.
func (r *Repository) ListActiveChatsForSchedule(ctx context.Context, hour int, minute int) ([]int64, error) {
	query := fmt.Sprintf(`
SELECT chat_id
FROM %s
WHERE is_active = true AND summary_hour_utc = $1 AND summary_minute_utc = $2`, tableChats)
	rows, err := r.db.QueryContext(ctx, query, hour, minute)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, err
		}
		ids = append(ids, chatID)
	}
	return ids, rows.Err()
}

// SaveNotionConfig stores per-chat Notion token and database ID.
func (r *Repository) SaveNotionConfig(ctx context.Context, chatID int64, token string, databaseID string) error {
	query := fmt.Sprintf(`
INSERT INTO %s (chat_id, notion_token, notion_database_id)
VALUES ($1, $2, $3)
ON CONFLICT (chat_id) DO UPDATE SET
notion_token = EXCLUDED.notion_token,
notion_database_id = EXCLUDED.notion_database_id,
updated_at = now()`, tableNotionConfig)
	_, err := r.db.ExecContext(ctx, query, chatID, token, databaseID)
	return err
}

// GetNotionConfig retrieves Notion settings for a chat.
func (r *Repository) GetNotionConfig(ctx context.Context, chatID int64) (model.NotionConfig, error) {
	query := fmt.Sprintf(`SELECT notion_token, notion_database_id, updated_at FROM %s WHERE chat_id = $1`, tableNotionConfig)
	var cfg model.NotionConfig
	err := r.db.QueryRowContext(ctx, query, chatID).Scan(&cfg.Token, &cfg.DatabaseID, &cfg.UpdatedAt)
	if err != nil {
		return cfg, err
	}
	cfg.ChatID = chatID
	cfg.Configured = cfg.Token != "" && cfg.DatabaseID != ""
	return cfg, nil
}

// nullableString converts an empty string to nil for nullable DB columns.
func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

// nullablePtr converts a pointer to either nil or the pointed value.
func nullablePtr(value *string) interface{} {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}
