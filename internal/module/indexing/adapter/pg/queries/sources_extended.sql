-- name: CreateSourceIfNotExists :one
INSERT INTO sources (name, source_type, product_id, metadata)
VALUES ($1, $2, $3, $4)
ON CONFLICT (name)
DO UPDATE SET
    source_type = EXCLUDED.source_type,
    product_id = EXCLUDED.product_id,
    metadata = EXCLUDED.metadata,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;
