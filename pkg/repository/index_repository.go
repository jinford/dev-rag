package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/pgvector/pgvector-go"
)

// IndexRepository はインデックス集約のデータベース操作を提供します
// 集約: File + Chunk + Embedding
type IndexRepository struct {
	pool *pgxpool.Pool
}

// NewIndexRepository は新しいIndexRepositoryを作成します
func NewIndexRepository(pool *pgxpool.Pool) *IndexRepository {
	return &IndexRepository{pool: pool}
}

// SearchFilter はベクトル検索のフィルタ条件を表します
type SearchFilter struct {
	PathPrefix  *string
	ContentType *string
}

// === File操作 ===

// CreateFile はファイルを作成します
func (r *IndexRepository) CreateFile(ctx context.Context, snapshotID uuid.UUID, path string, size int64, contentType *string, contentHash string) (*models.File, error) {
	query := `
		INSERT INTO files (snapshot_id, path, size, content_type, content_hash)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, snapshot_id, path, size, content_type, content_hash, created_at
	`

	var file models.File
	err := r.pool.QueryRow(ctx, query, snapshotID, path, size, contentType, contentHash).Scan(
		&file.ID,
		&file.SnapshotID,
		&file.Path,
		&file.Size,
		&file.ContentType,
		&file.ContentHash,
		&file.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return &file, nil
}

// GetFileByID はIDでファイルを取得します
func (r *IndexRepository) GetFileByID(ctx context.Context, id uuid.UUID) (*models.File, error) {
	query := `
		SELECT id, snapshot_id, path, size, content_type, content_hash, created_at
		FROM files
		WHERE id = $1
	`

	var file models.File
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&file.ID,
		&file.SnapshotID,
		&file.Path,
		&file.Size,
		&file.ContentType,
		&file.ContentHash,
		&file.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("file not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return &file, nil
}

// ListFilesBySnapshot はスナップショット内のファイル一覧を取得します
func (r *IndexRepository) ListFilesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*models.File, error) {
	query := `
		SELECT id, snapshot_id, path, size, content_type, content_hash, created_at
		FROM files
		WHERE snapshot_id = $1
		ORDER BY path
	`

	rows, err := r.pool.Query(ctx, query, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	defer rows.Close()

	var files []*models.File
	for rows.Next() {
		var file models.File
		if err := rows.Scan(
			&file.ID,
			&file.SnapshotID,
			&file.Path,
			&file.Size,
			&file.ContentType,
			&file.ContentHash,
			&file.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}
		files = append(files, &file)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating files: %w", err)
	}

	return files, nil
}

// GetFileHashesBySnapshot はスナップショット内のファイルパス→ハッシュマップを取得します（差分インデックス用）
func (r *IndexRepository) GetFileHashesBySnapshot(ctx context.Context, snapshotID uuid.UUID) (map[string]string, error) {
	query := `
		SELECT path, content_hash
		FROM files
		WHERE snapshot_id = $1
	`

	rows, err := r.pool.Query(ctx, query, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file hashes: %w", err)
	}
	defer rows.Close()

	hashes := make(map[string]string)
	for rows.Next() {
		var path, contentHash string
		if err := rows.Scan(&path, &contentHash); err != nil {
			return nil, fmt.Errorf("failed to scan file hash: %w", err)
		}
		hashes[path] = contentHash
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating file hashes: %w", err)
	}

	return hashes, nil
}

// DeleteFileByID はファイルを削除します（cascade削除）
func (r *IndexRepository) DeleteFileByID(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM files WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("file not found: %s", id)
	}

	return nil
}

// DeleteFilesByPaths はパス配列でファイルを削除します
func (r *IndexRepository) DeleteFilesByPaths(ctx context.Context, snapshotID uuid.UUID, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	query := `
		DELETE FROM files
		WHERE snapshot_id = $1 AND path = ANY($2)
	`

	_, err := r.pool.Exec(ctx, query, snapshotID, paths)
	if err != nil {
		return fmt.Errorf("failed to delete files by paths: %w", err)
	}

	return nil
}

// === Chunk操作（ファイルの一部） ===

// CreateChunk はチャンクを作成します
func (r *IndexRepository) CreateChunk(ctx context.Context, fileID uuid.UUID, ordinal int, startLine int, endLine int, content string, contentHash string, tokenCount *int) (*models.Chunk, error) {
	query := `
		INSERT INTO chunks (file_id, ordinal, start_line, end_line, content, content_hash, token_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, file_id, ordinal, start_line, end_line, content, content_hash, token_count, created_at
	`

	var chunk models.Chunk
	err := r.pool.QueryRow(ctx, query, fileID, ordinal, startLine, endLine, content, contentHash, tokenCount).Scan(
		&chunk.ID,
		&chunk.FileID,
		&chunk.Ordinal,
		&chunk.StartLine,
		&chunk.EndLine,
		&chunk.Content,
		&chunk.ContentHash,
		&chunk.TokenCount,
		&chunk.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunk: %w", err)
	}

	return &chunk, nil
}

// BatchCreateChunks はチャンクを一括作成します
func (r *IndexRepository) BatchCreateChunks(ctx context.Context, chunks []*models.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, chunk := range chunks {
		query := `
			INSERT INTO chunks (file_id, ordinal, start_line, end_line, content, content_hash, token_count)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
		batch.Queue(query, chunk.FileID, chunk.Ordinal, chunk.StartLine, chunk.EndLine, chunk.Content, chunk.ContentHash, chunk.TokenCount)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for i := 0; i < len(chunks); i++ {
		_, err := results.Exec()
		if err != nil {
			return fmt.Errorf("failed to batch create chunk %d: %w", i, err)
		}
	}

	return nil
}

// GetChunkByID はIDでチャンクを取得します
func (r *IndexRepository) GetChunkByID(ctx context.Context, id uuid.UUID) (*models.Chunk, error) {
	query := `
		SELECT id, file_id, ordinal, start_line, end_line, content, content_hash, token_count, created_at
		FROM chunks
		WHERE id = $1
	`

	var chunk models.Chunk
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&chunk.ID,
		&chunk.FileID,
		&chunk.Ordinal,
		&chunk.StartLine,
		&chunk.EndLine,
		&chunk.Content,
		&chunk.ContentHash,
		&chunk.TokenCount,
		&chunk.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("chunk not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get chunk: %w", err)
	}

	return &chunk, nil
}

// ListChunksByFile はファイル内のチャンク一覧を取得します
func (r *IndexRepository) ListChunksByFile(ctx context.Context, fileID uuid.UUID) ([]*models.Chunk, error) {
	query := `
		SELECT id, file_id, ordinal, start_line, end_line, content, content_hash, token_count, created_at
		FROM chunks
		WHERE file_id = $1
		ORDER BY ordinal
	`

	rows, err := r.pool.Query(ctx, query, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to list chunks: %w", err)
	}
	defer rows.Close()

	var chunks []*models.Chunk
	for rows.Next() {
		var chunk models.Chunk
		if err := rows.Scan(
			&chunk.ID,
			&chunk.FileID,
			&chunk.Ordinal,
			&chunk.StartLine,
			&chunk.EndLine,
			&chunk.Content,
			&chunk.ContentHash,
			&chunk.TokenCount,
			&chunk.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}
		chunks = append(chunks, &chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chunks: %w", err)
	}

	return chunks, nil
}

// GetChunkContext は前後コンテキストチャンクを取得します
func (r *IndexRepository) GetChunkContext(ctx context.Context, chunkID uuid.UUID, beforeCount int, afterCount int) ([]*models.Chunk, error) {
	// まず対象チャンクを取得して、file_idとordinalを取得
	targetChunk, err := r.GetChunkByID(ctx, chunkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get target chunk: %w", err)
	}

	query := `
		SELECT id, file_id, ordinal, start_line, end_line, content, content_hash, token_count, created_at
		FROM chunks
		WHERE file_id = $1 AND ordinal >= $2 AND ordinal <= $3
		ORDER BY ordinal
	`

	minOrdinal := targetChunk.Ordinal - beforeCount
	if minOrdinal < 0 {
		minOrdinal = 0
	}
	maxOrdinal := targetChunk.Ordinal + afterCount

	rows, err := r.pool.Query(ctx, query, targetChunk.FileID, minOrdinal, maxOrdinal)
	if err != nil {
		return nil, fmt.Errorf("failed to get context chunks: %w", err)
	}
	defer rows.Close()

	var chunks []*models.Chunk
	for rows.Next() {
		var chunk models.Chunk
		if err := rows.Scan(
			&chunk.ID,
			&chunk.FileID,
			&chunk.Ordinal,
			&chunk.StartLine,
			&chunk.EndLine,
			&chunk.Content,
			&chunk.ContentHash,
			&chunk.TokenCount,
			&chunk.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan context chunk: %w", err)
		}
		chunks = append(chunks, &chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating context chunks: %w", err)
	}

	return chunks, nil
}

// DeleteChunksByFileID はファイルIDでチャンクを削除します
func (r *IndexRepository) DeleteChunksByFileID(ctx context.Context, fileID uuid.UUID) error {
	query := `DELETE FROM chunks WHERE file_id = $1`

	_, err := r.pool.Exec(ctx, query, fileID)
	if err != nil {
		return fmt.Errorf("failed to delete chunks by file: %w", err)
	}

	return nil
}

// === Embedding操作（チャンクの一部） ===

// CreateEmbedding はEmbeddingを作成します
func (r *IndexRepository) CreateEmbedding(ctx context.Context, chunkID uuid.UUID, vector []float32, model string) error {
	query := `
		INSERT INTO embeddings (chunk_id, vector, model)
		VALUES ($1, $2, $3)
	`

	pgVector := pgvector.NewVector(vector)
	_, err := r.pool.Exec(ctx, query, chunkID, pgVector, model)
	if err != nil {
		return fmt.Errorf("failed to create embedding: %w", err)
	}

	return nil
}

// BatchCreateEmbeddings はEmbeddingを一括作成します
func (r *IndexRepository) BatchCreateEmbeddings(ctx context.Context, embeddings []*models.Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, embedding := range embeddings {
		query := `
			INSERT INTO embeddings (chunk_id, vector, model)
			VALUES ($1, $2, $3)
		`
		pgVector := pgvector.NewVector(embedding.Vector)
		batch.Queue(query, embedding.ChunkID, pgVector, embedding.Model)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for i := 0; i < len(embeddings); i++ {
		_, err := results.Exec()
		if err != nil {
			return fmt.Errorf("failed to batch create embedding %d: %w", i, err)
		}
	}

	return nil
}

// SearchByProduct はプロダクト単位でベクトル検索を実行します
func (r *IndexRepository) SearchByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*models.SearchResult, error) {
	baseQuery := `
		SELECT
			f.path,
			c.start_line,
			c.end_line,
			c.content,
			1 - (e.vector <=> $1) AS score
		FROM embeddings e
		INNER JOIN chunks c ON e.chunk_id = c.id
		INNER JOIN files f ON c.file_id = f.id
		INNER JOIN source_snapshots ss ON f.snapshot_id = ss.id
		INNER JOIN sources s ON ss.source_id = s.id
		WHERE s.product_id = $2
	`

	args := []interface{}{pgvector.NewVector(queryVector), productID}
	argIndex := 3

	// フィルタ条件を追加
	if filters.PathPrefix != nil {
		baseQuery += fmt.Sprintf(" AND f.path LIKE $%d", argIndex)
		args = append(args, *filters.PathPrefix+"%")
		argIndex++
	}

	if filters.ContentType != nil {
		baseQuery += fmt.Sprintf(" AND f.content_type = $%d", argIndex)
		args = append(args, *filters.ContentType)
		argIndex++
	}

	baseQuery += fmt.Sprintf(" ORDER BY e.vector <=> $1 LIMIT $%d", argIndex)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search by product: %w", err)
	}
	defer rows.Close()

	var results []*models.SearchResult
	for rows.Next() {
		var result models.SearchResult
		if err := rows.Scan(
			&result.FilePath,
			&result.StartLine,
			&result.EndLine,
			&result.Content,
			&result.Score,
		); err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}
		results = append(results, &result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return results, nil
}

// SearchBySource はソース単位でベクトル検索を実行します
func (r *IndexRepository) SearchBySource(ctx context.Context, sourceID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*models.SearchResult, error) {
	baseQuery := `
		SELECT
			f.path,
			c.start_line,
			c.end_line,
			c.content,
			1 - (e.vector <=> $1) AS score
		FROM embeddings e
		INNER JOIN chunks c ON e.chunk_id = c.id
		INNER JOIN files f ON c.file_id = f.id
		INNER JOIN source_snapshots ss ON f.snapshot_id = ss.id
		WHERE ss.source_id = $2
	`

	args := []interface{}{pgvector.NewVector(queryVector), sourceID}
	argIndex := 3

	// フィルタ条件を追加
	if filters.PathPrefix != nil {
		baseQuery += fmt.Sprintf(" AND f.path LIKE $%d", argIndex)
		args = append(args, *filters.PathPrefix+"%")
		argIndex++
	}

	if filters.ContentType != nil {
		baseQuery += fmt.Sprintf(" AND f.content_type = $%d", argIndex)
		args = append(args, *filters.ContentType)
		argIndex++
	}

	baseQuery += fmt.Sprintf(" ORDER BY e.vector <=> $1 LIMIT $%d", argIndex)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search by source: %w", err)
	}
	defer rows.Close()

	var results []*models.SearchResult
	for rows.Next() {
		var result models.SearchResult
		if err := rows.Scan(
			&result.FilePath,
			&result.StartLine,
			&result.EndLine,
			&result.Content,
			&result.Score,
		); err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}
		results = append(results, &result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return results, nil
}
