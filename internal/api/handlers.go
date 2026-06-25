package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"

	"chatvault/internal/auth"
	"chatvault/internal/model"
	"chatvault/internal/notion"
	"chatvault/internal/service"
	"chatvault/internal/storage"
)

// allowedActionItemStatuses mirrors the CHECK constraint on action_items.status.
var allowedActionItemStatuses = map[string]struct{}{
	"open":        {},
	"in_progress": {},
	"done":        {},
	"cancelled":   {},
}

const (
	summaryListLimit        = 30
	defaultNotionAPIVersion = "2022-06-28"
)

// Handler holds the dependencies for all dashboard API routes.
type Handler struct {
	services         *service.Services
	repo             *storage.Repository
	telegramBot      *tgbot.Bot
	telegramBotToken string
	notionOAuth      notion.OAuthConfig
	sessionSecret    string
	dashboardBaseURL string
	httpClient       *http.Client
}

// NewHandler creates a Handler. telegramBot is used to re-verify chat
// membership (via getChatMember) for routes that can't rely on the
// RequireChatMembership middleware because chat_id isn't in their URL path.
func NewHandler(services *service.Services, repo *storage.Repository, telegramBot *tgbot.Bot, telegramBotToken string, notionOAuth notion.OAuthConfig, sessionSecret string, dashboardBaseURL string, httpTimeout time.Duration) *Handler {
	return &Handler{
		services:         services,
		repo:             repo,
		telegramBot:      telegramBot,
		telegramBotToken: telegramBotToken,
		notionOAuth:      notionOAuth,
		sessionSecret:    sessionSecret,
		dashboardBaseURL: dashboardBaseURL,
		httpClient:       &http.Client{Timeout: httpTimeout},
	}
}

