package ask

import (
	"fmt"
	"strings"

	"github.com/jinford/dev-rag/internal/core/search"
)

// BuildAskPrompt はRAG質問応答用のプロンプトを構築する
func BuildAskPrompt(
	query string,
	summaries []*search.SummarySearchResult,
	chunks []*search.SearchResult,
) string {
	var sb strings.Builder

	// システムプロンプトとガイドライン
	sb.WriteString("あなたは社内リポジトリのコードベースに精通した技術アシスタントです。\n")
	sb.WriteString("以下のコンテキスト情報を基に、ユーザーの質問に正確かつ簡潔に回答してください。\n\n")

	sb.WriteString("## 回答のガイドライン\n")
	sb.WriteString("- コンテキストに含まれる情報のみを使用して回答してください\n")
	sb.WriteString("- コードの具体的な場所(ファイルパス、行番号)を明示してください\n")
	sb.WriteString("- 不明な点がある場合は、推測せずにその旨を述べてください\n\n")

	// アーキテクチャ・構造情報
	sb.WriteString("## コンテキスト: アーキテクチャ・構造情報\n")
	if len(summaries) > 0 {
		for i, summary := range summaries {
			sb.WriteString(fmt.Sprintf("### [要約 %d] ", i+1))
			sb.WriteString(formatSummaryInfo(summary))
			sb.WriteString("\n")
			sb.WriteString(summary.Content)
			sb.WriteString("\n\n")
		}
	} else {
		sb.WriteString("(該当する要約情報はありません)\n\n")
	}

	// 関連コード
	sb.WriteString("## コンテキスト: 関連コード\n")
	if len(chunks) > 0 {
		for i, chunk := range chunks {
			sb.WriteString(fmt.Sprintf("### [コード断片 %d]\n", i+1))
			sb.WriteString(fmt.Sprintf("ファイルパス: %s\n", chunk.FilePath))
			sb.WriteString(fmt.Sprintf("行番号: %d-%d\n", chunk.StartLine, chunk.EndLine))
			sb.WriteString(fmt.Sprintf("関連度スコア: %.3f\n", chunk.Score))
			sb.WriteString("```\n")
			sb.WriteString(chunk.Content)
			sb.WriteString("\n```\n\n")
		}
	} else {
		sb.WriteString("(該当するコード断片はありません)\n\n")
	}

	// ユーザーの質問
	sb.WriteString("## ユーザーの質問\n")
	sb.WriteString(query)
	sb.WriteString("\n\n")

	// 回答セクション
	sb.WriteString("## 回答\n")

	return sb.String()
}

// formatSummaryInfo は要約情報のヘッダー部分を整形する
func formatSummaryInfo(summary *search.SummarySearchResult) string {
	var parts []string

	// 要約タイプ
	switch summary.SummaryType {
	case "file":
		parts = append(parts, "ファイル要約")
	case "directory":
		parts = append(parts, "ディレクトリ要約")
	case "architecture":
		parts = append(parts, "アーキテクチャ要約")
	default:
		parts = append(parts, summary.SummaryType)
	}

	// 対象パス
	if summary.TargetPath != "" {
		parts = append(parts, fmt.Sprintf("対象: %s", summary.TargetPath))
	}

	// アーキテクチャタイプ
	if summary.ArchType != nil && *summary.ArchType != "" {
		parts = append(parts, fmt.Sprintf("種別: %s", *summary.ArchType))
	}

	// スコア
	parts = append(parts, fmt.Sprintf("関連度: %.3f", summary.Score))

	return strings.Join(parts, " | ")
}
