package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chatvault/internal/model"
)

// ErrSessionNotFound indicates a session token has no matching, unexpired row.
var ErrSessionNotFound = errors.New("dashboard session not found or expired")

// UpsertDashboardUser stores or refreshes a Telegram identity that has
// authenticated through the web dashboard's Telegram Login Widget.
func UpsertDashboardUser(ctx context.Context, pool *pgxpool.Pool, user model.DashboardUser) error {
	if pool == nil {
		return fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}
	const querySQL = `
		INSERT INTO dashboard_users (telegram_user_id, first_name, last_name, username, photo_url, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (telegram_user_id) DO UPDATE SET
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			username = EXCLUDED.username,
			photo_url = EXCLUDED.photo_url,
			updated_at = NOW()
	`
	_, err := pool.Exec(ctx, querySQL, user.TelegramUserID, user.FirstName, user.LastName, user.Username, user.PhotoURL)
	if err != nil {
		return fmt.Errorf("upsert dashboard user: %w", err)
	}
	return nil
}

// CreateDashboardSession persists a new session keyed by the sha256 hash of
// the raw session token (the raw token itself only ever lives in the cookie).
func CreateDashboardSession(ctx context.Context, pool *pgxpool.Pool, tokenHash string, telegramUserID int64, expiresAt time.Time) error {
	if pool == nil {
		return fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}
	const querySQL = `
		INSERT INTO dashboard_sessions (token_hash, telegram_user_id, expires_at)
		VALUES ($1, $2, $3)
	`
	_, err := pool.Exec(ctx, querySQL, tokenHash, telegramUserID, expiresAt)
	if err != nil {
		return fmt.Errorf("create dashboard session: %w", err)
	}
	return nil
}

// GetDashboardSession returns the Telegram user ID for a live (unexpired)
// session, or ErrSessionNotFound if the token is invalid, unknown, or expired.
func GetDashboardSession(ctx context.Context, pool *pgxpool.Pool, tokenHash string) (int64, error) {
	if pool == nil {
		return 0, fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}
	const querySQL = `
		SELECT telegram_user_id FROM dashboard_sessions
		WHERE token_hash = $1 AND expires_at > NOW()
	`
	var telegramUserID int64
	err := pool.QueryRow(ctx, querySQL, tokenHash).Scan(&telegramUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrSessionNotFound
		}
		return 0, fmt.Errorf("get dashboard session: %w", err)
	}
	return telegramUserID, nil
}

// DeleteDashboardSession revokes a session immediately (logout).
func DeleteDashboardSession(ctx context.Context, pool *pgxpool.Pool, tokenHash string) error {
	if pool == nil {
		return fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}
	_, err := pool.Exec(ctx, `DELETE FROM dashboard_sessions WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return fmt.Errorf("delete dashboard session: %w", err)
	}
	return nil
}

// GetChatMemberCache returns a cached membership verification for a
// (chat, user) pair, if one exists.
func GetChatMemberCache(ctx context.Context, pool *pgxpool.Pool, chatID int64, telegramUserID int64) (role string, verifiedAt time.Time, found bool, err error) {
	if pool == nil {
		return "", time.Time{}, false, fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}
	const querySQL = `
		SELECT role, verified_at FROM chat_members
		WHERE chat_id = $1 AND telegram_user_id = $2
	`
	scanErr := pool.QueryRow(ctx, querySQL, chatID, telegramUserID).Scan(&role, &verifiedAt)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return "", time.Time{}, false, nil
		}
		return "", time.Time{}, false, fmt.Errorf("get chat member cache: %w", scanErr)
	}
	return role, verifiedAt, true, nil
}

// UpsertChatMember records (or refreshes) a verified Telegram chat membership.
func UpsertChatMember(ctx context.Context, pool *pgxpool.Pool, chatID int64, telegramUserID int64, role string) error {
	if pool == nil {
		return fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}
	const querySQL = `
		INSERT INTO chat_members (chat_id, telegram_user_id, role, verified_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (chat_id, telegram_user_id) DO UPDATE SET
			role = EXCLUDED.role,
			verified_at = NOW()
	`
	_, err := pool.Exec(ctx, querySQL, chatID, telegramUserID, role)
	if err != nil {
		return fmt.Errorf("upsert chat member: %w", err)
	}
	return nil
}

// RemoveChatMember deletes a cached membership row, e.g. after a getChatMember
// refresh shows the user has left or been banned.
func RemoveChatMember(ctx context.Context, pool *pgxpool.Pool, chatID int64, telegramUserID int64) error {
	if pool == nil {
		return fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}
	_, err := pool.Exec(ctx, `DELETE FROM chat_members WHERE chat_id = $1 AND telegram_user_id = $2`, chatID, telegramUserID)
	if err != nil {
		return fmt.Errorf("remove chat member: %w", err)
	}
	return nil
}

// ListChatsForUser returns the chats a Telegram user has a verified, cached
// membership in -- i.e. chats they have previously opened in the dashboard.
func ListChatsForUser(ctx context.Context, pool *pgxpool.Pool, telegramUserID int64) ([]model.ChatSummaryRef, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool is nil; ensure DatabaseURL is configured")
	}
	const querySQL = `
		SELECT c.chat_id, c.chat_title, cm.role
		FROM chat_members cm
		JOIN chats c ON c.chat_id = cm.chat_id
		WHERE cm.telegram_user_id = $1
		ORDER BY c.chat_title ASC
	`
	rows, err := pool.Query(ctx, querySQL, telegramUserID)
	if err != nil {
		return nil, fmt.Errorf("list chats for user: %w", err)
	}
	defer rows.Close()

	chats, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.ChatSummaryRef, error) {
		var ref model.ChatSummaryRef
		if scanErr := row.Scan(&ref.ChatID, &ref.ChatTitle, &ref.Role); scanErr != nil {
			return ref, scanErr
		}
		return ref, nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan chats for user: %w", err)
	}
	return chats, nil
}
