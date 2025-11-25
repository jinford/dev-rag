package search

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	searchdomain "github.com/jinford/dev-rag/internal/module/search/domain"
)

type searchExecutor func(ctx context.Context, vector []float32, limit int, filters searchdomain.SearchFilter) ([]*searchdomain.SearchResult, error)

// searcher はベクトル検索を実行します（小文字で domain.Searcher の実装を隠蔽）
type searcher struct {
	repo                 searchdomain.SearchRepository
	embedder             searchdomain.Embedder
	hierarchicalSearcher *HierarchicalSearcher
	logger               *slog.Logger
}

// NewSearcher は検索用の構造体を生成します
func NewSearcher(repo searchdomain.SearchRepository, embedder searchdomain.Embedder) searchdomain.Searcher {
	if repo == nil {
		panic("search.NewSearcher: repository is nil")
	}
	if embedder == nil {
		panic("search.NewSearcher: embedder is nil")
	}

	logger := slog.Default()

	return &searcher{
		repo:                 repo,
		embedder:             embedder,
		hierarchicalSearcher: NewHierarchicalSearcher(repo, logger),
		logger:               logger,
	}
}

// SetLogger はカスタムロガーを設定します（nil の場合は無視）
func (s *searcher) SetLogger(logger *slog.Logger) {
	if logger != nil {
		s.logger = logger
		s.hierarchicalSearcher.logger = logger
	}
}

// Search は domain.Searcher の実装です
func (s *searcher) Search(ctx context.Context, params searchdomain.SearchParams) (*searchdomain.SearchResponse, error) {
	// ProductID または SourceID のいずれかが必須
	if params.ProductID == nil && params.SourceID == nil {
		return nil, fmt.Errorf("either productID or sourceID is required")
	}

	// クエリの検証（application で正規化済みだが念のため）
	if strings.TrimSpace(params.Query) == "" {
		return nil, fmt.Errorf("query is required")
	}

	// フィルタの構築
	filters := searchdomain.SearchFilter{
		PathPrefix:  normalizeOptionalString(params.PathPrefix),
		ContentType: normalizeOptionalString(params.ContentType),
	}

	// クエリのベクトル化
	vector, err := s.embedder.Embed(ctx, params.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to build query embedding: %w", err)
	}

	// 検索実行（ProductID か SourceID かで分岐）
	var results []*searchdomain.SearchResult
	if params.ProductID != nil {
		results, err = s.repo.SearchByProduct(ctx, *params.ProductID, vector, params.Limit, filters)
		if err != nil {
			return nil, fmt.Errorf("failed to search by product: %w", err)
		}
	} else {
		results, err = s.repo.SearchBySource(ctx, *params.SourceID, vector, params.Limit, filters)
		if err != nil {
			return nil, fmt.Errorf("failed to search by source: %w", err)
		}
	}

	// コンテキストチャンクの取得
	if len(results) > 0 && (params.ContextBefore > 0 || params.ContextAfter > 0) {
		if err := s.populateContext(ctx, results, params.ContextBefore, params.ContextAfter); err != nil {
			return nil, fmt.Errorf("failed to populate context: %w", err)
		}
	}

	return &searchdomain.SearchResponse{
		Results: results,
	}, nil
}

func (s *searcher) populateContext(ctx context.Context, chunks []*searchdomain.SearchResult, before, after int) error {
	for _, chunk := range chunks {
		contextChunks, err := s.repo.GetChunkContext(ctx, chunk.ChunkID, before, after)
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

func normalizeOptionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// GetHierarchicalSearcher は HierarchicalSearcher のインスタンスを返します
// 直接階層検索機能にアクセスしたい場合に使用します（将来の拡張用）
func (s *searcher) GetHierarchicalSearcher() *HierarchicalSearcher {
	return s.hierarchicalSearcher
}
