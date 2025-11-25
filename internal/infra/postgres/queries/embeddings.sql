-- name: CreateEmbedding :one
INSERT INTO embeddings (chunk_id, vector, model)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetEmbedding :one
SELECT * FROM embeddings
WHERE chunk_id = $1;

-- name: SearchSimilarChunks :many
SELECT
    e.chunk_id,
    e.vector,
    e.model,
    e.created_at,
    1 - (e.vector <=> $1::vector) as similarity
FROM embeddings e
ORDER BY e.vector <=> $1::vector
LIMIT $2;

-- name: DeleteEmbedding :exec
DELETE FROM embeddings
WHERE chunk_id = $1;

-- name: SearchChunksByProduct :many
WITH latest_snapshots AS (
    SELECT DISTINCT ON (source_id) id, source_id
    FROM source_snapshots
    WHERE indexed = TRUE
    ORDER BY source_id, indexed_at DESC NULLS LAST, created_at DESC
)
SELECT
    c.id AS chunk_id,
    f.path,
    c.start_line,
    c.end_line,
    c.content,
    (1::float8 - (e.vector <=> sqlc.arg(query_vector)::vector))::float8 AS score
FROM embeddings e
INNER JOIN chunks c ON e.chunk_id = c.id
INNER JOIN files f ON c.file_id = f.id
INNER JOIN latest_snapshots ls ON f.snapshot_id = ls.id
INNER JOIN sources s ON ls.source_id = s.id
WHERE s.product_id = sqlc.arg(product_id)
  AND (sqlc.narg(path_prefix)::text IS NULL OR f.path LIKE (sqlc.narg(path_prefix)::text || '%'))
  AND (sqlc.narg(content_type)::text IS NULL OR f.content_type = sqlc.narg(content_type)::text)
ORDER BY e.vector <=> sqlc.arg(query_vector)::vector
LIMIT sqlc.arg(row_limit);

-- name: SearchChunksBySource :many
WITH latest_snapshot AS (
    SELECT id
    FROM source_snapshots
    WHERE source_id = sqlc.arg(source_id)
      AND indexed = TRUE
    ORDER BY indexed_at DESC NULLS LAST, created_at DESC
    LIMIT 1
)
SELECT
    c.id AS chunk_id,
    f.path,
    c.start_line,
    c.end_line,
    c.content,
    (1::float8 - (e.vector <=> sqlc.arg(query_vector)::vector))::float8 AS score
FROM embeddings e
INNER JOIN chunks c ON e.chunk_id = c.id
INNER JOIN files f ON c.file_id = f.id
INNER JOIN latest_snapshot ls ON f.snapshot_id = ls.id
WHERE (sqlc.narg(path_prefix)::text IS NULL OR f.path LIKE (sqlc.narg(path_prefix)::text || '%'))
  AND (sqlc.narg(content_type)::text IS NULL OR f.content_type = sqlc.narg(content_type)::text)
ORDER BY e.vector <=> sqlc.arg(query_vector)::vector
LIMIT sqlc.arg(row_limit);

-- name: GetChunk :one
SELECT * FROM chunks
WHERE id = $1;

-- name: ListChunksByOrdinalRange :many
SELECT * FROM chunks
WHERE file_id = $1 AND ordinal BETWEEN $2 AND $3
ORDER BY ordinal;

-- name: GetParentChunk :one
SELECT c.*
FROM chunks c
INNER JOIN chunk_hierarchy ch ON c.id = ch.parent_chunk_id
WHERE ch.child_chunk_id = $1
LIMIT 1;

-- name: GetChildChunks :many
SELECT c.*
FROM chunks c
INNER JOIN chunk_hierarchy ch ON c.id = ch.child_chunk_id
WHERE ch.parent_chunk_id = $1
ORDER BY ch.ordinal;
