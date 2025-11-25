package indexing

import "context"

// Embedder はテキストをベクトル表現に変換するインターフェース
type Embedder interface {
	// Embed は単一テキストのEmbeddingを生成する
	Embed(ctx context.Context, text string) ([]float32, error)

	// BatchEmbed はバッチでEmbeddingを生成する
	BatchEmbed(ctx context.Context, texts []string) ([][]float32, error)

	// ModelName はモデル名を返す
	ModelName() string

	// Dimension はEmbeddingベクトルの次元数を返す
	Dimension() int
}

// Metadata は Embedder のメタデータを表す
type Metadata struct {
	ModelName string
	Dimension int
}
