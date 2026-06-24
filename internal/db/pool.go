// Package db provides a direct Postgres connection (pgx) for ChatVault
// features that need transactions, joins, or extensions (full-text/vector
// search, billing) that the Supabase PostgREST client in internal/storage
// cannot express cleanly. It operates on the same tables as that client;
// it does not replace it.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a connection pool against the given Postgres DSN.
// On Supabase, this must be the direct/transaction-pooler connection string,
// not the PostgREST REST URL used elsewhere in this codebase.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, fmt.Errorf("database dsn is empty")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database dsn: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
