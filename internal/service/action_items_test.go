package service

import (
	"context"
	"testing"
	"time"

	"chatvault/internal/model"
)

// fakeActionItemRepo is a minimal summaryRepository implementation that only
// tracks CreateActionItem invocations, for asserting action item persistence
// behavior in isolation from network-backed dependencies (Gemini, PostgREST).
type fakeActionItemRepo struct {
	createCalls []model.ActionItem

	searchResult []model.Message
	searchChatID int64
	searchQuery  string
}

func (f *fakeActionItemRepo) UpsertChat(ctx context.Context, chatID int64, chatTitle string, summaryHour int, summaryMinute int) error {
	return nil
}
func (f *fakeActionItemRepo) InsertMessage(ctx context.Context, msg model.Message) (int64, error) {
	return 0, nil
}
func (f *fakeActionItemRepo) UpdateClassification(ctx context.Context, chatID int64, messageID int, result model.ClassificationResult) error {
	return nil
}
func (f *fakeActionItemRepo) SaveVoiceProcessing(ctx context.Context, chatID int64, messageID int, transcript string, classification model.ClassificationResult) error {
	return nil
}
func (f *fakeActionItemRepo) ListMessagesForDate(ctx context.Context, chatID int64, dateUTC time.Time) ([]model.Message, error) {
	return nil, nil
}
func (f *fakeActionItemRepo) SaveSummary(ctx context.Context, summary model.DailySummary) (int64, error) {
	return 0, nil
}
func (f *fakeActionItemRepo) GetSummary(ctx context.Context, chatID int64, dateUTC string) (model.DailySummary, error) {
	return model.DailySummary{}, nil
}
func (f *fakeActionItemRepo) ListSummaries(ctx context.Context, chatID int64, limit int) ([]model.DailySummary, error) {
	return nil, nil
}
func (f *fakeActionItemRepo) ListMessagesByTagSince(ctx context.Context, chatID int64, aiTag string, since time.Time) ([]model.Message, error) {
	return nil, nil
}
func (f *fakeActionItemRepo) ListActiveChatsForSchedule(ctx context.Context, hour int, minute int) ([]int64, error) {
	return nil, nil
}
func (f *fakeActionItemRepo) SaveNotionConfig(ctx context.Context, chatID int64, token string, databaseID string) error {
	return nil
}
func (f *fakeActionItemRepo) GetNotionConfig(ctx context.Context, chatID int64) (model.NotionConfig, error) {
	return model.NotionConfig{}, nil
}
func (f *fakeActionItemRepo) SaveNotionOAuthConfig(ctx context.Context, chatID int64, tokenEncrypted []byte, workspaceID string, workspaceName string) error {
	return nil
}
func (f *fakeActionItemRepo) SetNotionDatabaseID(ctx context.Context, chatID int64, databaseID string) error {
	return nil
}
func (f *fakeActionItemRepo) CreateActionItem(ctx context.Context, item model.ActionItem) error {
	f.createCalls = append(f.createCalls, item)
	return nil
}
func (f *fakeActionItemRepo) ListActionItems(ctx context.Context, chatID int64, status string) ([]model.ActionItem, error) {
	return nil, nil
}
func (f *fakeActionItemRepo) GetActionItem(ctx context.Context, id int64) (model.ActionItem, error) {
	return model.ActionItem{}, nil
}
func (f *fakeActionItemRepo) UpdateActionItemStatus(ctx context.Context, id int64, status string) error {
	return nil
}
func (f *fakeActionItemRepo) SearchMessages(ctx context.Context, chatID int64, query string, limit int) ([]model.Message, error) {
	f.searchChatID = chatID
	f.searchQuery = query
	return f.searchResult, nil
}
func (f *fakeActionItemRepo) SemanticSearchMessages(ctx context.Context, chatID int64, queryEmbedding []float32, limit int) ([]model.Message, error) {
	return nil, nil
}
func (f *fakeActionItemRepo) UpsertMessageEmbedding(ctx context.Context, messageID int64, chatID int64, values []float32, modelVersion string) error {
	return nil
}

// TestCreateActionItemsForSummaryCallsRepoOncePerItem verifies that summary
// generation persists each extracted action item as a durable row via
// repo.CreateActionItem, rather than leaving the action_items table empty.
func TestCreateActionItemsForSummaryCallsRepoOncePerItem(t *testing.T) {
	repo := &fakeActionItemRepo{}
	s := &Services{repo: repo}

	owner := "alice"
	due := "2026-07-01"
	items := []model.ActionItem{
		{Task: "Ship release notes", Owner: &owner},
		{Task: "Schedule retro", DueDate: &due},
		{Task: "Update roadmap doc"},
	}

	const chatID = int64(555)
	const summaryID = int64(42)
	s.createActionItemsForSummary(context.Background(), chatID, summaryID, items)

	if len(repo.createCalls) != len(items) {
		t.Fatalf("expected CreateActionItem to be called %d times, got %d", len(items), len(repo.createCalls))
	}

	for i, call := range repo.createCalls {
		if call.ChatID != chatID {
			t.Errorf("call %d: expected chat_id %d, got %d", i, chatID, call.ChatID)
		}
		if call.SummaryID == nil || *call.SummaryID != summaryID {
			t.Errorf("call %d: expected summary_id %d, got %v", i, summaryID, call.SummaryID)
		}
		if call.Task != items[i].Task {
			t.Errorf("call %d: expected task %q, got %q", i, items[i].Task, call.Task)
		}
		if call.Status != "open" {
			t.Errorf("call %d: expected status 'open', got %q", i, call.Status)
		}
		if call.AssigneeUserID != nil {
			t.Errorf("call %d: expected nil assignee_user_id, got %v", i, *call.AssigneeUserID)
		}
	}

	if repo.createCalls[1].DueDate == nil || *repo.createCalls[1].DueDate != due {
		t.Errorf("expected due date %q to be passed through, got %v", due, repo.createCalls[1].DueDate)
	}
}

// TestCreateActionItemsForSummaryNoItems verifies no repo calls happen when
// there are no extracted action items.
func TestCreateActionItemsForSummaryNoItems(t *testing.T) {
	repo := &fakeActionItemRepo{}
	s := &Services{repo: repo}

	s.createActionItemsForSummary(context.Background(), 1, 1, nil)

	if len(repo.createCalls) != 0 {
		t.Fatalf("expected no CreateActionItem calls, got %d", len(repo.createCalls))
	}
}
