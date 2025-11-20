package search

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jinford/dev-rag/pkg/db"
	"github.com/jinford/dev-rag/pkg/indexer/embedder"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/repository"
	"github.com/jinford/dev-rag/pkg/sqlc"
)

const (
	defaultSearchLimit = 10
	maxSearchLimit     = 50
	maxContextWindow   = 3
)

// SearchParams は検索時に必要なパラメータを表します
type SearchParams struct {
	ProductID     *uuid.UUID
	SourceID      *uuid.UUID
	Query         string
	Limit         int
	PathPrefix    string
	ContentType   string
	ContextBefore int
	ContextAfter  int
}

// Result は検索結果と処理時間をまとめます
type Result struct {
	Chunks   []*models.SearchResult
	Duration time.Duration
}

// HierarchicalResult は階層情報を含む検索結果と処理時間をまとめます
type HierarchicalResult struct {
	Chunks   []*HierarchicalSearchResult
	Duration time.Duration
}

type searchExecutor func(ctx context.Context, vector []float32, limit int, filters repository.SearchFilter) ([]*models.SearchResult, error)

// Searcher はベクトル検索を実行します
type Searcher struct {
	indexRepo            *repository.IndexRepositoryR
	embedder             *embedder.Embedder
	hierarchicalSearcher *HierarchicalSearcher
	logger               *slog.Logger
}

// NewSearcher は検索用の構造体を生成します
func NewSearcher(database *db.DB, emb *embedder.Embedder) *Searcher {
	if database == nil {
		panic("search.NewSearcher: database is nil")
	}
	if emb == nil {
		panic("search.NewSearcher: embedder is nil")
	}

	queries := sqlc.New(database.Pool)
	indexRepo := repository.NewIndexRepositoryR(queries)
	logger := slog.Default()

	return &Searcher{
		indexRepo:            indexRepo,
		embedder:             emb,
		hierarchicalSearcher: NewHierarchicalSearcher(indexRepo, logger),
		logger:               logger,
	}
}

// SetLogger はカスタムロガーを設定します（nil の場合は無視）
func (s *Searcher) SetLogger(logger *slog.Logger) {
	if logger != nil {
		s.logger = logger
		s.hierarchicalSearcher.logger = logger
	}
}

// SearchByProduct はプロダクト単位でベクトル検索を実行します
func (s *Searcher) SearchByProduct(ctx context.Context, params SearchParams) (*Result, error) {
	if params.ProductID == nil {
		return nil, fmt.Errorf("productID is required")
	}

	exec := func(ctx context.Context, vector []float32, limit int, filters repository.SearchFilter) ([]*models.SearchResult, error) {
		results, err := s.indexRepo.SearchByProduct(ctx, *params.ProductID, vector, limit, filters)
		if err != nil {
			return nil, fmt.Errorf("failed to search by product: %w", err)
		}
		return results, nil
	}

	return s.search(ctx, "product", params, exec)
}

// SearchBySource はソース単位でベクトル検索を実行します
func (s *Searcher) SearchBySource(ctx context.Context, params SearchParams) (*Result, error) {
	if params.SourceID == nil {
		return nil, fmt.Errorf("sourceID is required")
	}

	exec := func(ctx context.Context, vector []float32, limit int, filters repository.SearchFilter) ([]*models.SearchResult, error) {
		results, err := s.indexRepo.SearchBySource(ctx, *params.SourceID, vector, limit, filters)
		if err != nil {
			return nil, fmt.Errorf("failed to search by source: %w", err)
		}
		return results, nil
	}

	return s.search(ctx, "source", params, exec)
}

func (s *Searcher) search(ctx context.Context, scope string, params SearchParams, exec searchExecutor) (*Result, error) {
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	before, err := normalizeContextWindow(params.ContextBefore)
	if err != nil {
		return nil, err
	}
	after, err := normalizeContextWindow(params.ContextAfter)
	if err != nil {
		return nil, err
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	} else if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	filters := repository.SearchFilter{
		PathPrefix:  normalizeOptionalString(params.PathPrefix),
		ContentType: normalizeOptionalString(params.ContentType),
	}

	start := time.Now()

	vector, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to build query embedding: %w", err)
	}

	results, err := exec(ctx, vector, limit, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to execute %s search: %w", scope, err)
	}

	if len(results) > 0 && (before > 0 || after > 0) {
		if err := s.populateContext(ctx, results, before, after); err != nil {
			return nil, fmt.Errorf("failed to populate %s context: %w", scope, err)
		}
	}

	return &Result{
		Chunks:   results,
		Duration: time.Since(start),
	}, nil
}

