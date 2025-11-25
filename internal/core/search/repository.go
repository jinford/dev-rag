package search

import (
	"context"

	"github.com/google/uuid"
)

// Repository は検索関連の全データアクセスを統合するインターフェース
type Repository interface {
	// SearchByProduct はプロダクト内でベクトル検索を実行する
	SearchByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*SearchResult, error)

	// SearchBySource はソース内でベクトル検索を実行する
	SearchBySource(ctx context.Context, sourceID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*SearchResult, error)

	// SearchChunksBySnapshot はスナップショット内でチャンク検索を実行する
	SearchChunksBySnapshot(ctx context.Context, snapshotID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*SearchResult, error)

	// SearchChunksByProduct はプロダクト横断でチャンク検索を実行する（HybridSearch用）
	SearchChunksByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*SearchResult, error)

	// SearchSummariesBySnapshot はスナップショット内で要約検索を実行する
	SearchSummariesBySnapshot(ctx context.Context, snapshotID uuid.UUID, queryVector []float32, limit int, filters SummarySearchFilter) ([]*SummarySearchResult, error)

	// SearchSummariesByProduct はプロダクト横断で要約検索を実行する（HybridSearch用）
	SearchSummariesByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters SummarySearchFilter) ([]*SummarySearchResult, error)

	// GetChunkContext は対象チャンクの前後コンテキストを取得する
	GetChunkContext(ctx context.Context, chunkID uuid.UUID, beforeCount int, afterCount int) ([]*ChunkContext, error)

	// GetParentChunk は親チャンクを取得する（階層検索用）
	GetParentChunk(ctx context.Context, chunkID uuid.UUID) (*ChunkContext, error)

	// GetChildChunks は子チャンクを取得する（階層検索用）
	GetChildChunks(ctx context.Context, chunkID uuid.UUID) ([]*ChunkContext, error)

	// GetChunkTree はルートチャンクから階層ツリーを取得する
	GetChunkTree(ctx context.Context, rootID uuid.UUID, maxDepth int) ([]*ChunkContext, error)
}
