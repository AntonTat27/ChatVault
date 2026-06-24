package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"chatvault/internal/model"
)

const (
	// searchQueryLimit is the maximum number of results to return from a search.
	searchQueryLimit = 50
)

// SearchMessages searches for messages matching the query using full-text search.
// Results are ranked by relevance and scoped to the given chat_id.
func SearchMessages(
	ctx context.Context,
	pool *pgxpool.Pool,
	chatID int64,
	query string,
	limit int,
) ([]model.Message, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}

	if limit <= 0 || limit > searchQueryLimit {
		limit = searchQueryLimit
	}

	// Use plainto_tsquery to safely escape user input and generate a query
	// Order by ts_rank to get most relevant results first
	const querySQL = `
		SELECT id, chat_id, message_id, sender_id, chat_title, message_text,
		       transcript, ai_tag, topic_tag, is_voice, created_at
		FROM messages
		WHERE chat_id = $1 AND search_vector @@ plainto_tsquery('english', $2)
		ORDER BY ts_rank(search_vector, plainto_tsquery('english', $2)) DESC, created_at DESC
		LIMIT $3
	`

	rows, err := pool.Query(ctx, querySQL, chatID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	messages, err := pgx.CollectRows(rows, scanMessage)
	if err != nil {
		return nil, fmt.Errorf("scan search results: %w", err)
	}

	return messages, nil
}

// SemanticSearchMessages finds messages whose stored embedding is closest to
// queryEmbedding (cosine/L2 distance via pgvector's <-> operator), scoped to
// the given chat_id.
func SemanticSearchMessages(
	ctx context.Context,
	pool *pgxpool.Pool,
	chatID int64,
	queryEmbedding []float32,
	limit int,
) ([]model.Message, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}

	if limit <= 0 || limit > searchQueryLimit {
		limit = searchQueryLimit
	}

	const querySQL = `
		SELECT m.id, m.chat_id, m.message_id, m.sender_id, m.chat_title, m.message_text,
		       m.transcript, m.ai_tag, m.topic_tag, m.is_voice, m.created_at
		FROM message_embeddings e
		JOIN messages m ON m.id = e.message_id
		WHERE e.chat_id = $1
		ORDER BY e.embedding <-> $2
		LIMIT $3
	`

	rows, err := pool.Query(ctx, querySQL, chatID, pgvector.NewVector(queryEmbedding), limit)
	if err != nil {
		return nil, fmt.Errorf("semantic search query failed: %w", err)
	}
	defer rows.Close()

	messages, err := pgx.CollectRows(rows, scanMessage)
	if err != nil {
		return nil, fmt.Errorf("scan semantic search results: %w", err)
	}

	return messages, nil
}

// scanMessage is a row scanner for the Message model.
func scanMessage(row pgx.CollectableRow) (model.Message, error) {
	var m model.Message
	var transcript *string
	var topic *string

	err := row.Scan(
		&m.ID,
		&m.ChatID,
		&m.MessageID,
		&m.SenderID,
		&m.ChatTitle,
		&m.Text,
		&transcript,
		&m.AIType,
		&topic,
		&m.IsVoice,
		&m.CreatedAt,
	)
	if err != nil {
		return m, fmt.Errorf("scan message row: %w", err)
	}

	if transcript != nil {
		m.Transcript = *transcript
	}
	m.Topic = topic

	return m, nil
}
