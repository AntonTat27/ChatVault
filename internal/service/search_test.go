package service

import (
	"context"
	"testing"

	"chatvault/internal/model"
)

// TestSearchMessages_DelegatesToRepo verifies that SearchMessages passes the
// chat ID and query through to the repository (PostgREST search_messages RPC)
// and returns whatever it gets back.
func TestSearchMessages_DelegatesToRepo(t *testing.T) {
	want := []model.Message{{ID: 1, Text: "hello"}}
	repo := &fakeActionItemRepo{searchResult: want}
	s := &Services{repo: repo}

	got, err := s.SearchMessages(context.Background(), 12345, "test query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("expected delegated result, got %v", got)
	}
	if repo.searchChatID != 12345 || repo.searchQuery != "test query" {
		t.Fatalf("expected args to be forwarded, got chat_id=%d query=%q", repo.searchChatID, repo.searchQuery)
	}
}
