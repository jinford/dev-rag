package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/sqlc"
	pgvector "github.com/pgvector/pgvector-go"
)

// IndexRepositoryR はファイル/チャンク/Embedding 集約への読み取り専用アクセスを提供します
type IndexRepositoryR struct {
	q sqlc.Querier
}

// NewIndexRepositoryR は sqlc の DBTX を受け取り、読み取り専用リポジトリを初期化します
func NewIndexRepositoryR(q sqlc.Querier) *IndexRepositoryR {
	return &IndexRepositoryR{q: q}
}

// IndexRepositoryRW は IndexRepositoryR を埋め込み、読み書き操作を提供します
type IndexRepositoryRW struct {
	*IndexRepositoryR
}

// NewIndexRepositoryRW は読み書き操作を提供するリポジトリを初期化します
func NewIndexRepositoryRW(q sqlc.Querier) *IndexRepositoryRW {
	return &IndexRepositoryRW{IndexRepositoryR: NewIndexRepositoryR(q)}
}

// SearchFilter は検索時の任意フィルタを表します
type SearchFilter struct {
	PathPrefix  *string
	ContentType *string
}

// === File 操作 ===

// CreateFile はファイルレコードを作成します
func (rw *IndexRepositoryRW) CreateFile(ctx context.Context, snapshotID uuid.UUID, path string, size int64, contentType string, contentHash string) (*models.File, error) {
	file, err := rw.q.CreateFile(ctx, sqlc.CreateFileParams{
		SnapshotID:  UUIDToPgtype(snapshotID),
		Path:        path,
		Size:        size,
		ContentType: contentType,
		ContentHash: contentHash,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return convertSQLCFile(file), nil
}

// GetFileByID は ID でファイルを取得します
func (r *IndexRepositoryR) GetFileByID(ctx context.Context, id uuid.UUID) (*models.File, error) {
	file, err := r.q.GetFile(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("file not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return convertSQLCFile(file), nil
}

// ListFilesBySnapshot はスナップショット配下のファイル一覧を取得します
func (r *IndexRepositoryR) ListFilesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*models.File, error) {
	rows, err := r.q.ListFilesBySnapshot(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := make([]*models.File, 0, len(rows))
	for _, row := range rows {
		files = append(files, convertSQLCFile(row))
	}

	return files, nil
}

// GetFileHashesBySnapshot は差分判定用に path->hash を返します
func (r *IndexRepositoryR) GetFileHashesBySnapshot(ctx context.Context, snapshotID uuid.UUID) (map[string]string, error) {
	rows, err := r.q.GetFileHashesBySnapshot(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to get file hashes: %w", err)
	}

	hashes := make(map[string]string, len(rows))
	for _, row := range rows {
		hashes[row.Path] = row.ContentHash
	}

	return hashes, nil
}

// DeleteFileByID は単一ファイルを削除します
func (rw *IndexRepositoryRW) DeleteFileByID(ctx context.Context, id uuid.UUID) error {
	if _, err := rw.q.GetFile(ctx, UUIDToPgtype(id)); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("file not found: %s", id)
		}
		return fmt.Errorf("failed to get file: %w", err)
	}

	if err := rw.q.DeleteFile(ctx, UUIDToPgtype(id)); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// DeleteFilesByPaths は snapshot+paths 条件で一括削除します
func (rw *IndexRepositoryRW) DeleteFilesByPaths(ctx context.Context, snapshotID uuid.UUID, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	if err := rw.q.DeleteFilesByPaths(ctx, sqlc.DeleteFilesByPathsParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		Column2:    paths,
	}); err != nil {
		return fmt.Errorf("failed to delete files by paths: %w", err)
	}

	return nil
}

// === Chunk 操作 ===

// CreateChunk はチャンクを1件作成します
func (rw *IndexRepositoryRW) CreateChunk(ctx context.Context, fileID uuid.UUID, ordinal int, startLine int, endLine int, content string, contentHash string, tokenCount int) (*models.Chunk, error) {
	chunk, err := rw.q.CreateChunk(ctx, sqlc.CreateChunkParams{
		FileID:      UUIDToPgtype(fileID),
		Ordinal:     int32(ordinal),
		StartLine:   int32(startLine),
		EndLine:     int32(endLine),
		Content:     content,
		ContentHash: contentHash,
		TokenCount:  IntToPgtype(tokenCount),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create chunk: %w", err)
	}

	return convertSQLCChunk(chunk), nil
}

// BatchCreateChunks はチャンクを CopyFrom で一括登録します
func (rw *IndexRepositoryRW) BatchCreateChunks(ctx context.Context, chunks []*models.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	rows := make([]sqlc.CreateChunkBatchParams, 0, len(chunks))
	for _, chunk := range chunks {
		rows = append(rows, sqlc.CreateChunkBatchParams{
			FileID:      UUIDToPgtype(chunk.FileID),
			Ordinal:     int32(chunk.Ordinal),
			StartLine:   int32(chunk.StartLine),
			EndLine:     int32(chunk.EndLine),
			Content:     chunk.Content,
			ContentHash: chunk.ContentHash,
			TokenCount:  IntToPgtype(chunk.TokenCount),
		})
	}

	if _, err := rw.q.CreateChunkBatch(ctx, rows); err != nil {
		return fmt.Errorf("failed to batch create chunks: %w", err)
	}

	return nil
}

// GetChunkByID はチャンクを取得します
func (r *IndexRepositoryR) GetChunkByID(ctx context.Context, id uuid.UUID) (*models.Chunk, error) {
	chunk, err := r.q.GetChunk(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("chunk not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get chunk: %w", err)
	}

	return convertSQLCChunk(chunk), nil
}

// ListChunksByFile はファイル内のチャンクを ordinal 順に取得します
func (r *IndexRepositoryR) ListChunksByFile(ctx context.Context, fileID uuid.UUID) ([]*models.Chunk, error) {
	rows, err := r.q.ListChunksByFile(ctx, UUIDToPgtype(fileID))
	if err != nil {
		return nil, fmt.Errorf("failed to list chunks: %w", err)
	}

	chunks := make([]*models.Chunk, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, convertSQLCChunk(row))
	}

	return chunks, nil
}

// GetChunkContext は対象チャンクの前後コンテキストを取得します
func (r *IndexRepositoryR) GetChunkContext(ctx context.Context, chunkID uuid.UUID, beforeCount int, afterCount int) ([]*models.Chunk, error) {
	target, err := r.q.GetChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("chunk not found: %s", chunkID)
		}
		return nil, fmt.Errorf("failed to get target chunk: %w", err)
	}

	minOrdinal := target.Ordinal - int32(beforeCount)
	if minOrdinal < 0 {
		minOrdinal = 0
	}
	maxOrdinal := target.Ordinal + int32(afterCount)

	rows, err := r.q.ListChunksByOrdinalRange(ctx, sqlc.ListChunksByOrdinalRangeParams{
		FileID:    target.FileID,
		Ordinal:   minOrdinal,
		Ordinal_2: maxOrdinal,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get context chunks: %w", err)
	}

	chunks := make([]*models.Chunk, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, convertSQLCChunk(row))
	}

	return chunks, nil
}

