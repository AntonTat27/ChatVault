CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS message_embeddings (
    message_id BIGINT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    embedding VECTOR(768) NOT NULL,
    model_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_message_embeddings_chat ON message_embeddings(chat_id);
