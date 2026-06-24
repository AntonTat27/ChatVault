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
	"chatvault/internal/db"

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
	if cfg.DatabaseURL == "" {
		log.Fatalf("DATABASE_URL is required to run the embedding backfill")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database pool init failed: %v", err)
	}
	defer pool.Close()

	geminiClient := ai.NewGeminiClient(cfg.GeminiAPIKey, cfg.GeminiClassificationModel, cfg.GeminiSummaryModel, cfg.GeminiEmbeddingModel, cfg.HTTPTimeout)

	var afterID int64
	var processed, failed int
	for {
		messages, err := db.ListMessagesMissingEmbeddings(ctx, pool, afterID, backfillBatchSize)
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
			if err := db.UpsertMessageEmbedding(ctx, pool, msg.ID, msg.ChatID, values, cfg.GeminiEmbeddingModel); err != nil {
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
