package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"chatvault/internal/model"
	"chatvault/internal/service"
)

const (
	telegramAPIBase          = "https://api.telegram.org"
	telegramCommandSummary   = "/summary"
	telegramCommandIdeas     = "/ideas"
	telegramCommandDecisions = "/decisions"
	telegramCommandActions   = "/actions"
	telegramCommandExport    = "/export"
	telegramCommandNotion    = "/notion"
	processingSummaryMessage = "Generating summary... I will post it shortly."
	processingVoiceMessage   = "Voice received. I am transcribing and tagging it now."
	setupInstructionsMessage = "ChatVault is active. Commands: /summary /ideas /decisions /actions /export"
	maxNotionArgCount        = 3
	reqTimeoutSeconds        = 30
)

var (
	notionCommandRegexp = regexp.MustCompile(`^/notion\s+([^\s]+)\s+([^\s]+)$`)
	doneCommandRegexp   = regexp.MustCompile(`^/done\s+(\d+)$`)
)

// Handler wires Telegram update handling with ChatVault services.
type Handler struct {
	services      *service.Services
	httpClient    *http.Client
	telegramToken string
}

// NewHandler creates a message handler instance.
func NewHandler(services *service.Services, telegramToken string) *Handler {
	return &Handler{
		services:      services,
		httpClient:    &http.Client{Timeout: reqTimeoutSeconds * time.Second},
		telegramToken: telegramToken,
	}
}

// RegisterHandlers adds command and default handlers to the Telegram bot.
func (h *Handler) RegisterHandlers(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, h.handleStart)
	b.RegisterHandler(bot.HandlerTypeMessageText, telegramCommandSummary, bot.MatchTypeExact, h.handleSummary)
	b.RegisterHandler(bot.HandlerTypeMessageText, telegramCommandIdeas, bot.MatchTypeExact, h.handleIdeas)
	b.RegisterHandler(bot.HandlerTypeMessageText, telegramCommandDecisions, bot.MatchTypeExact, h.handleDecisions)
	b.RegisterHandler(bot.HandlerTypeMessageText, telegramCommandActions, bot.MatchTypeExact, h.handleActions)
	b.RegisterHandler(bot.HandlerTypeMessageText, telegramCommandExport, bot.MatchTypeExact, h.handleExport)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, notionCommandRegexp, h.handleNotion)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, doneCommandRegexp, h.handleDone)
}

// DefaultHandler stores every incoming message and triggers async processing.
func (h *Handler) DefaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	// Handle chat events: new members, member left, title changes, pinned messages
	if len(update.Message.NewChatMembers) > 0 {
		for _, m := range update.Message.NewChatMembers {
			display := displayName(m.FirstName, m.LastName, m.Username)
			entry := model.Message{
				ChatID:    update.Message.Chat.ID,
				MessageID: update.Message.ID,
				SenderID:  m.ID,
				ChatTitle: update.Message.Chat.Title,
				Text:      fmt.Sprintf("member_joined: %s", display),
			}
			// store event message (async classification will run)
			_ = h.services.HandleIncomingMessage(ctx, entry)
			// welcome message
			h.replyText(ctx, b, update, fmt.Sprintf("Welcome %s!", display))
		}
		return
	}
	if update.Message.LeftChatMember != nil {
		m := update.Message.LeftChatMember
		display := displayName(m.FirstName, m.LastName, m.Username)
		entry := model.Message{
			ChatID:    update.Message.Chat.ID,
			MessageID: update.Message.ID,
			SenderID:  m.ID,
			ChatTitle: update.Message.Chat.Title,
			Text:      fmt.Sprintf("member_left: %s", display),
		}
		_ = h.services.HandleIncomingMessage(ctx, entry)
		h.replyText(ctx, b, update, fmt.Sprintf("%s left the chat.", display))
		return
	}
	if update.Message.NewChatTitle != "" {
		title := update.Message.NewChatTitle
		entry := model.Message{
			ChatID:    update.Message.Chat.ID,
			MessageID: update.Message.ID,
			SenderID:  update.Message.From.ID,
			ChatTitle: title,
			Text:      fmt.Sprintf("chat_title_changed: %s", title),
		}
		_ = h.services.HandleIncomingMessage(ctx, entry)
		h.replyText(ctx, b, update, fmt.Sprintf("Chat title updated to: %s", title))
		return
	}

	text := update.Message.Text
	if text == "" {
		text = update.Message.Caption
	}

	entry := model.Message{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.ID,
		SenderID:  update.Message.From.ID,
		ChatTitle: update.Message.Chat.Title,
		Text:      text,
		IsVoice:   update.Message.Voice != nil,
	}

	if err := h.services.HandleIncomingMessage(ctx, entry); err != nil {
		h.replyText(ctx, b, update, "Failed to store message.")
		return
	}

	if update.Message.Voice != nil {
		h.replyText(ctx, b, update, processingVoiceMessage)
		voiceFileID := update.Message.Voice.FileID
		go h.processVoice(context.WithoutCancel(ctx), b, update, entry, voiceFileID)
	}
}

// handleStart sends setup instructions to the chat.
func (h *Handler) handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	h.replyText(ctx, b, update, setupInstructionsMessage)
}

// handleSummary triggers asynchronous daily summary generation.
func (h *Handler) handleSummary(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	h.replyText(ctx, b, update, processingSummaryMessage)
	h.services.GenerateSummaryAsync(context.WithoutCancel(ctx), update.Message.Chat.ID, time.Now().UTC(), h.sendText)
}

