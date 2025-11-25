package domain

import (
	"context"

	"github.com/google/uuid"
)

// === Search Repository Port ===

// SearchRepository はベクトル検索の永続化ポートです
type SearchRepository interface {
	SearchReader
	ChunkContextReader
}

// SearchReader はベクトル検索の読み取り操作を定義します
type SearchReader interface {
	SearchByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*SearchResult, error)
	SearchBySource(ctx context.Context, sourceID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*SearchResult, error)
}

// ChunkContextReader はチャンクのコンテキスト情報を取得するポートです
type ChunkContextReader interface {
	// GetChunkContext は対象チャンクの前後コンテキストを取得します
	GetChunkContext(ctx context.Context, chunkID uuid.UUID, beforeCount int, afterCount int) ([]*ChunkContext, error)

	// GetParentChunk は親チャンクを取得します（階層検索用）
	GetParentChunk(ctx context.Context, chunkID uuid.UUID) (*ChunkContext, error)

	// GetChildChunks は子チャンクを取得します（階層検索用）
	GetChildChunks(ctx context.Context, chunkID uuid.UUID) ([]*ChunkContext, error)

	// GetChunkTree はルートチャンクから階層ツリーを取得します
	GetChunkTree(ctx context.Context, rootID uuid.UUID, maxDepth int) ([]*ChunkContext, error)
}

// Embedder はテキストをベクトルに変換するポートです
type Embedder interface {
	// Embed は単一テキストのEmbeddingを生成します
	Embed(ctx context.Context, text string) ([]float32, error)
}
