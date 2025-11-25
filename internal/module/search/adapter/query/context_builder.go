package query

import (
	"fmt"
	"strings"
)

// ContextBuilder はLLMへのコンテキストを構築します
type ContextBuilder struct {
	maxTokens int // 最大トークン数
}

// NewContextBuilder は新しいContextBuilderを作成します
//
// 使用例:
//
//	contextBuilder := query.NewContextBuilder(8000)
//	llmContext := contextBuilder.BuildContextWithHierarchy(results)
//	llmContext = contextBuilder.TruncateToTokenLimit(llmContext)
func NewContextBuilder(maxTokens int) *ContextBuilder {
	return &ContextBuilder{
		maxTokens: maxTokens,
	}
}

// BuildContextWithHierarchy は階層情報を含むコンテキストを構築します
//
// このメソッドは、検索結果に含まれる階層情報（親チャンク、子チャンク）を
// LLMに送信するためのテキスト形式に整形します。
//
// コンテキストの構造:
//  1. 親チャンク（存在する場合）
//  2. メインチャンク（検索結果）
//  3. 子チャンク（存在する場合）
//
// 使用例:
//
//	querier := query.NewQuerier(indexRepo)
//	options := &query.SearchOptions{
//	    IncludeParent:   true,
//	    IncludeChildren: true,
//	}
//	results, err := querier.EnrichSearchResultsWithHierarchy(ctx, baseResults, options)
//	if err != nil {
//	    return err
//	}
//
//	contextBuilder := query.NewContextBuilder(8000)
//	llmContext := contextBuilder.BuildContextWithHierarchy(results)
func (cb *ContextBuilder) BuildContextWithHierarchy(results []*EnhancedSearchResult) string {
	var builder strings.Builder

	for i, result := range results {
		// セパレータ
		if i > 0 {
			builder.WriteString("\n---\n\n")
		}

		// 親チャンクがある場合は先に表示
		if result.ParentChunk != nil {
			builder.WriteString(fmt.Sprintf("## Parent Context (File: %s)\n\n", result.FilePath))
			builder.WriteString(result.ParentChunk.Content)
			builder.WriteString("\n\n")
		}

		// メインチャンク
		builder.WriteString(fmt.Sprintf("## Search Result %d (File: %s, Lines: %d-%d)\n\n",
			i+1, result.FilePath, result.StartLine, result.EndLine))
		builder.WriteString(result.Content)
		builder.WriteString("\n\n")

		// 子チャンクがある場合は後に表示
		if len(result.ChildChunks) > 0 {
			builder.WriteString("### Sub-sections:\n\n")
			for j, child := range result.ChildChunks {
				builder.WriteString(fmt.Sprintf("#### Sub-section %d:\n", j+1))
				builder.WriteString(child.Content)
				builder.WriteString("\n\n")
			}
		}
	}

	return builder.String()
}

// TruncateToTokenLimit はトークン制限内に収まるようにコンテキストを切り詰めます
//
// この実装は簡易版で、文字数ベースで切り詰めを行います。
// より正確なトークン数計算には、tiktokenなどのライブラリを使用してください。
//
// 概算: 1トークン ≈ 4文字（英語の場合、日本語では異なる）
//
// 使用例:
//
//	contextBuilder := query.NewContextBuilder(8000)
//	llmContext := contextBuilder.BuildContextWithHierarchy(results)
//	llmContext = contextBuilder.TruncateToTokenLimit(llmContext)
//
//	// LLMに送信
//	response := sendToLLM(llmContext, userQuestion)
func (cb *ContextBuilder) TruncateToTokenLimit(context string) string {
	// 簡易実装: 文字数ベースで切り詰め（実際にはtiktokenを使用すべき）
	// 1トークン ≈ 4文字として概算
	maxChars := cb.maxTokens * 4

	if len(context) <= maxChars {
		return context
	}

	// 切り詰め
	truncated := context[:maxChars]
	truncated += "\n\n... (truncated)"

	return truncated
}

