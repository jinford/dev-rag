-- name: CreateSource :one
INSERT INTO sources (product_id, name, source_type, metadata)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSource :one
SELECT * FROM sources
WHERE id = $1;

-- name: GetSourceByName :one
SELECT * FROM sources
WHERE name = $1;

-- name: ListSourcesByProduct :many
SELECT * FROM sources
WHERE product_id = $1
ORDER BY created_at DESC;

-- name: ListSourcesByType :many
SELECT * FROM sources
WHERE source_type = $1
ORDER BY created_at DESC;

-- name: UpdateSource :one
UPDATE sources
SET name = $2, source_type = $3, metadata = $4, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteSource :exec
DELETE FROM sources
WHERE id = $1;
