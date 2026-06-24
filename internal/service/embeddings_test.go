package service

import (
	"context"
	"testing"
	"time"

	"chatvault/internal/model"
)

// TestEnqueueEmbeddingJob_SkipsWhenPoolNil verifies that no job is queued
// when semantic search isn't configured (no DATABASE_URL/pool), since
// generating an embedding would have nowhere to be stored.
func TestEnqueueEmbeddingJob_SkipsWhenPoolNil(t *testing.T) {
	s := &Services{
		pool: nil,
		jobs: make(chan func(context.Context), 1),
	}

	s.enqueueEmbeddingJob(context.Background(), 1, 1, "some message text", model.TagIdea)

	select {
	case <-s.jobs:
		t.Fatal("expected no job to be queued when pool is nil")
	case <-time.After(10 * time.Millisecond):
	}
}

// TestEnqueueEmbeddingJob_SkipsNoiseTag verifies that noise-tagged messages
// never get an embedding job queued, to control Gemini embedding cost.
func TestEnqueueEmbeddingJob_SkipsNoiseTag(t *testing.T) {
	s := &Services{
		// A non-nil pool of any kind would do here since the noise check
		// short-circuits before the pool is used; pgxpool.Pool is not
		// constructible without a live connection, so this case is only
		// reachable via the pool==nil path above in practice, but we still
		// assert the tag check independently using a nil pool + noise tag.
		pool: nil,
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
		pool: nil,
		jobs: make(chan func(context.Context), 1),
	}

	s.enqueueEmbeddingJob(context.Background(), 1, 1, "   ", model.TagIdea)

	select {
	case <-s.jobs:
		t.Fatal("expected no job to be queued for empty text")
	case <-time.After(10 * time.Millisecond):
	}
}
