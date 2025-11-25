package summarizer

import (
	"fmt"
	"strings"

	"github.com/jinford/dev-rag/internal/module/wiki/domain"
)

// buildDirectorySummaryPrompt はディレクトリ要約生成用のプロンプトを構築する
// セクションB: DirectorySummarizer用プロンプト（docs/architecture-wiki-prompt-template.md）に従う
func buildDirectorySummaryPrompt(directory *domain.DirectoryInfo, filesContent, subdirContent string) string {
	// 言語リストの構築
	var languages []string
	for lang := range directory.Languages {
		languages = append(languages, lang)
	}
	languageList := strings.Join(languages, ", ")
	if languageList == "" {
		languageList = "不明"
	}

	// 親ディレクトリパスの表示
	parentDisplay := directory.ParentPath
	if parentDisplay == "" {
		parentDisplay = "(ルート)"
	}

	// プロンプト構築
	var builder strings.Builder

	builder.WriteString("あなたはソフトウェアアーキテクトです。以下の情報を基に、ディレクトリ `")
	builder.WriteString(directory.Path)
	builder.WriteString("` の要約をMarkdownで作成してください。\n\n")

	builder.WriteString("## メタ情報\n")
	builder.WriteString(fmt.Sprintf("- 親: %s\n", parentDisplay))
	builder.WriteString(fmt.Sprintf("- 深さ: %d\n", directory.Depth))
	builder.WriteString(fmt.Sprintf("- 使用言語: %s\n", languageList))
	builder.WriteString(fmt.Sprintf("- 直下ファイル数: %d\n", len(directory.Files)))
	builder.WriteString(fmt.Sprintf("- サブディレクトリ数: %d\n\n", len(directory.Subdirectories)))

	// ファイル要約セクション
	if filesContent != "" {
		builder.WriteString("## ファイル要約\n")
		builder.WriteString(filesContent)
		builder.WriteString("\n\n")
	}

	// サブディレクトリ要約セクション
	if subdirContent != "" {
		builder.WriteString("## サブディレクトリ要約\n")
		builder.WriteString(subdirContent)
		builder.WriteString("\n\n")
	}

	// 出力要件
	builder.WriteString("出力は「主な責務 / 含まれる機能 / 親ディレクトリとの関係 / 主要なファイル」の順で、")
	builder.WriteString("各セクションに必ず本文を入れてください。不明な事項は「不明」と書いてください。\n\n")

	builder.WriteString("出力形式:\n")
	builder.WriteString("### 主な責務\n")
	builder.WriteString("(1-2文でこのディレクトリの主な責務を説明)\n\n")
	builder.WriteString("### 含まれる機能\n")
	builder.WriteString("(3-5行の箇条書きで主要な機能を列挙)\n\n")
	builder.WriteString("### 親ディレクトリとの関係\n")
	builder.WriteString("(1-2文で親ディレクトリとの関係を説明)\n\n")
	builder.WriteString("### 主要なファイル\n")
	builder.WriteString("(3-6行で「ファイル名: 役割」の形式で列挙)\n")

	return builder.String()
}

// buildArchitectureSummaryPrompt はアーキテクチャ要約生成用のプロンプトを構築する
// セクションA: ArchitectureSummarizer用プロンプト（docs/architecture-wiki-prompt-template.md）に従う
func buildArchitectureSummaryPrompt(structure *domain.RepoStructure, directorySummariesContent, summaryType string) string {
	var builder strings.Builder

	builder.WriteString("あなたはソフトウェアアーキテクトです。以下のディレクトリ要約だけを根拠に、`")
	builder.WriteString(summaryType)
	builder.WriteString("` に対応するアーキテクチャ要約をMarkdownで出力してください。\n\n")

	// リポジトリ統計
	builder.WriteString("## リポジトリ統計\n")
	builder.WriteString(fmt.Sprintf("- ディレクトリ数: %d\n", len(structure.Directories)))
	builder.WriteString(fmt.Sprintf("- ファイル数: %d\n\n", len(structure.Files)))

	// ディレクトリ要約
	builder.WriteString("## 全ディレクトリ要約\n")
	builder.WriteString(directorySummariesContent)
	builder.WriteString("\n\n")

	// summary_type別の出力要件
	builder.WriteString("出力要件:\n")
	builder.WriteString("- summary_type=")
	builder.WriteString(summaryType)
	builder.WriteString(" に対応したセクション構造を守る。\n")
	builder.WriteString("- 根拠が無い場合は「不明」と明記。\n")
	builder.WriteString("- 簡潔な日本語で記述。\n\n")

	switch summaryType {
	case "overview":
		builder.WriteString("出力形式:\n")
		builder.WriteString("### システム概要\n")
		builder.WriteString("(2-3文でシステムの目的と全体像を説明)\n\n")
		builder.WriteString("### 主要機能\n")
		builder.WriteString("(3-5行の箇条書きで主要機能を列挙)\n\n")
		builder.WriteString("### アーキテクチャパターン\n")
		builder.WriteString("(採用しているパターンやレイヤー分離を箇条書き)\n")

	case "tech_stack":
		builder.WriteString("出力形式:\n")
		builder.WriteString("### 言語\n")
		builder.WriteString("(使用されている主要なプログラミング言語)\n\n")
		builder.WriteString("### フレームワーク・ライブラリ\n")
		builder.WriteString("(使用されている主要なフレームワークとライブラリ)\n\n")
		builder.WriteString("### データベース・ストレージ\n")
		builder.WriteString("(使用されているDBやストレージ)\n\n")
		builder.WriteString("### 外部サービス\n")
		builder.WriteString("(統合されている外部サービスやAPI)\n")

	case "data_flow":
		builder.WriteString("出力形式:\n")
		builder.WriteString("### エントリーポイント\n")
		builder.WriteString("(システムの起動点や入口)\n\n")
		builder.WriteString("### 処理フロー\n")
		builder.WriteString("(3-5ステップで主要な処理の流れを説明)\n\n")
		builder.WriteString("### 永続化\n")
		builder.WriteString("(データの保存・取得方法)\n")

	case "components":
		builder.WriteString("出力形式:\n")
		builder.WriteString("### 主要コンポーネント\n")
		builder.WriteString("(3-6個のコンポーネントを「名前: 役割」の形式で列挙)\n\n")
		builder.WriteString("### 関係性\n")
		builder.WriteString("(コンポーネント間の依存関係や通信方法)\n\n")
		builder.WriteString("### レイヤー構成\n")
		builder.WriteString("(レイヤー構造がある場合、その説明)\n")
	}

	return builder.String()
}
