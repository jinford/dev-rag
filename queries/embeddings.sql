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
SELECT
    f.path,
    c.start_line,
    c.end_line,
    c.content,
    (1::float8 - (e.vector <=> sqlc.arg(query_vector)::vector))::float8 AS score
FROM embeddings e
INNER JOIN chunks c ON e.chunk_id = c.id
INNER JOIN files f ON c.file_id = f.id
INNER JOIN source_snapshots ss ON f.snapshot_id = ss.id
INNER JOIN sources s ON ss.source_id = s.id
WHERE s.product_id = sqlc.arg(product_id)
  AND (sqlc.narg(path_prefix)::text IS NULL OR f.path LIKE (sqlc.narg(path_prefix)::text || '%'))
  AND (sqlc.narg(content_type)::text IS NULL OR f.content_type = sqlc.narg(content_type)::text)
ORDER BY e.vector <=> sqlc.arg(query_vector)::vector
LIMIT sqlc.arg(row_limit);

-- name: SearchChunksBySource :many
SELECT
    f.path,
    c.start_line,
    c.end_line,
    c.content,
    (1::float8 - (e.vector <=> sqlc.arg(query_vector)::vector))::float8 AS score
FROM embeddings e
INNER JOIN chunks c ON e.chunk_id = c.id
INNER JOIN files f ON c.file_id = f.id
INNER JOIN source_snapshots ss ON f.snapshot_id = ss.id
WHERE ss.source_id = sqlc.arg(source_id)
  AND (sqlc.narg(path_prefix)::text IS NULL OR f.path LIKE (sqlc.narg(path_prefix)::text || '%'))
  AND (sqlc.narg(content_type)::text IS NULL OR f.content_type = sqlc.narg(content_type)::text)
ORDER BY e.vector <=> sqlc.arg(query_vector)::vector
LIMIT sqlc.arg(row_limit);
