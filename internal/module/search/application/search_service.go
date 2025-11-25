package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	searchdomain "github.com/jinford/dev-rag/internal/module/search/domain"
)

const (
	defaultSearchLimit = 10
	maxSearchLimit     = 50
	maxContextWindow   = 3
)

// SearchService は検索のユースケースを提供します
type SearchService struct {
	searcher searchdomain.Searcher
	log      *slog.Logger
}

// NewSearchService は新しいSearchServiceを作成します
func NewSearchService(searcher searchdomain.Searcher, log *slog.Logger) *SearchService {
	return &SearchService{
		searcher: searcher,
		log:      log,
	}
}

// SearchChunksParams はチャンク検索のパラメータ
type SearchChunksParams struct {
	ProductID     *uuid.UUID
	SourceID      *uuid.UUID
	Query         string
	Limit         int
	PathPrefix    string
	ContentType   string
	ContextBefore int
	ContextAfter  int
}

// SearchChunksResult はチャンク検索の結果
type SearchChunksResult struct {
	Chunks []*searchdomain.SearchResult
}

// SearchChunks はチャンクを検索します
func (s *SearchService) SearchChunks(ctx context.Context, params SearchChunksParams) (*SearchChunksResult, error) {
	// バリデーション
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// ProductIDまたはSourceIDのいずれかが必須
	if params.ProductID == nil && params.SourceID == nil {
		return nil, fmt.Errorf("either productID or sourceID is required")
	}

	// Limit の正規化
	limit := params.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	} else if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	// ContextBefore/After の正規化
	contextBefore, err := normalizeContextWindow(params.ContextBefore)
	if err != nil {
		return nil, fmt.Errorf("invalid contextBefore: %w", err)
	}
	contextAfter, err := normalizeContextWindow(params.ContextAfter)
	if err != nil {
		return nil, fmt.Errorf("invalid contextAfter: %w", err)
	}

	// PathPrefix と ContentType の正規化
	pathPrefix := strings.TrimSpace(params.PathPrefix)
	contentType := strings.TrimSpace(params.ContentType)

	s.log.Info("Starting chunk search",
		"query", query,
		"productID", params.ProductID,
		"sourceID", params.SourceID,
		"limit", limit,
	)

	// domain.SearchParams に変換
	searchParams := searchdomain.SearchParams{
		Query:         query,
		Limit:         limit,
		ProductID:     params.ProductID,
		SourceID:      params.SourceID,
		PathPrefix:    pathPrefix,
		ContentType:   contentType,
		ContextBefore: contextBefore,
		ContextAfter:  contextAfter,
	}

	// 検索を実行
	result, err := s.searcher.Search(ctx, searchParams)
	if err != nil {
		s.log.Error("Chunk search failed",
			"query", query,
			"error", err,
		)
		return nil, fmt.Errorf("failed to search chunks: %w", err)
	}

	s.log.Info("Chunk search completed",
		"query", query,
		"results", len(result.Results),
	)

	return &SearchChunksResult{
		Chunks: result.Results,
	}, nil
}

// normalizeContextWindow は context window の値を正規化します
func normalizeContextWindow(value int) (int, error) {
	if value < 0 {
		return 0, fmt.Errorf("context window must be >= 0")
	}
	if value > maxContextWindow {
		return maxContextWindow, nil
	}
	return value, nil
}
