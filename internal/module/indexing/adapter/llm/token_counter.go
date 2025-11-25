package llm

import (
	"fmt"

	"github.com/pkoukk/tiktoken-go"
)

// TokenCounter はトークン数をカウントする機能を提供する
type TokenCounter struct {
	encoding *tiktoken.Tiktoken
}

// NewTokenCounter は新しいTokenCounterを作成する
// cl100k_baseエンコーディングを使用する
func NewTokenCounter() (*TokenCounter, error) {
	encoding, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding: %w", err)
	}

	return &TokenCounter{
		encoding: encoding,
	}, nil
}

// CountTokens はテキストのトークン数をカウントする
func (tc *TokenCounter) CountTokens(text string) int {
	if tc.encoding == nil {
		// エンコーディングが初期化されていない場合は0を返す
		return 0
	}
	tokens := tc.encoding.Encode(text, nil, nil)
	return len(tokens)
}

// CountPromptAndResponse はプロンプトとレスポンスの合計トークン数を返す
func (tc *TokenCounter) CountPromptAndResponse(prompt, response string) TokenUsage {
	promptTokens := tc.CountTokens(prompt)
	responseTokens := tc.CountTokens(response)

	return TokenUsage{
		PromptTokens:   promptTokens,
		ResponseTokens: responseTokens,
		TotalTokens:    promptTokens + responseTokens,
	}
}

// TokenUsage はトークン使用量を表す
type TokenUsage struct {
	// PromptTokens はプロンプトで使用されたトークン数
	PromptTokens int

	// ResponseTokens はレスポンスで使用されたトークン数
	ResponseTokens int

	// TotalTokens は合計トークン数
	TotalTokens int
}

// EstimateTokens はテキストの推定トークン数を返す
// 正確にカウントせず、大まかな推定値を返す（文字数を基準）
func EstimateTokens(text string) int {
	// 英語の場合: 約4文字で1トークン
	// 日本語の場合: 約1文字で1トークン
	// ここでは平均的な値として3文字で1トークンとする
	return len([]rune(text)) / 3
}
