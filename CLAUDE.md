# CLAUDE.md

Context for AI agents working on ChatVault. Read this before making changes — it captures architecture decisions and gotchas that aren't obvious from the code alone. `README.md` is the user-facing setup/usage doc; `PRODUCT_ROADMAP.md` is a chronological build log of the phased feature work (Phase 0–5) — both are still accurate but written for a different audience/purpose than this file.

## What this is

A Telegram group knowledge-base bot (Go) with AI tagging, voice transcription, daily summaries, action-item tracking, full-text/semantic search, a Notion export integration, and a web dashboard (separate API binary + React frontend in `landing/`).

## Architecture

Two independently deployable Go binaries plus one frontend:
- `cmd/chatvault` — the Telegram long-polling bot process (message ingestion, AI tagging, daily summary scheduler, slash commands).
- `cmd/chatvault-api` — the dashboard's HTTP API (Telegram-Login-Widget auth, per-chat data endpoints, Notion OAuth callback).
- `cmd/backfill-embeddings` — one-off CLI to generate embeddings for messages stored before semantic search was enabled.
- `landing/` — Vite + React + Tailwind SPA. Marketing page plus dashboard routes (`/login`, `/dashboard`, `/dashboard/chats/:id`, `/dashboard/chats/:id/integrations`). No router/data-fetching library beyond what's needed at this scale (plain `fetch` wrapper, not react-query).

**There is no direct Postgres connection anywhere in this codebase.** Every feature — message storage, full-text search, semantic/vector search, action items, daily summaries, Notion config, dashboard sessions, chat-membership caching — talks to the Supabase Postgres database exclusively through Supabase's **PostgREST REST API**, via the single data-access type `internal/storage.Repository`. A prior iteration of this project briefly added a direct `pgx`/`pgxpool`/`DATABASE_URL` connection (`internal/db`) for search and dashboard reads; that was **removed** in favor of extending `Repository` to cover those needs through REST + two SQL RPC functions. Do not reintroduce a direct DB connection — extend `Repository` instead.

### Why PostgREST can cover everything, including search

PostgREST's query language (`url.Values` filters like `eq.`, `gte.`, `gt.`; `select=` for column lists and embedded-resource joins; `order=`) covers ordinary CRUD and even joins (e.g. `chat_members?select=chat_id,role,chats(chat_title)`). It also exposes SQL views as queryable tables (`messages_missing_embeddings`, used by the embeddings backfill). The two things it genuinely can't express are *ordering by a computed/non-column expression* — `ts_rank()` for full-text search, and pgvector's `<->` distance operator for semantic search. Both are solved by wrapping the ranking/ordering logic in a Postgres function and calling it via PostgREST's RPC convention (`POST /rest/v1/rpc/<function_name>`, JSON body = function args). See `migrations/007_supabase_rpc.sql` (`search_messages`, `semantic_search_messages`) and the corresponding `Repository.SearchMessages` / `Repository.SemanticSearchMessages` methods.

## Key packages

