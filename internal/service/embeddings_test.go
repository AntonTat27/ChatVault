package service

import (
	"context"
	"testing"
	"time"

	"chatvault/internal/model"
)

// TestEnqueueEmbeddingJob_SkipsNoiseTag verifies that noise-tagged messages
// never get an embedding job queued, to control Gemini embedding cost.
func TestEnqueueEmbeddingJob_SkipsNoiseTag(t *testing.T) {
	s := &Services{
		jobs: make(chan func(context.Context), 1),
	}

	s.enqueueEmbeddingJob(context.Background(), 1, 1, "some message text", model.TagNoise)

	select {
	case <-s.jobs:
		t.Fatal("expected no job to be queued for a noise-tagged message")
	case <-time.After(10 * time.Millisecond):
	}
}

// TestEnqueueEmbeddingJob_SkipsEmptyText verifies that messages with no
// usable text (e.g. failed transcription with empty fallback) don't queue
// an embedding job.
func TestEnqueueEmbeddingJob_SkipsEmptyText(t *testing.T) {
	s := &Services{
		jobs: make(chan func(context.Context), 1),
	}

	s.enqueueEmbeddingJob(context.Background(), 1, 1, "   ", model.TagIdea)

	select {
	case <-s.jobs:
		t.Fatal("expected no job to be queued for empty text")
	case <-time.After(10 * time.Millisecond):
	}
}
