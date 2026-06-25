# ChatVault

Telegram group knowledge-base bot with AI-first features:
- auto-store group messages
- async AI tagging (`idea`, `decision`, `action-item`, `question`, `document`, `noise`) + optional topic
- voice transcription pipeline (Telegram `.ogg` -> Supabase Storage -> Gemini -> transcript + tagging)
- daily summaries with Gemini and Telegram posting
- commands: `/summary`, `/ideas`, `/decisions`, `/actions`, `/notion`, `/export`, `/dashboard`
- web dashboard (`cmd/chatvault-api`): Telegram-login-gated chat view with summaries, action items, search, and Notion OAuth onboarding

## Environment variables

Required (both binaries):
- `TELEGRAM_BOT_TOKEN`
- `GEMINI_API_KEY`
- `SUPABASE_URL`
- `SUPABASE_SECRET_KEY`

Optional:
- `GEMINI_MODEL` (default `gemini-3.5-flash`)
- `GEMINI_SUMMARY_MODEL` (default `gemini-2.0-flash`)
- `GEMINI_TRANSCRIBE_MODEL` (default `gemini-3.5-flash`)
- `GEMINI_EMBEDDING_MODEL` (default `text-embedding-004`) — used for `/semantic-search` and the embedding backfill
- `SUPABASE_STORAGE_BUCKET` (default `chatvault`)
- `DAILY_SUMMARY_HOUR_UTC` (default `18`)
- `DAILY_SUMMARY_MINUTE_UTC` (default `0`)
- `HTTP_TIMEOUT_SECONDS` (default `30`)
- `NOTION_VERSION` (default `2022-06-28`)
- `DASHBOARD_BASE_URL` — public URL of the deployed dashboard frontend (e.g. `https://app.example.com`); used by `/dashboard` and the new `/notion` deep link, and as the OAuth post-connect redirect target

