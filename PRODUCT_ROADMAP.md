# ChatVault: Path to a Sellable Product — Implementation Plan

> Handoff document for whichever agent/session picks up this work next. Self-contained: read this file and the files it references; you should not need prior conversation context to continue.

## Context

ChatVault is currently a single-purpose Telegram bot: it auto-stores group messages, AI-tags them (idea/decision/action-item/question/document/noise via Gemini), transcribes voice notes, posts daily AI summaries, and does a one-way Notion export using a plaintext token pasted into chat (`/notion <token> <db_id>`). There's no web presence beyond a static marketing page (`landing/`), no billing, no real search, no action-item lifecycle, and no HTTP server anywhere in the codebase.

To become a sellable product it needs the things that turn "a bot" into "a product people pay for": durable action-item tracking, search over chat history, a dashboard usable without typing commands, a secure onboarding flow for integrations, and billing.

This plan sequences five phases, each independently shippable, in priority order: **action-item lifecycle → search → web dashboard → Notion OAuth → Stripe billing**. A Phase 0 (pgx plumbing) precedes all of them as shared infrastructure.

### Architecture decisions that apply across every phase
- **DB access stays split**: `internal/storage.Repository` (Supabase PostgREST REST client) is untouched for all existing code paths. A new `internal/db` package wraps a direct `pgxpool.Pool` connection, used only by new code that needs transactions, joins, or pgvector (search, dashboard reads, billing webhooks). Both clients operate on the same physical tables — no data duplication.
- **Dashboard lives inside `landing/`**: the existing Vite+React+Tailwind app gets new authenticated routes/pages rather than a separate package, backed by a new Go HTTP API (`internal/api/`).

### Confirmed current architecture (as of this plan's writing)
- `internal/storage/repository.go` — hand-rolled Supabase PostgREST REST client (not pgx/database/sql). Tables: `chats`, `messages`, `daily_summaries`, `notion_configs`. CRUD via raw `map[string]any` + PostgREST filter syntax (`eq.`, `gte.`). `doRequest` builds REST calls with apikey/Bearer headers.
- `internal/service/services.go` — `Services` struct holds `repo`, `gemini`, `transcriber`, `storageClient`, `notionClient`, plus an internal job queue (`chan func(context.Context)`, buffer 256) with **one** worker goroutine (`runWorker`). Telegram posting injected via `PostMessageFn` callback. **No HTTP server exists anywhere in the codebase.**
- `internal/bot/handler.go` — commands registered via `b.RegisterHandler(bot.HandlerTypeMessageText, "/cmd", bot.MatchTypeExact, h.handlerFunc)`; one regex handler for `/notion <token> <db_id>` (`notionCommandRegexp`). Adding a command = add const + handler method + `RegisterHandler` call.
- `internal/config/config.go` — simple `os.Getenv` + defaults, validated in `Load()`. Required: `TELEGRAM_BOT_TOKEN`, `SUPABASE_URL`, `SUPABASE_SECRET_KEY`.
- `internal/model/types.go` — plain structs. `ActionItem{Task, Owner *string}` has **no status/due-date/assignee fields**. `AllowedAITags` map is the single source of truth for the 6 tags. `NotionConfig{ChatID, Token, DatabaseID, UpdatedAt, Configured, ChatName, MessageCount}`.
- `internal/notion/client.go` — token used directly as a Bearer token per-request, no OAuth, no encryption at rest (`notion_configs.notion_token` is plaintext `TEXT NOT NULL`).
- `cmd/chatvault/main.go` — linear wiring (config → `Repository`, `GeminiClient`, `GeminiTranscribeClient`, `StorageClient`, `notion.Client` → `service.NewServices` → `bot.NewHandler` → `telegrambot.New` → `RegisterHandlers` → `go RunDailySummaryScheduler` → `telegramBot.Start(ctx)` long-polling). Graceful shutdown via `signal.NotifyContext`.
- `internal/supabase/storage.go` — pure file storage (voice upload to Supabase Storage REST), unrelated to the PostgREST DB client.
- `migrations/001_init.sql` — single raw SQL file, `CREATE TABLE IF NOT EXISTS`, no migration tool/version tracking (no golang-migrate/goose).
- `landing/` — Vite+React+Tailwind static SPA, zero backend integration today (no fetch/axios calls), just marketing content + a Telegram deep link. Deps are minimal: `react`, `react-dom`, Vite, Tailwind 4, oxlint — **no router, no data-fetching library**.
- `go.mod` — direct deps: `go-telegram/bot`, `joho/godotenv`. No HTTP framework (gin/echo/chi/fiber), no JWT lib, no Stripe SDK, no app-level OAuth2 usage (`golang.org/x/oauth2` only indirect via the Google SDK) — **as of writing this section, before Phase 0 work began**.

