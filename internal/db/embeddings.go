package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"chatvault/internal/model"
)

// UpsertMessageEmbedding stores or replaces the embedding for a message.
func UpsertMessageEmbedding(
	ctx context.Context,
	pool *pgxpool.Pool,
	messageID int64,
	chatID int64,
	values []float32,
	modelVersion string,
) error {
	if pool == nil {
		return fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}

	const upsertSQL = `
		INSERT INTO message_embeddings (message_id, chat_id, embedding, model_version)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (message_id) DO UPDATE
		SET embedding = EXCLUDED.embedding, model_version = EXCLUDED.model_version, chat_id = EXCLUDED.chat_id
	`

	_, err := pool.Exec(ctx, upsertSQL, messageID, chatID, pgvector.NewVector(values), modelVersion)
	if err != nil {
		return fmt.Errorf("upsert message embedding: %w", err)
	}
	return nil
}

// ListMessagesMissingEmbeddings returns up to limit messages (with id > afterID,
// ordered by id) that have no row in message_embeddings yet. Noise-tagged
// messages are excluded to control embedding generation cost, matching the
// same skip rule applied to live traffic in Services.enqueueEmbeddingJob.
func ListMessagesMissingEmbeddings(ctx context.Context, pool *pgxpool.Pool, afterID int64, limit int) ([]model.Message, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}

	const querySQL = `
		SELECT m.id, m.chat_id, m.message_id, m.sender_id, m.chat_title, m.message_text,
		       m.transcript, m.ai_tag, m.topic_tag, m.is_voice, m.created_at
		FROM messages m
		LEFT JOIN message_embeddings e ON e.message_id = m.id
		WHERE e.message_id IS NULL AND m.id > $1 AND (m.ai_tag IS NULL OR m.ai_tag != 'noise')
		ORDER BY m.id
		LIMIT $2
	`

	rows, err := pool.Query(ctx, querySQL, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list messages missing embeddings: %w", err)
	}
	defer rows.Close()

	messages, err := pgx.CollectRows(rows, scanMessage)
	if err != nil {
		return nil, fmt.Errorf("scan messages missing embeddings: %w", err)
	}
	return messages, nil
}
