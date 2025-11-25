package prompts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jinford/dev-rag/internal/module/indexing/adapter/llm"
)

const (
	// ChunkSummaryPromptVersion はチャンク要約プロンプトのバージョン
	ChunkSummaryPromptVersion = "1.1"

	// ChunkSummaryTemperature はチャンク要約生成の温度設定
	// より決定論的にする
	ChunkSummaryTemperature = 0.2

	// ChunkSummaryMaxTokens は生成する最大トークン数
	ChunkSummaryMaxTokens = 150
)

// ChunkSummaryPrompt はチャンク要約生成プロンプトを構築します
const chunkSummarySystemPrompt = `You are a code summarization assistant.

Your task is to summarize code chunks in 1-2 declarative sentences.

Guidelines:
- Use declarative sentences, not imperative form
- Keep summary within 80 tokens
- Focus on what the code does and what entities it interacts with
- Do not speculate on missing information - explicitly state "undefined" if unclear
- Use only values that exist in the original code
- Include at most 3 code identifiers
- Return a valid JSON response`

// ChunkSummaryRequest はチャンク要約生成リクエスト
type ChunkSummaryRequest struct {
	ChunkID       string
	ParentContext string // 親チャンクのサマリー（存在しない場合はnull）
	ChunkContent  string
}

// ChunkSummaryResponse はチャンク要約生成レスポンス
type ChunkSummaryResponse struct {
	PromptVersion   string   `json:"prompt_version"`
	ChunkID         string   `json:"chunk_id"`
	SummarySentence string   `json:"summary_sentence"`
	FocusEntities   []string `json:"focus_entities"`
	Confidence      float64  `json:"confidence"`
}

// ChunkSummaryGenerator はチャンク要約生成を担当します
type ChunkSummaryGenerator struct {
	llmClient    llm.LLMClient
	tokenCounter *llm.TokenCounter
}

// NewChunkSummaryGenerator は新しいChunkSummaryGeneratorを作成します
func NewChunkSummaryGenerator(llmClient llm.LLMClient, tokenCounter *llm.TokenCounter) *ChunkSummaryGenerator {
	return &ChunkSummaryGenerator{
		llmClient:    llmClient,
		tokenCounter: tokenCounter,
	}
}

// GenerateChunkSummaryPrompt はチャンク要約生成プロンプトを構築します
func GenerateChunkSummaryPrompt(req ChunkSummaryRequest) string {
	parentContextText := "null"
	if req.ParentContext != "" {
		parentContextText = req.ParentContext
	}

	return fmt.Sprintf(`Summarize the following code chunk in 1-2 declarative sentences within 80 tokens.

Parent Context: %s

Chunk Content:
%s

Return a JSON response with the following structure:
{
  "prompt_version": "1.1",
  "chunk_id": "%s",
  "summary_sentence": "...",
  "focus_entities": ["entity1", "entity2", "entity3"],
  "confidence": 0.75
}

Confidence guidelines:
- Clear input/output and side effects: 0.75
- Abstract description or insufficient information: 0.55
- Potential contradiction with parent context: 0.35 or lower`, parentContextText, req.ChunkContent, req.ChunkID)
}

// Generate はLLMを使用してチャンク要約を生成します
// エラー発生時にはエラーログを記録し、エラーを返します
// フォールバック: チャンク要約生成失敗時は要約なしで処理を継続すべき（呼び出し側で制御）
func (g *ChunkSummaryGenerator) Generate(ctx context.Context, req ChunkSummaryRequest) (*ChunkSummaryResponse, error) {
	// プロンプトを構築
	userPrompt := GenerateChunkSummaryPrompt(req)

	// システムプロンプトとユーザープロンプトを結合
	fullPrompt := fmt.Sprintf("%s\n\n%s", chunkSummarySystemPrompt, userPrompt)

	// トークン数を計算
	tokens := g.tokenCounter.CountTokens(fullPrompt)

	// トークン数が多すぎる場合はエラー
	const maxInputTokens = 50000
	if tokens > maxInputTokens {
		return nil, fmt.Errorf("prompt too long: %d tokens (max: %d)", tokens, maxInputTokens)
	}

	// LLMを呼び出し
	llmReq := llm.CompletionRequest{
		Prompt:         fullPrompt,
		Temperature:    ChunkSummaryTemperature,
		MaxTokens:      ChunkSummaryMaxTokens,
		ResponseFormat: "json",
	}

	resp, err := g.llmClient.GenerateCompletion(ctx, llmReq)
	if err != nil {
		// エラーログを記録
		g.logError(fullPrompt, "", err, 0)
		return nil, fmt.Errorf("failed to generate completion: %w", err)
	}

	// JSON応答をパース
	var summaryResp ChunkSummaryResponse
	if err := json.Unmarshal([]byte(resp.Content), &summaryResp); err != nil {
		// JSON解析エラーをログに記録
		g.logError(fullPrompt, resp.Content, err, 1)
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// プロンプトバージョンを検証
	llm.DefaultPromptVersionRegistry.ValidateVersion(llm.PromptTypeChunkSummary, summaryResp.PromptVersion)

	// 信頼度の範囲を検証
	if summaryResp.Confidence < 0.2 || summaryResp.Confidence > 0.85 {
		return nil, fmt.Errorf("confidence out of range: %.2f (expected 0.2-0.85)", summaryResp.Confidence)
	}

	// FocusEntitiesが3件以下であることを確認
	if len(summaryResp.FocusEntities) > 3 {
		// 最初の3件のみを残す
		summaryResp.FocusEntities = summaryResp.FocusEntities[:3]
	}

	return &summaryResp, nil
}

// logError はエラーをグローバルエラーハンドラーに記録します
func (g *ChunkSummaryGenerator) logError(prompt, response string, err error, retryCount int) {
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
		PromptSection: llm.PromptSectionChunkSummary,
		Prompt:        llm.TruncateString(prompt, 5000),
		Response:      llm.TruncateString(response, 5000),
		ErrorMessage:  err.Error(),
		RetryCount:    retryCount,
	}

	_ = llm.GlobalErrorHandler.LogError(record)
}

// BuildEmbeddingContext はチャンク要約をEmbedding用テキストの冒頭に追加します
func BuildEmbeddingContext(summary *ChunkSummaryResponse, originalContent string) string {
	return fmt.Sprintf("Summary: %s\n\n%s", summary.SummarySentence, originalContent)
}
