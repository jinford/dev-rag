-- name: CreateProduct :one
INSERT INTO products (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: GetProduct :one
SELECT * FROM products
WHERE id = $1;

-- name: GetProductByName :one
SELECT * FROM products
WHERE name = $1;

-- name: ListProducts :many
SELECT * FROM products
ORDER BY created_at DESC;

-- name: UpdateProduct :one
UPDATE products
SET name = $2, description = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteProduct :exec
DELETE FROM products
WHERE id = $1;

-- name: ListProductsWithStats :many
SELECT
    p.id,
    p.name,
    p.description,
    p.created_at,
    p.updated_at,
    COUNT(DISTINCT s.id)::int AS source_count,
    MAX(ss.indexed_at) AS last_indexed_at,
    MAX(wm.generated_at) AS wiki_generated_at
FROM products p
LEFT JOIN sources s ON p.id = s.product_id
LEFT JOIN source_snapshots ss ON s.id = ss.source_id AND ss.indexed = TRUE
LEFT JOIN wiki_metadata wm ON p.id = wm.product_id
GROUP BY p.id, p.name, p.description, p.created_at, p.updated_at
ORDER BY p.name;
