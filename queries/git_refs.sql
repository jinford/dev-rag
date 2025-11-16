-- name: CreateGitRef :one
INSERT INTO git_refs (source_id, ref_name, snapshot_id)
VALUES ($1, $2, $3)
ON CONFLICT (source_id, ref_name)
DO UPDATE SET snapshot_id = EXCLUDED.snapshot_id, updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetGitRef :one
SELECT * FROM git_refs
WHERE id = $1;

-- name: GetGitRefByName :one
SELECT * FROM git_refs
WHERE source_id = $1 AND ref_name = $2;

-- name: ListGitRefsBySource :many
SELECT * FROM git_refs
WHERE source_id = $1
ORDER BY ref_name;

-- name: UpdateGitRef :one
UPDATE git_refs
SET snapshot_id = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteGitRef :exec
DELETE FROM git_refs
WHERE id = $1;
