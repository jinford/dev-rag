package embedder

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Embedder はテキストをベクトルに変換します
type Embedder struct {
	client    openai.Client
	model     string
	dimension int
}

// NewEmbedder は新しいEmbedderを作成します
func NewEmbedder(apiKey, model string, dimension int) *Embedder {
	return &Embedder{
		client: openai.NewClient(
			option.WithAPIKey(apiKey),
		),
		model:     model,
		dimension: dimension,
	}
}

// Embed は単一テキストのEmbeddingを生成します
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// バッチ処理を使用
	embeddings, err := e.BatchEmbed(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings generated")
	}

	return embeddings[0], nil
}

// BatchEmbed はバッチでEmbeddingを生成します（最大100件）
func (e *Embedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	if len(texts) > 100 {
		return nil, fmt.Errorf("batch size exceeds maximum of 100")
	}

	// リクエストパラメータを作成
	params := openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(e.model),
	}

	// Input を設定（単一または配列）
	if len(texts) == 1 {
		params.Input = openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(texts[0]),
		}
	} else {
		params.Input = openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: texts,
		}
	}

	// dimensionパラメータを追加（text-embedding-3-smallなどで有効）
	if e.dimension > 0 {
		params.Dimensions = openai.Int(int64(e.dimension))
	}

	// OpenAI Embeddings APIを呼び出し
	resp, err := e.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// レスポンスからベクトルを抽出
	var embeddings [][]float32
	for _, data := range resp.Data {
		// float64からfloat32に変換
		vector := make([]float32, len(data.Embedding))
		for i, v := range data.Embedding {
			vector[i] = float32(v)
		}
		embeddings = append(embeddings, vector)
	}

	return embeddings, nil
}

// GetModelName はモデル名を取得します
func (e *Embedder) GetModelName() string {
	return e.model
}

// GetDimension はベクトル次元数を取得します
func (e *Embedder) GetDimension() int {
	return e.dimension
}
