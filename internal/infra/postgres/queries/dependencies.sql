-- name: CreateDependency :exec
INSERT INTO chunk_dependencies (
    from_chunk_id,
    to_chunk_id,
    dep_type,
    symbol
) VALUES (
    $1, $2, $3, $4
) ON CONFLICT (from_chunk_id, to_chunk_id, dep_type, symbol) DO NOTHING;

-- name: GetDependenciesByChunk :many
SELECT * FROM chunk_dependencies
WHERE from_chunk_id = $1
ORDER BY dep_type, symbol;

-- name: GetDependenciesByChunkAndType :many
SELECT * FROM chunk_dependencies
WHERE from_chunk_id = $1 AND dep_type = $2
ORDER BY symbol;

-- name: GetIncomingDependenciesByChunk :many
SELECT * FROM chunk_dependencies
WHERE to_chunk_id = $1
ORDER BY dep_type, symbol;

-- name: DeleteDependenciesByChunk :exec
DELETE FROM chunk_dependencies
WHERE from_chunk_id = $1 OR to_chunk_id = $1;

-- name: GetDependencyCount :one
SELECT COUNT(*) FROM chunk_dependencies
WHERE from_chunk_id = $1;

-- name: GetIncomingDependencyCount :one
SELECT COUNT(*) FROM chunk_dependencies
WHERE to_chunk_id = $1;

-- name: GetAllDependencies :many
SELECT * FROM chunk_dependencies
ORDER BY created_at DESC;
