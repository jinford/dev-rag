package wiki

import (
	"fmt"
	"strings"

	"github.com/jinford/dev-rag/internal/core/search"
)

// WikiSection はWikiのセクションを表す
type WikiSection string

const (
	SectionOverview   WikiSection = "overview"
	SectionTechStack  WikiSection = "tech_stack"
	SectionDataFlow   WikiSection = "data_flow"
	SectionComponents WikiSection = "components"
)

// SectionConfig はセクション生成の設定
type SectionConfig struct {
	Section     WikiSection
	Query       string
	Title       string
	Description string
	FileName    string
}

// GetSectionConfigs は全セクションの設定を返す
func GetSectionConfigs() []SectionConfig {
	return []SectionConfig{
		{
			Section:     SectionOverview,
			Query:       "プロダクトの目的、解決する課題、提供する価値",
			Title:       "概要",
			Description: "プロダクトの目的、主要機能、全体構造の概要",
			FileName:    "README.md",
		},
		{
			Section:     SectionTechStack,
			Query:       "使用技術、ツール、プラットフォーム、依存関係",
			Title:       "技術スタック",
			Description: "使用している技術、ツール、プラットフォームの一覧と説明",
			FileName:    "tech-stack.md",
		},
		{
			Section:     SectionDataFlow,
			Query:       "情報の流れ、処理の流れ、ワークフロー",
			Title:       "処理フロー",
			Description: "プロダクト内の情報やデータの流れと処理の流れ",
			FileName:    "data-flow.md",
		},
		{
			Section:     SectionComponents,
			Query:       "主要な構成要素、機能モジュール、それらの関係性",
			Title:       "構成要素",
			Description: "プロダクトを構成する主要な要素とその関係",
			FileName:    "components.md",
		},
	}
}

// BuildSectionPrompt はセクションのプロンプトを構築する
func BuildSectionPrompt(config SectionConfig, summaries []*search.SummarySearchResult, chunks []*search.SearchResult) string {
	var sb strings.Builder

	// ヘッダー
	sb.WriteString(fmt.Sprintf("# タスク: %sセクションのWikiページ生成\n\n", config.Title))
	sb.WriteString(fmt.Sprintf("## 目的\n%s\n\n", config.Description))

	// コンテキスト: 構造要約
	if len(summaries) > 0 {
		sb.WriteString("## コンテキスト: 構造要約\n\n")
		for i, summary := range summaries {
			sb.WriteString(fmt.Sprintf("### 要約 %d: %s\n", i+1, summary.TargetPath))
			if summary.ArchType != nil {
				sb.WriteString(fmt.Sprintf("タイプ: %s\n", *summary.ArchType))
			}
			sb.WriteString(fmt.Sprintf("関連度: %.3f\n\n", summary.Score))
			sb.WriteString("```\n")
			sb.WriteString(summary.Content)
			sb.WriteString("\n```\n\n")
		}
	}

	// コンテキスト: 詳細コンテンツ
	if len(chunks) > 0 {
		sb.WriteString("## コンテキスト: 関連コンテンツ\n\n")
		for i, chunk := range chunks {
			sb.WriteString(fmt.Sprintf("### コンテンツ %d: %s (L%d-L%d)\n", i+1, chunk.FilePath, chunk.StartLine, chunk.EndLine))
			sb.WriteString(fmt.Sprintf("関連度: %.3f\n\n", chunk.Score))
			sb.WriteString("```\n")
			sb.WriteString(chunk.Content)
			sb.WriteString("\n```\n\n")
		}
	}

	// 指示
	sb.WriteString("## 指示\n\n")
	sb.WriteString("上記のコンテキストを基に、以下の形式でMarkdownドキュメントを生成してください：\n\n")

	switch config.Section {
	case SectionOverview:
		sb.WriteString(`1. **プロダクト概要**: プロダクトの目的と解決する課題
2. **主要機能・提供価値**: 提供する主要な機能や価値
3. **全体構造**: 高レベルの構造や構成の説明
4. **構成の特徴**: 構造上の重要な特徴や設計方針

`)
	case SectionTechStack:
		sb.WriteString(`1. **主要技術**: 使用している主要な技術やツール
2. **フレームワーク・ライブラリ**: 使用しているフレームワークやライブラリ
3. **プラットフォーム・インフラ**: 使用しているプラットフォームやインフラストラクチャ
4. **開発・運用ツール**: 開発や運用で使用しているツール
5. **依存関係**: 主要な外部依存関係

`)
	case SectionDataFlow:
		sb.WriteString(`1. **入力**: プロダクトへの情報やデータの入力
2. **処理フロー**: 情報やデータがどのように処理されるか
3. **変換・加工**: 情報やデータの変換や加工の詳細
4. **出力**: 処理結果や成果物の出力
5. **図解**: 可能であればMermaid図を含める

`)
	case SectionComponents:
		sb.WriteString(`1. **構成要素一覧**: 主要な構成要素のリスト
2. **各要素の説明**: 各構成要素の役割と責務
3. **関係性**: 構成要素間の関係性や依存関係
4. **図解**: 可能であればMermaid図を含める

`)
	}

	sb.WriteString("## 注意事項\n\n")
	sb.WriteString("- Markdown形式で出力してください\n")
	sb.WriteString("- コンテキストに情報がない場合は、その旨を記載してください\n")
	sb.WriteString("- 具体的な例や詳細情報がある場合は、適切にコードブロックや引用を使用してください\n")
	sb.WriteString("- 正確で分かりやすい記述を心がけてください\n")
	sb.WriteString("- 見出しは ## から始めてください（# は使用しないでください）\n\n")

	sb.WriteString("## 出力\n\n")
	sb.WriteString("Markdownドキュメント:\n")

	return sb.String()
}

// BuildFollowUpPrompt は追加情報が必要な場合のフォローアッププロンプトを構築する
func BuildFollowUpPrompt(config SectionConfig, initialContent string, additionalChunks []*search.SearchResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# タスク: %sセクションの追加情報による改善\n\n", config.Title))

	sb.WriteString("## 既存のコンテンツ\n\n")
	sb.WriteString("```markdown\n")
	sb.WriteString(initialContent)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## 追加のコンテキスト\n\n")
	for i, chunk := range additionalChunks {
		sb.WriteString(fmt.Sprintf("### コンテンツ %d: %s (L%d-L%d)\n", i+1, chunk.FilePath, chunk.StartLine, chunk.EndLine))
		sb.WriteString(fmt.Sprintf("関連度: %.3f\n\n", chunk.Score))
		sb.WriteString("```\n")
		sb.WriteString(chunk.Content)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("## 指示\n\n")
	sb.WriteString("追加のコンテキストを参考に、既存のコンテンツを改善してください。\n")
	sb.WriteString("新しい情報があれば追加し、不正確な情報があれば修正してください。\n\n")
	sb.WriteString("改善されたMarkdownドキュメント:\n")

	return sb.String()
}
