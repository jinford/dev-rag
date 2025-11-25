package domain

import (
	"context"
)

// EmbedderMetadata は Embedder のメタデータを表します
type EmbedderMetadata struct {
	ModelName string
	Dimension int
}

// Embedder はテキストをベクトルに変換するインターフェース
type Embedder interface {
	// Embed は単一テキストのEmbeddingを生成します
	Embed(ctx context.Context, text string) ([]float32, error)

	// BatchEmbed はバッチでEmbeddingを生成します
	BatchEmbed(ctx context.Context, texts []string) ([][]float32, error)

	// GetModelName はモデル名を取得します
	GetModelName() string

	// GetDimension はベクトル次元数を取得します
	GetDimension() int

	// GetMetadata はモデル情報を取得します
	GetMetadata() (*EmbedderMetadata, error)
}
