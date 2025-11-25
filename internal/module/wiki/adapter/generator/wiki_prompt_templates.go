package generator

import (
	"fmt"
	"strings"

	"github.com/jinford/dev-rag/internal/module/wiki/domain"
)

// buildArchitectureWikiPrompt はアーキテクチャWikiページ生成用のプロンプトを構築する
//
// このプロンプトは docs/architecture-wiki-prompt-template.md のセクション5に基づく。
// アーキテクチャ要約（最優先）とRAG検索結果（補足）を統合して、
// システム全体のアーキテクチャをMarkdown形式で説明するWikiページを生成する。
//
// 入力:
//   - architectureSummaries: アーキテクチャ要約のリスト（overview, tech_stack, data_flow, components）
//   - searchResults: RAG検索結果
//   - pseudoQuery: 疑似クエリ文字列
//   - maxTokens: コンテキスト長の上限（トークン数）
//
// 出力:
//   - LLMに渡すプロンプト文字列
func buildArchitectureWikiPrompt(
	architectureSummaries []string,
	searchResults []*domain.SearchResult,
	pseudoQuery string,
	maxTokens int,
) string {
	// コンテキストの準備
	summariesText := formatSummaries(architectureSummaries)

	// RAG検索結果をコンテキスト長に合わせて切り詰める
	ragItems := make([]string, 0, len(searchResults))
	for _, result := range searchResults {
		ragItems = append(ragItems, fmt.Sprintf(
			"**%s** (行 %d-%d, スコア: %.3f)\n```\n%s\n```",
			result.FilePath,
			result.StartLine,
			result.EndLine,
			result.Score,
			result.Content,
		))
	}

	truncatedRAG, wasTruncated := truncateContext(ragItems, maxTokens/2) // 半分をRAG用に割り当て
	ragText := strings.Join(truncatedRAG, "\n\n")
	if wasTruncated {
		ragText += "\n\n... (残りの検索結果は省略されました)"
	}

	// プロンプトテンプレート（docs/architecture-wiki-prompt-template.md セクション5に基づく）
	template := `あなたは技術ドキュメントの専門家です。以下のコンテキストだけを根拠に、アーキテクチャWikiページをMarkdownで作成してください。

<CONTEXT>
## アーキテクチャ要約
%s

## RAG検索結果
%s

## ユーザークエリ（擬似）
%s
</CONTEXT>

出力要件:
- 以下のセクション構造・順序を厳守する。
- 内容はすべてコンテキストに基づく。欠けていれば「不明」と記載。
- 言語: 日本語。Markdownのみ。箇条書きは短文で簡潔に。
- Mermaid: flowchart TD または sequenceDiagram を使用。ノードは12個以内。ラベルは20文字以内。
- ファイル参照: パスはバッククォートで囲む（例: ` + "`pkg/wiki/generator/prompt_builder.go`" + `）。
- 長さ: 全文で1200語以内。各セクションは見出し直下に本文を置き、空のセクションを作らない。
- 禁止事項: TODO/仮置き表現、外部リンク、HTMLタグ、無根拠の推測。

## システム概要
- 2〜3文でシステムの目的と全体像。

## アーキテクチャパターン
- 採用しているパターンやレイヤー分離を箇条書きで。

## 主要コンポーネント
- 3〜6個のコンポーネントを「名前: 役割」の形式で列挙。

## データフロー
` + "```mermaid" + `
flowchart TD
  %% 12ノード以内で構成
  ENTRY[エントリポイント]
  PROC1[主要処理1]
  PROC2[主要処理2]
  STORE[(永続化)]
  ENTRY --> PROC1 --> PROC2 --> STORE
` + "```" + `

## 技術スタック
- 言語 / フレームワーク / データストア / 外部サービスを短く列挙。

## 根拠となるファイル/要約
- ` + "`path`" + `: どの情報を参照したかを1行ずつ列挙。`

	return fmt.Sprintf(template, summariesText, ragText, pseudoQuery)
}