func (s *Searcher) populateContext(ctx context.Context, chunks []*models.SearchResult, before, after int) error {
	for _, chunk := range chunks {
		contextChunks, err := s.indexRepo.GetChunkContext(ctx, chunk.ChunkID, before, after)
		if err != nil {
			return fmt.Errorf("failed to get chunk context: %w", err)
		}
		if len(contextChunks) == 0 {
			s.logger.Warn("context chunks not found", "chunkID", chunk.ChunkID)
			continue
		}

		var targetOrdinal *int
		for _, ctxChunk := range contextChunks {
			if ctxChunk.ID == chunk.ChunkID {
				ord := ctxChunk.Ordinal
				targetOrdinal = &ord
				break
			}
		}
		if targetOrdinal == nil {
			s.logger.Warn("target chunk missing in context result", "chunkID", chunk.ChunkID)
			continue
		}

		var prevParts []string
		var nextParts []string
		for _, ctxChunk := range contextChunks {
			if ctxChunk.ID == chunk.ChunkID {
				continue
			}
			if ctxChunk.Ordinal < *targetOrdinal {
				prevParts = append(prevParts, ctxChunk.Content)
			} else if ctxChunk.Ordinal > *targetOrdinal {
				nextParts = append(nextParts, ctxChunk.Content)
			}
		}

		if len(prevParts) > 0 {
			prev := strings.Join(prevParts, "\n")
			chunk.PrevContent = &prev
		}
		if len(nextParts) > 0 {
			next := strings.Join(nextParts, "\n")
			chunk.NextContent = &next
		}
	}

	return nil
}

func normalizeContextWindow(value int) (int, error) {
	if value < 0 {
		return 0, fmt.Errorf("context window must be >= 0")
	}
	if value > maxContextWindow {
		return maxContextWindow, nil
	}
	return value, nil
}

func normalizeOptionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// SearchByProductWithHierarchy はプロダクト単位でベクトル検索を実行し、階層情報を含めます
func (s *Searcher) SearchByProductWithHierarchy(ctx context.Context, params SearchParams, options HierarchicalSearchOptions) (*HierarchicalResult, error) {
	// 通常の検索を実行
	result, err := s.SearchByProduct(ctx, params)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	// 階層情報を追加
	enriched, err := s.hierarchicalSearcher.EnrichWithHierarchy(ctx, result.Chunks, options)
	if err != nil {
		return nil, fmt.Errorf("failed to enrich with hierarchy: %w", err)
	}

	return &HierarchicalResult{
		Chunks:   enriched,
		Duration: result.Duration + time.Since(start),
	}, nil
}

// SearchBySourceWithHierarchy はソース単位でベクトル検索を実行し、階層情報を含めます
func (s *Searcher) SearchBySourceWithHierarchy(ctx context.Context, params SearchParams, options HierarchicalSearchOptions) (*HierarchicalResult, error) {
	// 通常の検索を実行
	result, err := s.SearchBySource(ctx, params)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	// 階層情報を追加
	enriched, err := s.hierarchicalSearcher.EnrichWithHierarchy(ctx, result.Chunks, options)
	if err != nil {
		return nil, fmt.Errorf("failed to enrich with hierarchy: %w", err)
	}

	return &HierarchicalResult{
		Chunks:   enriched,
		Duration: result.Duration + time.Since(start),
	}, nil
}

// GetHierarchicalSearcher は HierarchicalSearcher のインスタンスを返します
// 直接階層検索機能にアクセスしたい場合に使用します
func (s *Searcher) GetHierarchicalSearcher() *HierarchicalSearcher {
	return s.hierarchicalSearcher
}