---

## ⚠️ Current implementation state (read this before doing anything else)

Work has **already started** on Phase 0 in the worktree at:
```
C:\Users\anton\GolandProjects\ChatVault\.claude\worktrees\chatvault-product-phases
```
(branch `worktree-chatvault-product-phases`). It is **uncommitted**. Current `git status --porcelain` in that worktree:
```
 M cmd/chatvault/main.go
 M go.mod
 M go.sum
 M internal/config/config.go
?? internal/db/
```

What's done:
- Added `github.com/jackc/pgx/v5` (+ `pgxpool`) to `go.mod` via `go get` + `go mod tidy`.
- **Important gotcha already discovered**: `go mod tidy` repeatedly bumps the `go` directive in `go.mod` from `1.22` to `1.25.0` and **will keep doing so** — pgx v5.10.0 (or a transitive dependency) genuinely requires Go ≥1.25. Manually editing it back to `1.22` does not stick; the next `go mod tidy`/`go build` re-bumps it. **Decision: accept `go 1.25.0` as the new minimum** rather than fighting the toolchain. This has a real consequence: check `Dockerfile` and any CI/deploy config (Heroku buildpack, `.github/workflows/deploy-heroku.yml`) for a pinned Go version and bump it to match, or the deploy will fail on a stale toolchain. This had **not yet been checked or fixed** when work paused.
- Created `internal/db/pool.go` with `NewPool(ctx, dsn) (*pgxpool.Pool, error)` — parses the DSN, creates the pool, pings it, returns a wrapped error on failure. This part is done and builds cleanly.
- Added `DatabaseURL string` field to `config.Config` (sourced from `DATABASE_URL` env var, no validation — optional by design so bot-only deploys don't break) in `internal/config/config.go`. Done.
- Started editing `cmd/chatvault/main.go`: added the `chatvault/internal/db` import. **Not yet done**: the actual `db.NewPool(ctx, cfg.DatabaseURL)` call has not been added to `main()`, and the pool isn't being closed on shutdown. The import was added but is currently unused — **this file will not compile until either the pool construction call is added or the import is removed**. Fix this first.
- `go build ./...` succeeded as of the last full build (before the `main.go` import was added). Re-verify with `go build ./...` after finishing the `main.go` edit.

### Immediate next steps to finish Phase 0
1. In `cmd/chatvault/main.go`, after `repo := storage.NewRepository(...)`, add something like:
   ```go
   var dbPool *pgxpool.Pool
   if cfg.DatabaseURL != "" {
       dbPool, err = db.NewPool(ctx, cfg.DatabaseURL)
       if err != nil {
           log.Fatalf("database pool init failed: %v", err)
       }
       defer dbPool.Close()
   }
   ```
   (Needs `"github.com/jackc/pgx/v5/pgxpool"` imported for the type, or expose a type alias from `internal/db` to avoid leaking the pgx import into `main.go` — either is fine, pick one and be consistent.) The pool is constructed but **not yet wired into `Services`** — that wiring happens in Phase 1/2 as each feature needs it, per the phase plan below. Phase 0's only goal is "pgx connects and the bot still boots."
2. Run `go build ./... && go test ./...` to confirm everything still compiles and passes.
3. Check `Dockerfile` and `.github/workflows/deploy-heroku.yml` for a pinned Go version; bump to ≥1.25 if pinned lower.
4. Document the new `DATABASE_URL` env var in `README.md` (it currently lists `TELEGRAM_BOT_TOKEN`, `GEMINI_API_KEY`, `SUPABASE_URL`, `SUPABASE_SECRET_KEY` as required and a list of optional vars — add `DATABASE_URL` to the optional list with a one-line note that it's the Supabase **transaction pooler** DSN, not the REST URL).
5. Commit this phase on its own before starting Phase 1 (small, isolated, easy to review).

---

## Phase 1 — Action-item lifecycle management

**Goal:** Action items become trackable rows with status/owner/due-date, not just JSONB text inside a daily summary blob.

**Schema (`migrations/002_action_items.sql`):**
```sql
CREATE TABLE IF NOT EXISTS action_items (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    source_message_id BIGINT REFERENCES messages(id) ON DELETE SET NULL,
    summary_id BIGINT REFERENCES daily_summaries(id) ON DELETE SET NULL,
    task TEXT NOT NULL,
    owner TEXT,
    assignee_user_id BIGINT,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'done', 'cancelled')),
    due_date DATE,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_action_items_chat_status ON action_items(chat_id, status);
CREATE INDEX IF NOT EXISTS idx_action_items_chat_due_date ON action_items(chat_id, due_date) WHERE due_date IS NOT NULL;
```
`daily_summaries.action_items` JSONB stays for historical display; new summaries also insert rows here.

**Go changes:**
- `internal/model/types.go`: extend `ActionItem` with `ID *int64`, `Status string`, `DueDate *string`, `AssigneeUserID *int64` (new fields `omitempty` so old JSONB still unmarshals).
- `internal/storage/repository.go`: add `CreateActionItem`, `ListActionItems(chatID, status)`, `UpdateActionItemStatus(id, status)` — plain PATCH/POST via the existing PostgREST pattern, no pgx needed for this CRUD shape.
- `internal/service/services.go`: after summary generation, insert one `action_items` row per extracted item; add `MarkActionItemDone`, `ListOpenActionItems`.
- `internal/bot/handler.go`: add `/done <id>` (regex command like the existing `/notion` pattern); change `/actions` to render structured open items (status/due date) instead of raw tagged messages — this is a **deliberate, visible behavior change**, call it out in the commit/PR description. Optional stretch: inline "Mark Done" buttons via `bot.HandlerTypeCallbackQueryData`.

**Known gap, deliberately deferred:** resolving free-text "owner" strings to Telegram user IDs has no directory to match against yet — leave `assignee_user_id` null for v1; the dashboard (Phase 3) can let users self-claim items later.

**Verification:** `go test ./...`; manually trigger a summary, confirm `action_items` rows are created, `/done <id>` flips status, `/actions` reflects it.

---

## Phase 2 — Search

### 2a. Full-text search (ship first)
**Schema (`migrations/003_fts.sql`):**
```sql
ALTER TABLE messages ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('english', coalesce(message_text, '') || ' ' || coalesce(transcript, ''))) STORED;
CREATE INDEX IF NOT EXISTS idx_messages_search_vector ON messages USING GIN (search_vector);
```
- New `internal/db/search.go`: `SearchMessages(ctx, chatID, query, limit)` using `plainto_tsquery` + `ts_rank` — first real use of the Phase 0 pgx pool.
- `Services.SearchMessages` delegates to it (constructor signature change for `NewServices`).
- `/search <query>` command in `internal/bot/handler.go`.

### 2b. Semantic search (ship after 2a is stable)
**Schema (`migrations/004_embeddings.sql`):**
```sql
CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE IF NOT EXISTS message_embeddings (
    message_id BIGINT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    embedding VECTOR(768) NOT NULL,  -- confirm exact Gemini embedding model dimension first
    model_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_message_embeddings_chat ON message_embeddings(chat_id);
```
- `internal/ai/embeddings.go`: `GenerateEmbedding(ctx, text) ([]float32, error)` via Gemini's embedding endpoint.
- Enqueue an embedding job after classification succeeds (existing `Services.jobs` channel) — **but bump `runWorker` from one goroutine to a small pool (3–5)** here, since embedding calls add latency that would otherwise head-of-line-block transcription/classification.
- `internal/db/search.go`: `SemanticSearchMessages` via `github.com/pgvector/pgvector-go` (new dep), `<->` distance ordering.
- One-off backfill: new `cmd/backfill-embeddings/main.go` for existing messages. Consider skipping embeddings for `noise`-tagged messages to control Gemini cost.

**Why split 2a/2b:** 2a has no new infra risk (just SQL + the pgx pool already proven in Phase 0). 2b adds a new Postgres extension, new external API cost, a backfill job, and a worker-pool refactor — let 2a ship value while de-risking 2b separately.

**Verification:** seed messages, run `/search`, confirm ranked results; for 2b, run the backfill binary against a test chat and confirm `<->` ordering returns semantically relevant matches.

---

## Phase 3 — Web dashboard

**Identity problem and resolution:** Telegram is the only identity source; the dashboard must prove "this logged-in user is a member of this specific chat."
1. **Telegram Login Widget** on the login page → signed payload to a callback.
2. Backend verifies the HMAC-SHA256 `hash` per Telegram's documented algorithm (proves identity, not membership).
3. Membership proof: call `getChatMember(chat_id, user_id)` against the Bot API, cache result in a `chat_members` table, refresh on demand.
4. Issue a signed session cookie/JWT after both checks pass — no separate password system.

**Schema (`migrations/005_dashboard_auth.sql`):** `dashboard_users` (telegram_user_id PK + profile fields), `chat_members` (chat_id, telegram_user_id, role, verified_at), `dashboard_sessions` (id, telegram_user_id, expires_at) for revocation capability ahead of Phase 5 billing.

**Go changes:**
- `internal/auth/`: `VerifyTelegramLoginHash`, session issuance/validation, `RequireAuth` and `RequireChatMembership(chatID)` middleware.
- `internal/api/` (new package): `server.go` (mirrors `main.go`'s `signal.NotifyContext` shutdown pattern), `router.go` (`POST /auth/telegram/callback`, `GET /api/chats`, `GET /api/chats/{id}/summaries`, `GET /api/chats/{id}/action-items`, `PATCH /api/action-items/{id}`, `GET /api/chats/{id}/search`), `middleware.go` (auth, CORS, logging).
- New `cmd/chatvault-api/main.go` binary (decoupled from the bot process for independent scaling/deploy) wiring config → `internal/db.Pool` → `internal/api.Server`.
- `internal/config/config.go`: add `SessionSecret`, `APIPort`, `AllowedOrigins`, `DashboardBaseURL`.

**Frontend (`landing/`):**
- Add `react-router` for `/login`, `/dashboard`, `/dashboard/chats/:chatId`, `/dashboard/chats/:chatId/search`.
- Skip `react-query` initially — a thin `src/lib/api.js` fetch wrapper is enough at this scale; add it later only if the API surface grows past ~5 resources.
- New files: `pages/Login.jsx`, `pages/Dashboard.jsx`, `pages/ChatDetail.jsx`, `pages/Search.jsx`, `components/TelegramLoginButton.jsx`, `lib/api.js`, `lib/auth.js`.
- `vite.config.js`: dev proxy `/api` → local Go API server port.
- Keep API and static site as two origins in production (explicit CORS in `internal/api/middleware.go`) rather than having Go serve the built SPA — less coupling, matches "extend the existing app" without forcing a deploy-pipeline change.

**Operational prerequisite:** bot domain must be registered via `@BotFather /setdomain` for the Login Widget to work — document this as a deploy step.

**Verification:** log in via the Telegram Login Widget against a test bot/domain, confirm a non-member is rejected by `RequireChatMembership` and a member sees their chat's data; `npm run dev` in `landing/` with the API proxy working end-to-end.

---

## Phase 4 — Notion OAuth onboarding

**Hard dependency, not just an ordering preference:** the OAuth callback needs to know *which chat* the user is connecting on behalf of, which only exists once an authenticated dashboard session (Phase 3) carries that context. **Do not attempt this before Phase 3 ships.**

**Schema (`migrations/006_notion_oauth.sql`):** add `notion_token_encrypted BYTEA`, `oauth_workspace_id`, `oauth_workspace_name` to `notion_configs`. Keep the existing plaintext `notion_token` column for chats still on the old flow; don't auto-migrate (OAuth needs interactive consent) — surface a "reconnect Notion" banner in the dashboard instead.

**Go changes:**
- `internal/notion/oauth.go`: `BuildAuthorizationURL(state)`, `ExchangeCodeForToken` (standard OAuth2 authorization-code flow, promote `golang.org/x/oauth2` from indirect to direct dependency).
- New `internal/crypto/` package: AES-GCM encrypt/decrypt using a `NOTION_ENCRYPTION_KEY` env var — first at-rest secret encryption in this codebase; the same primitive gets reused for Stripe secrets in Phase 5.
- `internal/notion/client.go`: take a decrypted token at call time, never log/persist it decrypted.
- `internal/api/`: `GET /auth/notion/start?chat_id=`, `GET /auth/notion/callback` (exchange code, save encrypted config, redirect to dashboard).
- `internal/bot/handler.go`: `/notion` now replies with a deep link to `/dashboard/chats/{id}/integrations` instead of accepting a pasted token; deprecate `notionCommandRegexp`'s plaintext path with a transition message.
- **UX note:** Notion OAuth grants workspace-level access with a page/database picker, not a direct database ID — the dashboard needs a "select database" step after the OAuth redirect completes, which is new UI, not just a backend swap.

**Verification:** full OAuth round-trip against a real Notion workspace in a sandbox, confirm `/export` still posts pages using the decrypted token.

---

## Phase 5 — Stripe billing / multi-tenancy

**Schema (`migrations/007_billing.sql`):** `subscriptions` (chat_id unique, stripe_customer_id, stripe_subscription_id, plan, status, current_period_end), `usage_counters` (chat_id, period_month, message/summary/search counts), `billing_events` (stripe_event_id UNIQUE, event_type, payload, processed_at) — the unique constraint on `stripe_event_id` makes webhook processing idempotent against Stripe's retries.

**Go changes:**
- Add `github.com/stripe/stripe-go/v81`.
- `internal/billing/`: `CreateCheckoutSession`, `HandleWebhook` (verify signature, process `checkout.session.completed` / `customer.subscription.updated|deleted` inside a pgx transaction updating `subscriptions` + `billing_events` atomically), `GetEntitlements`, `CheckEntitlement(ctx, chatID, feature)` reused by both the bot and the API to gate features (e.g., semantic search, Notion export) behind plan tier.
- `internal/api/`: `POST /api/billing/checkout`, `POST /api/billing/webhook` (raw body + signature check, must sit outside `RequireAuth` and outside any JSON-parsing middleware), `GET /api/billing/portal`.
- Usage increments: atomic `ON CONFLICT ... DO UPDATE SET count = count + 1` via pgx (avoids the read-then-write race the REST client would have) from the existing message/summary/search code paths.
- v1 ships as Stripe Checkout redirect only — no Stripe Elements/`@stripe/stripe-js` needed, no PCI scope.

**Open product question to confirm before building:** billing is modeled per-chat (one Telegram group = one Stripe customer) for consistency with the rest of the chat-scoped schema. If the intended model is per-admin-user covering multiple chats instead, the `subscriptions` table's foreign key needs to change before writing code — **confirm this with the project owner first**, it's expensive to restructure after the fact.

**Verification:** Stripe test-mode checkout + CLI-triggered webhook events (`stripe trigger checkout.session.completed`), confirm idempotent processing by sending the same event twice.

---

## Sequencing notes (explicit, not silent reordering)
- Phase 3 → Phase 4 dependency is **structural** (OAuth callback needs an authenticated chat-scoped session), not just priority — do not swap.
- Phase 2a (full-text) before 2b (semantic) is a recommended split: 2b carries materially higher infra risk.
- Phase 0 should be finished and committed as its own small unit before Phase 1 begins — don't let it get silently absorbed into Phase 2 where the first hard pgx requirement (full-text search) appears.

## General working notes for whoever continues this
- Work happens in the worktree `C:\Users\anton\GolandProjects\ChatVault\.claude\worktrees\chatvault-product-phases` (branch `worktree-chatvault-product-phases`) unless told otherwise — don't edit the main checkout directly.
- Commit each phase (or sub-phase, e.g. 2a/2b) separately rather than one giant commit — they're independently shippable and reviewable.
- No migration rollback tooling exists — schema changes are forward-only `00N_*.sql` files; don't introduce golang-migrate/goose unless asked, just follow the existing convention.
- Re-run `go build ./...` and `go test ./...` after every phase before moving to the next.
