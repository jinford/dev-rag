-- カバレッジマップ構築 - snapshot_files操作

-- name: CreateSnapshotFile :one
INSERT INTO snapshot_files (snapshot_id, file_path, file_size, domain, indexed, skip_reason)
VALUES ($1, $2, $3, sqlc.narg('domain'), $4, sqlc.narg('skip_reason'))
RETURNING *;

-- name: UpdateSnapshotFileIndexed :exec
UPDATE snapshot_files
SET indexed = $3
WHERE snapshot_id = $1 AND file_path = $2;

-- name: GetSnapshotFilesBySnapshot :many
SELECT * FROM snapshot_files
WHERE snapshot_id = $1
ORDER BY file_path;

-- name: GetDomainCoverageStats :many
SELECT
    COALESCE(sf.domain, 'unknown') AS domain,
    COUNT(sf.id)::bigint AS total_files,
    SUM(CASE WHEN sf.indexed THEN 1 ELSE 0 END)::bigint AS indexed_files,
    COALESCE(SUM(chunk_counts.chunk_count), 0)::bigint AS indexed_chunks,
    ROUND(
        CASE
            WHEN COUNT(sf.id) > 0 THEN
                SUM(CASE WHEN sf.indexed THEN 1 ELSE 0 END)::numeric / COUNT(sf.id) * 100
            ELSE 0
        END,
        2
    )::numeric AS coverage_rate,
    COALESCE(AVG(chunk_stats.avg_comment_ratio), 0.0)::numeric AS avg_comment_ratio,
    COALESCE(AVG(chunk_stats.avg_complexity), 0.0)::numeric AS avg_complexity
FROM snapshot_files sf
LEFT JOIN files f ON sf.snapshot_id = f.snapshot_id AND sf.file_path = f.path
LEFT JOIN (
    SELECT file_id, COUNT(*) AS chunk_count
    FROM chunks
    GROUP BY file_id
) chunk_counts ON f.id = chunk_counts.file_id
LEFT JOIN (
    SELECT
        c.file_id,
        AVG(COALESCE(c.comment_ratio, 0.0)) AS avg_comment_ratio,
        AVG(COALESCE(c.cyclomatic_complexity, 0)) AS avg_complexity
    FROM chunks c
    GROUP BY c.file_id
) chunk_stats ON f.id = chunk_stats.file_id
WHERE sf.snapshot_id = $1
GROUP BY COALESCE(sf.domain, 'unknown')
ORDER BY COALESCE(sf.domain, 'unknown');

-- name: GetUnindexedImportantFiles :many
SELECT file_path
FROM snapshot_files
WHERE snapshot_id = $1
AND indexed = false
AND (
    file_path ILIKE '%README.md'
    OR file_path ILIKE '%/docs/adr/%'
    OR file_path ILIKE '%/docs/design/%'
    OR file_path ILIKE '%/docs/decisions/%'
    OR file_path IN ('package.json', 'go.mod', 'Dockerfile', 'docker-compose.yml')
)
ORDER BY file_path;
