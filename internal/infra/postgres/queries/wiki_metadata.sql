-- name: CreateWikiMetadata :one
INSERT INTO wiki_metadata (product_id, output_path, file_count, generated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (product_id)
DO UPDATE SET
    output_path = EXCLUDED.output_path,
    file_count = EXCLUDED.file_count,
    generated_at = EXCLUDED.generated_at
RETURNING *;

-- name: GetWikiMetadata :one
SELECT * FROM wiki_metadata
WHERE id = $1;

-- name: GetWikiMetadataByProduct :one
SELECT * FROM wiki_metadata
WHERE product_id = $1;

-- name: ListWikiMetadata :many
SELECT * FROM wiki_metadata
ORDER BY generated_at DESC;

-- name: DeleteWikiMetadata :exec
DELETE FROM wiki_metadata
WHERE id = $1;
