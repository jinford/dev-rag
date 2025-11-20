-- name: CreateFile :one
INSERT INTO files (snapshot_id, path, size, content_type, content_hash, language, domain)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetFile :one
SELECT * FROM files
WHERE id = $1;

-- name: GetFileByPath :one
SELECT * FROM files
WHERE snapshot_id = $1 AND path = $2;

-- name: ListFilesBySnapshot :many
SELECT * FROM files
WHERE snapshot_id = $1
ORDER BY path;

-- name: GetFileHashesBySnapshot :many
SELECT path, content_hash
FROM files
WHERE snapshot_id = $1;

-- name: DeleteFilesByPaths :exec
DELETE FROM files
WHERE snapshot_id = $1 AND path = ANY($2::text[]);

-- name: ListFilesByContentType :many
SELECT * FROM files
WHERE snapshot_id = $1 AND content_type = $2
ORDER BY path;

-- name: FindFilesByContentHash :many
SELECT * FROM files
WHERE content_hash = $1
ORDER BY created_at DESC;

-- name: DeleteFile :exec
DELETE FROM files
WHERE id = $1;

-- name: DeleteFilesBySnapshot :exec
DELETE FROM files
WHERE snapshot_id = $1;
