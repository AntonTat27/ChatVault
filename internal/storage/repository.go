package storage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"chatvault/internal/model"
)

const (
	tableChats        = "chats"
	tableMessages     = "messages"
	tableSummaries    = "daily_summaries"
	tableNotionConfig = "notion_configs"
	tableActionItems  = "action_items"
)

// Repository provides data access methods for ChatVault via Supabase PostgREST.
type Repository struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewRepository creates a repository backed by Supabase PostgREST.
func NewRepository(supabaseURL string, apiKey string, timeout time.Duration) *Repository {
	baseURL := strings.TrimRight(supabaseURL, "/") + "/rest/v1"
	return &Repository{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// UpsertChat ensures chat metadata and schedule configuration exist.
func (r *Repository) UpsertChat(ctx context.Context, chatID int64, chatTitle string, hour int, minute int) error {
	payload := map[string]any{
		"chat_id":            chatID,
		"chat_title":         chatTitle,
		"summary_hour_utc":   hour,
		"summary_minute_utc": minute,
		"is_active":          true,
	}
	_, _, err := r.doRequest(ctx, http.MethodPost, tableChats, url.Values{"on_conflict": []string{"chat_id"}}, payload, "resolution=merge-duplicates")
	return err
}

// InsertMessage stores a Telegram message and returns the stored record ID.
func (r *Repository) InsertMessage(ctx context.Context, msg model.Message) (int64, error) {
	payload := map[string]any{
		"chat_id":      msg.ChatID,
		"message_id":   msg.MessageID,
		"sender_id":    msg.SenderID,
		"chat_title":   msg.ChatTitle,
		"message_text": msg.Text,
		"is_voice":     msg.IsVoice,
		"transcript":   nullableString(msg.Transcript),
	}
	query := url.Values{
		"on_conflict": []string{"chat_id,message_id"},
		"select":      []string{"id"},
	}
	data, _, err := r.doRequest(ctx, http.MethodPost, tableMessages, query, payload, "resolution=merge-duplicates,return=representation")
	if err != nil {
		return 0, err
	}
	var rows []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, sql.ErrNoRows
	}
	return rows[0].ID, nil
}

// UpdateClassification updates ai_tag and topic_tag for a message.
func (r *Repository) UpdateClassification(ctx context.Context, chatID int64, messageID int, result model.ClassificationResult) error {
	payload := map[string]any{
		"ai_tag":    result.Type,
		"topic_tag": nullablePtr(result.Topic),
	}
	query := url.Values{
		"chat_id":    []string{fmt.Sprintf("eq.%d", chatID)},
		"message_id": []string{fmt.Sprintf("eq.%d", messageID)},
	}
	_, _, err := r.doRequest(ctx, http.MethodPatch, tableMessages, query, payload, "return=minimal")
	return err
}

// SaveVoiceProcessing updates transcript and tags as a transaction.
func (r *Repository) SaveVoiceProcessing(ctx context.Context, chatID int64, messageID int, transcript string, result model.ClassificationResult) error {
	payload := map[string]any{
		"transcript": transcript,
		"ai_tag":     result.Type,
		"topic_tag":  nullablePtr(result.Topic),
	}
	query := url.Values{
		"chat_id":    []string{fmt.Sprintf("eq.%d", chatID)},
		"message_id": []string{fmt.Sprintf("eq.%d", messageID)},
	}
	_, _, err := r.doRequest(ctx, http.MethodPatch, tableMessages, query, payload, "return=minimal")
	return err
}

// ListMessagesForDate returns messages for a UTC date for summary generation.
func (r *Repository) ListMessagesForDate(ctx context.Context, chatID int64, day time.Time) ([]model.Message, error) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	query := url.Values{
		"chat_id":    []string{fmt.Sprintf("eq.%d", chatID)},
		"created_at": []string{fmt.Sprintf("gte.%s", start.Format(time.RFC3339)), fmt.Sprintf("lt.%s", end.Format(time.RFC3339))},
		"order":      []string{"created_at.asc"},
		"select": []string{strings.Join([]string{
			"id",
			"chat_id",
			"message_id",
			"sender_id",
			"chat_title",
			"message_text",
			"transcript",
			"ai_tag",
			"topic_tag",
			"is_voice",
			"created_at",
		}, ",")},
	}
	data, _, err := r.doRequest(ctx, http.MethodGet, tableMessages, query, nil, "")
	if err != nil {
		return nil, err
	}
	var rows []messageRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return hydrateMessages(rows), nil
}

