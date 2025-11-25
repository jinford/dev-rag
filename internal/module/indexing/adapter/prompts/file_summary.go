package prompts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jinford/dev-rag/internal/module/indexing/adapter/llm"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

const (
	// FileSummaryPromptVersion はファイルサマリープロンプトのバージョン
	FileSummaryPromptVersion = "1.1"

	// FileSummaryTemperature はファイルサマリー生成の温度設定
	FileSummaryTemperature = 0.3

	// FileSummaryMaxTokens は生成する最大トークン数
	FileSummaryMaxTokens = 600
)

// FileSummaryPrompt はファイルサマリー生成プロンプトを構築します
const fileSummarySystemPrompt = `You are a code summarization assistant.

Your task is to summarize source code files and documentation in a structured format.

Guidelines:
- Focus on important functions, classes, sections, dependencies, and side effects
- Order items by importance (most important first)
- Be concise and factual - avoid speculation
- Do not include code blocks longer than 2 lines
- Keep the summary within 400 tokens
- Return a valid JSON response`

// 内部型定義（adapter 層でのみ使用）
type fileSummaryRequest struct {
	FilePath    string
	Language    string
	FileContent string
}

// fileSummaryResponse は内部的なレスポンス型
type fileSummaryResponse struct {
	PromptVersion string                      `json:"prompt_version"`
	Summary       []string                    `json:"summary"`
	Risks         []string                    `json:"risks"`
	Metadata      fileSummaryMetadata         `json:"metadata"`
}

// fileSummaryMetadata は内部的なメタデータ型
type fileSummaryMetadata struct {
	PrimaryTopics []string `json:"primary_topics"`
	KeySymbols    []string `json:"key_symbols"`
}

// toDomainResponse は内部型を domain 型に変換します
func (r *fileSummaryResponse) toDomainResponse() *domain.FileSummaryResponse {
	return &domain.FileSummaryResponse{
		PromptVersion: r.PromptVersion,
		Summary:       r.Summary,
		Risks:         r.Risks,
		Metadata: domain.FileSummaryMetadata{
			PrimaryTopics: r.Metadata.PrimaryTopics,
			KeySymbols:    r.Metadata.KeySymbols,
		},
	}
}

// fileSummaryGenerator はファイルサマリー生成を担当します（domain.FileSummaryGenerator の実装）
type fileSummaryGenerator struct {
	llmClient    llm.LLMClient
	tokenCounter *llm.TokenCounter
}

// NewFileSummaryGenerator は新しい domain.FileSummaryGenerator を作成します
func NewFileSummaryGenerator(llmClient llm.LLMClient, tokenCounter *llm.TokenCounter) domain.FileSummaryGenerator {
	return &fileSummaryGenerator{
		llmClient:    llmClient,
		tokenCounter: tokenCounter,
	}
}

// generateFileSummaryPrompt はファイルサマリー生成プロンプトを構築します
func generateFileSummaryPrompt(req fileSummaryRequest) string {
	return fmt.Sprintf(`Summarize the following file in Markdown format within 400 tokens.
Include important functions, classes, sections, dependencies, and side effects.
Order items by importance (most important first).
Avoid speculation. Do not include code blocks longer than 2 lines.

File: %s
Language: %s

Content:
%s

Return a JSON response with the following structure:
{
  "prompt_version": "1.1",
  "summary": ["item1", "item2", ...],
  "risks": ["risk1", ...],
  "metadata": {
    "primary_topics": ["topic1", ...],
    "key_symbols": ["symbol1", ...]
  }
}`, req.FilePath, req.Language, req.FileContent)
}

// Generate はLLMを使用してファイルサマリーを生成します（domain.FileSummaryGenerator の実装）
func (g *fileSummaryGenerator) Generate(ctx context.Context, req domain.FileSummaryRequest) (*domain.FileSummaryResponse, error) {
	// domain 型を内部型に変換
	internalReq := fileSummaryRequest{
		FilePath:    req.FilePath,
		Language:    req.Language,
		FileContent: req.FileContent,
	}

	// プロンプトを構築
	userPrompt := generateFileSummaryPrompt(internalReq)

	// システムプロンプトとユーザープロンプトを結合
	fullPrompt := fmt.Sprintf("%s\n\n%s", fileSummarySystemPrompt, userPrompt)

	// トークン数を計算
	tokens := g.tokenCounter.CountTokens(fullPrompt)

	// トークン数が多すぎる場合はエラー
	// OpenAIのコンテキスト制限を考慮（例: gpt-4o-miniは128k）
	const maxInputTokens = 100000
	if tokens > maxInputTokens {
		return nil, fmt.Errorf("prompt too long: %d tokens (max: %d)", tokens, maxInputTokens)
	}

	// LLMを呼び出し
	llmReq := llm.CompletionRequest{
		Prompt:         fullPrompt,
		Temperature:    FileSummaryTemperature,
		MaxTokens:      FileSummaryMaxTokens,
		ResponseFormat: "json",
	}

	resp, err := g.llmClient.GenerateCompletion(ctx, llmReq)
	if err != nil {
		// エラーログを記録
		g.logError(fullPrompt, "", err, 0)
		return nil, fmt.Errorf("failed to generate completion: %w", err)
	}

	// JSON応答をパース
	var summaryResp fileSummaryResponse
	if err := json.Unmarshal([]byte(resp.Content), &summaryResp); err != nil {
		// JSON解析エラーをログに記録
		// このエラーは通常、OpenAIClientのリトライで既に1回リトライ済み
		g.logError(fullPrompt, resp.Content, err, 1)
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// プロンプトバージョンを検証
	llm.DefaultPromptVersionRegistry.ValidateVersion(llm.PromptTypeFileSummary, summaryResp.PromptVersion)

	return summaryResp.toDomainResponse(), nil
}

// logError はエラーをグローバルエラーハンドラーに記録します
func (g *fileSummaryGenerator) logError(prompt, response string, err error, retryCount int) {
	if llm.GlobalErrorHandler == nil {
		return
	}

	// エラータイプを判定
	errorType := llm.ErrorTypeUnknown
	if errors.Is(err, context.DeadlineExceeded) {
		errorType = llm.ErrorTypeTimeout
	} else if errors.Is(err, llm.ErrMaxRetriesExceeded) {
		errorType = llm.ErrorTypeRateLimitExceeded
	} else if strings.Contains(err.Error(), "parse") || strings.Contains(err.Error(), "unmarshal") {
		errorType = llm.ErrorTypeJSONParseFailed
	}

	record := llm.ErrorRecord{
		Timestamp:     time.Now(),
		ErrorType:     errorType,
		PromptSection: llm.PromptSectionFileSummary,
		Prompt:        llm.TruncateString(prompt, 5000),
		Response:      llm.TruncateString(response, 5000),
		ErrorMessage:  err.Error(),
		RetryCount:    retryCount,
	}

	// ログ記録エラーは無視（ログ記録自体が失敗してもメイン処理を止めない）
	_ = llm.GlobalErrorHandler.LogError(record)
}