type telegramLoginPayload struct {
	ID        json.Number `json:"id"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	Username  string      `json:"username"`
	PhotoURL  string      `json:"photo_url"`
	AuthDate  json.Number `json:"auth_date"`
	Hash      string      `json:"hash"`
}

// handleTelegramCallback verifies a Telegram Login Widget payload, upserts
// the dashboard user, and issues a session cookie.
func (h *Handler) handleTelegramCallback(w http.ResponseWriter, r *http.Request) {
	var payload telegramLoginPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("ERROR: failed to decode telegram payload: %v, content-type: %s", err, r.Header.Get("Content-Type"))
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	idStr := string(payload.ID)
	authDateStr := string(payload.AuthDate)
	if idStr == "" || authDateStr == "" || payload.Hash == "" {
		log.Printf("ERROR: missing required fields - id: '%s', auth_date: '%s', hash: '%s'", idStr, authDateStr, payload.Hash)
		http.Error(w, "missing required login fields", http.StatusBadRequest)
		return
	}

	fields := map[string]string{
		"id":         idStr,
		"first_name": payload.FirstName,
		"auth_date":  authDateStr,
	}
	if payload.LastName != "" {
		fields["last_name"] = payload.LastName
	}
	if payload.Username != "" {
		fields["username"] = payload.Username
	}
	if payload.PhotoURL != "" {
		fields["photo_url"] = payload.PhotoURL
	}

	if !auth.VerifyTelegramLoginHash(h.telegramBotToken, fields, payload.Hash) {
		http.Error(w, "invalid login signature", http.StatusUnauthorized)
		return
	}
	if !auth.IsAuthDateFresh(authDateStr) {
		http.Error(w, "login payload expired", http.StatusUnauthorized)
		return
	}

	telegramUserID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid telegram user id", http.StatusBadRequest)
		return
	}

	user := model.DashboardUser{
		TelegramUserID: telegramUserID,
		FirstName:      payload.FirstName,
		LastName:       payload.LastName,
		Username:       payload.Username,
		PhotoURL:       payload.PhotoURL,
	}
	if err := h.repo.UpsertDashboardUser(r.Context(), user); err != nil {
		http.Error(w, "failed to save user", http.StatusInternalServerError)
		return
	}

	rawToken, tokenHash, err := auth.GenerateSessionToken()
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	expiresAt := time.Now().Add(auth.SessionTTL)
	if err := h.repo.CreateDashboardSession(r.Context(), tokenHash, telegramUserID, expiresAt); err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    rawToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteNoneMode,
	})

	writeJSON(w, http.StatusOK, user)
}

// handleLogout revokes the current session and clears the cookie.
func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil && cookie.Value != "" {
		_ = h.repo.DeleteDashboardSession(r.Context(), auth.HashToken(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// isSecureRequest reports whether the request arrived over HTTPS, directly
// or via a TLS-terminating proxy (Railway/Fly/etc. set X-Forwarded-Proto).
// The session cookie's Secure flag must follow this rather than being
// hardcoded true, since browsers silently refuse to store Secure cookies on
// plain http://localhost -- which is exactly how `npm run dev` talks to this
// API locally.
func isSecureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// handleListChats returns the chats the authenticated user has a verified,
// cached membership in.
func (h *Handler) handleListChats(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	chats, err := h.repo.ListChatsForUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to list chats", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, chats)
}

// handleSummaries returns recent daily summaries for a chat the caller's
// membership has already been verified for by RequireChatMembership.
func (h *Handler) handleSummaries(w http.ResponseWriter, r *http.Request) {
	chatID, ok := auth.ChatIDFromContext(r.Context())
	if !ok {
		http.Error(w, "missing chat id", http.StatusBadRequest)
		return
	}
	summaries, err := h.services.ListSummaries(r.Context(), chatID, summaryListLimit)
	if err != nil {
		http.Error(w, "failed to list summaries", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, summaries)
}

// handleActionItems returns action items for a chat, optionally filtered by
// the "status" query param.
func (h *Handler) handleActionItems(w http.ResponseWriter, r *http.Request) {
	chatID, ok := auth.ChatIDFromContext(r.Context())
	if !ok {
		http.Error(w, "missing chat id", http.StatusBadRequest)
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" {
		if _, valid := allowedActionItemStatuses[status]; !valid {
			http.Error(w, "invalid status filter", http.StatusBadRequest)
			return
		}
	}
	items, err := h.services.ListActionItems(r.Context(), chatID, status)
	if err != nil {
		http.Error(w, "failed to list action items", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type patchActionItemRequest struct {
	Status string `json:"status"`
}

// handlePatchActionItem updates an action item's status, after confirming
// the caller has a cached, verified membership in the item's chat. The route
// is not nested under /chats/{id}, so membership can't be checked by the
// router-level RequireChatMembership middleware; it's checked here instead.
func (h *Handler) handlePatchActionItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid action item id", http.StatusBadRequest)
		return
	}

	var body patchActionItemRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if _, valid := allowedActionItemStatuses[body.Status]; !valid {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	item, err := h.services.GetActionItem(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "action item not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load action item", http.StatusInternalServerError)
		return
	}

	if _, ok, err := auth.VerifyChatMembership(r.Context(), h.telegramBot, h.repo, item.ChatID, userID, false); err != nil {
		http.Error(w, "membership verification failed", http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "not a member of this action item's chat", http.StatusForbidden)
		return
	}

	if err := h.services.UpdateActionItemStatus(r.Context(), id, body.Status); err != nil {
		http.Error(w, "failed to update action item", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSearch runs full-text (default) or semantic (?mode=semantic) search
// over a chat's message history.
func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	chatID, ok := auth.ChatIDFromContext(r.Context())
	if !ok {
		http.Error(w, "missing chat id", http.StatusBadRequest)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "missing query parameter q", http.StatusBadRequest)
		return
	}

	var (
		messages []model.Message
		err      error
	)
	if r.URL.Query().Get("mode") == "semantic" {
		messages, err = h.services.SemanticSearchMessages(r.Context(), chatID, query)
	} else {
		messages, err = h.services.SearchMessages(r.Context(), chatID, query)
	}
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// buildNotionState produces a tamper-evident OAuth state value that embeds
// the chat_id, since Notion only echoes the opaque "state" string back on
// callback -- it can't carry our own query params through the redirect.
func buildNotionState(sessionSecret string, chatID int64) string {
	payload := strconv.FormatInt(chatID, 10)
	mac := hmac.New(sha256.New, []byte(sessionSecret))
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

// verifyNotionState reverses buildNotionState, rejecting a forged or
// mismatched state so a malicious actor can't redirect their own Notion
// grant into someone else's chat.
func verifyNotionState(sessionSecret string, state string) (int64, bool) {
	payload, sig, found := strings.Cut(state, ".")
	if !found {
		return 0, false
	}
	mac := hmac.New(sha256.New, []byte(sessionSecret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return 0, false
	}
	chatID, err := strconv.ParseInt(payload, 10, 64)
	if err != nil {
		return 0, false
	}
	return chatID, true
}

// handleNotionOAuthStart redirects to Notion's authorization screen for the
// chat identified by the "id" path value. RequireChatMembership (applied at
// the router level) has already confirmed the caller belongs to that chat.
func (h *Handler) handleNotionOAuthStart(w http.ResponseWriter, r *http.Request) {
	chatID, ok := auth.ChatIDFromContext(r.Context())
	if !ok {
		http.Error(w, "missing chat id", http.StatusBadRequest)
		return
	}
	state := buildNotionState(h.sessionSecret, chatID)
	http.Redirect(w, r, notion.BuildAuthorizationURL(h.notionOAuth, state), http.StatusFound)
}

// handleNotionOAuthCallback completes the OAuth handshake: it recovers the
// chat_id from state, confirms the logged-in user is a cached member of that
// chat, exchanges the code for an access token, and stores it encrypted.
// This route runs behind RequireAuth only (not RequireChatMembership),
// because the chat_id arrives via state, not the URL path, on this leg.
func (h *Handler) handleNotionOAuthCallback(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}
	chatID, ok := verifyNotionState(h.sessionSecret, state)
	if !ok {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	if _, ok, err := auth.VerifyChatMembership(r.Context(), h.telegramBot, h.repo, chatID, userID, false); err != nil {
		http.Error(w, "membership verification failed", http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "not a member of this chat", http.StatusForbidden)
		return
	}

	token, err := notion.ExchangeCodeForToken(r.Context(), h.httpClient, h.notionOAuth, code)
	if err != nil {
		http.Error(w, "notion oauth exchange failed", http.StatusBadGateway)
		return
	}
	if err := h.services.SaveNotionOAuthConfig(r.Context(), chatID, token.AccessToken, token.WorkspaceID, token.WorkspaceName); err != nil {
		http.Error(w, "failed to save notion connection", http.StatusInternalServerError)
		return
	}

	redirectURL := fmt.Sprintf("%s/dashboard/chats/%d/integrations?notion=connected", strings.TrimRight(h.dashboardBaseURL, "/"), chatID)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

type notionStatusResponse struct {
	Configured    bool   `json:"configured"`
	WorkspaceName string `json:"workspace_name"`
	DatabaseID    string `json:"database_id"`
}

// handleGetNotionStatus returns whether a chat has Notion connected and, if
// so, which workspace/database -- without ever exposing the access token.
func (h *Handler) handleGetNotionStatus(w http.ResponseWriter, r *http.Request) {
	chatID, ok := auth.ChatIDFromContext(r.Context())
	if !ok {
		http.Error(w, "missing chat id", http.StatusBadRequest)
		return
	}
	cfg, err := h.services.GetNotionConfig(r.Context(), chatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusOK, notionStatusResponse{})
			return
		}
		http.Error(w, "failed to load notion connection", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, notionStatusResponse{
		Configured:    cfg.Configured,
		WorkspaceName: cfg.OAuthWorkspaceName,
		DatabaseID:    cfg.DatabaseID,
	})
}

// handleListNotionDatabases lists the databases the chat's connected Notion
// workspace can see, for the post-OAuth database picker (Notion's OAuth
// grant is workspace-scoped, not database-scoped).
func (h *Handler) handleListNotionDatabases(w http.ResponseWriter, r *http.Request) {
	chatID, ok := auth.ChatIDFromContext(r.Context())
	if !ok {
		http.Error(w, "missing chat id", http.StatusBadRequest)
		return
	}
	cfg, err := h.services.GetNotionConfig(r.Context(), chatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "notion is not connected for this chat", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load notion connection", http.StatusInternalServerError)
		return
	}
	if cfg.Token == "" {
		http.Error(w, "notion is not connected for this chat", http.StatusNotFound)
		return
	}

	databases, err := notion.SearchDatabases(r.Context(), h.httpClient, cfg.Token, defaultNotionAPIVersion)
	if err != nil {
		http.Error(w, "failed to list notion databases", http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, databases)
}

type setNotionDatabaseRequest struct {
	DatabaseID string `json:"database_id"`
}

// handleSetNotionDatabase records which database the user picked after
// connecting Notion via OAuth.
func (h *Handler) handleSetNotionDatabase(w http.ResponseWriter, r *http.Request) {
	chatID, ok := auth.ChatIDFromContext(r.Context())
	if !ok {
		http.Error(w, "missing chat id", http.StatusBadRequest)
		return
	}
	var body setNotionDatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.DatabaseID == "" {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.services.SetNotionDatabaseID(r.Context(), chatID, body.DatabaseID); err != nil {
		http.Error(w, "failed to set notion database", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
