-- Exposes the two queries PostgREST's table/view query language cannot
-- express on its own (ranking by a computed expression, ordering by vector
-- distance) as RPC functions, and the missing-embeddings lookup as a plain
-- view, so internal/db's direct pgx connection can be retired in favor of
-- accessing everything through Supabase's REST API (PostgREST).

CREATE OR REPLACE FUNCTION search_messages(p_chat_id BIGINT, p_query TEXT, p_limit INT)
RETURNS SETOF messages
LANGUAGE sql STABLE
AS $$
    SELECT *
    FROM messages
    WHERE chat_id = p_chat_id AND search_vector @@ plainto_tsquery('english', p_query)
    ORDER BY ts_rank(search_vector, plainto_tsquery('english', p_query)) DESC, created_at DESC
    LIMIT p_limit;
$$;

CREATE OR REPLACE FUNCTION semantic_search_messages(p_chat_id BIGINT, p_query_embedding TEXT, p_limit INT)
RETURNS SETOF messages
LANGUAGE sql STABLE
AS $$
    SELECT m.*
    FROM message_embeddings e
    JOIN messages m ON m.id = e.message_id
    WHERE e.chat_id = p_chat_id
    ORDER BY e.embedding <-> p_query_embedding::vector
    LIMIT p_limit;
$$;

CREATE OR REPLACE VIEW messages_missing_embeddings AS
SELECT m.*
FROM messages m
LEFT JOIN message_embeddings e ON e.message_id = m.id
WHERE e.message_id IS NULL AND (m.ai_tag IS NULL OR m.ai_tag != 'noise');
