-- name: AddChunkRelation :exec
INSERT INTO chunk_hierarchy (parent_chunk_id, child_chunk_id, ordinal)
VALUES ($1, $2, $3);

-- name: RemoveChunkRelation :exec
DELETE FROM chunk_hierarchy
WHERE parent_chunk_id = $1 AND child_chunk_id = $2;

-- name: GetChildChunkIDs :many
SELECT child_chunk_id
FROM chunk_hierarchy
WHERE parent_chunk_id = $1
ORDER BY ordinal;

-- name: GetParentChunkID :one
SELECT parent_chunk_id
FROM chunk_hierarchy
WHERE child_chunk_id = $1
LIMIT 1;

-- name: GetChildChunks :many
SELECT c.*
FROM chunks c
INNER JOIN chunk_hierarchy ch ON c.id = ch.child_chunk_id
WHERE ch.parent_chunk_id = $1
ORDER BY ch.ordinal;

-- name: GetParentChunk :one
SELECT c.*
FROM chunks c
INNER JOIN chunk_hierarchy ch ON c.id = ch.parent_chunk_id
WHERE ch.child_chunk_id = $1
LIMIT 1;

-- name: HasChildren :one
SELECT EXISTS(
    SELECT 1
    FROM chunk_hierarchy
    WHERE parent_chunk_id = $1
) AS has_children;

-- name: HasParent :one
SELECT EXISTS(
    SELECT 1
    FROM chunk_hierarchy
    WHERE child_chunk_id = $1
) AS has_parent;

-- name: CountChildChunks :one
SELECT COUNT(*) AS child_count
FROM chunk_hierarchy
WHERE parent_chunk_id = $1;

-- name: DeleteChunkHierarchyByParent :exec
DELETE FROM chunk_hierarchy
WHERE parent_chunk_id = $1;

-- name: DeleteChunkHierarchyByChild :exec
DELETE FROM chunk_hierarchy
WHERE child_chunk_id = $1;
