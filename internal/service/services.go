package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chatvault/internal/ai"
	"chatvault/internal/db"
	"chatvault/internal/model"
	"chatvault/internal/notion"
	"chatvault/internal/storage"
	"chatvault/internal/supabase"
)

const (
	commandWindowDays           = 7
	schedulerTickInterval       = time.Minute
	dailySummaryDateLayout      = "2006-01-02"
	maxMessagePreviewRuneLength = 180
	// embeddingWorkerPoolSize replaces the original single-worker job queue.
	// Embedding generation adds real per-job latency (an extra Gemini round
	// trip beyond classification), so a single worker would head-of-line
	// block transcription/classification jobs behind slow embedding calls.
	embeddingWorkerPoolSize = 4
)

// PostMessageFn defines a callback for posting text back to Telegram.
type PostMessageFn func(ctx context.Context, chatID int64, text string) error

// summaryRepository defines the repository methods Services depends on for
// summary generation and action item tracking. *storage.Repository satisfies
// this implicitly; tests can substitute a fake to assert call behavior.
type summaryRepository interface {
	UpsertChat(ctx context.Context, chatID int64, chatTitle string, summaryHour int, summaryMinute int) error
	InsertMessage(ctx context.Context, msg model.Message) (int64, error)
	UpdateClassification(ctx context.Context, chatID int64, messageID int, result model.ClassificationResult) error
	SaveVoiceProcessing(ctx context.Context, chatID int64, messageID int, transcript string, classification model.ClassificationResult) error
	ListMessagesForDate(ctx context.Context, chatID int64, dateUTC time.Time) ([]model.Message, error)
	SaveSummary(ctx context.Context, summary model.DailySummary) (int64, error)
	GetSummary(ctx context.Context, chatID int64, dateUTC string) (model.DailySummary, error)
	ListMessagesByTagSince(ctx context.Context, chatID int64, aiTag string, since time.Time) ([]model.Message, error)
	ListActiveChatsForSchedule(ctx context.Context, hour int, minute int) ([]int64, error)
	SaveNotionConfig(ctx context.Context, chatID int64, token string, databaseID string) error
	GetNotionConfig(ctx context.Context, chatID int64) (model.NotionConfig, error)
	CreateActionItem(ctx context.Context, item model.ActionItem) error
	ListActionItems(ctx context.Context, chatID int64, status string) ([]model.ActionItem, error)
	UpdateActionItemStatus(ctx context.Context, id int64, status string) error
}

// Services coordinates storage, AI processing, summary generation, and integrations.
type Services struct {
	repo           summaryRepository
	gemini         *ai.GeminiClient
	transcriber    *ai.GeminiTranscribeClient
	storageClient  *supabase.StorageClient
	notionClient   *notion.Client
	pool           *pgxpool.Pool
	embeddingModel string
	summaryHour    int
	summaryMinute  int
	jobs           chan func(context.Context)
	wg             sync.WaitGroup
}

// NewServices creates an orchestrator and starts worker goroutines.
func NewServices(
	ctx context.Context,
	repo *storage.Repository,
	geminiClient *ai.GeminiClient,
	transcriberClient *ai.GeminiTranscribeClient,
	storageClient *supabase.StorageClient,
	notionClient *notion.Client,
	pool *pgxpool.Pool,
	embeddingModel string,
	summaryHour int,
	summaryMinute int,
) *Services {
	s := &Services{
		repo:           repo,
		gemini:         geminiClient,
		transcriber:    transcriberClient,
		storageClient:  storageClient,
		notionClient:   notionClient,
		pool:           pool,
		embeddingModel: embeddingModel,
		summaryHour:    summaryHour,
		summaryMinute:  summaryMinute,
		jobs:           make(chan func(context.Context), 256),
	}
	for i := 0; i < embeddingWorkerPoolSize; i++ {
		s.wg.Add(1)
		go s.runWorker(ctx)
	}
	return s
}

// Close waits for running background jobs to complete.
func (s *Services) Close() {
	close(s.jobs)
	s.wg.Wait()
}

