package llm

import (
	"context"
	"fmt"
	"time"

	llmdomain "github.com/jinford/dev-rag/internal/module/llm/domain"
	"github.com/jinford/dev-rag/internal/module/wiki/domain"
)

// Adapter は llm/domain.Client を wiki.LLMClient に適応させるアダプター
type Adapter struct {
	llmClient   llmdomain.Client
	embedder    llmdomain.Embedder
	temperature float64
	maxTokens   int
}

// NewAdapter は新しいAdapterを作成する
func NewAdapter(llmClient llmdomain.Client, embedder llmdomain.Embedder, temperature float64, maxTokens int) *Adapter {
	return &Adapter{
		llmClient:   llmClient,
		embedder:    embedder,
		temperature: temperature,
		maxTokens:   maxTokens,
	}
}

// Generate はプロンプトを受け取り、LLMによる応答を生成する
func (a *Adapter) Generate(ctx context.Context, prompt string) (string, error) {
	req := llmdomain.CompletionRequest{
		Prompt:      prompt,
		Temperature: a.temperature,
		MaxTokens:   a.maxTokens,
	}

	resp, err := a.llmClient.GenerateCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("LLM completion failed: %w", err)
	}

	return resp.Content, nil
}

// GenerateWithRetry はプロンプトを受け取り、失敗時にリトライしながらLLMによる応答を生成する
func (a *Adapter) GenerateWithRetry(ctx context.Context, prompt string, maxRetries int) (string, error) {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		result, err := a.Generate(ctx, prompt)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// 最後の試行でない場合は待機
		if i < maxRetries {
			// Exponential backoff: 2秒, 4秒, 8秒...
			backoff := time.Duration(1<<uint(i)) * 2 * time.Second
			if backoff > 32*time.Second {
				backoff = 32 * time.Second
			}

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
				// 続行
			}
		}
	}

	return "", fmt.Errorf("max retries (%d) exceeded: %w", maxRetries, lastErr)
}

// CreateEmbedding はテキストからEmbeddingベクトルを生成する
func (a *Adapter) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if a.embedder == nil {
		return nil, fmt.Errorf("embedder is not configured")
	}

	embedding, err := a.embedder.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("embedding creation failed: %w", err)
	}

	return embedding, nil
}

// CreateEmbeddingWithRetry はテキストからEmbeddingベクトルを生成し、失敗時にリトライする
func (a *Adapter) CreateEmbeddingWithRetry(ctx context.Context, text string, maxRetries int) ([]float32, error) {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		result, err := a.CreateEmbedding(ctx, text)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// 最後の試行でない場合は待機
		if i < maxRetries {
			// Exponential backoff: 2秒, 4秒, 8秒...
			backoff := time.Duration(1<<uint(i)) * 2 * time.Second
			if backoff > 32*time.Second {
				backoff = 32 * time.Second
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// 続行
			}
		}
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", maxRetries, lastErr)
}

// GetModelName はLLMのモデル名を返す
func (a *Adapter) GetModelName() string {
	// embedder からモデル名を取得する代わりに、固定値を返す
	// 将来的には Client interface に GetModelName を追加することを検討
	return "gpt-4o-mini"
}

// インターフェース実装の確認
var _ domain.LLMClient = (*Adapter)(nil)
