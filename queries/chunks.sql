-- name: CreateChunk :one
INSERT INTO chunks (file_id, ordinal, start_line, end_line, content, content_hash, token_count)
VALUES ($1, $2, $3, $4, $5, $6, $7)
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
