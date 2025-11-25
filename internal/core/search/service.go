package search

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Embedder はテキストのEmbedding生成インターフェース
type Embedder interface {
	// Embed は単一テキストのEmbeddingを生成する
	Embed(ctx context.Context, text string) ([]float32, error)
}

// SearchService は検索のビジネスロジックを提供する
type SearchService struct {
	repo     Repository
	embedder Embedder
}

// NewSearchService は新しいSearchServiceを作成する
func NewSearchService(repo Repository, embedder Embedder) *SearchService {
	return &SearchService{
		repo:     repo,
		embedder: embedder,
	}
}

// SearchParams は検索パラメータを表す
type SearchParams struct {
	ProductID *uuid.UUID
	SourceID  *uuid.UUID
	Query     string
	Limit     int
	Filter    *SearchFilter
}

// Search はクエリに基づいてベクトル検索を実行する
func (s *SearchService) Search(ctx context.Context, params SearchParams) ([]*SearchResult, error) {
	// バリデーション
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if params.ProductID == nil && params.SourceID == nil {
		return nil, fmt.Errorf("either productID or sourceID is required")
	}

	// クエリをEmbeddingに変換
	queryVector, err := s.embedder.Embed(ctx, params.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// デフォルトのLimit設定
	limit := params.Limit
	if limit <= 0 {
		limit = 10
	}

	// フィルタの準備
	filter := SearchFilter{}
	if params.Filter != nil {
		filter = *params.Filter
	}

	// ProductID または SourceID に基づいて検索
	var results []*SearchResult
	if params.ProductID != nil {
		results, err = s.repo.SearchByProduct(ctx, *params.ProductID, queryVector, limit, filter)
	} else {
		results, err = s.repo.SearchBySource(ctx, *params.SourceID, queryVector, limit, filter)
	}

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return results, nil
}

// GetChunkContext は指定されたチャンクの前後コンテキストを取得する
func (s *SearchService) GetChunkContext(ctx context.Context, chunkID uuid.UUID, beforeCount, afterCount int) ([]*ChunkContext, error) {
	if chunkID == uuid.Nil {
		return nil, fmt.Errorf("chunkID is required")
	}

	contexts, err := s.repo.GetChunkContext(ctx, chunkID, beforeCount, afterCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk context: %w", err)
	}

	return contexts, nil
}

// GetChunkTree は指定されたルートチャンクから階層ツリーを取得する
func (s *SearchService) GetChunkTree(ctx context.Context, rootID uuid.UUID, maxDepth int) ([]*ChunkContext, error) {
	if rootID == uuid.Nil {
		return nil, fmt.Errorf("rootID is required")
	}

	tree, err := s.repo.GetChunkTree(ctx, rootID, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk tree: %w", err)
	}

	return tree, nil
}