- `internal/storage/repository.go` — **the only data-access layer**. Hand-rolled PostgREST client (no pgx/database/sql, no ORM). Every method does `doRequest(ctx, method, path, query url.Values, body any, prefer string)` and unmarshals JSON. Tables: `chats`, `messages`, `daily_summaries`, `notion_configs`, `action_items`, `message_embeddings`, `dashboard_users`, `dashboard_sessions`, `chat_members`; view: `messages_missing_embeddings`; RPCs: `rpc/search_messages`, `rpc/semantic_search_messages`.
- `internal/service/services.go` — `Services` struct orchestrates everything: holds `repo` (the `summaryRepository` interface, implicitly satisfied by `*storage.Repository`), `gemini`, `transcriber`, `storageClient`, `notionClient`, plus an internal job queue (buffered channel + worker goroutine(s)) for async work (classification, embedding generation, Notion export). Telegram posting is injected via a `PostMessageFn` callback rather than a direct dependency on the bot package.
- `internal/auth/` — dashboard auth: Telegram Login Widget HMAC verification (`telegram.go`), session issuance/lookup (`session.go`), and middleware (`middleware.go`: `RequireAuth`, `RequireChatMembership`) that all take `*storage.Repository` directly (not a pool) and call `repo.GetDashboardSession`, `repo.GetChatMemberCache`, `repo.UpsertChatMember`, `repo.RemoveChatMember`. Membership is verified against the Telegram Bot API (`getChatMember`) and cached in `chat_members` for an hour.
- `internal/api/` — the dashboard HTTP API: `handlers.go`, `router.go`, `middleware.go` (CORS, logging), `server.go` (graceful shutdown via `signal.NotifyContext`, mirroring `cmd/chatvault/main.go`'s pattern). `Handler` and `NewRouter` take `*storage.Repository`, not a connection pool.
- `internal/bot/handler.go` — Telegram command registration via `b.RegisterHandler(bot.HandlerTypeMessageText, "/cmd", bot.MatchTypeExact, h.handlerFunc)`; one regex handler for the legacy `/notion <token> <db_id>` command. Adding a command = add a const + handler method + `RegisterHandler` call.
- `internal/ai/` — Gemini clients: classification/tagging (`gemini.go`), summarization, transcription (`gemini_transcribe.go`, `whisper.go` fallback), embeddings (`embeddings.go`), plus an `anthropic.go` variant.
- `internal/notion/` — `client.go` (page/database REST calls), `oauth.go` (authorization-code flow, `golang.org/x/oauth2`), `search.go`.
- `internal/crypto/` — AES-256-GCM encrypt/decrypt for Notion OAuth tokens at rest (`NOTION_ENCRYPTION_KEY`). The legacy plaintext `/notion <token> <db_id>` flow bypasses this entirely (stored as plaintext `notion_token`).
- `internal/config/config.go` — `os.Getenv` + defaults, validated in `Load()`. Only `TELEGRAM_BOT_TOKEN`, `SUPABASE_URL`, `SUPABASE_SECRET_KEY` are required to load config at all; `cmd/chatvault-api` additionally requires `SESSION_SECRET` (checked at that binary's startup, not in `Load()`).
- `internal/model/types.go` — plain structs, no ORM tags beyond `json`. `ActionItem` has both summary-blob fields (used inside `DailySummary.ActionItems` JSONB) and durable-row fields (`ID`, `Status`, `DueDate`, `AssigneeUserID`) — same struct serves both shapes via `omitempty`.
- `internal/supabase/storage.go` — separate concern: Supabase **Storage** (file/blob upload for voice notes), unrelated to the PostgREST DB client in `internal/storage`.

## Gotchas: Postgres types over PostgREST's JSON wire format

PostgREST round-trips most Postgres types through plain JSON, but two types need hand-encoding because their Postgres text representation isn't what `encoding/json` produces by default:

1. **`bytea`** (e.g. `notion_configs.notion_token_encrypted`) — PostgREST renders/accepts it as `"\x<hex>"`, not base64 (which is what Go's `json` package does for a raw `[]byte` field). Use `encodeBytea`/`decodeBytea` in `internal/storage/repository.go` — never unmarshal a bytea column straight into `[]byte`.
2. **`vector`** (pgvector, e.g. `message_embeddings.embedding`, and the `p_query_embedding` RPC argument) — must be passed as the bracketed literal string `"[v1,v2,...]"`, not a JSON array of numbers. Use `formatVector(values []float32) string` in `internal/storage/repository.go`.

If you add a new column of either type (or another type with a non-JSON-native Postgres text format), follow the same pattern: encode/decode explicitly at the `Repository` boundary, never rely on default marshaling.

## Database migrations

Plain numbered SQL files in `migrations/`, run in order, no migration tool (no golang-migrate/goose) and no rollback tooling — forward-only `00N_*.sql`. Current set:
1. `001_init.sql` — chats, messages, daily_summaries, notion_configs.
2. `002_action_items.sql` — durable `action_items` table.
3. `003_fts.sql` — generated `tsvector` column + GIN index on `messages`.
4. `004_embeddings.sql` — `vector` extension + `message_embeddings` table.
5. `005_dashboard_auth.sql` — `dashboard_users`, `chat_members`, `dashboard_sessions`.
6. `006_notion_oauth.sql` — adds `notion_token_encrypted`, `oauth_workspace_id`, `oauth_workspace_name` to `notion_configs` (plaintext `notion_token` column kept for chats still on the legacy flow).
7. `007_supabase_rpc.sql` — `search_messages` and `semantic_search_messages` RPC functions, and the `messages_missing_embeddings` view. This is what makes ranked full-text search and vector-distance semantic search possible over PostgREST (see above). Run against the Supabase SQL Editor or `psql` — there's no automated migration runner.

## Environment variables

Required (both Go binaries): `TELEGRAM_BOT_TOKEN`, `GEMINI_API_KEY`, `SUPABASE_URL`, `SUPABASE_SECRET_KEY`.

Required only for `cmd/chatvault-api`: `SESSION_SECRET`, `API_PORT` (default `:8081`), `ALLOWED_ORIGINS`.

Required only for Notion OAuth onboarding: `NOTION_ENCRYPTION_KEY`, `NOTION_OAUTH_CLIENT_ID`, `NOTION_OAUTH_CLIENT_SECRET`, `NOTION_OAUTH_REDIRECT_URL`.

Everything else (model names, timeouts, scheduler hour/minute, storage bucket, `DASHBOARD_BASE_URL`) is optional with sane defaults — see `README.md` for the full list and `internal/config/config.go` for the actual defaults. There is deliberately **no `DATABASE_URL`** — don't add one back.

Frontend (`landing/`) build-time vars (`VITE_*`) are documented in `README.md` and unrelated to the Go config.

## Testing conventions

- Standard `go test ./...`, no testify/mocking framework. Fakes are hand-written structs.
- The dominant pattern: `fakeActionItemRepo` in `internal/service/action_items_test.go` implements the full `summaryRepository` interface (defined in `internal/service/services.go`) and is reused across multiple `_test.go` files in the `service` package (`embeddings_test.go`, `search_test.go`, `action_items_test.go`) to assert call behavior without hitting the network. When you add a new repository method to `summaryRepository`, add a corresponding no-op (or recording) method to `fakeActionItemRepo` or every existing test in that package will fail to compile.
- Run `go build ./...`, `go vet ./...`, and `go test ./...` after any change — there's no CI config to lean on for this in this worktree; verify locally.

## Git / process conventions for this project

- Never `git add -A`. Stage files explicitly by name.
- Investigate unfamiliar untracked/staged state before deleting or overwriting it — it may be another in-progress session's work.
- Never skip hooks (`--no-verify`) or force-push, and never push to `origin` without explicit user instruction — leave commits local and say so.
- Prefer `git merge --ff-only` over `git reset --hard` to bring a stale branch up to date.
- Commit each logically-independent change as its own commit rather than one giant commit.
