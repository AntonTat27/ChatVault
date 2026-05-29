CREATE TABLE IF NOT EXISTS chats (
    chat_id BIGINT PRIMARY KEY,
    chat_title TEXT NOT NULL DEFAULT '',
    summary_hour_utc SMALLINT NOT NULL DEFAULT 18 CHECK (summary_hour_utc BETWEEN 0 AND 23),
    summary_minute_utc SMALLINT NOT NULL DEFAULT 0 CHECK (summary_minute_utc BETWEEN 0 AND 59),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    message_id INTEGER NOT NULL,
    sender_id BIGINT NOT NULL,
    chat_title TEXT NOT NULL DEFAULT '',
    message_text TEXT NOT NULL DEFAULT '',
    transcript TEXT,
    ai_tag TEXT CHECK (ai_tag IN ('idea', 'decision', 'action-item', 'question', 'document', 'noise')),
    topic_tag TEXT,
    is_voice BOOLEAN NOT NULL DEFAULT FALSE,
    is_action_completed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(chat_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_messages_chat_created_at ON messages(chat_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_chat_ai_tag_created_at ON messages(chat_id, ai_tag, created_at DESC);

CREATE TABLE IF NOT EXISTS daily_summaries (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    summary_date_utc DATE NOT NULL,
    summary_text TEXT NOT NULL,
    decisions JSONB NOT NULL DEFAULT '[]'::jsonb,
    action_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    ideas JSONB NOT NULL DEFAULT '[]'::jsonb,
    open_questions JSONB NOT NULL DEFAULT '[]'::jsonb,
    message_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(chat_id, summary_date_utc)
);

CREATE TABLE IF NOT EXISTS notion_configs (
    chat_id BIGINT PRIMARY KEY REFERENCES chats(chat_id) ON DELETE CASCADE,
    notion_token TEXT NOT NULL,
    notion_database_id TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
