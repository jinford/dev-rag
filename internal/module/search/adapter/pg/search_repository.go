package pg

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jinford/dev-rag/internal/module/search/adapter/pg/sqlc"
	"github.com/jinford/dev-rag/internal/module/search/domain"

	pgvector "github.com/pgvector/pgvector-go"
)

// SearchRepository はベクトル検索の永続化アダプターです
type SearchRepository struct {
	q sqlc.Querier
}

// NewSearchRepository は新しい検索リポジトリを作成します
func NewSearchRepository(q sqlc.Querier) domain.SearchRepository {
	return &SearchRepository{q: q}
}

// Ensure SearchRepository implements all interfaces
var _ domain.SearchReader = (*SearchRepository)(nil)
var _ domain.ChunkContextReader = (*SearchRepository)(nil)
var _ domain.SearchRepository = (*SearchRepository)(nil)

// SearchByProduct はプロダクト単位でベクトル検索を実行します
func (r *SearchRepository) SearchByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters domain.SearchFilter) ([]*domain.SearchResult, error) {
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

	results := make([]*domain.SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &domain.SearchResult{
			ChunkID:   PgtypeToUUID(row.ChunkID),
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
func (r *SearchRepository) SearchBySource(ctx context.Context, sourceID uuid.UUID, queryVector []float32, limit int, filters domain.SearchFilter) ([]*domain.SearchResult, error) {
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

	results := make([]*domain.SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &domain.SearchResult{
			ChunkID:   PgtypeToUUID(row.ChunkID),
			FilePath:  row.Path,
			StartLine: int(row.StartLine),
			EndLine:   int(row.EndLine),
			Content:   row.Content,
			Score:     row.Score,
		})
	}

	return results, nil
}

// GetChunkContext は対象チャンクの前後コンテキストを取得します
func (r *SearchRepository) GetChunkContext(ctx context.Context, chunkID uuid.UUID, beforeCount int, afterCount int) ([]*domain.ChunkContext, error) {
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

	chunks := make([]*domain.ChunkContext, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, convertToChunkContext(row))
	}

	return chunks, nil
}

// GetParentChunk は親チャンクを取得します
func (r *SearchRepository) GetParentChunk(ctx context.Context, chunkID uuid.UUID) (*domain.ChunkContext, error) {
	chunk, err := r.q.GetParentChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // 親がいない場合
		}
		return nil, fmt.Errorf("failed to get parent chunk: %w", err)
	}

	return convertToChunkContext(chunk), nil
}

// GetChildChunks は子チャンクを取得します
func (r *SearchRepository) GetChildChunks(ctx context.Context, chunkID uuid.UUID) ([]*domain.ChunkContext, error) {
	rows, err := r.q.GetChildChunks(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		return nil, fmt.Errorf("failed to get child chunks: %w", err)
	}

	chunks := make([]*domain.ChunkContext, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, convertToChunkContext(row))
	}

	return chunks, nil
}

// GetChunkTree はルートチャンクから階層ツリーを取得します
func (r *SearchRepository) GetChunkTree(ctx context.Context, rootID uuid.UUID, maxDepth int) ([]*domain.ChunkContext, error) {
	result := make([]*domain.ChunkContext, 0)
	visited := make(map[uuid.UUID]bool)

	var traverse func(parentID uuid.UUID, depth int) error
	traverse = func(parentID uuid.UUID, depth int) error {
		if depth > maxDepth {
			return nil
		}
		if visited[parentID] {
			return nil // 循環参照を防止
		}
		visited[parentID] = true

		// 親チャンクを取得
		parent, err := r.q.GetChunk(ctx, UUIDToPgtype(parentID))
		if err != nil {
			return err
		}
		result = append(result, convertToChunkContext(parent))

		// 子チャンクを取得
		children, err := r.q.GetChildChunks(ctx, UUIDToPgtype(parentID))
		if err != nil {
			return err
		}

		// 再帰的に子をたどる
		for _, child := range children {
			childID := PgtypeToUUID(child.ID)
			if err := traverse(childID, depth+1); err != nil {
				return err
			}
		}

		return nil
	}

	if err := traverse(rootID, 1); err != nil {
		return nil, fmt.Errorf("failed to get chunk tree: %w", err)
	}

	return result, nil
}

// convertToChunkContext は sqlc.Chunk を domain.ChunkContext に変換します
func convertToChunkContext(row sqlc.Chunk) *domain.ChunkContext {
	return &domain.ChunkContext{
		ID:         PgtypeToUUID(row.ID),
		FileID:     PgtypeToUUID(row.FileID),
		Ordinal:    int(row.Ordinal),
		StartLine:  int(row.StartLine),
		EndLine:    int(row.EndLine),
		Content:    row.Content,
		CreatedAt:  PgtypeToTime(row.CreatedAt),
		Type:       PgtextToStringPtr(row.ChunkType),
		Name:       PgtextToStringPtr(row.ChunkName),
		ParentName: PgtextToStringPtr(row.ParentName),
		Level:      int(row.Level),
	}
}
