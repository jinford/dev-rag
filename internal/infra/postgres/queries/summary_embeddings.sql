-- name: CreateSummaryEmbedding :one
INSERT INTO summary_embeddings (summary_id, vector, model)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSummaryEmbedding :one
SELECT * FROM summary_embeddings WHERE summary_id = $1;

-- name: UpsertSummaryEmbedding :one
INSERT INTO summary_embeddings (summary_id, vector, model)
VALUES ($1, $2, $3)
ON CONFLICT (summary_id)
DO UPDATE SET vector = EXCLUDED.vector, model = EXCLUDED.model, created_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: DeleteSummaryEmbedding :exec
DELETE FROM summary_embeddings WHERE summary_id = $1;

-- name: SearchSummaryEmbeddings :many
SELECT s.*, se.vector, (1 - (se.vector <=> $1::vector))::float8 AS score
FROM summaries s
JOIN summary_embeddings se ON s.id = se.summary_id
WHERE s.snapshot_id = $2
ORDER BY se.vector <=> $1::vector
LIMIT $3;

-- name: SearchFileSummaryEmbeddings :many
SELECT s.*, se.vector, (1 - (se.vector <=> $1::vector))::float8 AS score
FROM summaries s
JOIN summary_embeddings se ON s.id = se.summary_id
WHERE s.snapshot_id = $2 AND s.summary_type = 'file'
ORDER BY se.vector <=> $1::vector
LIMIT $3;

-- name: SearchDirectorySummaryEmbeddings :many
SELECT s.*, se.vector, (1 - (se.vector <=> $1::vector))::float8 AS score
FROM summaries s
JOIN summary_embeddings se ON s.id = se.summary_id
WHERE s.snapshot_id = $2 AND s.summary_type = 'directory'
ORDER BY se.vector <=> $1::vector
LIMIT $3;

-- name: SearchArchitectureSummaryEmbeddings :many
SELECT s.*, se.vector, (1 - (se.vector <=> $1::vector))::float8 AS score
FROM summaries s
JOIN summary_embeddings se ON s.id = se.summary_id
WHERE s.snapshot_id = $2 AND s.summary_type = 'architecture'
ORDER BY se.vector <=> $1::vector
LIMIT $3;

-- name: DeleteSummaryEmbeddingsBySnapshot :exec
DELETE FROM summary_embeddings
WHERE summary_id IN (
    SELECT id FROM summaries WHERE snapshot_id = $1
);

-- name: CountSummaryEmbeddingsBySnapshot :one
SELECT COUNT(*) FROM summary_embeddings se
JOIN summaries s ON se.summary_id = s.id
WHERE s.snapshot_id = $1;

-- name: SearchSummariesBySnapshot :many
SELECT
    s.id,
    s.summary_type,
    s.target_path,
    s.arch_type,
    s.content,
    (1 - (se.vector <=> sqlc.arg(query_vector)::vector))::float8 as score
FROM summaries s
JOIN summary_embeddings se ON s.id = se.summary_id
WHERE s.snapshot_id = sqlc.arg(snapshot_id)
  AND (cardinality(sqlc.arg(summary_types)::text[]) = 0 OR s.summary_type = ANY(sqlc.arg(summary_types)::text[]))
  AND (sqlc.arg(path_prefix)::text IS NULL OR s.target_path LIKE sqlc.arg(path_prefix)::text || '%')
ORDER BY se.vector <=> sqlc.arg(query_vector)::vector
LIMIT sqlc.arg(limit_val);

-- name: SearchSummariesByProduct :many
WITH latest_snapshots AS (
    SELECT DISTINCT ON (source_id) id, source_id
    FROM source_snapshots
    WHERE indexed = TRUE
    ORDER BY source_id, indexed_at DESC NULLS LAST, created_at DESC
)
SELECT
    s.id,
    s.snapshot_id,
    s.summary_type,
    s.target_path,
    s.arch_type,
    s.content,
    (1 - (se.vector <=> sqlc.arg(query_vector)::vector))::float8 as score
FROM summaries s
JOIN summary_embeddings se ON s.id = se.summary_id
JOIN latest_snapshots ls ON s.snapshot_id = ls.id
JOIN sources src ON ls.source_id = src.id
WHERE src.product_id = sqlc.arg(product_id)
  AND (cardinality(sqlc.arg(summary_types)::text[]) = 0 OR s.summary_type = ANY(sqlc.arg(summary_types)::text[]))
  AND (sqlc.narg(path_prefix)::text IS NULL OR s.target_path LIKE sqlc.narg(path_prefix)::text || '%')
ORDER BY se.vector <=> sqlc.arg(query_vector)::vector
LIMIT sqlc.arg(limit_val);
