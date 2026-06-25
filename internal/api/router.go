package api

import (
	"net/http"

	tgbot "github.com/go-telegram/bot"

	"chatvault/internal/auth"
	"chatvault/internal/storage"
)

// NewRouter builds the dashboard API's route table.
func NewRouter(h *Handler, telegramBot *tgbot.Bot, repo *storage.Repository, allowedOrigins []string) http.Handler {
	mux := http.NewServeMux()

	requireAuth := auth.RequireAuth(repo)
	requireChatMembership := auth.RequireChatMembership(telegramBot, repo)
	authed := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(handler)
	}
	authedChatScoped := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(requireChatMembership(handler))
	}

	mux.HandleFunc("POST /auth/telegram/callback", h.handleTelegramCallback)
	mux.HandleFunc("POST /auth/logout", h.handleLogout)
	mux.Handle("GET /auth/notion/start/{id}", authedChatScoped(h.handleNotionOAuthStart))
	mux.Handle("GET /auth/notion/callback", authed(h.handleNotionOAuthCallback))

	mux.Handle("GET /api/chats", authed(h.handleListChats))
	mux.Handle("GET /api/chats/{id}/summaries", authedChatScoped(h.handleSummaries))
	mux.Handle("GET /api/chats/{id}/action-items", authedChatScoped(h.handleActionItems))
	mux.Handle("GET /api/chats/{id}/search", authedChatScoped(h.handleSearch))
	mux.Handle("PATCH /api/action-items/{id}", authed(h.handlePatchActionItem))
	mux.Handle("GET /api/chats/{id}/notion", authedChatScoped(h.handleGetNotionStatus))
	mux.Handle("GET /api/chats/{id}/notion/databases", authedChatScoped(h.handleListNotionDatabases))
	mux.Handle("PATCH /api/chats/{id}/notion/database", authedChatScoped(h.handleSetNotionDatabase))

	return Logging(CORS(allowedOrigins)(mux))
}