// HandleIncomingMessage stores a message and queues async classification.
func (s *Services) HandleIncomingMessage(ctx context.Context, message model.Message) error {
	if err := s.repo.UpsertChat(ctx, message.ChatID, message.ChatTitle, s.summaryHour, s.summaryMinute); err != nil {
		return err
	}
	messageRowID, err := s.repo.InsertMessage(ctx, message)
	if err != nil {
		return err
	}

	s.enqueue(ctx, func(jobCtx context.Context) {
		textForTag := message.Text
		if message.Transcript != "" {
			textForTag = message.Transcript
		}
		result, err := s.gemini.ClassifyMessage(jobCtx, textForTag)
		if err != nil {
			logProcessingError(message.ChatID, message.MessageID, "classification", err)
			return
		}
		if err := s.repo.UpdateClassification(jobCtx, message.ChatID, message.MessageID, result); err != nil {
			logProcessingError(message.ChatID, message.MessageID, "classification_write", err)
		}
		s.enqueueEmbeddingJob(jobCtx, message.ChatID, messageRowID, textForTag, result.Type)
	})
	return nil
}

// enqueueEmbeddingJob queues embedding generation for a classified message.
// Noise-tagged messages are skipped to control Gemini cost, and the job is a
// no-op if semantic search isn't configured (no DATABASE_URL/pool).
func (s *Services) enqueueEmbeddingJob(ctx context.Context, chatID int64, messageRowID int64, text string, aiTag string) {
	if s.pool == nil || aiTag == model.TagNoise || strings.TrimSpace(text) == "" {
		return
	}
	s.enqueue(ctx, func(jobCtx context.Context) {
		values, err := s.gemini.GenerateEmbedding(jobCtx, text)
		if err != nil {
			logProcessingError(chatID, 0, "embedding_generation", err)
			return
		}
		if err := db.UpsertMessageEmbedding(jobCtx, s.pool, messageRowID, chatID, values, s.embeddingModel); err != nil {
			logProcessingError(chatID, 0, "embedding_write", err)
		}
	})
}

// ProcessVoiceMessage handles storage upload, transcription, and transcript-based tagging.
func (s *Services) ProcessVoiceMessage(ctx context.Context, message model.Message, voiceBytes []byte) error {
	storagePath := fmt.Sprintf("voice/%d/%d.ogg", message.ChatID, message.MessageID)
	if err := s.storageClient.UploadVoice(ctx, storagePath, voiceBytes); err != nil {
		return fmt.Errorf("storage upload failed: %w", err)
	}

	transcript, err := s.transcriber.Transcribe(ctx, fmt.Sprintf("%d_%d.ogg", message.ChatID, message.MessageID), voiceBytes)
	if err != nil {
		logProcessingError(message.ChatID, message.MessageID, "transcription", err)
		transcript = model.VoiceTranscriptFallback
	}

	classification, classifyErr := s.gemini.ClassifyMessage(ctx, transcript)
	if classifyErr != nil {
		logProcessingError(message.ChatID, message.MessageID, "voice_classification", classifyErr)
		classification = model.ClassificationResult{Type: model.TagDocument}
	}

	if err := s.repo.SaveVoiceProcessing(ctx, message.ChatID, message.MessageID, transcript, classification); err != nil {
		return fmt.Errorf("voice write failed: %w", err)
	}
	return nil
}

// GenerateSummaryForChat builds and stores a daily summary for a chat, then posts it.
func (s *Services) GenerateSummaryForChat(ctx context.Context, chatID int64, dateUTC time.Time, post PostMessageFn) error {
	messages, err := s.repo.ListMessagesForDate(ctx, chatID, dateUTC)
	if err != nil {
		return err
	}
	messagesPayload, err := buildSummaryMessagesJSON(messages)
	if err != nil {
		return err
	}

	summary, err := s.gemini.GenerateSummary(ctx, messagesPayload)
	if err != nil {
		return err
	}
	summary.ChatID = chatID
	summary.SummaryDateUTC = dateUTC.UTC().Format(dailySummaryDateLayout)
	summary.MessageCount = len(messages)
	summaryID, err := s.repo.SaveSummary(ctx, summary)
	if err != nil {
		return err
	}

	s.createActionItemsForSummary(ctx, chatID, summaryID, summary.ActionItems)

	if err := post(ctx, chatID, FormatSummaryMessage(summary)); err != nil {
		return err
	}
	if err := s.exportSummaryToNotion(ctx, summary, firstChatTitle(messages)); err != nil {
		logProcessingError(chatID, 0, "notion_export", err)
	}
	return nil
}