// DeleteChunksByFileID はファイル配下のチャンクを削除します
func (rw *IndexRepositoryRW) DeleteChunksByFileID(ctx context.Context, fileID uuid.UUID) error {
	if err := rw.q.DeleteChunksByFile(ctx, UUIDToPgtype(fileID)); err != nil {
		return fmt.Errorf("failed to delete chunks by file: %w", err)
	}
	return nil
}

// === Embedding 操作 ===

// CreateEmbedding は単一 Embedding を作成します
func (rw *IndexRepositoryRW) CreateEmbedding(ctx context.Context, chunkID uuid.UUID, vector []float32, model string) error {
	_, err := rw.q.CreateEmbedding(ctx, sqlc.CreateEmbeddingParams{
		ChunkID: UUIDToPgtype(chunkID),
		Vector:  pgvector.NewVector(vector),
		Model:   model,
	})
	if err != nil {
		return fmt.Errorf("failed to create embedding: %w", err)
	}
	return nil
}

// BatchCreateEmbeddings は Embedding を一括登録します
func (rw *IndexRepositoryRW) BatchCreateEmbeddings(ctx context.Context, embeddings []*models.Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	rows := make([]sqlc.CreateEmbeddingBatchParams, 0, len(embeddings))
	for _, embedding := range embeddings {
		rows = append(rows, sqlc.CreateEmbeddingBatchParams{
			ChunkID: UUIDToPgtype(embedding.ChunkID),
			Vector:  pgvector.NewVector(embedding.Vector),
			Model:   embedding.Model,
		})
	}

	if _, err := rw.q.CreateEmbeddingBatch(ctx, rows); err != nil {
		return fmt.Errorf("failed to batch create embeddings: %w", err)
	}

	return nil
}

