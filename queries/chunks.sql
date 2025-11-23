-- name: CreateChunk :one
INSERT INTO chunks (
    file_id, ordinal, start_line, end_line, content, content_hash, token_count,
    chunk_type, chunk_name, parent_name, signature, doc_comment, imports, calls,
    lines_of_code, comment_ratio, cyclomatic_complexity, embedding_context,
    level, importance_score,
    standard_imports, external_imports, internal_calls, external_calls, type_dependencies,
    source_snapshot_id, git_commit_hash, author, updated_at, indexed_at,
    file_version, is_latest, chunk_key
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33)
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

-- name: UpdateChunkImportanceScore :exec
UPDATE chunks
SET importance_score = $2
WHERE id = $1;

-- Phase 4タスク8: インデックス鮮度の監視用クエリ

-- name: GetChunksWithGitInfo :many
-- 鮮度チェックのためにgit_commit_hash付きチャンクを取得
SELECT
    c.id,
    c.chunk_key,
    c.git_commit_hash,
    c.updated_at,
    c.indexed_at,
    c.is_latest,
    f.path as file_path
FROM chunks c
INNER JOIN files f ON c.file_id = f.id
WHERE c.is_latest = true
  AND c.git_commit_hash IS NOT NULL
ORDER BY c.indexed_at DESC;

-- name: GetStaleChunks :many
-- 指定日数以上古いチャンクを取得
SELECT
    c.id,
    c.chunk_key,
    c.git_commit_hash,
    c.updated_at,
    c.indexed_at,
    c.is_latest,
    f.path as file_path
FROM chunks c
INNER JOIN files f ON c.file_id = f.id
WHERE c.is_latest = true
  AND c.git_commit_hash IS NOT NULL
  AND c.indexed_at < NOW() - INTERVAL '1 day' * $1
ORDER BY c.indexed_at ASC;

-- name: CountStaleChunks :one
-- 指定日数以上古いチャンクの数を取得
SELECT COUNT(*) as stale_count
FROM chunks c
WHERE c.is_latest = true
  AND c.git_commit_hash IS NOT NULL
  AND c.indexed_at < NOW() - INTERVAL '1 day' * $1;
