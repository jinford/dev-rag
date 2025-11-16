-- name: CreateChunkBatch :copyfrom
INSERT INTO chunks (file_id, ordinal, start_line, end_line, content, content_hash, token_count)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: CreateEmbeddingBatch :copyfrom
INSERT INTO embeddings (chunk_id, vector, model)
VALUES ($1, $2, $3);