// createActionItemsForSummary inserts a durable action_items row for each item
// extracted during summary generation. Failures are logged but do not abort
// summary delivery, since the JSONB blob on the summary remains the source of
// truth for the Telegram message itself.
func (s *Services) createActionItemsForSummary(ctx context.Context, chatID int64, summaryID int64, items []model.ActionItem) {
	for _, item := range items {
		record := model.ActionItem{
			ChatID:         chatID,
			SummaryID:      &summaryID,
			Task:           item.Task,
			Owner:          item.Owner,
			Status:         "open",
			DueDate:        item.DueDate,
			AssigneeUserID: nil,
		}
		if err := s.repo.CreateActionItem(ctx, record); err != nil {
			logProcessingError(chatID, 0, "action_item_create", err)
		}
	}
}

// GenerateSummaryAsync queues summary generation and sends completion/failure follow-up.
func (s *Services) GenerateSummaryAsync(ctx context.Context, chatID int64, dateUTC time.Time, post PostMessageFn) {
	s.enqueue(ctx, func(jobCtx context.Context) {
		if err := s.GenerateSummaryForChat(jobCtx, chatID, dateUTC, post); err != nil {
			logProcessingError(chatID, 0, "summary_generation", err)
			_ = post(jobCtx, chatID, "Summary generation failed. Please try again later.")
		}
	})
}

// ListTaggedMessages returns tagged messages from the last commandWindowDays days.
func (s *Services) ListTaggedMessages(ctx context.Context, chatID int64, tag string) ([]model.Message, error) {
	since := time.Now().UTC().AddDate(0, 0, -commandWindowDays)
	return s.repo.ListMessagesByTagSince(ctx, chatID, tag, since)
}

// SearchMessages searches for messages matching a query using full-text search.
// Returns an error if the database pool is not configured.
func (s *Services) SearchMessages(ctx context.Context, chatID int64, query string) ([]model.Message, error) {
	return db.SearchMessages(ctx, s.pool, chatID, query, 50)
}

// SemanticSearchMessages searches for messages whose meaning is closest to
// the query text, using a Gemini embedding compared via pgvector distance.
// Returns an error if the database pool is not configured.
func (s *Services) SemanticSearchMessages(ctx context.Context, chatID int64, query string) ([]model.Message, error) {
	if s.pool == nil {
		return nil, fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}
	queryEmbedding, err := s.gemini.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}
	return db.SemanticSearchMessages(ctx, s.pool, chatID, queryEmbedding, 50)
}

// SaveNotionConfig stores Notion integration credentials for a chat.
func (s *Services) SaveNotionConfig(ctx context.Context, chatID int64, token string, databaseID string) error {
	return s.repo.SaveNotionConfig(ctx, chatID, token, databaseID)
}

// ExportSummaryToNotionNow exports today's summary immediately when configured.
func (s *Services) ExportSummaryToNotionNow(ctx context.Context, chatID int64) error {
	dateUTC := time.Now().UTC().Format(dailySummaryDateLayout)
	summary, err := s.repo.GetSummary(ctx, chatID, dateUTC)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no summary exists for today")
		}
		return err
	}
	messages, _ := s.repo.ListMessagesForDate(ctx, chatID, time.Now().UTC())
	return s.exportSummaryToNotion(ctx, summary, firstChatTitle(messages))
}

// MarkActionItemDone marks an action item as completed.
func (s *Services) MarkActionItemDone(ctx context.Context, id int64) error {
	return s.repo.UpdateActionItemStatus(ctx, id, "done")
}

// ListOpenActionItems returns action items with 'open' status for a chat.
func (s *Services) ListOpenActionItems(ctx context.Context, chatID int64) ([]model.ActionItem, error) {
	return s.repo.ListActionItems(ctx, chatID, "open")
}

// RunDailySummaryScheduler runs a minute-based scheduler for configured daily summary time.
func (s *Services) RunDailySummaryScheduler(ctx context.Context, post PostMessageFn) {
	ticker := time.NewTicker(schedulerTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			nowUTC := now.UTC()
			if nowUTC.Hour() != s.summaryHour || nowUTC.Minute() != s.summaryMinute {
				continue
			}
			chatIDs, err := s.repo.ListActiveChatsForSchedule(ctx, s.summaryHour, s.summaryMinute)
			if err != nil {
				logProcessingError(0, 0, "scheduler_load_chats", err)
				continue
			}
			for _, chatID := range chatIDs {
				s.GenerateSummaryAsync(ctx, chatID, nowUTC, post)
			}
		}
	}
}

