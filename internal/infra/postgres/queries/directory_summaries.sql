-- name: CreateDirectorySummary :one
INSERT INTO directory_summaries (
    snapshot_id,
    path,
    parent_path,
    depth,
    summary,
    embedding,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: UpsertDirectorySummary :one
INSERT INTO directory_summaries (
    snapshot_id,
    path,
    parent_path,
    depth,
    summary,
    embedding,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (snapshot_id, path)
DO UPDATE SET
    parent_path = EXCLUDED.parent_path,
    depth = EXCLUDED.depth,
    summary = EXCLUDED.summary,
    embedding = EXCLUDED.embedding,
    metadata = EXCLUDED.metadata,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetDirectorySummaryByPath :one
SELECT summary FROM directory_summaries
WHERE snapshot_id = $1 AND path = $2;

-- name: GetDirectorySummaryByID :one
SELECT * FROM directory_summaries
WHERE id = $1;

-- name: ListDirectorySummariesBySnapshot :many
SELECT * FROM directory_summaries
WHERE snapshot_id = $1
ORDER BY depth ASC, path ASC;

-- name: ListDirectorySummariesByDepth :many
SELECT * FROM directory_summaries
WHERE snapshot_id = $1 AND depth = $2
ORDER BY path ASC;

-- name: ListDirectorySummariesByParentPath :many
SELECT * FROM directory_summaries
WHERE snapshot_id = $1 AND parent_path = $2
ORDER BY path ASC;

-- name: CountDirectorySummariesBySnapshot :one
SELECT COUNT(*) FROM directory_summaries
WHERE snapshot_id = $1;

-- name: DeleteDirectorySummaryByPath :exec
DELETE FROM directory_summaries
WHERE snapshot_id = $1 AND path = $2;

-- name: DeleteDirectorySummariesBySnapshot :exec
DELETE FROM directory_summaries
WHERE snapshot_id = $1;

-- name: SearchDirectorySummariesByEmbedding :many
SELECT
    *,
    1 - (embedding <=> $1::vector) AS similarity
FROM directory_summaries
WHERE snapshot_id = $2
ORDER BY embedding <=> $1::vector
LIMIT $3;

-- name: GetMaxDepthBySnapshot :one
SELECT COALESCE(MAX(depth), 0) AS max_depth
FROM directory_summaries
WHERE snapshot_id = $1;
