package adapter

import (
	"context"
	"fmt"

	"github.com/jinford/dev-rag/internal/module/llm/domain"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// OpenAIEmbedder はOpenAI APIを使用したEmbedder実装
type OpenAIEmbedder struct {
	client    openai.Client
	model     string
	dimension int
}

// NewOpenAIEmbedder は新しいOpenAIEmbedderを作成します
func NewOpenAIEmbedder(apiKey, model string, dimension int) (*OpenAIEmbedder, error) {
	if apiKey == "" {
		return nil, ErrAPIKeyNotSet
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	return &OpenAIEmbedder{
		client:    client,
		model:     model,
		dimension: dimension,
	}, nil
}

// Embed はテキストからEmbeddingベクトルを生成する
// domain.Embedderインターフェースを実装
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
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

// Dimension はEmbeddingベクトルの次元数を返す
// domain.Embedderインターフェースを実装
func (e *OpenAIEmbedder) Dimension() int {
	return e.dimension
}

// BatchEmbed はバッチでEmbeddingを生成します（最大100件）
func (e *OpenAIEmbedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
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
func (e *OpenAIEmbedder) GetModelName() string {
	return e.model
}

// インターフェース実装の確認
var _ domain.Embedder = (*OpenAIEmbedder)(nil)
