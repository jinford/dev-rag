-- name: CreateSourceSnapshot :one
INSERT INTO source_snapshots (source_id, version_identifier)
VALUES ($1, $2)
RETURNING *;

-- name: GetSourceSnapshot :one
SELECT * FROM source_snapshots
WHERE id = $1;

-- name: GetSourceSnapshotByVersion :one
SELECT * FROM source_snapshots
WHERE source_id = $1 AND version_identifier = $2;

-- name: ListSourceSnapshotsBySource :many
SELECT * FROM source_snapshots
WHERE source_id = $1
ORDER BY created_at DESC;

-- name: GetLatestIndexedSnapshot :one
SELECT * FROM source_snapshots
WHERE source_id = $1 AND indexed = TRUE
ORDER BY indexed_at DESC NULLS LAST, created_at DESC
LIMIT 1;

-- name: ListIndexedSnapshots :many
SELECT * FROM source_snapshots
WHERE indexed = TRUE
ORDER BY indexed_at DESC;

-- name: MarkSnapshotIndexed :one
UPDATE source_snapshots
SET indexed = TRUE, indexed_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteSourceSnapshot :exec
DELETE FROM source_snapshots
WHERE id = $1;