// SaveSummary persists a generated summary payload and returns the stored record ID.
func (r *Repository) SaveSummary(ctx context.Context, summary model.DailySummary) (int64, error) {
	summary = normalizeDailySummary(summary)
	payload := map[string]any{
		"chat_id":          summary.ChatID,
		"summary_date_utc": summary.SummaryDateUTC,
		"summary_text":     summary.Summary,
		"decisions":        summary.Decisions,
		"action_items":     summary.ActionItems,
		"ideas":            summary.Ideas,
		"open_questions":   summary.OpenQuestions,
		"message_count":    summary.MessageCount,
	}
	query := url.Values{
		"on_conflict": []string{"chat_id,summary_date_utc"},
		"select":      []string{"id"},
	}
	data, _, err := r.doRequest(ctx, http.MethodPost, tableSummaries, query, payload, "resolution=merge-duplicates,return=representation")
	if err != nil {
		return 0, err
	}
	var rows []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, sql.ErrNoRows
	}
	return rows[0].ID, nil
}

// normalizeDailySummary ensures nil slice fields are encoded as empty JSON arrays, not null.
func normalizeDailySummary(summary model.DailySummary) model.DailySummary {
	if summary.Decisions == nil {
		summary.Decisions = []string{}
	}
	if summary.ActionItems == nil {
		summary.ActionItems = []model.ActionItem{}
	}
	if summary.Ideas == nil {
		summary.Ideas = []string{}
	}
	if summary.OpenQuestions == nil {
		summary.OpenQuestions = []string{}
	}
	return summary
}

// GetSummary loads a daily summary by chat and date.
func (r *Repository) GetSummary(ctx context.Context, chatID int64, dateUTC string) (model.DailySummary, error) {
	query := url.Values{
		"chat_id":          []string{fmt.Sprintf("eq.%d", chatID)},
		"summary_date_utc": []string{fmt.Sprintf("eq.%s", dateUTC)},
		"select":           []string{"summary_text,decisions,action_items,ideas,open_questions,message_count,created_at"},
	}
	data, _, err := r.doRequest(ctx, http.MethodGet, tableSummaries, query, nil, "")
	if err != nil {
		return model.DailySummary{}, err
	}
	var rows []summaryRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return model.DailySummary{}, err
	}
	if len(rows) == 0 {
		return model.DailySummary{}, sql.ErrNoRows
	}
	return rows[0].toSummary(chatID, dateUTC), nil
}

// ListSummaries returns the most recent daily summaries for a chat, newest first.
func (r *Repository) ListSummaries(ctx context.Context, chatID int64, limit int) ([]model.DailySummary, error) {
	query := url.Values{
		"chat_id": []string{fmt.Sprintf("eq.%d", chatID)},
		"order":   []string{"summary_date_utc.desc"},
		"limit":   []string{fmt.Sprintf("%d", limit)},
		"select":  []string{"summary_date_utc,summary_text,decisions,action_items,ideas,open_questions,message_count,created_at"},
	}
	data, _, err := r.doRequest(ctx, http.MethodGet, tableSummaries, query, nil, "")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		summaryRow
		SummaryDateUTC string `json:"summary_date_utc"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	summaries := make([]model.DailySummary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, row.summaryRow.toSummary(chatID, row.SummaryDateUTC))
	}
	return summaries, nil
}

// ListMessagesByTagSince returns tagged messages for a time window.
func (r *Repository) ListMessagesByTagSince(ctx context.Context, chatID int64, aiTag string, since time.Time) ([]model.Message, error) {
	query := url.Values{
		"chat_id":    []string{fmt.Sprintf("eq.%d", chatID)},
		"ai_tag":     []string{fmt.Sprintf("eq.%s", aiTag)},
		"created_at": []string{fmt.Sprintf("gte.%s", since.Format(time.RFC3339))},
		"order":      []string{"created_at.desc"},
		"select": []string{strings.Join([]string{
			"id",
			"chat_id",
			"message_id",
			"sender_id",
			"chat_title",
			"message_text",
			"transcript",
			"ai_tag",
			"topic_tag",
			"is_voice",
			"created_at",
		}, ",")},
	}
	data, _, err := r.doRequest(ctx, http.MethodGet, tableMessages, query, nil, "")
	if err != nil {
		return nil, err
	}
	var rows []messageRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return hydrateMessages(rows), nil
}

// ListActiveChatsForSchedule returns active chats due for summary at UTC hour/minute.
func (r *Repository) ListActiveChatsForSchedule(ctx context.Context, hour int, minute int) ([]int64, error) {
	query := url.Values{
		"is_active":          []string{"eq.true"},
		"summary_hour_utc":   []string{fmt.Sprintf("eq.%d", hour)},
		"summary_minute_utc": []string{fmt.Sprintf("eq.%d", minute)},
		"select":             []string{"chat_id"},
	}
	data, _, err := r.doRequest(ctx, http.MethodGet, tableChats, query, nil, "")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ChatID int64 `json:"chat_id"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ChatID)
	}
	return ids, nil
}

