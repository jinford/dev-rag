-- name: CreateChunk :one
INSERT INTO chunks (
    file_id, ordinal, start_line, end_line, content, content_hash, token_count,
    chunk_type, chunk_name, parent_name, signature, doc_comment, imports, calls,
    lines_of_code, comment_ratio, cyclomatic_complexity, embedding_context,
    source_snapshot_id, git_commit_hash, author, updated_at, indexed_at,
    file_version, is_latest, chunk_key
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26)
RETURNING *;

-- name: GetChunk :one
SELECT * FROM chunks
WHERE id = $1;

-- name: ListChunksByFile :many
SELECT * FROM chunks
WHERE file_id = $1
ORDER BY ordinal;

-- name: ListChunksByOrdinalRange :many
SELECT * FROM chunks
WHERE file_id = $1 AND ordinal BETWEEN $2 AND $3
ORDER BY ordinal;

-- name: FindChunksByContentHash :many
SELECT * FROM chunks
WHERE content_hash = $1
ORDER BY created_at DESC;

-- name: DeleteChunk :exec
DELETE FROM chunks
WHERE id = $1;

-- name: DeleteChunksByFile :exec
DELETE FROM chunks
WHERE file_id = $1;
