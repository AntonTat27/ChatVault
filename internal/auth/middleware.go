package auth

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5/pgxpool"

	"chatvault/internal/db"
)

type contextKey string

const (
	contextKeyUserID contextKey = "dashboard_user_id"
	contextKeyChatID contextKey = "dashboard_chat_id"
	contextKeyRole   contextKey = "dashboard_chat_role"
)

// chatMembershipCacheTTL bounds how long a cached chat_members row is
// trusted before RequireChatMembership re-verifies against the Bot API.
const chatMembershipCacheTTL = 1 * time.Hour

// UserIDFromContext returns the authenticated Telegram user ID set by
// RequireAuth, and whether one was present.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(contextKeyUserID).(int64)
	return id, ok
}

// ChatIDFromContext returns the verified chat ID set by RequireChatMembership.
func ChatIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(contextKeyChatID).(int64)
	return id, ok
}

// RequireAuth validates the session cookie against dashboard_sessions and
// puts the authenticated Telegram user ID on the request context.
func RequireAuth(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				http.Error(w, "not authenticated", http.StatusUnauthorized)
				return
			}
			userID, err := db.GetDashboardSession(r.Context(), pool, HashToken(cookie.Value))
			if err != nil {
				if errors.Is(err, db.ErrSessionNotFound) {
					http.Error(w, "session expired", http.StatusUnauthorized)
					return
				}
				http.Error(w, "session lookup failed", http.StatusInternalServerError)
				return
			}
			ctx := context.WithValue(r.Context(), contextKeyUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireChatMembership verifies the authenticated user (set by RequireAuth,
// which must run first) is currently a member of the chat identified by the
// "id" path value. It trusts a cached chat_members row for
// chatMembershipCacheTTL before re-checking the Bot API, and the caller can
// force a refresh via ?refresh=true.
func RequireChatMembership(telegramBot *tgbot.Bot, pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				http.Error(w, "not authenticated", http.StatusUnauthorized)
				return
			}
			chatID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
			if err != nil {
				http.Error(w, "invalid chat id", http.StatusBadRequest)
				return
			}

			forceRefresh := r.URL.Query().Get("refresh") == "true"
			role, ok, err := VerifyChatMembership(r.Context(), telegramBot, pool, chatID, userID, forceRefresh)
			if err != nil {
				http.Error(w, "membership verification failed", http.StatusInternalServerError)
				return
			}
			if !ok {
				http.Error(w, "not a member of this chat", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyChatID, chatID)
			ctx = context.WithValue(ctx, contextKeyRole, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// VerifyChatMembership is the shared policy behind RequireChatMembership: it
// trusts a cached chat_members row for chatMembershipCacheTTL, then falls
// back to a live getChatMember call. Exported so handlers whose route
// doesn't carry chat_id in the URL path (and so can't use the middleware
// directly, e.g. PATCH /api/action-items/{id} or the Notion OAuth callback)
// enforce the same recency policy instead of trusting a cache row forever.
func VerifyChatMembership(ctx context.Context, telegramBot *tgbot.Bot, pool *pgxpool.Pool, chatID int64, userID int64, forceRefresh bool) (role string, ok bool, err error) {
	role, verifiedAt, found, err := db.GetChatMemberCache(ctx, pool, chatID, userID)
	if err != nil {
		return "", false, err
	}

	if !found || forceRefresh || time.Since(verifiedAt) > chatMembershipCacheTTL {
		role, err = verifyMembershipViaBotAPI(ctx, telegramBot, pool, chatID, userID)
		if err != nil {
			return "", false, err
		}
	}
	return role, role != "", nil
}

// verifyMembershipViaBotAPI calls Telegram's getChatMember, updates the
// chat_members cache, and returns the member's role (empty if they have
// left or been banned).
func verifyMembershipViaBotAPI(ctx context.Context, telegramBot *tgbot.Bot, pool *pgxpool.Pool, chatID int64, userID int64) (string, error) {
	member, err := telegramBot.GetChatMember(ctx, &tgbot.GetChatMemberParams{
		ChatID: chatID,
		UserID: userID,
	})
	if err != nil {
		return "", err
	}

	role := currentRole(member)
	if role == "" {
		_ = db.RemoveChatMember(ctx, pool, chatID, userID)
		return "", nil
	}
	if err := db.UpsertChatMember(ctx, pool, chatID, userID, role); err != nil {
		return "", err
	}
	return role, nil
}

// currentRole maps a Telegram ChatMember to a role string, or "" if the user
// is not currently a member (left or banned).
func currentRole(member *models.ChatMember) string {
	switch member.Type {
	case models.ChatMemberTypeLeft, models.ChatMemberTypeBanned:
		return ""
	case models.ChatMemberTypeOwner:
		return "owner"
	case models.ChatMemberTypeAdministrator:
		return "administrator"
	case models.ChatMemberTypeRestricted:
		return "restricted"
	default:
		return "member"
	}
}
