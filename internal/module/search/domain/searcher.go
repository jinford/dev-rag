package domain

import (
	"context"

	"github.com/google/uuid"
)

// Searcher はベクトル検索を実行するインターフェース
type Searcher interface {
	// Search は検索を実行します
	Search(ctx context.Context, params SearchParams) (*SearchResponse, error)
}

// SearchParams は検索パラメータ
type SearchParams struct {
	Query         string
	Limit         int
	ProductID     *uuid.UUID
	SourceID      *uuid.UUID
	PathPrefix    string
	ContentType   string
	ContextBefore int
	ContextAfter  int
}

// SearchResponse は検索レスポンス
type SearchResponse struct {
	Results []*SearchResult
}
