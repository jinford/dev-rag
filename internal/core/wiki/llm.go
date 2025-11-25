package wiki

import "context"

// Client はLLMサービスとのやり取りを抽象化する共通インターフェース
type Client interface {
	// GenerateCompletion はプロンプトに基づいてLLMから応答を生成する
	GenerateCompletion(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// CompletionRequest はLLMへのリクエストパラメータ
type CompletionRequest struct {
	// Prompt はLLMに送信するプロンプト
	Prompt string

	// Temperature は生成の多様性を制御する (0.0-2.0)
	Temperature float64

	// MaxTokens は生成する最大トークン数
	MaxTokens int

	// ResponseFormat はレスポンスの形式 ("json" or "text")
	ResponseFormat string

	// Model はLLMモデル名 (省略時はデフォルトモデルを使用)
	Model string
}

// CompletionResponse はLLMからのレスポンス
type CompletionResponse struct {
	// Content は生成されたテキスト
	Content string

	// TokensUsed は使用されたトークン数
	TokensUsed int

	// PromptVersion はプロンプトのバージョン (トレーサビリティ用)
	PromptVersion string

	// Model は実際に使用されたモデル名
	Model string
}
