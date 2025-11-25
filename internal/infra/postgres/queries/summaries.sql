-- name: CreateSummary :one
INSERT INTO summaries (snapshot_id, summary_type, target_path, depth, parent_path, arch_type, content, content_hash, source_hash, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetSummaryByID :one
SELECT * FROM summaries WHERE id = $1;

-- name: GetFileSummary :one
SELECT * FROM summaries
WHERE snapshot_id = $1 AND summary_type = 'file' AND target_path = $2;

-- name: GetDirectorySummary :one
SELECT * FROM summaries
WHERE snapshot_id = $1 AND summary_type = 'directory' AND target_path = $2;

-- name: GetArchitectureSummary :one
SELECT * FROM summaries
WHERE snapshot_id = $1 AND summary_type = 'architecture' AND arch_type = $2;

-- name: ListFileSummariesBySnapshot :many
SELECT * FROM summaries
WHERE snapshot_id = $1 AND summary_type = 'file'
ORDER BY target_path;

-- name: ListDirectorySummariesBySnapshot :many
SELECT * FROM summaries
WHERE snapshot_id = $1 AND summary_type = 'directory'
ORDER BY depth DESC, target_path;

-- name: ListDirectorySummariesByDepth :many
SELECT * FROM summaries
WHERE snapshot_id = $1 AND summary_type = 'directory' AND depth = $2
ORDER BY target_path;

-- name: ListArchitectureSummariesBySnapshot :many
SELECT * FROM summaries
WHERE snapshot_id = $1 AND summary_type = 'architecture';

-- name: UpdateSummary :one
UPDATE summaries
SET content = $2, content_hash = $3, source_hash = $4, metadata = $5, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteSummariesBySnapshot :exec
DELETE FROM summaries WHERE snapshot_id = $1;

-- name: DeleteSummary :exec
DELETE FROM summaries WHERE id = $1;

-- name: GetMaxDirectoryDepth :one
SELECT COALESCE(MAX(depth), 0)::int FROM summaries
WHERE snapshot_id = $1 AND summary_type = 'directory';

-- name: ListSummariesByType :many
SELECT * FROM summaries
WHERE snapshot_id = $1 AND summary_type = $2
ORDER BY created_at DESC;

-- name: CountSummariesByType :one
SELECT COUNT(*) FROM summaries
WHERE snapshot_id = $1 AND summary_type = $2;