// buildDirectoryWikiPrompt はディレクトリWikiページ生成用のプロンプトを構築する
//
// このプロンプトは docs/architecture-wiki-prompt-template.md のセクションCに基づく。
// ディレクトリ要約（優先）とRAG検索結果（実装詳細）を統合して、
// ディレクトリの責務と実装詳細をMarkdown形式で説明するWikiページを生成する。
//
// 入力:
//   - directorySummary: ディレクトリ要約
//   - searchResults: RAG検索結果（パスフィルタ適用済み）
//   - pseudoQuery: 疑似クエリ文字列
//   - maxTokens: コンテキスト長の上限（トークン数）
//
// 出力:
//   - LLMに渡すプロンプト文字列
func buildDirectoryWikiPrompt(
	directorySummary string,
	searchResults []*domain.SearchResult,
	pseudoQuery string,
	maxTokens int,
) string {
	// ディレクトリ要約の準備
	summaryText := directorySummary
	if summaryText == "" {
		summaryText = "（ディレクトリ要約が見つかりませんでした）"
	}

	// RAG検索結果をコンテキスト長に合わせて切り詰める
	ragItems := make([]string, 0, len(searchResults))
	for _, result := range searchResults {
		ragItems = append(ragItems, fmt.Sprintf(
			"**%s** (行 %d-%d, スコア: %.3f)\n```\n%s\n```",
			result.FilePath,
			result.StartLine,
			result.EndLine,
			result.Score,
			result.Content,
		))
	}

	truncatedRAG, wasTruncated := truncateContext(ragItems, maxTokens/2) // 半分をRAG用に割り当て
	ragText := strings.Join(truncatedRAG, "\n\n")
	if wasTruncated {
		ragText += "\n\n... (残りの検索結果は省略されました)"
	}
	if ragText == "" {
		ragText = "（検索結果が見つかりませんでした）"
	}

	// プロンプトテンプレート（docs/architecture-wiki-prompt-template.md セクションCに基づく）
	template := `あなたは技術ドキュメントの専門家です。以下のコンテキストを基にディレクトリWikiページをMarkdownで生成してください。

<CONTEXT>
## ディレクトリ要約
%s

## RAG検索結果
%s

## 擬似クエリ
%s
</CONTEXT>

出力要件:
- 見出しは「責務 / 含まれる機能 / 主要ファイル / 処理フロー」の順。
- 言語: 日本語。Markdownのみ。
- Mermaidを入れる場合は flowchart または sequenceDiagram を使用。ノードは12個以内。
- パス表記はバッククォート。
- 不明点は「不明」と明記。
- 禁止事項: TODO/仮置き表現、外部リンク、HTMLタグ、無根拠の推測。

## 責務
このディレクトリの主な責務を1-2文で説明してください。

## 含まれる機能
このディレクトリに含まれる主要な機能を箇条書き（3-5行）で列挙してください。

## 主要ファイル
このディレクトリ内の主要なファイルを「` + "`ファイル名`" + `: 役割」の形式で3-6行列挙してください。

## 処理フロー
可能であれば、このディレクトリ内の処理フローをMermaid図で示してください。
図が不要な場合は、テキストで簡潔に説明してください。`

	return fmt.Sprintf(template, summaryText, ragText, pseudoQuery)
}

// buildArchitectureSummaryPrompt はアーキテクチャ要約生成用のプロンプトを構築する
//
// このプロンプトは docs/architecture-wiki-prompt-template.md のセクションAに基づく。
// ディレクトリ要約を抽象化し、アーキテクチャレベルの要約を生成する。
//
// 注意: この関数は pkg/wiki/summarizer パッケージから使用されることを想定しているが、
//
//	テンプレート関数として generator パッケージに配置する。
//
// 入力:
//   - repositoryStats: リポジトリ統計情報（ディレクトリ数/ファイル数など）
//   - directorySummaries: 全ディレクトリ要約（Markdown形式、深さ順に連結）
//   - summaryType: 要約種別（overview/tech_stack/data_flow/components）
//
// 出力:
//   - LLMに渡すプロンプト文字列
func BuildArchitectureSummaryPrompt(
	repositoryStats string,
	directorySummaries string,
	summaryType string,
) string {
	// プロンプトテンプレート（docs/architecture-wiki-prompt-template.md セクションAに基づく）
	template := `あなたはソフトウェアアーキテクトです。以下のディレクトリ要約だけを根拠に、%s に対応するアーキテクチャ要約をMarkdownで出力してください。

## リポジトリ統計
%s

## 全ディレクトリ要約
%s

出力要件:
- summary_type=%s に対応したセクション構造を守る。
- 根拠が無い場合は「不明」と明記。
- 簡潔な日本語で記述。
- 文字数上限: 900語。
- 箇条書きは5行以内。例やファイルパスは不要。

%s`

	// summary_type別のセクション指示
	var sectionInstructions string
	switch summaryType {
	case "overview":
		sectionInstructions = `
### システム概要
システムの目的と全体像を2-3文で記述してください。

### 主要機能
システムの主要機能を3-5行の箇条書きで列挙してください。

### アーキテクチャパターン
採用されているアーキテクチャパターンを2-4行の箇条書きで記述してください。`

	case "tech_stack":
		sectionInstructions = `
### 言語
使用されているプログラミング言語を列挙してください。

### フレームワーク・ライブラリ
使用されている主要なフレームワークとライブラリを列挙してください。

### データベース・ストレージ
使用されているデータベースやストレージを列挙してください。

### 外部サービス
連携している外部サービスがあれば列挙してください。`

	case "data_flow":
		sectionInstructions = `
### エントリーポイント
システムのエントリーポイントを1-2文で説明してください。

### 処理フロー
主要な処理フローを3-5ステップの箇条書きで記述してください。

### 永続化
データの永続化方法を1-2文で説明してください。`

	case "components":
		sectionInstructions = `
### 主要コンポーネント
システムの主要コンポーネントを「名前: 役割」の形式で3-6個列挙してください。

### 関係性
コンポーネント間の主要な関係性を2-4行の箇条書きで記述してください。

### レイヤー構成
レイヤー構成がある場合、それを2-4行の箇条書きで記述してください。`

	default:
		sectionInstructions = "適切なセクション構造で要約を作成してください。"
	}

	return fmt.Sprintf(template, summaryType, repositoryStats, directorySummaries, summaryType, sectionInstructions)
}

