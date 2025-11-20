package prompts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jinford/dev-rag/pkg/indexer/llm"
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

// FileSummaryRequest はファイルサマリー生成リクエスト
type FileSummaryRequest struct {
	FilePath    string
	Language    string
	FileContent string
}

// FileSummaryResponse はファイルサマリー生成レスポンス
type FileSummaryResponse struct {
	PromptVersion string              `json:"prompt_version"`
	Summary       []string            `json:"summary"`
	Risks         []string            `json:"risks"`
	Metadata      FileSummaryMetadata `json:"metadata"`
}

// FileSummaryMetadata はファイルサマリーのメタデータ
type FileSummaryMetadata struct {
	PrimaryTopics []string `json:"primary_topics"`
	KeySymbols    []string `json:"key_symbols"`
}

// FileSummaryGenerator はファイルサマリー生成を担当します
type FileSummaryGenerator struct {
	llmClient    llm.LLMClient
	tokenCounter *llm.TokenCounter
}

// NewFileSummaryGenerator は新しいFileSummaryGeneratorを作成します
func NewFileSummaryGenerator(llmClient llm.LLMClient, tokenCounter *llm.TokenCounter) *FileSummaryGenerator {
	return &FileSummaryGenerator{
		llmClient:    llmClient,
		tokenCounter: tokenCounter,
	}
}

// GenerateFileSummaryPrompt はファイルサマリー生成プロンプトを構築します
func GenerateFileSummaryPrompt(req FileSummaryRequest) string {
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

// Generate はLLMを使用してファイルサマリーを生成します
// エラー発生時にはエラーログを記録し、エラーを返します
func (g *FileSummaryGenerator) Generate(ctx context.Context, req FileSummaryRequest) (*FileSummaryResponse, error) {
	// プロンプトを構築
	userPrompt := GenerateFileSummaryPrompt(req)

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
	var summaryResp FileSummaryResponse
	if err := json.Unmarshal([]byte(resp.Content), &summaryResp); err != nil {
		// JSON解析エラーをログに記録
		// このエラーは通常、OpenAIClientのリトライで既に1回リトライ済み
		g.logError(fullPrompt, resp.Content, err, 1)
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// プロンプトバージョンを検証
	llm.DefaultPromptVersionRegistry.ValidateVersion(llm.PromptTypeFileSummary, summaryResp.PromptVersion)

	return &summaryResp, nil
}

// logError はエラーをグローバルエラーハンドラーに記録します
func (g *FileSummaryGenerator) logError(prompt, response string, err error, retryCount int) {
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

// GenerateSummaryText はFileSummaryResponseからMarkdown形式のサマリーテキストを生成します
func GenerateSummaryText(resp *FileSummaryResponse) string {
	var text string

	// サマリー項目を箇条書きで追加
	if len(resp.Summary) > 0 {
		text += "## Summary\n\n"
		for _, item := range resp.Summary {
			text += fmt.Sprintf("- %s\n", item)
		}
		text += "\n"
	}

	// リスクを追加
	if len(resp.Risks) > 0 {
		text += "## Risks\n\n"
		for _, risk := range resp.Risks {
			text += fmt.Sprintf("- %s\n", risk)
		}
		text += "\n"
	}

	// メタデータを追加
	if len(resp.Metadata.PrimaryTopics) > 0 || len(resp.Metadata.KeySymbols) > 0 {
		text += "## Metadata\n\n"
		if len(resp.Metadata.PrimaryTopics) > 0 {
			text += fmt.Sprintf("**Primary Topics:** %v\n\n", resp.Metadata.PrimaryTopics)
		}
		if len(resp.Metadata.KeySymbols) > 0 {
			text += fmt.Sprintf("**Key Symbols:** %v\n\n", resp.Metadata.KeySymbols)
		}
	}

	return text
}