// SaveNotionConfig stores per-chat Notion token and database ID.
func (r *Repository) SaveNotionConfig(ctx context.Context, chatID int64, token string, databaseID string) error {
	payload := map[string]any{
		"chat_id":            chatID,
		"notion_token":       token,
		"notion_database_id": databaseID,
		"updated_at":         time.Now().UTC().Format(time.RFC3339),
	}
	query := url.Values{"on_conflict": []string{"chat_id"}}
	_, _, err := r.doRequest(ctx, http.MethodPost, tableNotionConfig, query, payload, "resolution=merge-duplicates")
	return err
}

// GetNotionConfig retrieves Notion settings for a chat. TokenEncrypted is
// returned as raw ciphertext; decrypting it is the caller's responsibility
// (this package has no access to NOTION_ENCRYPTION_KEY).
func (r *Repository) GetNotionConfig(ctx context.Context, chatID int64) (model.NotionConfig, error) {
	query := url.Values{
		"chat_id": []string{fmt.Sprintf("eq.%d", chatID)},
		"select":  []string{"notion_token,notion_token_encrypted,notion_database_id,oauth_workspace_id,oauth_workspace_name,updated_at"},
	}
	data, _, err := r.doRequest(ctx, http.MethodGet, tableNotionConfig, query, nil, "")
	if err != nil {
		return model.NotionConfig{}, err
	}
	// notion_token_encrypted is a bytea column. PostgREST renders bytea as
	// Postgres's own "\x<hex>" text format, not base64 -- unmarshaling it
	// straight into a []byte field would have Go's json package try (and
	// fail) to base64-decode it. Decode it ourselves via decodeBytea instead.
	var rows []struct {
		Token              string    `json:"notion_token"`
		TokenEncrypted     string    `json:"notion_token_encrypted"`
		DatabaseID         string    `json:"notion_database_id"`
		OAuthWorkspaceID   string    `json:"oauth_workspace_id"`
		OAuthWorkspaceName string    `json:"oauth_workspace_name"`
		UpdatedAt          time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return model.NotionConfig{}, err
	}
	if len(rows) == 0 {
		return model.NotionConfig{}, sql.ErrNoRows
	}
	tokenEncrypted, err := decodeBytea(rows[0].TokenEncrypted)
	if err != nil {
		return model.NotionConfig{}, fmt.Errorf("decode notion_token_encrypted: %w", err)
	}
	cfg := model.NotionConfig{
		ChatID:             chatID,
		Token:              rows[0].Token,
		TokenEncrypted:     tokenEncrypted,
		DatabaseID:         rows[0].DatabaseID,
		OAuthWorkspaceID:   rows[0].OAuthWorkspaceID,
		OAuthWorkspaceName: rows[0].OAuthWorkspaceName,
		UpdatedAt:          rows[0].UpdatedAt,
	}
	cfg.Configured = (cfg.Token != "" || len(cfg.TokenEncrypted) > 0) && cfg.DatabaseID != ""
	return cfg, nil
}

