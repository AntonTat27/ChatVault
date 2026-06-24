CREATE TABLE IF NOT EXISTS dashboard_users (
    telegram_user_id BIGINT PRIMARY KEY,
    first_name TEXT NOT NULL DEFAULT '',
    last_name TEXT NOT NULL DEFAULT '',
    username TEXT,
    photo_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chat_members (
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    telegram_user_id BIGINT NOT NULL REFERENCES dashboard_users(telegram_user_id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (chat_id, telegram_user_id)
);

-- token_hash is sha256(raw session token); the raw token never touches the
-- database, only the cookie. Deleting a row revokes that session immediately.
CREATE TABLE IF NOT EXISTS dashboard_sessions (
    token_hash TEXT PRIMARY KEY,
    telegram_user_id BIGINT NOT NULL REFERENCES dashboard_users(telegram_user_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_dashboard_sessions_user ON dashboard_sessions(telegram_user_id);
CREATE INDEX IF NOT EXISTS idx_dashboard_sessions_expires_at ON dashboard_sessions(expires_at);
