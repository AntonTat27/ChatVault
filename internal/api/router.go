package api

import (
	"net/http"
	"strconv"

	"chatvault/internal/auth"
	"chatvault/internal/storage"
)

// NewRouter builds the dashboard API's route table.
func NewRouter(h *Handler, repo *storage.Repository, allowedOrigins []string, devAuthBypass bool) http.Handler {
	mux := http.NewServeMux()

	requireAuth := auth.RequireAuth(repo)
	authed := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(handler)
	}

	var authedChatScoped func(http.HandlerFunc) http.Handler
	if devAuthBypass {
		// In dev bypass mode skip the Telegram Bot API membership check entirely —
		// the fake dev user won't be a member of any real chat.
		authedChatScoped = func(handler http.HandlerFunc) http.Handler {
			return requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				chatID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
				if err != nil {
					http.Error(w, "invalid chat id", http.StatusBadRequest)
					return
				}
				handler.ServeHTTP(w, r.WithContext(auth.WithChatID(r.Context(), chatID)))
			}))
		}
	} else {
		requireChatMembership := auth.RequireChatMembership(repo)
		authedChatScoped = func(handler http.HandlerFunc) http.Handler {
			return requireAuth(requireChatMembership(handler))
		}
	}

	mux.HandleFunc("POST /auth/telegram/callback", h.handleTelegramCallback)
	mux.HandleFunc("POST /auth/logout", h.handleLogout)
	mux.Handle("GET /auth/notion/start/{id}", authedChatScoped(h.handleNotionOAuthStart))
	mux.Handle("GET /auth/notion/callback", authed(h.handleNotionOAuthCallback))

	mux.Handle("GET /api/chats", authed(h.handleListChats))
	mux.Handle("GET /api/chats/{id}/summaries", authedChatScoped(h.handleSummaries))
	mux.Handle("GET /api/chats/{id}/action-items", authedChatScoped(h.handleActionItems))
	mux.Handle("GET /api/chats/{id}/messages", authedChatScoped(h.handleListMessages))
	mux.Handle("GET /api/chats/{id}/search", authedChatScoped(h.handleSearch))
	mux.Handle("PATCH /api/action-items/{id}", authed(h.handlePatchActionItem))
	mux.Handle("GET /api/chats/{id}/notion", authedChatScoped(h.handleGetNotionStatus))
	mux.Handle("GET /api/chats/{id}/notion/databases", authedChatScoped(h.handleListNotionDatabases))
	mux.Handle("PATCH /api/chats/{id}/notion/database", authedChatScoped(h.handleSetNotionDatabase))

	return Recover(Logging(CORS(allowedOrigins)(mux)))
}
