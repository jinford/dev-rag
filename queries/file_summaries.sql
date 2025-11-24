-- name: CreateFileSummary :one
INSERT INTO file_summaries (
    file_id,
    summary,
    embedding,
    metadata
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: UpsertFileSummary :one
INSERT INTO file_summaries (
    file_id,
    summary,
    embedding,
    metadata
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT (file_id)
DO UPDATE SET
    summary = EXCLUDED.summary,
    embedding = EXCLUDED.embedding,
    metadata = EXCLUDED.metadata,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetFileSummaryByFileID :one
SELECT * FROM file_summaries
WHERE file_id = $1;

-- name: GetFileSummaryByPath :one
SELECT fs.summary
FROM file_summaries fs
JOIN files f ON fs.file_id = f.id
WHERE f.snapshot_id = $1 AND f.path = $2;

-- name: ListFileSummariesBySnapshot :many
SELECT fs.*
FROM file_summaries fs
JOIN files f ON fs.file_id = f.id
WHERE f.snapshot_id = $1
ORDER BY f.path;

-- name: CountFileSummariesBySnapshot :one
SELECT COUNT(*)
FROM file_summaries fs
JOIN files f ON fs.file_id = f.id
WHERE f.snapshot_id = $1;

-- name: DeleteFileSummaryByFileID :exec
DELETE FROM file_summaries
WHERE file_id = $1;

-- name: DeleteFileSummariesBySnapshot :exec
DELETE FROM file_summaries
WHERE file_id IN (
    SELECT id FROM files WHERE snapshot_id = $1
);

-- name: SearchFileSummariesByEmbedding :many
SELECT
    fs.*,
    f.path,
    f.language,
    1 - (fs.embedding <=> $1::vector) AS similarity
FROM file_summaries fs
JOIN files f ON fs.file_id = f.id
WHERE f.snapshot_id = $2
ORDER BY fs.embedding <=> $1::vector
LIMIT $3;