// SaveNotionOAuthConfig stores an OAuth-derived Notion connection: an
// encrypted access token and workspace metadata. The database ID is set
// separately once the user picks one from the post-OAuth database picker
// (see SetNotionDatabaseID), since Notion's OAuth grant is workspace-scoped
// and does not include a database selection.
func (r *Repository) SaveNotionOAuthConfig(ctx context.Context, chatID int64, tokenEncrypted []byte, workspaceID string, workspaceName string) error {
	payload := map[string]any{
		"chat_id":                chatID,
		"notion_token_encrypted": encodeBytea(tokenEncrypted),
		"oauth_workspace_id":     workspaceID,
		"oauth_workspace_name":   workspaceName,
		"updated_at":             time.Now().UTC().Format(time.RFC3339),
	}
	query := url.Values{"on_conflict": []string{"chat_id"}}
	_, _, err := r.doRequest(ctx, http.MethodPost, tableNotionConfig, query, payload, "resolution=merge-duplicates")
	return err
}

// SetNotionDatabaseID records the database the user picked after an OAuth
// connection, without touching the stored token.
func (r *Repository) SetNotionDatabaseID(ctx context.Context, chatID int64, databaseID string) error {
	payload := map[string]any{
		"notion_database_id": databaseID,
		"updated_at":         time.Now().UTC().Format(time.RFC3339),
	}
	query := url.Values{"chat_id": []string{fmt.Sprintf("eq.%d", chatID)}}
	_, _, err := r.doRequest(ctx, http.MethodPatch, tableNotionConfig, query, payload, "return=minimal")
	return err
}

// CreateActionItem creates a new action item row in the database.
func (r *Repository) CreateActionItem(ctx context.Context, item model.ActionItem) error {
	payload := map[string]any{
		"chat_id":           item.ChatID,
		"source_message_id": nullablePtr64(item.SourceMessageID),
		"summary_id":        nullablePtr64(item.SummaryID),
		"task":              item.Task,
		"owner":             nullablePtr(item.Owner),
		"assignee_user_id":  nullablePtr64(item.AssigneeUserID),
		"status":            item.Status,
		"due_date":          nullablePtr(item.DueDate),
	}
	_, _, err := r.doRequest(ctx, http.MethodPost, tableActionItems, url.Values{}, payload, "")
	return err
}

// ListActionItems returns action items for a chat, optionally filtered by status.
// If status is empty, all action items are returned.
func (r *Repository) ListActionItems(ctx context.Context, chatID int64, status string) ([]model.ActionItem, error) {
	query := url.Values{
		"chat_id": []string{fmt.Sprintf("eq.%d", chatID)},
		"order":   []string{"created_at.desc"},
		"select": []string{strings.Join([]string{
			"id",
			"chat_id",
			"task",
			"owner",
			"assignee_user_id",
			"status",
			"due_date",
			"created_at",
		}, ",")},
	}
	if status != "" {
		query["status"] = []string{fmt.Sprintf("eq.%s", status)}
	}
	data, _, err := r.doRequest(ctx, http.MethodGet, tableActionItems, query, nil, "")
	if err != nil {
		return nil, err
	}
	var rows []actionItemRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return hydrateActionItems(rows), nil
}

// GetActionItem loads a single action item by ID, used by the dashboard API
// to confirm a chat membership check before allowing a status update.
func (r *Repository) GetActionItem(ctx context.Context, id int64) (model.ActionItem, error) {
	query := url.Values{
		"id":     []string{fmt.Sprintf("eq.%d", id)},
		"select": []string{"id,chat_id,task,owner,assignee_user_id,status,due_date,created_at"},
	}
	data, _, err := r.doRequest(ctx, http.MethodGet, tableActionItems, query, nil, "")
	if err != nil {
		return model.ActionItem{}, err
	}
	var rows []actionItemRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return model.ActionItem{}, err
	}
	if len(rows) == 0 {
		return model.ActionItem{}, sql.ErrNoRows
	}
	items := hydrateActionItems(rows)
	return items[0], nil
}

