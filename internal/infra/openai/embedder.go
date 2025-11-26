package openai

import (
	"context"
	"fmt"

	"github.com/jinford/dev-rag/internal/core/ingestion"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Embedder は OpenAI API を使用してテキストをベクトルに変換する
type Embedder struct {
	client    openai.Client
	model     string
	dimension int
}

// NewEmbedder は新しい Embedder を作成する
func NewEmbedder(apiKey, model string, dimension int) *Embedder {
	return &Embedder{
		client: openai.NewClient(
			option.WithAPIKey(apiKey),
		),
		model:     model,
		dimension: dimension,
	}
}

// Embed は単一テキストの Embedding を生成する
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := e.BatchEmbed(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings generated")
	}

	return embeddings[0], nil
}

// BatchEmbed はバッチで Embedding を生成する（最大100件）
func (e *Embedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	if len(texts) > 100 {
		return nil, fmt.Errorf("batch size exceeds maximum of 100")
	}

	params := openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(e.model),
	}

	if len(texts) == 1 {
		params.Input = openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(texts[0]),
		}
	} else {
		params.Input = openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: texts,
		}
	}

	if e.dimension > 0 {
		params.Dimensions = openai.Int(int64(e.dimension))
	}

	resp, err := e.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	var embeddings [][]float32
	for _, data := range resp.Data {
		vector := make([]float32, len(data.Embedding))
		for i, v := range data.Embedding {
			vector[i] = float32(v)
		}
		embeddings = append(embeddings, vector)
	}

	return embeddings, nil
}

// ModelName はモデル名を返す
func (e *Embedder) ModelName() string {
	return e.model
}

// Dimension はベクトル次元数を返す
func (e *Embedder) Dimension() int {
	return e.dimension
}

// MaxBatchSize はバッチ処理の最大サイズを返す（OpenAI APIは最大100件）
func (e *Embedder) MaxBatchSize() int {
	return 100
}

// Metadata はモデル情報を返す
func (e *Embedder) Metadata() ingestion.Metadata {
	return ingestion.Metadata{
		ModelName: e.model,
		Dimension: e.dimension,
	}
}

// インターフェース実装の確認
var _ ingestion.Embedder = (*Embedder)(nil)
