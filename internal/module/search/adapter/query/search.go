package query

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	searchdomain "github.com/jinford/dev-rag/internal/module/search/domain"
)

// SearchOptions は検索オプションを表します
type SearchOptions struct {
	// 階層検索オプション
	IncludeParent   bool `json:"includeParent,omitempty"`   // 親チャンクを含める
	IncludeChildren bool `json:"includeChildren,omitempty"` // 子チャンクを含める
	MaxDepth        int  `json:"maxDepth,omitempty"`        // 階層の最大深さ（デフォルト: 1）
}

// EnhancedSearchResult は階層情報を含む検索結果を表します
type EnhancedSearchResult struct {
	*searchdomain.SearchResult

	// 階層情報
	ParentChunk *searchdomain.ChunkContext   `json:"parentChunk,omitempty"`
	ChildChunks []*searchdomain.ChunkContext `json:"childChunks,omitempty"`
}

// Querier は検索とコンテキスト構築を行います
type Querier struct {
	repo searchdomain.ChunkContextReader
}

// NewQuerier は新しいQuerierを作成します
func NewQuerier(repo searchdomain.ChunkContextReader) *Querier {
	if repo == nil {
		panic("query.NewQuerier: repo is nil")
	}

	return &Querier{
		repo: repo,
	}
}

// SearchWithHierarchy は階層情報を含む検索を実行します
//
// 使用例:
//
//	querier := query.NewQuerier(indexRepo)
//	options := &query.SearchOptions{
//	    IncludeParent:   true,
//	    IncludeChildren: true,
//	    MaxDepth:        1,
//	}
//
//	// 既存の検索結果から階層情報を追加
//	baseResults, err := searcher.SearchByProduct(ctx, searchParams)
//	if err != nil {
//	    return err
//	}
//
//	enhanced, err := querier.EnrichSearchResultsWithHierarchy(ctx, baseResults.Chunks, options)
//	if err != nil {
//	    return err
//	}
func (q *Querier) EnrichSearchResultsWithHierarchy(
	ctx context.Context,
	baseResults []*searchdomain.SearchResult,
	options *SearchOptions,
) ([]*EnhancedSearchResult, error) {
	// オプションがない、または階層検索が不要な場合は基本結果を返す
	if options == nil || (!options.IncludeParent && !options.IncludeChildren) {
		enhanced := make([]*EnhancedSearchResult, len(baseResults))
		for i, r := range baseResults {
			enhanced[i] = &EnhancedSearchResult{SearchResult: r}
		}
		return enhanced, nil
	}

	// 階層情報を付与
	enhanced := make([]*EnhancedSearchResult, len(baseResults))
	for i, result := range baseResults {
		enhanced[i] = &EnhancedSearchResult{SearchResult: result}

		// 親チャンクを取得
		if options.IncludeParent {
			parent, err := q.repo.GetParentChunk(ctx, result.ChunkID)
			if err != nil {
				// エラーが発生しても処理を続行（ログに記録して nil を設定）
				enhanced[i].ParentChunk = nil
			} else {
				enhanced[i].ParentChunk = parent
			}
		}

		// 子チャンクを取得
		if options.IncludeChildren {
			children, err := q.repo.GetChildChunks(ctx, result.ChunkID)
			if err != nil {
				// エラーが発生しても処理を続行（空スライスを設定）
				enhanced[i].ChildChunks = []*searchdomain.ChunkContext{}
			} else {
				enhanced[i].ChildChunks = children
			}
		}
	}

	return enhanced, nil
}

// GetParentChunk は指定されたチャンクの親チャンクを取得します
func (q *Querier) GetParentChunk(ctx context.Context, chunkID interface{}) (*searchdomain.ChunkContext, error) {
	// chunkIDをuuid.UUIDに変換
	var id uuid.UUID
	switch v := chunkID.(type) {
	case uuid.UUID:
		id = v
	case string:
		parsed, err := uuid.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID string: %w", err)
		}
		id = parsed
	default:
		return nil, fmt.Errorf("unsupported chunkID type: %T", chunkID)
	}

	// Repository層から取得
	parent, err := q.repo.GetParentChunk(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent chunk: %w", err)
	}

	return parent, nil
}

// GetChildChunks は指定されたチャンクの子チャンクを取得します
func (q *Querier) GetChildChunks(ctx context.Context, chunkID interface{}) ([]*searchdomain.ChunkContext, error) {
	// chunkIDをuuid.UUIDに変換
	var id uuid.UUID
	switch v := chunkID.(type) {
	case uuid.UUID:
		id = v
	case string:
		parsed, err := uuid.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID string: %w", err)
		}
		id = parsed
	default:
		return nil, fmt.Errorf("unsupported chunkID type: %T", chunkID)
	}

	// Repository層から取得
	children, err := q.repo.GetChildChunks(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get child chunks: %w", err)
	}

	return children, nil
}
