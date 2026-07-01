package auth

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"chatvault/internal/storage"
)

type contextKey string

const (
	contextKeyUserID contextKey = "dashboard_user_id"
	contextKeyChatID contextKey = "dashboard_chat_id"
)

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

// WithChatID injects a chat ID into the context, bypassing membership
// verification. Use only in dev bypass mode.
func WithChatID(ctx context.Context, chatID int64) context.Context {
	return context.WithValue(ctx, contextKeyChatID, chatID)
}

// RequireAuth validates the session cookie against dashboard_sessions and
// puts the authenticated Telegram user ID on the request context.
func RequireAuth(repo *storage.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				http.Error(w, "not authenticated", http.StatusUnauthorized)
				return
			}
			userID, err := repo.GetDashboardSession(r.Context(), HashToken(cookie.Value))
			if err != nil {
				if errors.Is(err, storage.ErrSessionNotFound) {
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
// which must run first) has been granted dashboard access to the chat
// identified by the "id" path value -- either as the chat's owner (recorded
// automatically when the bot was added) or by having run /dashboard
// themselves. This is a plain lookup against a permanent grant; there is no
// live Telegram re-check and no expiry.
func RequireChatMembership(repo *storage.Repository) func(http.Handler) http.Handler {
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

			authorized, err := VerifyChatMembership(r.Context(), repo, chatID, userID)
			if err != nil {
				http.Error(w, "membership verification failed", http.StatusInternalServerError)
				return
			}
			if !authorized {
				http.Error(w, "not authorized for this chat", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyChatID, chatID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// VerifyChatMembership is the shared policy behind RequireChatMembership,
// exported so handlers whose route doesn't carry chat_id in the URL path
// (and so can't use the middleware directly, e.g. PATCH /api/action-items/{id}
// or the Notion OAuth callback) enforce the same access check.
func VerifyChatMembership(ctx context.Context, repo *storage.Repository, chatID int64, userID int64) (bool, error) {
	return repo.IsChatAuthorized(ctx, chatID, userID)
}
