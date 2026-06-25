// Command backfill-embeddings generates pgvector embeddings for existing
// messages that predate semantic search (Phase 2b), so historical chat
// history is searchable via /semantic-search alongside new traffic.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"chatvault/internal/ai"
	"chatvault/internal/config"
	"chatvault/internal/storage"

	"github.com/joho/godotenv"
)

const backfillBatchSize = 50

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	repo := storage.NewRepository(cfg.SupabaseURL, cfg.SupabaseSecretKey, cfg.HTTPTimeout)
	geminiClient := ai.NewGeminiClient(cfg.GeminiAPIKey, cfg.GeminiClassificationModel, cfg.GeminiSummaryModel, cfg.GeminiEmbeddingModel, cfg.HTTPTimeout)

	var afterID int64
	var processed, failed int
	for {
		messages, err := repo.ListMessagesMissingEmbeddings(ctx, afterID, backfillBatchSize)
		if err != nil {
			log.Fatalf("list messages missing embeddings: %v", err)
		}
		if len(messages) == 0 {
			break
		}

		for _, msg := range messages {
			afterID = msg.ID
			text := msg.Text
			if msg.Transcript != "" {
				text = msg.Transcript
			}
			if text == "" {
				continue
			}

			values, err := geminiClient.GenerateEmbedding(ctx, text)
			if err != nil {
				log.Printf("embedding generation failed message_id=%d: %v", msg.ID, err)
				failed++
				continue
			}
			if err := repo.UpsertMessageEmbedding(ctx, msg.ID, msg.ChatID, values, cfg.GeminiEmbeddingModel); err != nil {
				log.Printf("embedding write failed message_id=%d: %v", msg.ID, err)
				failed++
				continue
			}
			processed++
		}

		select {
		case <-ctx.Done():
			log.Printf("backfill interrupted: processed=%d failed=%d", processed, failed)
			return
		default:
		}
	}

	log.Printf("backfill complete: processed=%d failed=%d", processed, failed)
}
