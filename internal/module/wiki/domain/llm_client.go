package domain

import "context"

// LLMClient はリトライ機能付きLLMクライアントインターフェース
// 既存のllm.LLMClientをラップして、リトライ機能を追加する
type LLMClient interface {
	// Generate はプロンプトを受け取り、LLMによる応答を生成する
	Generate(ctx context.Context, prompt string) (string, error)

	// GenerateWithRetry はプロンプトを受け取り、失敗時にリトライしながらLLMによる応答を生成する
	GenerateWithRetry(ctx context.Context, prompt string, maxRetries int) (string, error)

	// CreateEmbedding はテキストからEmbeddingベクトルを生成する
	CreateEmbedding(ctx context.Context, text string) ([]float32, error)

	// CreateEmbeddingWithRetry はテキストからEmbeddingベクトルを生成し、失敗時にリトライする
	CreateEmbeddingWithRetry(ctx context.Context, text string, maxRetries int) ([]float32, error)

	// GetModelName はLLMのモデル名を返す
	GetModelName() string
}
