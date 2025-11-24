package domain

import "context"

// Embedder はテキストをベクトル表現に変換するインターフェース
type Embedder interface {
	// Embed はテキストからEmbeddingベクトルを生成する
	Embed(ctx context.Context, text string) ([]float32, error)

	// Dimension はEmbeddingベクトルの次元数を返す
	Dimension() int
}
