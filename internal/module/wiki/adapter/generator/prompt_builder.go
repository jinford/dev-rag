package generator

import (
	"fmt"
	"log"
	"strings"

	"github.com/jinford/dev-rag/internal/module/wiki/domain"
)

const (
	// maxContextTokens はプロンプトのコンテキスト長の上限（トークン数）
	// 設計書に従い、8000トークンを上限とする
	maxContextTokens = 8000

	// safetyMarginRatio はコンテキスト長の安全マージン（20%）
	safetyMarginRatio = 0.8

	// estimatedCharsPerToken はトークン数推定用の文字数/トークン比率
	// 日本語を含むため、やや保守的に4文字/トークンと仮定
	estimatedCharsPerToken = 4
)

// PromptBuilder は階層的要約 + RAG検索結果を統合したプロンプトを構築する
type PromptBuilder struct {
	maxContextTokens int // デフォルト: 8000トークン
}

// NewPromptBuilder は新しいPromptBuilderを作成する
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		maxContextTokens: maxContextTokens,
	}
}

// BuildArchitecturePrompt はアーキテクチャページ生成用のプロンプトを構築する
//
// 入力:
//   - architectureSummaries: architecture_summariesテーブルから取得した要約（最優先コンテキスト）
//   - searchResults: RAG検索結果（補足コンテキスト）
//   - pseudoQuery: 疑似クエリ文字列
//
// 出力:
//   - LLMに渡すプロンプト文字列
func (p *PromptBuilder) BuildArchitecturePrompt(
	architectureSummaries []string,
	searchResults []*domain.SearchResult,
	pseudoQuery string,
) string {
	log.Printf("アーキテクチャプロンプト構築開始 (要約: %d件, 検索結果: %d件)", len(architectureSummaries), len(searchResults))

	// プロンプトテンプレートを使用
	return buildArchitectureWikiPrompt(architectureSummaries, searchResults, pseudoQuery, p.maxContextTokens)
}

// BuildDirectoryPrompt はディレクトリページ生成用のプロンプトを構築する
//
// 入力:
//   - directorySummary: directory_summariesテーブルから取得した要約（優先コンテキスト）
//   - searchResults: RAG検索結果（実装詳細）
//   - pseudoQuery: 疑似クエリ文字列
//
// 出力:
//   - LLMに渡すプロンプト文字列
func (p *PromptBuilder) BuildDirectoryPrompt(
	directorySummary string,
	searchResults []*domain.SearchResult,
	pseudoQuery string,
) string {
	log.Printf("ディレクトリプロンプト構築開始 (要約: %d文字, 検索結果: %d件)", len(directorySummary), len(searchResults))

	// プロンプトテンプレートを使用
	return buildDirectoryWikiPrompt(directorySummary, searchResults, pseudoQuery, p.maxContextTokens)
}

// groupByFilePath はRAG検索結果をファイルパスごとにグループ化する
// これにより、同じファイルの複数のチャンクをまとめて扱うことができる
func (p *PromptBuilder) groupByFilePath(results []*domain.SearchResult) map[string][]*domain.SearchResult {
	grouped := make(map[string][]*domain.SearchResult)
	for _, result := range results {
		grouped[result.FilePath] = append(grouped[result.FilePath], result)
	}
	return grouped
}

// estimateTokens はテキストのトークン数を推定する
// 実際のトークナイザーを使わず、文字数から概算する（文字数 / 4）
func estimateTokens(text string) int {
	return len(text) / estimatedCharsPerToken
}

// truncateContext はコンテキストをトークン数上限に合わせて切り詰める
// maxTokens の 80% を安全マージンとして使用
func truncateContext(items []string, maxTokens int) ([]string, bool) {
	safeMaxTokens := int(float64(maxTokens) * safetyMarginRatio)
	totalTokens := 0
	var result []string
	truncated := false

	for i, item := range items {
		tokens := estimateTokens(item)
		if totalTokens+tokens > safeMaxTokens {
			log.Printf("警告: コンテキスト長が上限に達しました (%d/%d items, %d/%d tokens)",
				i, len(items), totalTokens, safeMaxTokens)
			truncated = true
			break
		}
		result = append(result, item)
		totalTokens += tokens
	}

	log.Printf("コンテキスト: %d/%d items, 推定トークン数: %d/%d",
		len(result), len(items), totalTokens, safeMaxTokens)

	return result, truncated
}

// formatSearchResults はRAG検索結果をMarkdown形式に整形する
func formatSearchResults(results []*domain.SearchResult) string {
	if len(results) == 0 {
		return "（検索結果なし）"
	}

	var parts []string
	for i, result := range results {
		parts = append(parts, fmt.Sprintf(
			"### 検索結果 %d (スコア: %.3f)\n"+
				"- ファイル: `%s`\n"+
				"- 行: %d-%d\n"+
				"```\n%s\n```",
			i+1,
			result.Score,
			result.FilePath,
			result.StartLine,
			result.EndLine,
			result.Content,
		))
	}

	return strings.Join(parts, "\n\n")
}

// formatSummaries は要約リストをMarkdown形式に整形する
func formatSummaries(summaries []string) string {
	if len(summaries) == 0 {
		return "（要約なし）"
	}

	return strings.Join(summaries, "\n\n---\n\n")
}