// FormatTaggedMessages renders command response text for tagged message listings.
func FormatTaggedMessages(label string, messages []model.Message) string {
	if len(messages) == 0 {
		return fmt.Sprintf("No %s messages found in the last %d days.", label, commandWindowDays)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s (last %d days):\n", label, commandWindowDays))
	for idx, message := range messages {
		if idx >= 15 {
			break
		}
		preview := truncateRunes(message.Text, maxMessagePreviewRuneLength)
		b.WriteString(fmt.Sprintf("- %s\n", preview))
	}
	return b.String()
}

// FormatSummaryMessage renders a Telegram-friendly summary from structured JSON fields.
func FormatSummaryMessage(summary model.DailySummary) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Daily Summary (%s)\n\n", summary.SummaryDateUTC))
	b.WriteString(summary.Summary)
	b.WriteString("\n\nDecisions:\n")
	b.WriteString(formatStringList(summary.Decisions))
	b.WriteString("\nAction Items:\n")
	if len(summary.ActionItems) == 0 {
		b.WriteString("- None\n")
	} else {
		for _, item := range summary.ActionItems {
			owner := "unassigned"
			if item.Owner != nil && *item.Owner != "" {
				owner = *item.Owner
			}
			b.WriteString(fmt.Sprintf("- %s (owner: %s)\n", item.Task, owner))
		}
	}
	b.WriteString("Ideas:\n")
	b.WriteString(formatStringList(summary.Ideas))
	b.WriteString("Open Questions:\n")
	b.WriteString(formatStringList(summary.OpenQuestions))
	return b.String()
}

// buildSummaryMessagesJSON marshals the prompt input structure for summary generation.
func buildSummaryMessagesJSON(messages []model.Message) (string, error) {
	payload := make([]map[string]string, 0, len(messages))
	for _, message := range messages {
		payload = append(payload, map[string]string{
			"text": message.Text,
			"type": message.AIType,
		})
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// exportSummaryToNotion sends summary output to Notion when integration is configured.
func (s *Services) exportSummaryToNotion(ctx context.Context, summary model.DailySummary, chatName string) error {
	cfg, err := s.repo.GetNotionConfig(ctx, summary.ChatID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	if !cfg.Configured {
		return nil
	}
	return s.notionClient.CreateDailySummaryPage(ctx, cfg, summary, chatName)
}

// firstChatTitle returns the first non-empty chat title from a message list.
func firstChatTitle(messages []model.Message) string {
	for _, message := range messages {
		if strings.TrimSpace(message.ChatTitle) != "" {
			return message.ChatTitle
		}
	}
	return "Telegram Chat"
}

// formatStringList converts strings into markdown-like bullet lines.
func formatStringList(values []string) string {
	if len(values) == 0 {
		return "- None\n"
	}
	var b strings.Builder
	for _, value := range values {
		b.WriteString(fmt.Sprintf("- %s\n", value))
	}
	return b.String()
}

// truncateRunes truncates a string to a rune limit with ellipsis.
func truncateRunes(input string, max int) string {
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "…"
}

// enqueue submits a background job unless context is cancelled.
func (s *Services) enqueue(ctx context.Context, job func(context.Context)) {
	select {
	case <-ctx.Done():
	case s.jobs <- job:
	}
}

// runWorker processes asynchronous jobs until shutdown.
func (s *Services) runWorker(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-s.jobs:
			if !ok {
				return
			}
			job(ctx)
		}
	}
}

// FormatActionItemsList renders a Telegram-friendly list of action items.
func FormatActionItemsList(items []model.ActionItem) string {
	if len(items) == 0 {
		return "No open action items."
	}
	var b strings.Builder
	b.WriteString("Open Action Items:\n\n")
	for _, item := range items {
		if item.ID != nil {
			b.WriteString(fmt.Sprintf("ID: %d\n", *item.ID))
		}
		b.WriteString(fmt.Sprintf("Task: %s\n", item.Task))
		owner := "unassigned"
		if item.Owner != nil && *item.Owner != "" {
			owner = *item.Owner
		}
		b.WriteString(fmt.Sprintf("Owner: %s\n", owner))
		if item.Status != "" {
			b.WriteString(fmt.Sprintf("Status: %s\n", item.Status))
		}
		if item.DueDate != nil && *item.DueDate != "" {
			b.WriteString(fmt.Sprintf("Due: %s\n", *item.DueDate))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// logProcessingError logs processing failures with required metadata fields.
func logProcessingError(chatID int64, messageID int, errorType string, err error) {
	log.Printf("chat_id=%d message_id=%d error_type=%s timestamp=%s err=%s", chatID, messageID, errorType, time.Now().UTC().Format(time.RFC3339), err.Error())
}
