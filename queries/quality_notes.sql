-- name: CreateQualityNote :one
INSERT INTO quality_notes (note_id, severity, note_text, linked_files, linked_chunks, reviewer, status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetQualityNote :one
SELECT * FROM quality_notes
WHERE id = $1;

-- name: GetQualityNoteByNoteID :one
SELECT * FROM quality_notes
WHERE note_id = $1;

-- name: ListQualityNotes :many
SELECT * FROM quality_notes
WHERE
    (@severity::text IS NULL OR severity = @severity) AND
    (@status::text IS NULL OR status = @status) AND
    (@start_date::timestamp IS NULL OR created_at >= @start_date) AND
    (@end_date::timestamp IS NULL OR created_at <= @end_date)
ORDER BY created_at DESC
LIMIT COALESCE(@limit_count, 100);

-- name: ListQualityNotesBySeverity :many
SELECT * FROM quality_notes
WHERE severity = $1
ORDER BY created_at DESC;

-- name: ListQualityNotesByStatus :many
SELECT * FROM quality_notes
WHERE status = $1
ORDER BY created_at DESC;

-- name: ListQualityNotesByDateRange :many
SELECT * FROM quality_notes
WHERE created_at >= $1 AND created_at <= $2
ORDER BY created_at DESC;

-- name: UpdateQualityNoteStatus :one
UPDATE quality_notes
SET status = $2, resolved_at = $3
WHERE id = $1
RETURNING *;

-- name: UpdateQualityNote :one
UPDATE quality_notes
SET
    note_text = $2,
    linked_files = $3,
    linked_chunks = $4,
    severity = $5
WHERE id = $1
RETURNING *;

-- name: DeleteQualityNote :exec
DELETE FROM quality_notes
WHERE id = $1;

-- name: CountQualityNotesBySeverity :one
SELECT severity, COUNT(*) as count
FROM quality_notes
WHERE status = $1
GROUP BY severity
ORDER BY
    CASE severity
        WHEN 'critical' THEN 1
        WHEN 'high' THEN 2
        WHEN 'medium' THEN 3
        WHEN 'low' THEN 4
    END;

-- name: GetRecentQualityNotes :many
SELECT * FROM quality_notes
WHERE created_at >= NOW() - INTERVAL '7 days'
ORDER BY severity DESC, created_at DESC;
