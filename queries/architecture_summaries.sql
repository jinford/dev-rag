-- name: CreateArchitectureSummary :one
INSERT INTO architecture_summaries (
    snapshot_id,
    summary_type,
    summary,
    embedding,
    metadata
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: UpsertArchitectureSummary :one
INSERT INTO architecture_summaries (
    snapshot_id,
    summary_type,
    summary,
    embedding,
    metadata
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (snapshot_id, summary_type)
DO UPDATE SET
    summary = EXCLUDED.summary,
    embedding = EXCLUDED.embedding,
    metadata = EXCLUDED.metadata,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetArchitectureSummary :one
SELECT summary FROM architecture_summaries
WHERE snapshot_id = $1 AND summary_type = $2;

-- name: GetArchitectureSummaryByID :one
SELECT * FROM architecture_summaries
WHERE id = $1;

-- name: ListArchitectureSummariesBySnapshot :many
SELECT * FROM architecture_summaries
WHERE snapshot_id = $1
ORDER BY summary_type ASC;

-- name: ListArchitectureSummariesByType :many
SELECT * FROM architecture_summaries
WHERE summary_type = $1
ORDER BY created_at DESC;

-- name: CountArchitectureSummaries :one
SELECT COUNT(*) FROM architecture_summaries
WHERE snapshot_id = $1;

-- name: HasAllRequiredArchitectureSummaries :one
SELECT
    COUNT(DISTINCT summary_type) = 4 AS has_all
FROM architecture_summaries
WHERE snapshot_id = $1
AND summary_type IN ('overview', 'tech_stack', 'data_flow', 'components');

-- name: DeleteArchitectureSummaryByType :exec
DELETE FROM architecture_summaries
WHERE snapshot_id = $1 AND summary_type = $2;

-- name: DeleteArchitectureSummariesBySnapshot :exec
DELETE FROM architecture_summaries
WHERE snapshot_id = $1;

-- name: SearchArchitectureSummariesByEmbedding :many
SELECT
    *,
    1 - (embedding <=> $1::vector) AS similarity
FROM architecture_summaries
WHERE snapshot_id = $2
ORDER BY embedding <=> $1::vector
LIMIT $3;

-- name: GetAllArchitectureSummaryTypes :many
SELECT DISTINCT summary_type
FROM architecture_summaries
ORDER BY summary_type;