// UpdateActionItemStatus updates the status of an action item by ID.
// When status becomes 'done', also sets completed_at to now. Always updates updated_at.
func (r *Repository) UpdateActionItemStatus(ctx context.Context, id int64, status string) error {
	payload := map[string]any{
		"status":     status,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	if status == "done" {
		payload["completed_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	query := url.Values{
		"id": []string{fmt.Sprintf("eq.%d", id)},
	}
	_, _, err := r.doRequest(ctx, http.MethodPatch, tableActionItems, query, payload, "return=minimal")
	return err
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

// nullablePtr64 converts a pointer to int64 to either nil or the pointed value.
func nullablePtr64(value *int64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

type messageRow struct {
	ID          int64     `json:"id"`
	ChatID      int64     `json:"chat_id"`
	MessageID   int       `json:"message_id"`
	SenderID    int64     `json:"sender_id"`
	ChatTitle   string    `json:"chat_title"`
	MessageText string    `json:"message_text"`
	Transcript  string    `json:"transcript"`
	AIType      string    `json:"ai_tag"`
	TopicTag    *string   `json:"topic_tag"`
	IsVoice     bool      `json:"is_voice"`
	CreatedAt   time.Time `json:"created_at"`
}

type summaryRow struct {
	SummaryText   string             `json:"summary_text"`
	Decisions     []string           `json:"decisions"`
	ActionItems   []model.ActionItem `json:"action_items"`
	Ideas         []string           `json:"ideas"`
	OpenQuestions []string           `json:"open_questions"`
	MessageCount  int                `json:"message_count"`
	CreatedAt     time.Time          `json:"created_at"`
}

type actionItemRow struct {
	ID             int64     `json:"id"`
	ChatID         int64     `json:"chat_id"`
	Task           string    `json:"task"`
	Owner          *string   `json:"owner"`
	AssigneeUserID *int64    `json:"assignee_user_id"`
	Status         string    `json:"status"`
	DueDate        *string   `json:"due_date"`
	CreatedAt      time.Time `json:"created_at"`
}

// encodeBytea renders bytes as Postgres's "\x<hex>" bytea text input format,
// the format PostgREST/PostgreSQL expect over the REST API -- not base64,
// which is what encoding/json would otherwise produce for a raw []byte field.
func encodeBytea(b []byte) string {
	return "\\x" + hex.EncodeToString(b)
}

// decodeBytea reverses encodeBytea. An empty string (no row, or a NULL
// column) decodes to a nil slice rather than an error.
func decodeBytea(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	s = strings.TrimPrefix(s, "\\x")
	return hex.DecodeString(s)
}

func (s summaryRow) toSummary(chatID int64, dateUTC string) model.DailySummary {
	return model.DailySummary{
		ChatID:         chatID,
		SummaryDateUTC: dateUTC,
		Summary:        s.SummaryText,
		Decisions:      s.Decisions,
		ActionItems:    s.ActionItems,
		Ideas:          s.Ideas,
		OpenQuestions:  s.OpenQuestions,
		MessageCount:   s.MessageCount,
		CreatedAt:      s.CreatedAt,
	}
}

func hydrateMessages(rows []messageRow) []model.Message {
	messages := make([]model.Message, 0, len(rows))
	for _, row := range rows {
		text := row.MessageText
		if strings.TrimSpace(row.Transcript) != "" {
			text = row.Transcript
		}
		messages = append(messages, model.Message{
			ID:         row.ID,
			ChatID:     row.ChatID,
			MessageID:  row.MessageID,
			SenderID:   row.SenderID,
			ChatTitle:  row.ChatTitle,
			Text:       text,
			Transcript: row.Transcript,
			AIType:     row.AIType,
			Topic:      row.TopicTag,
			IsVoice:    row.IsVoice,
			CreatedAt:  row.CreatedAt,
		})
	}
	return messages
}

func hydrateActionItems(rows []actionItemRow) []model.ActionItem {
	items := make([]model.ActionItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, model.ActionItem{
			ID:             &row.ID,
			ChatID:         row.ChatID,
			Task:           row.Task,
			Owner:          row.Owner,
			Status:         row.Status,
			DueDate:        row.DueDate,
			AssigneeUserID: row.AssigneeUserID,
		})
	}
	return items
}

func (r *Repository) doRequest(ctx context.Context, method string, path string, query url.Values, body any, prefer string) ([]byte, int, error) {
	if r.baseURL == "" || r.apiKey == "" {
		return nil, 0, fmt.Errorf("supabase api configuration is missing")
	}
	endpoint, err := r.buildURL(path, query)
	if err != nil {
		return nil, 0, err
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("apikey", r.apiKey)
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if prefer != "" {
		req.Header.Set("Prefer", prefer)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, resp.StatusCode, fmt.Errorf("supabase status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, resp.StatusCode, nil
}

func (r *Repository) buildURL(path string, query url.Values) (string, error) {
	parsed, err := url.Parse(r.baseURL)
	if err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(path, "/")
	if query != nil {
		parsed.RawQuery = query.Encode()
	}
	return parsed.String(), nil
}