// SearchByProduct はプロダクト単位でベクトル検索を実行します
func (r *IndexRepositoryR) SearchByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*models.SearchResult, error) {
	rows, err := r.q.SearchChunksByProduct(ctx, sqlc.SearchChunksByProductParams{
		QueryVector: pgvector.NewVector(queryVector),
		ProductID:   UUIDToPgtype(productID),
		PathPrefix:  StringPtrToPgtext(filters.PathPrefix),
		ContentType: StringPtrToPgtext(filters.ContentType),
		RowLimit:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search by product: %w", err)
	}

	results := make([]*models.SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &models.SearchResult{
			FilePath:  row.Path,
			StartLine: int(row.StartLine),
			EndLine:   int(row.EndLine),
			Content:   row.Content,
			Score:     row.Score,
		})
	}

	return results, nil
}

// SearchBySource はソース単位でベクトル検索を実行します
func (r *IndexRepositoryR) SearchBySource(ctx context.Context, sourceID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*models.SearchResult, error) {
	rows, err := r.q.SearchChunksBySource(ctx, sqlc.SearchChunksBySourceParams{
		QueryVector: pgvector.NewVector(queryVector),
		SourceID:    UUIDToPgtype(sourceID),
		PathPrefix:  StringPtrToPgtext(filters.PathPrefix),
		ContentType: StringPtrToPgtext(filters.ContentType),
		RowLimit:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search by source: %w", err)
	}

	results := make([]*models.SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &models.SearchResult{
			FilePath:  row.Path,
			StartLine: int(row.StartLine),
			EndLine:   int(row.EndLine),
			Content:   row.Content,
			Score:     row.Score,
		})
	}

	return results, nil
}

// === Private helpers ===

func convertSQLCFile(row sqlc.File) *models.File {
	return &models.File{
		ID:          PgtypeToUUID(row.ID),
		SnapshotID:  PgtypeToUUID(row.SnapshotID),
		Path:        row.Path,
		Size:        row.Size,
		ContentType: row.ContentType,
		ContentHash: row.ContentHash,
		CreatedAt:   PgtypeToTime(row.CreatedAt),
	}
}

func convertSQLCChunk(row sqlc.Chunk) *models.Chunk {
	return &models.Chunk{
		ID:          PgtypeToUUID(row.ID),
		FileID:      PgtypeToUUID(row.FileID),
		Ordinal:     int(row.Ordinal),
		StartLine:   int(row.StartLine),
		EndLine:     int(row.EndLine),
		Content:     row.Content,
		ContentHash: row.ContentHash,
		TokenCount:  PgtypeToInt(row.TokenCount),
		CreatedAt:   PgtypeToTime(row.CreatedAt),
	}
}