// BuildDirectorySummaryPrompt はディレクトリ要約生成用のプロンプトを構築する
//
// このプロンプトは docs/architecture-wiki-prompt-template.md のセクションBに基づく。
// ディレクトリ直下のファイル要約を統合し、ディレクトリレベルの要約を生成する。
//
// 注意: この関数は pkg/wiki/summarizer パッケージから使用されることを想定しているが、
//
//	テンプレート関数として generator パッケージに配置する。
//
// 入力:
//   - directoryPath: ディレクトリパス
//   - parentPath: 親ディレクトリパス
//   - depth: 深さ
//   - languageList: 使用言語リスト
//   - fileCount: 直下ファイル数
//   - subdirCount: サブディレクトリ数
//   - fileSummaries: ファイル要約（Markdown形式）
//   - subdirSummaries: サブディレクトリ要約（Markdown形式）
//
// 出力:
//   - LLMに渡すプロンプト文字列
func BuildDirectorySummaryPrompt(
	directoryPath string,
	parentPath string,
	depth int,
	languageList string,
	fileCount int,
	subdirCount int,
	fileSummaries string,
	subdirSummaries string,
) string {
	// ファイル要約がない場合のデフォルト値
	if fileSummaries == "" {
		fileSummaries = "（ファイル要約が見つかりませんでした）"
	}

	// サブディレクトリ要約のセクション
	subdirSection := ""
	if subdirSummaries != "" {
		subdirSection = fmt.Sprintf(`

## サブディレクトリ要約
%s`, subdirSummaries)
	}

	// プロンプトテンプレート（docs/architecture-wiki-prompt-template.md セクションBに基づく）
	template := `あなたはソフトウェアアーキテクトです。以下の情報を基に、ディレクトリ %s の要約をMarkdownで作成してください。

## メタ情報
- パス: %s
- 親: %s
- 深さ: %d
- 使用言語: %s
- 直下ファイル数: %d
- サブディレクトリ数: %d

## ファイル要約
%s%s

出力は「主な責務 / 含まれる機能 / 親ディレクトリとの関係 / 主要なファイル」の順で、各セクションに必ず本文を入れてください。不明な事項は「不明」と書いてください。

制約:
- 1200語以内。日本語Markdown。過剰な推測は禁止。
- 根拠はファイル要約とサブディレクトリ要約のみ。
- 箇条書きは短文で簡潔に。

### 主な責務
このディレクトリの主な責務を1-2文で説明してください。

### 含まれる機能
このディレクトリに含まれる機能を3-5行の箇条書きで列挙してください。

### 親ディレクトリとの関係
親ディレクトリとの関係を1-2文で説明してください。

### 主要なファイル
主要なファイルを「ファイル名: 役割」の形式で3-6行列挙してください。`

	return fmt.Sprintf(
		template,
		directoryPath,
		directoryPath,
		parentPath,
		depth,
		languageList,
		fileCount,
		subdirCount,
		fileSummaries,
		subdirSection,
	)
}