// EstimateTokenCount はコンテキストのトークン数を概算します
//
// この実装は簡易版で、文字数ベースで概算します。
// より正確なトークン数計算には、tiktokenなどのライブラリを使用してください。
//
// 概算: 1トークン ≈ 4文字（英語の場合、日本語では異なる）
func (cb *ContextBuilder) EstimateTokenCount(context string) int {
	// 簡易実装: 文字数 ÷ 4
	return len(context) / 4
}

// BuildSimpleContext はシンプルなコンテキストを構築します（階層情報なし）
//
// このメソッドは、階層情報を含まない基本的な検索結果からコンテキストを構築します。
func (cb *ContextBuilder) BuildSimpleContext(results []*EnhancedSearchResult) string {
	var builder strings.Builder

	for i, result := range results {
		// セパレータ
		if i > 0 {
			builder.WriteString("\n---\n\n")
		}

		// メインチャンク
		builder.WriteString(fmt.Sprintf("## Search Result %d (File: %s, Lines: %d-%d)\n\n",
			i+1, result.FilePath, result.StartLine, result.EndLine))
		builder.WriteString(result.Content)
		builder.WriteString("\n\n")
	}

	return builder.String()
}

// BuildContextWithMetadata はメタデータを含むコンテキストを構築します
//
// このメソッドは、検索結果に含まれるメタデータ（スコア、ファイルパスなど）を
// より詳細に表示するコンテキストを構築します。
func (cb *ContextBuilder) BuildContextWithMetadata(results []*EnhancedSearchResult) string {
	var builder strings.Builder

	builder.WriteString("# Search Results\n\n")

	for i, result := range results {
		// セパレータ
		if i > 0 {
			builder.WriteString("\n---\n\n")
		}

		// メタデータ
		builder.WriteString(fmt.Sprintf("## Result %d\n\n", i+1))
		builder.WriteString(fmt.Sprintf("- **File**: %s\n", result.FilePath))
		builder.WriteString(fmt.Sprintf("- **Lines**: %d-%d\n", result.StartLine, result.EndLine))
		builder.WriteString(fmt.Sprintf("- **Relevance Score**: %.4f\n\n", result.Score))

		// 親チャンクがある場合
		if result.ParentChunk != nil {
			builder.WriteString("### Parent Context\n\n")
			builder.WriteString("```\n")
			builder.WriteString(result.ParentChunk.Content)
			builder.WriteString("\n```\n\n")
		}

		// メインコンテンツ
		builder.WriteString("### Main Content\n\n")
		builder.WriteString("```\n")
		builder.WriteString(result.Content)
		builder.WriteString("\n```\n\n")

		// 子チャンクがある場合
		if len(result.ChildChunks) > 0 {
			builder.WriteString("### Sub-sections\n\n")
			for j, child := range result.ChildChunks {
				builder.WriteString(fmt.Sprintf("#### Sub-section %d\n\n", j+1))
				builder.WriteString("```\n")
				builder.WriteString(child.Content)
				builder.WriteString("\n```\n\n")
			}
		}
	}

	return builder.String()
}

// BuildCompactContext はコンパクトなコンテキストを構築します
//
// このメソッドは、トークン制限が厳しい場合に、
// 必要最小限の情報のみを含むコンパクトなコンテキストを構築します。
//
// 親チャンクと子チャンクは含まず、検索結果のみを返します。
func (cb *ContextBuilder) BuildCompactContext(results []*EnhancedSearchResult) string {
	var builder strings.Builder

	for i, result := range results {
		if i > 0 {
			builder.WriteString("\n---\n")
		}
		builder.WriteString(fmt.Sprintf("[%d] %s (L%d-L%d)\n",
			i+1, result.FilePath, result.StartLine, result.EndLine))
		builder.WriteString(result.Content)
		builder.WriteString("\n")
	}

	return builder.String()
}