Required to run `cmd/chatvault-api` (the dashboard):
- `SESSION_SECRET` — random secret used to sign Notion OAuth `state` values (`openssl rand -hex 32`); the binary refuses to start without it
- `API_PORT` (default `:8081`)
- `ALLOWED_ORIGINS` — comma-separated list of origins allowed to call the API with credentials (the dashboard's own origin)

Required only for Notion OAuth onboarding (Phase 4 of the dashboard):
- `NOTION_ENCRYPTION_KEY` — 32-byte AES-256 key, hex-encoded (`openssl rand -hex 32`); encrypts OAuth access tokens at rest
- `NOTION_OAUTH_CLIENT_ID`, `NOTION_OAUTH_CLIENT_SECRET` — from a Notion public integration
- `NOTION_OAUTH_REDIRECT_URL` — must exactly match the redirect URI registered with Notion, e.g. `https://api.example.com/auth/notion/callback`

The legacy `/notion <token> <database_id>` plaintext flow keeps working without any of the above and doesn't require `NOTION_ENCRYPTION_KEY`.

### Frontend (`landing/`) build-time variables

These are read by Vite at build time (`import.meta.env.VITE_*`), not by either Go binary:
- `VITE_TELEGRAM_BOT_USERNAME` (default `ChatVault1Bot`) — bot username (no `@`), passed to the Telegram Login Widget
- `VITE_API_BASE_URL` (default `/api`), `VITE_AUTH_BASE_URL` (default `/auth`) — only need to be set to an absolute URL (`https://api.example.com/api`) when the dashboard frontend and `cmd/chatvault-api` are deployed on **different origins**; the relative defaults assume a reverse proxy puts both under one origin, matching `vite.config.js`'s dev proxy

Recommended for daily Telegram summaries on a free/test quota:
- `GEMINI_SUMMARY_MODEL=gemini-2.0-flash`
- keep `temperature` at `0.0`; the app now scales `max_output_tokens` from the number of messages, with a small floor and a hard cap

## Database migration (Supabase Postgres)

Run SQL from, in order:
- `migrations/001_init.sql`
- `migrations/002_action_items.sql`
- `migrations/003_fts.sql`
- `migrations/004_embeddings.sql` (requires the `vector` extension; only needed for `/semantic-search`)
- `migrations/005_dashboard_auth.sql` (only needed to run `cmd/chatvault-api`)
- `migrations/006_notion_oauth.sql` (only needed for Notion OAuth onboarding)
- `migrations/007_supabase_rpc.sql` (only needed for `/search` and `/semantic-search`; adds the `search_messages`/`semantic_search_messages` RPC functions and the `messages_missing_embeddings` view PostgREST needs, since ranking and vector-distance ordering can't be expressed as a plain REST filter)

Use Supabase SQL Editor or psql against the Supabase project database. There is no direct Postgres connection anywhere in this codebase — every feature (including search and the dashboard's session/membership caching) talks to the database exclusively through Supabase's PostgREST REST API via `internal/storage.Repository`.

## Run locally

```bash
cd chatvault
go mod tidy
go test ./...
go run ./cmd/chatvault
```

To run the dashboard locally:

```bash
go run ./cmd/chatvault-api   # serves the API on API_PORT (default :8081)
cd landing
npm install
npm run dev                  # proxies /api and /auth to localhost:8081, see vite.config.js
```

The Telegram Login Widget requires the bot's domain to be registered first via `@BotFather` -> `/setdomain`.

## Docker

Two Dockerfiles, one per binary:

```bash
# Telegram bot (cmd/chatvault)
docker build -t chatvault -f Dockerfile .
docker run --rm --env-file .env chatvault

# Dashboard API (cmd/chatvault-api)
docker build -t chatvault-api -f Dockerfile.api .
docker run --rm --env-file .env -p 8081:8081 chatvault-api
```

The dashboard API binds to `$PORT` if set (falls back to `$API_PORT`, then `:8081`) so it works as a Heroku web dyno, which assigns its own port at runtime.

## Telegram commands

- `/summary` -> async summary for today
- `/ideas` -> idea messages from last 7 days
- `/decisions` -> decision messages from last 7 days
- `/actions` -> open action items (status/owner/due date), tracked durably in the `action_items` table
- `/done <id>` -> mark an action item completed
- `/search <query>` -> full-text search over message history (requires `migrations/007_supabase_rpc.sql`)
- `/semantic-search <query>` -> meaning-based search via Gemini embeddings (requires `migrations/007_supabase_rpc.sql` and `GEMINI_EMBEDDING_MODEL`)
- `/notion` -> connect Notion via OAuth through the dashboard (requires `DASHBOARD_BASE_URL`)
- `/notion <token> <database_id>` -> legacy plaintext connect; still works, deprecated in favor of the OAuth flow above
- `/export` -> export today's summary to Notion
- `/dashboard` -> deep link to this chat's web dashboard (requires `DASHBOARD_BASE_URL`)

Run `go run ./cmd/backfill-embeddings` once after enabling semantic search to generate embeddings for messages stored before it was turned on (skips `noise`-tagged messages).

## Web dashboard

`cmd/chatvault-api` is a separate HTTP API binary (not the Telegram long-polling process) backing the `landing/` React app's dashboard routes (`/login`, `/dashboard`, `/dashboard/chats/:id`, `/dashboard/chats/:id/integrations`):
- Auth: Telegram Login Widget -> `POST /auth/telegram/callback` verifies the widget's signed payload, then issues an httpOnly session cookie backed by a `dashboard_sessions` row (hashed token, revocable by deleting the row).
- Per-chat access: `GET /api/chats/{id}/...` routes verify the caller is currently a member via Telegram's `getChatMember`, caching the result in `chat_members` for an hour before re-checking (pass `?refresh=true` to force it).
- Notion OAuth: `GET /auth/notion/start/{id}` redirects to Notion; `GET /auth/notion/callback` exchanges the code, encrypts the access token (AES-256-GCM, `NOTION_ENCRYPTION_KEY`), and stores it. The user then picks a database from `GET /api/chats/{id}/notion/databases` via `PATCH /api/chats/{id}/notion/database`, since Notion's OAuth grant is workspace-scoped, not database-scoped.

## Notion export behavior

When configured, daily summary export creates a page:
- Title: `[Date] — Daily Summary`
- Properties: Date, Chat Name, Message Count
- Body sections: Summary, Decisions, Action Items (checkbox items), Ideas, Open Questions

## Deployment notes

### Heroku

`.github/workflows/deploy-heroku.yml` deploys both binaries to a single Heroku app on push to `main`: `Dockerfile` (the bot) is released as the `worker` process type, `Dockerfile.api` (the dashboard API) as the `web` process type.

Required GitHub Actions secrets: `HEROKU_API_KEY`, `HEROKU_EMAIL`, `HEROKU_APP_NAME`.

Set as Heroku config vars on the app (`heroku config:set KEY=value --app=<name>`), in addition to the bot's required vars: `SESSION_SECRET`, `ALLOWED_ORIGINS` (the deployed dashboard frontend's origin), `DASHBOARD_BASE_URL`, and the Notion OAuth vars if onboarding is enabled. Don't set `API_PORT` — Heroku assigns `$PORT` at runtime and the API binds to it automatically.

### Railway
1. Create a new Railway service from this repository.
2. Add all required environment variables.
3. Set start command to `/chatvault` when using Docker deployment.
4. Ensure outbound network access to Telegram, Gemini, Supabase, Notion.

### Fly.io
1. `fly launch --no-deploy` in repository root.
2. Configure secrets with `fly secrets set ...` for all required env vars.
3. Deploy via `fly deploy`.
4. Keep one instance always-on for scheduler + webhook/long polling processing.
