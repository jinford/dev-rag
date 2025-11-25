package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	pgvector "github.com/pgvector/pgvector-go"

	"github.com/jinford/dev-rag/internal/core/search"
	"github.com/jinford/dev-rag/internal/infra/postgres/sqlc"
)

// SearchRepository は core/search.Repository を実装する PostgreSQL リポジトリ。
type SearchRepository struct {
	q sqlc.Querier
}

// NewSearchRepository は新しい SearchRepository を返す。
func NewSearchRepository(q sqlc.Querier) *SearchRepository {
	return &SearchRepository{q: q}
}

var _ search.Repository = (*SearchRepository)(nil)

func (r *SearchRepository) SearchByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters search.SearchFilter) ([]*search.SearchResult, error) {
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

	results := make([]*search.SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &search.SearchResult{
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

func (r *SearchRepository) SearchBySource(ctx context.Context, sourceID uuid.UUID, queryVector []float32, limit int, filters search.SearchFilter) ([]*search.SearchResult, error) {
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

	results := make([]*search.SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &search.SearchResult{
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

func (r *SearchRepository) SearchChunksBySnapshot(ctx context.Context, snapshotID uuid.UUID, queryVector []float32, limit int, filters search.SearchFilter) ([]*search.SearchResult, error) {
	rows, err := r.q.SearchChunksBySnapshot(ctx, sqlc.SearchChunksBySnapshotParams{
		QueryVector: pgvector.NewVector(queryVector),
		SnapshotID:  UUIDToPgtype(snapshotID),
		PathPrefix:  StringPtrToPgtext(filters.PathPrefix),
		ContentType: StringPtrToPgtext(filters.ContentType),
		LimitVal:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search chunks by snapshot: %w", err)
	}

	results := make([]*search.SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &search.SearchResult{
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

func (r *SearchRepository) GetChunkContext(ctx context.Context, chunkID uuid.UUID, beforeCount int, afterCount int) ([]*search.ChunkContext, error) {
	target, err := r.q.GetChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
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

	chunks := make([]*search.ChunkContext, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, convertSearchChunk(row))
	}
	return chunks, nil
}

func (r *SearchRepository) GetParentChunk(ctx context.Context, chunkID uuid.UUID) (*search.ChunkContext, error) {
	row, err := r.q.GetParentChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		return nil, fmt.Errorf("failed to get parent chunk: %w", err)
	}
	return convertSearchChunk(row), nil
}

func (r *SearchRepository) GetChildChunks(ctx context.Context, chunkID uuid.UUID) ([]*search.ChunkContext, error) {
	rows, err := r.q.GetChildChunks(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		return nil, fmt.Errorf("failed to get child chunks: %w", err)
	}

	chunks := make([]*search.ChunkContext, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, convertSearchChunk(row))
	}
	return chunks, nil
}

func (r *SearchRepository) GetChunkTree(ctx context.Context, rootID uuid.UUID, maxDepth int) ([]*search.ChunkContext, error) {
	result := make([]*search.ChunkContext, 0)
	visited := make(map[uuid.UUID]bool)

	var traverse func(parentID uuid.UUID, depth int) error
	traverse = func(parentID uuid.UUID, depth int) error {
		if depth > maxDepth {
			return nil
		}
		if visited[parentID] {
			return nil
		}
		visited[parentID] = true

		parent, err := r.q.GetChunk(ctx, UUIDToPgtype(parentID))
		if err != nil {
			return fmt.Errorf("failed to get chunk: %w", err)
		}
		result = append(result, convertSearchChunk(parent))

		children, err := r.q.GetChildChunks(ctx, UUIDToPgtype(parentID))
		if err != nil {
			return fmt.Errorf("failed to get child chunks: %w", err)
		}

		for _, child := range children {
			childID := PgtypeToUUID(child.ID)
			if err := traverse(childID, depth+1); err != nil {
				return err
			}
		}
		return nil
	}

	if err := traverse(rootID, 1); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *SearchRepository) SearchChunksByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters search.SearchFilter) ([]*search.SearchResult, error) {
	return r.SearchByProduct(ctx, productID, queryVector, limit, filters)
}

func (r *SearchRepository) SearchSummariesBySnapshot(ctx context.Context, snapshotID uuid.UUID, queryVector []float32, limit int, filters search.SummarySearchFilter) ([]*search.SummarySearchResult, error) {
	// summary_typesの準備
	summaryTypes := filters.SummaryTypes
	if summaryTypes == nil {
		summaryTypes = []string{}
	}

	// path_prefixの準備
	pathPrefix := ""
	if filters.PathPrefix != nil {
		pathPrefix = *filters.PathPrefix
	}

	rows, err := r.q.SearchSummariesBySnapshot(ctx, sqlc.SearchSummariesBySnapshotParams{
		QueryVector:  pgvector.NewVector(queryVector),
		SnapshotID:   UUIDToPgtype(snapshotID),
		SummaryTypes: summaryTypes,
		PathPrefix:   pathPrefix,
		LimitVal:     int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search summaries by snapshot: %w", err)
	}

	results := make([]*search.SummarySearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &search.SummarySearchResult{
			SummaryID:   PgtypeToUUID(row.ID),
			SummaryType: row.SummaryType,
			TargetPath:  row.TargetPath,
			ArchType:    PgtextToStringPtr(row.ArchType),
			Content:     row.Content,
			Score:       float64(row.Score),
		})
	}
	return results, nil
}

func (r *SearchRepository) SearchSummariesByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters search.SummarySearchFilter) ([]*search.SummarySearchResult, error) {
	// summary_typesの準備
	summaryTypes := filters.SummaryTypes
	if summaryTypes == nil {
		summaryTypes = []string{}
	}

	rows, err := r.q.SearchSummariesByProduct(ctx, sqlc.SearchSummariesByProductParams{
		QueryVector:  pgvector.NewVector(queryVector),
		ProductID:    UUIDToPgtype(productID),
		SummaryTypes: summaryTypes,
		PathPrefix:   StringPtrToPgtext(filters.PathPrefix),
		LimitVal:     int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search summaries by product: %w", err)
	}

	results := make([]*search.SummarySearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &search.SummarySearchResult{
			SummaryID:   PgtypeToUUID(row.ID),
			SummaryType: row.SummaryType,
			TargetPath:  row.TargetPath,
			ArchType:    PgtextToStringPtr(row.ArchType),
			Content:     row.Content,
			Score:       float64(row.Score),
		})
	}
	return results, nil
}

// convertSearchChunk は searchsqlc.Chunk を search.ChunkContext に変換する。
func convertSearchChunk(row sqlc.Chunk) *search.ChunkContext {
	return &search.ChunkContext{
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
