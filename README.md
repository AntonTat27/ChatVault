# ChatVault

Telegram group knowledge-base bot with AI-first features:
- auto-store group messages
- async AI tagging (`idea`, `decision`, `action-item`, `question`, `document`, `noise`) + optional topic
- voice transcription pipeline (Telegram `.ogg` -> Supabase Storage -> Whisper -> transcript + tagging)
- daily summaries with Anthropic and Telegram posting
- commands: `/summary`, `/ideas`, `/decisions`, `/actions`, `/notion`, `/export`

## Environment variables

Required:
- `TELEGRAM_BOT_TOKEN`
- `DATABASE_URL`
- `ANTHROPIC_API_KEY`
- `OPENAI_API_KEY`
- `SUPABASE_URL`
- `SUPABASE_SERVICE_ROLE_KEY`

Optional:
- `ANTHROPIC_MODEL` (default `claude-3-5-haiku-latest`)
- `OPENAI_WHISPER_MODEL` (default `whisper-1`)
- `SUPABASE_STORAGE_BUCKET` (default `chatvault`)
- `DAILY_SUMMARY_HOUR_UTC` (default `18`)
- `DAILY_SUMMARY_MINUTE_UTC` (default `0`)
- `HTTP_TIMEOUT_SECONDS` (default `30`)
- `NOTION_VERSION` (default `2022-06-28`)

## Database migration (Supabase Postgres)

Run SQL from:
- `migrations/001_init.sql`

Use Supabase SQL Editor or psql against `DATABASE_URL`.

## Run locally

```bash
cd chatvault
go mod tidy
go test ./...
go run ./cmd/chatvault
```

## Docker

Build and run:

```bash
docker build -t chatvault .
docker run --rm --env-file .env chatvault
```

## Telegram commands

- `/summary` -> async summary for today
- `/ideas` -> idea messages from last 7 days
- `/decisions` -> decision messages from last 7 days
- `/actions` -> action-item messages from last 7 days
- `/notion <token> <database_id>` -> configure Notion per chat
- `/export` -> export today's summary to Notion

## Notion export behavior

When configured, daily summary export creates a page:
- Title: `[Date] — Daily Summary`
- Properties: Date, Chat Name, Message Count
- Body sections: Summary, Decisions, Action Items (checkbox items), Ideas, Open Questions

## Deployment notes

### Railway
1. Create a new Railway service from this repository.
2. Add all required environment variables.
3. Set start command to `/chatvault` when using Docker deployment.
4. Ensure outbound network access to Telegram, Anthropic, OpenAI, Supabase, Notion.

### Fly.io
1. `fly launch --no-deploy` in repository root.
2. Configure secrets with `fly secrets set ...` for all required env vars.
3. Deploy via `fly deploy`.
4. Keep one instance always-on for scheduler + webhook/long polling processing.
