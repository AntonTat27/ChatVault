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
