-- name: CreateChunkBatch :copyfrom
INSERT INTO chunks (
    file_id, ordinal, start_line, end_line, content, content_hash, token_count,
    chunk_type, chunk_name, parent_name, signature, doc_comment, imports, calls,
    lines_of_code, comment_ratio, cyclomatic_complexity, embedding_context,
    source_snapshot_id, git_commit_hash, author, updated_at, indexed_at,
    file_version, is_latest, chunk_key
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26);

-- name: CreateEmbeddingBatch :batchexec
INSERT INTO embeddings (chunk_id, vector, model)
VALUES ($1, $2, $3);