// handleIdeas returns idea-tagged messages from the last 7 days.
func (h *Handler) handleIdeas(ctx context.Context, b *bot.Bot, update *models.Update) {
	h.handleTaggedLookup(ctx, b, update, model.TagIdea, "Ideas")
}

// handleDecisions returns decision-tagged messages from the last 7 days.
func (h *Handler) handleDecisions(ctx context.Context, b *bot.Bot, update *models.Update) {
	h.handleTaggedLookup(ctx, b, update, model.TagDecision, "Decisions")
}

// handleActions returns open action items as structured entries from the action_items table.
func (h *Handler) handleActions(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	items, err := h.services.ListOpenActionItems(ctx, update.Message.Chat.ID)
	if err != nil {
		h.replyText(ctx, b, update, "Failed to retrieve action items.")
		return
	}
	h.replyText(ctx, b, update, service.FormatActionItemsList(items))
}

// handleDone marks an action item as completed by its ID.
func (h *Handler) handleDone(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	matches := doneCommandRegexp.FindStringSubmatch(update.Message.Text)
	if len(matches) != 2 {
		h.replyText(ctx, b, update, "Usage: /done <id>")
		return
	}
	var id int64
	if _, err := fmt.Sscanf(matches[1], "%d", &id); err != nil {
		h.replyText(ctx, b, update, "Invalid action item ID.")
		return
	}
	if err := h.services.MarkActionItemDone(ctx, id); err != nil {
		h.replyText(ctx, b, update, "Failed to mark action item as done.")
		return
	}
	h.replyText(ctx, b, update, "Action item marked as done.")
}

// handleExport exports today's summary to Notion when connected.
func (h *Handler) handleExport(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	if err := h.services.ExportSummaryToNotionNow(ctx, update.Message.Chat.ID); err != nil {
		h.replyText(ctx, b, update, "Export failed: "+err.Error())
		return
	}
	h.replyText(ctx, b, update, "Summary exported to Notion.")
}

// handleNotion configures Notion token and database ID for the chat.
func (h *Handler) handleNotion(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	parts := strings.Fields(update.Message.Text)
	if len(parts) != maxNotionArgCount {
		h.replyText(ctx, b, update, "Usage: /notion <token> <database_id>")
		return
	}
	if err := h.services.SaveNotionConfig(ctx, update.Message.Chat.ID, parts[1], parts[2]); err != nil {
		h.replyText(ctx, b, update, "Notion configuration failed.")
		return
	}
	h.replyText(ctx, b, update, "Notion integration connected.")
}

// handleTaggedLookup fetches tagged messages and responds with formatted output.
func (h *Handler) handleTaggedLookup(ctx context.Context, b *bot.Bot, update *models.Update, tag string, label string) {
	if update.Message == nil {
		return
	}
	messages, err := h.services.ListTaggedMessages(ctx, update.Message.Chat.ID, tag)
	if err != nil {
		h.replyText(ctx, b, update, "Request failed. Try again later.")
		return
	}
	h.replyText(ctx, b, update, service.FormatTaggedMessages(label, messages))
}

// processVoice runs voice download and processing asynchronously.
func (h *Handler) processVoice(ctx context.Context, b *bot.Bot, update *models.Update, entry model.Message, fileID string) {
	voiceBytes, err := h.downloadTelegramFile(ctx, fileID)
	if err != nil {
		h.replyText(ctx, b, update, "Voice download failed.")
		return
	}
	if err := h.services.ProcessVoiceMessage(ctx, entry, voiceBytes); err != nil {
		h.replyText(ctx, b, update, "Voice processing failed.")
		return
	}
	h.replyText(ctx, b, update, "Voice processed successfully.")
}

// downloadTelegramFile resolves a Telegram file path and downloads file bytes.
func (h *Handler) downloadTelegramFile(ctx context.Context, fileID string) ([]byte, error) {
	getFileURL := fmt.Sprintf("%s/bot%s/getFile?file_id=%s", telegramAPIBase, h.telegramToken, url.QueryEscape(fileID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getFileURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("telegram getFile status %d", resp.StatusCode)
	}

	var getFileResponse struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&getFileResponse); err != nil {
		return nil, err
	}
	if !getFileResponse.OK || getFileResponse.Result.FilePath == "" {
		return nil, fmt.Errorf("telegram getFile returned empty path")
	}

	downloadURL := fmt.Sprintf("%s/file/bot%s/%s", telegramAPIBase, h.telegramToken, getFileResponse.Result.FilePath)
	downloadReq, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	downloadResp, err := h.httpClient.Do(downloadReq)
	if err != nil {
		return nil, err
	}
	defer downloadResp.Body.Close()
	if downloadResp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("telegram file download status %d", downloadResp.StatusCode)
	}

	return io.ReadAll(downloadResp.Body)
}

// sendText posts a plain message to Telegram.
func (h *Handler) sendText(ctx context.Context, chatID int64, text string) error {
	payload, err := json.Marshal(map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		return err
	}
	sendURL := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBase, h.telegramToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("telegram sendMessage status %d", resp.StatusCode)
	}
	return nil
}

// SendText posts a plain message to Telegram and is safe for scheduler callbacks.
func (h *Handler) SendText(ctx context.Context, chatID int64, text string) error {
	return h.sendText(ctx, chatID, text)
}

// replyText sends a message to the current chat and ignores send errors.
func (h *Handler) replyText(ctx context.Context, b *bot.Bot, update *models.Update, text string) {
	if update.Message == nil {
		return
	}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	})
}

// displayName returns a readable name for a user.
func displayName(first, last, username string) string {
	if username != "" {
		return "@" + username
	}
	if last != "" {
		return strings.TrimSpace(first + " " + last)
	}
	return strings.TrimSpace(first)
}
