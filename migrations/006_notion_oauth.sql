-- Adds OAuth-based Notion connection support alongside the existing
-- plaintext-token path (/notion <token> <database_id>). Existing rows are
-- left untouched; notion_token/notion_database_id are relaxed to nullable so
-- an OAuth-initiated row can exist before a database has been picked.
ALTER TABLE notion_configs
    ALTER COLUMN notion_token DROP NOT NULL,
    ALTER COLUMN notion_database_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS notion_token_encrypted BYTEA,
    ADD COLUMN IF NOT EXISTS oauth_workspace_id TEXT,
    ADD COLUMN IF NOT EXISTS oauth_workspace_name TEXT;
