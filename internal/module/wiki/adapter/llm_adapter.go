package wiki

import (
	"context"
	"fmt"
	"math"
	"time"

	llmdomain "github.com/jinford/dev-rag/internal/module/llm/domain"
	llmadapter "github.com/jinford/dev-rag/internal/module/llm/adapter"
)

// LLMAdapter は既存のLLMClientとEmbedderをラップして、
// wiki.LLMClientインターフェースを実装するアダプター
type LLMAdapter struct {
	llmClient *llmadapter.OpenAIClient
	embedder  llmdomain.Embedder
}

// NewLLMAdapter は新しいLLMAdapterを作成する
func NewLLMAdapter(llmClient *llmadapter.OpenAIClient, embedder llmdomain.Embedder) *LLMAdapter {
	return &LLMAdapter{
		llmClient: llmClient,
		embedder:  embedder,
	}
}

// Generate はプロンプトを受け取り、LLMによる応答を生成する
func (a *LLMAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	req := llmdomain.CompletionRequest{
		Prompt:      prompt,
		Temperature: 0.3, // 要約生成用の低めの温度
		MaxTokens:   4000,
	}

	resp, err := a.llmClient.GenerateCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	return resp.Content, nil
}

// GenerateWithRetry はプロンプトを受け取り、失敗時にリトライしながらLLMによる応答を生成する
func (a *LLMAdapter) GenerateWithRetry(ctx context.Context, prompt string, maxRetries int) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential Backoff
			backoffDuration := time.Duration(math.Pow(2, float64(attempt-1))) * 2 * time.Second
			maxBackoff := 32 * time.Second
			if backoffDuration > maxBackoff {
				backoffDuration = maxBackoff
			}

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoffDuration):
				// バックオフ後、再試行
			}
		}

		result, err := a.Generate(ctx, prompt)
		if err != nil {
			lastErr = err
			continue
		}

		return result, nil
	}

	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// CreateEmbedding はテキストからEmbeddingベクトルを生成する
func (a *LLMAdapter) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	embedding, err := a.embedder.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("embedding creation failed: %w", err)
	}
	return embedding, nil
}

// CreateEmbeddingWithRetry はテキストからEmbeddingベクトルを生成し、失敗時にリトライする
func (a *LLMAdapter) CreateEmbeddingWithRetry(ctx context.Context, text string, maxRetries int) ([]float32, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential Backoff
			backoffDuration := time.Duration(math.Pow(2, float64(attempt-1))) * 2 * time.Second
			maxBackoff := 32 * time.Second
			if backoffDuration > maxBackoff {
				backoffDuration = maxBackoff
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDuration):
				// バックオフ後、再試行
			}
		}

		result, err := a.CreateEmbedding(ctx, text)
		if err != nil {
			lastErr = err
			continue
		}

		return result, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// GetModelName はLLMのモデル名を返す
func (a *LLMAdapter) GetModelName() string {
	return a.llmClient.GetModelName()
}
