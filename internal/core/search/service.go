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

// SearchSummaries はクエリに基づいて要約検索を実行する
func (s *SearchService) SearchSummaries(ctx context.Context, params SummarySearchParams) ([]*SummarySearchResult, error) {
	// バリデーション
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if params.SnapshotID == uuid.Nil {
		return nil, fmt.Errorf("snapshotID is required")
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
	filter := SummarySearchFilter{}
	if params.Filter != nil {
		filter = *params.Filter
	}

	// 要約検索を実行
	results, err := s.repo.SearchSummariesBySnapshot(ctx, params.SnapshotID, queryVector, limit, filter)
	if err != nil {
		return nil, fmt.Errorf("summary search failed: %w", err)
	}

	return results, nil
}

// HybridSearch はチャンク検索と要約検索の両方を実行してマージする
func (s *SearchService) HybridSearch(ctx context.Context, params HybridSearchParams) (*HybridSearchResult, error) {
	// バリデーション
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	// ProductIDとSnapshotIDは排他的（どちらか一方のみ指定可能）
	if params.ProductID != nil && params.SnapshotID != uuid.Nil {
		return nil, fmt.Errorf("productID and snapshotID are mutually exclusive")
	}
	if params.ProductID == nil && params.SnapshotID == uuid.Nil {
		return nil, fmt.Errorf("either productID or snapshotID is required")
	}

	// クエリをEmbeddingに変換
	queryVector, err := s.embedder.Embed(ctx, params.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// デフォルトのLimit設定
	chunkLimit := params.ChunkLimit
	if chunkLimit <= 0 {
		chunkLimit = 10
	}
	summaryLimit := params.SummaryLimit
	if summaryLimit <= 0 {
		summaryLimit = 5
	}

	// フィルタの準備
	chunkFilter := SearchFilter{}
	if params.ChunkFilter != nil {
		chunkFilter = *params.ChunkFilter
	}
	summaryFilter := SummarySearchFilter{}
	if params.SummaryFilter != nil {
		summaryFilter = *params.SummaryFilter
	}

	// チャンク検索と要約検索を並行実行
	type chunkResult struct {
		chunks []*SearchResult
		err    error
	}
	type summaryResult struct {
		summaries []*SummarySearchResult
		err       error
	}

	chunkCh := make(chan chunkResult, 1)
	summaryCh := make(chan summaryResult, 1)

	// ProductIDが指定されている場合はプロダクト横断検索、そうでなければスナップショット検索
	if params.ProductID != nil {
		go func() {
			chunks, err := s.repo.SearchChunksByProduct(ctx, *params.ProductID, queryVector, chunkLimit, chunkFilter)
			chunkCh <- chunkResult{chunks: chunks, err: err}
		}()

		go func() {
			summaries, err := s.repo.SearchSummariesByProduct(ctx, *params.ProductID, queryVector, summaryLimit, summaryFilter)
			summaryCh <- summaryResult{summaries: summaries, err: err}
		}()
	} else {
		go func() {
			chunks, err := s.repo.SearchChunksBySnapshot(ctx, params.SnapshotID, queryVector, chunkLimit, chunkFilter)
			chunkCh <- chunkResult{chunks: chunks, err: err}
		}()

		go func() {
			summaries, err := s.repo.SearchSummariesBySnapshot(ctx, params.SnapshotID, queryVector, summaryLimit, summaryFilter)
			summaryCh <- summaryResult{summaries: summaries, err: err}
		}()
	}

	// 結果を待つ
	chunkRes := <-chunkCh
	summaryRes := <-summaryCh

	if chunkRes.err != nil {
		return nil, fmt.Errorf("chunk search failed: %w", chunkRes.err)
	}
	if summaryRes.err != nil {
		return nil, fmt.Errorf("summary search failed: %w", summaryRes.err)
	}

	return &HybridSearchResult{
		Chunks:    chunkRes.chunks,
		Summaries: summaryRes.summaries,
	}, nil
}
