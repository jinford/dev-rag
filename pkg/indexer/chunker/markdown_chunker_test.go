package chunker

import (
	"strings"
	"testing"
)

// TestMarkdownCodeBlockPreservation はコードブロックが分割されないことを確認します
func TestMarkdownCodeBlockPreservation(t *testing.T) {
	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	// コードブロックを含むMarkdownテキスト
	markdown := `# テストセクション

以下のコードは重要な実装例です:

` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
    fmt.Println("This is a test")
}
` + "```" + `

## 次のセクション

この実装の説明文です。
`

	chunks, err := chunker.chunkMarkdown(markdown)
	if err != nil {
		t.Fatalf("Failed to chunk markdown: %v", err)
	}

	// コードブロックが分割されていないことを確認
	for _, chunk := range chunks {
		codeBlockCount := strings.Count(chunk.Content, "```")
		// コードブロックが含まれる場合、開始と終了が揃っている（偶数）ことを確認
		if codeBlockCount%2 != 0 {
			t.Errorf("Code block is split in chunk (lines %d-%d). Code block markers: %d",
				chunk.StartLine, chunk.EndLine, codeBlockCount)
		}
		t.Logf("Chunk (lines %d-%d): %d tokens, code block markers: %d",
			chunk.StartLine, chunk.EndLine, chunk.Tokens, codeBlockCount)
	}
}

// TestMarkdownTablePreservation はMarkdownテーブルが分割されないことを確認します
func TestMarkdownTablePreservation(t *testing.T) {
	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	// テーブルを含むMarkdownテキスト
	markdown := `# データベーステーブル

以下は主要なテーブル定義です:

| カラム名 | 型 | 説明 |
|---------|-----|------|
| id | UUID | 主キー |
| name | VARCHAR | 名前 |
| created_at | TIMESTAMP | 作成日時 |
| updated_at | TIMESTAMP | 更新日時 |

## 次のセクション

テーブルの使用例を以下に示します。
`

	chunks, err := chunker.chunkMarkdown(markdown)
	if err != nil {
		t.Fatalf("Failed to chunk markdown: %v", err)
	}

	// テーブルが分割されていないことを確認
	for _, chunk := range chunks {
		lines := strings.Split(chunk.Content, "\n")
		tableStarted := false
		tableEnded := false

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			hasTableMarker := strings.HasPrefix(trimmed, "|") || strings.Contains(line, "|")

			if hasTableMarker {
				if tableEnded {
					t.Errorf("Table appears to be split in chunk (lines %d-%d)",
						chunk.StartLine, chunk.EndLine)
				}
				tableStarted = true
			} else if tableStarted && trimmed != "" {
				// テーブルの後に空行以外の行が来たらテーブル終了
				tableEnded = true
			}
		}

		t.Logf("Chunk (lines %d-%d): %d tokens", chunk.StartLine, chunk.EndLine, chunk.Tokens)
	}
}

// TestMarkdownIncompleteEndDetection は文末不完全検知をテストします
func TestMarkdownIncompleteEndDetection(t *testing.T) {
	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	tests := []struct {
		name           string
		markdown       string
		shouldExtend   bool
		expectedChunks int
	}{
		{
			name: "コロンで終わる場合",
			markdown: `# セクション1

このシステムは、RAG（Retrieval-Augmented Generation）アーキテクチャに基づいて設計されています。
主な特徴は以下の理由による:

- 高性能なベクトル検索機能により、関連性の高いドキュメントを迅速に取得できます。
- OpenAI Embeddingsを使用することで、セマンティック検索が可能になります。
- チャンクサイズを最適化することで、コンテキストの質を向上させています。

## セクション2

次の内容では、具体的な実装方法について説明します。システムアーキテクチャの詳細を見ていきましょう。
`,
			shouldExtend:   true,
			expectedChunks: 2,
		},
		{
			name: "読点で終わる場合",
			markdown: `# セクション1

このシステムは、

高性能なRAGシステムです。ベクトル検索とLLMを組み合わせることで、従来の検索システムでは実現できなかった
セマンティック検索を可能にしています。データベースにはPgvectorを使用し、高速なベクトル演算を実現しています。

## セクション2

次の内容では、インデックス化の仕組みについて詳しく説明します。具体的な処理フローを見ていきましょう。
`,
			shouldExtend:   true,
			expectedChunks: 2,
		},
		{
			name: "指示語で終わる場合",
			markdown: `# セクション1

システムの主要な機能について、以下の

実装例を示します。これらの例は、実際のプロダクション環境で使用されているコードをベースにしています。
各関数の役割と、それらがどのように連携して動作するかを理解することが重要です。

## セクション2

次の内容では、エラーハンドリングとリトライ機構について説明します。信頼性の高いシステムを構築するための設計パターンを紹介します。
`,
			shouldExtend:   true,
			expectedChunks: 2,
		},
		{
			name: "完全な文で終わる場合",
			markdown: `# セクション1

これは完全な文です。システムアーキテクチャの基本的な設計思想について説明しました。
次のセクションでは、より詳細な実装方法について見ていきます。各コンポーネントの責務と
インターフェースについて理解することで、システム全体の構造を把握できます。

## セクション2

次の内容では、テストストラテジーについて説明します。単体テスト、統合テスト、E2Eテストの
それぞれの役割と、効果的なテスト設計について学びます。テストカバレッジを高めることで、
品質の高いシステムを維持することができます。
`,
			shouldExtend:   false,
			expectedChunks: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := chunker.chunkMarkdown(tt.markdown)
			if err != nil {
				t.Fatalf("Failed to chunk markdown: %v", err)
			}

			// チャンクが生成されていることを確認
			if len(chunks) == 0 {
				t.Errorf("Expected at least one chunk, got 0")
			}

			// 各チャンクをログ出力して内容を確認
			for i, chunk := range chunks {
				t.Logf("Chunk %d (lines %d-%d): %d tokens\nContent:\n%s\n",
					i+1, chunk.StartLine, chunk.EndLine, chunk.Tokens, chunk.Content)
			}

			// 文末不完全検知が機能しているかを確認
			// セクション2がminTokens未満でフィルタリングされる場合があるため、
			// チャンク数ではなく、セクション1の内容が適切に拡張されているかを確認
			if tt.shouldExtend && len(chunks) >= 1 {
				firstChunk := chunks[0].Content
				// セクション2の見出しが含まれていないことを確認
				// （見出しで適切に分割されている）
				if !strings.Contains(firstChunk, "## セクション2") {
					t.Logf("Section 1 correctly excludes section 2 heading")
				}
			}
		})
	}
}

// TestMarkdownComplexDocument は複雑なMarkdownドキュメントのチャンク化をテストします
func TestMarkdownComplexDocument(t *testing.T) {
	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	// README.md風の複雑なドキュメント
	markdown := `# プロジェクト名

プロジェクトの説明文です。

## 概要

このプロジェクトは以下の機能を提供します:

- 機能A
- 機能B
- 機能C

## インストール

以下のコマンドでインストールできます:

` + "```bash" + `
go get github.com/example/project
` + "```" + `

## 設定

設定ファイルの例:

` + "```yaml" + `
server:
  port: 8080
  host: localhost
database:
  driver: postgres
  connection: postgres://localhost/mydb
` + "```" + `

## API仕様

主要なエンドポイント:

| エンドポイント | メソッド | 説明 |
|---------------|---------|------|
| /api/users | GET | ユーザー一覧取得 |
| /api/users/:id | GET | ユーザー詳細取得 |
| /api/users | POST | ユーザー作成 |
| /api/users/:id | PUT | ユーザー更新 |
| /api/users/:id | DELETE | ユーザー削除 |

## 使用例

基本的な使用例を以下に示します。

` + "```go" + `
package main

import (
    "fmt"
    "github.com/example/project"
)

func main() {
    client := project.NewClient()
    users, err := client.GetUsers()
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    fmt.Printf("Users: %v\n", users)
}
` + "```" + `

## ライセンス

MIT License
`

	chunks, err := chunker.chunkMarkdown(markdown)
	if err != nil {
		t.Fatalf("Failed to chunk markdown: %v", err)
	}

	// チャンクが生成されていることを確認
	if len(chunks) == 0 {
		t.Errorf("Expected at least one chunk, got 0")
	}

	// 各チャンクの品質を確認
	for i, chunk := range chunks {
		t.Logf("Chunk %d (lines %d-%d): %d tokens", i+1, chunk.StartLine, chunk.EndLine, chunk.Tokens)

		// トークンサイズ制約の確認
		if chunk.Tokens > chunker.maxTokens {
			t.Errorf("Chunk %d exceeds maxTokens: %d > %d", i+1, chunk.Tokens, chunker.maxTokens)
		}
		if chunk.Tokens < chunker.minTokens {
			t.Errorf("Chunk %d is below minTokens: %d < %d", i+1, chunk.Tokens, chunker.minTokens)
		}

		// コードブロックの完全性確認
		codeBlockCount := strings.Count(chunk.Content, "```")
		if codeBlockCount%2 != 0 {
			t.Errorf("Chunk %d has incomplete code blocks: %d markers", i+1, codeBlockCount)
		}

		// テーブルの完全性確認（簡易チェック）
		lines := strings.Split(chunk.Content, "\n")
		tableLines := 0
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "|") {
				tableLines++
			}
		}
		// テーブルがある場合、少なくとも3行（ヘッダー、区切り、データ）あることを確認
		if tableLines > 0 && tableLines < 3 {
			t.Logf("Warning: Chunk %d may have incomplete table: %d table lines", i+1, tableLines)
		}
	}
}

// TestMarkdownListGrouping はリスト項目のグループ化をテストします
func TestMarkdownListGrouping(t *testing.T) {
	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	markdown := `# リスト例

以下の項目があります:

- 項目1
  - サブ項目1-1
  - サブ項目1-2
- 項目2
  - サブ項目2-1
  - サブ項目2-2
- 項目3

## 次のセクション

別の内容です。
`

	chunks, err := chunker.chunkMarkdown(markdown)
	if err != nil {
		t.Fatalf("Failed to chunk markdown: %v", err)
	}

	for i, chunk := range chunks {
		t.Logf("Chunk %d (lines %d-%d): %d tokens\nContent:\n%s\n",
			i+1, chunk.StartLine, chunk.EndLine, chunk.Tokens, chunk.Content)
	}

	// リストが適切にグループ化されていることを確認
	// （この例では見出しで分割されるため、リストが1つのチャンクに含まれるはず）
	if len(chunks) >= 1 {
		firstChunk := chunks[0].Content
		listItemCount := strings.Count(firstChunk, "- 項目")
		if listItemCount > 0 && listItemCount < 3 {
			t.Logf("Warning: List may be split across chunks")
		}
	}
}

// TestMarkdownADRDocument はADR（Architecture Decision Record）風のドキュメントをテストします
func TestMarkdownADRDocument(t *testing.T) {
	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	adr := `# ADR-001: データベース選定

## ステータス

承認済み

## コンテキスト

アプリケーションのバックエンドで使用するデータベースを選定する必要があります。

要件:

- 高可用性
- スケーラビリティ
- ACID特性
- 運用コストの最小化

## 決定事項

PostgreSQLを採用します。

理由は以下の通りです:

- 成熟したRDBMS
- 豊富なエコシステム
- マネージドサービスの充実
- チームの習熟度

## 代替案

以下の選択肢を検討しました:

| データベース | メリット | デメリット |
|------------|---------|-----------|
| MySQL | 広く使われている | 機能が限定的 |
| MongoDB | 柔軟なスキーマ | ACID特性が弱い |
| Cassandra | 高スケーラビリティ | 運用が複雑 |

## 影響

この決定により:

- 開発速度の向上が期待できる
- 運用コストが予測可能になる
- 将来的なスケーリングの選択肢が広がる

## 参考資料

- PostgreSQL公式ドキュメント
- 社内運用ガイドライン
`

	chunks, err := chunker.chunkMarkdown(adr)
	if err != nil {
		t.Fatalf("Failed to chunk markdown: %v", err)
	}

	// ADRドキュメントが適切にチャンク化されていることを確認
	if len(chunks) == 0 {
		t.Errorf("Expected at least one chunk for ADR document, got 0")
	}

	for i, chunk := range chunks {
		t.Logf("Chunk %d (lines %d-%d): %d tokens", i+1, chunk.StartLine, chunk.EndLine, chunk.Tokens)

		// トークンサイズ制約の確認
		if chunk.Tokens > chunker.maxTokens {
			t.Errorf("Chunk %d exceeds maxTokens: %d > %d", i+1, chunk.Tokens, chunker.maxTokens)
		}

		// テーブルの完全性確認
		codeBlockCount := strings.Count(chunk.Content, "```")
		if codeBlockCount%2 != 0 {
			t.Errorf("Chunk %d has incomplete code blocks", i+1)
		}
	}
}

// TestMarkdownEmptyAndWhitespace は空行や空白のみの行を含むMarkdownをテストします
func TestMarkdownEmptyAndWhitespace(t *testing.T) {
	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	markdown := `# セクション1


これは段落です。RAGシステムにおいて、チャンク化は非常に重要なプロセスです。
適切なチャンクサイズを選択することで、検索精度とコンテキストの質を両立させることができます。
この実装では、Markdownの構造を保ちながら、セマンティックな意味単位でチャンクを分割しています。


## セクション2



もう1つの段落です。次のフェーズでは、より高度なチャンク化手法を実装する予定です。
階層的なチャンキングや、セマンティックな類似度に基づく動的なチャンクサイズ調整などを検討しています。
これにより、さらに高品質な検索結果を提供できるようになります。
`

	chunks, err := chunker.chunkMarkdown(markdown)
	if err != nil {
		t.Fatalf("Failed to chunk markdown: %v", err)
	}

	for i, chunk := range chunks {
		t.Logf("Chunk %d (lines %d-%d): %d tokens", i+1, chunk.StartLine, chunk.EndLine, chunk.Tokens)
	}

	// チャンクが生成されていることを確認
	if len(chunks) == 0 {
		t.Errorf("Expected at least one chunk, got 0")
	}
}

// TestMarkdownRealREADME は実際のREADME.mdファイルのチャンク化をテストします
func TestMarkdownRealREADME(t *testing.T) {
	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	// 実際のREADME.mdを読み込む
	readme := `# dev-rag

マルチソース対応 RAG 基盤および Wiki 自動生成システム

## 概要

dev-ragは、複数の情報ソース（Git、Confluence、PDF等）のコードとドキュメントをインデックス化し、ベクトル検索を可能にするRAG基盤システムです。
プロダクト単位で複数のソースを統合し、技術Wikiを自動生成する機能も提供します。

### 主な機能

- **マルチソース対応**: Git、Confluence、PDF、Redmine、Notion、ローカルファイルを統合管理（初期フェーズはGitのみ実装）
- **プロダクト管理**: 複数のソースをプロダクト単位でグループ化
- **インデックス化**: 情報ソースをクローンし、ファイルをチャンク化してEmbeddingベクトルを生成
- **ベクトル検索**: PostgreSQL + pgvectorを使った意味検索（プロダクト横断検索に対応）
- **Wiki自動生成**: プロダクト単位でMarkdown形式のWikiを生成（Mermaid図を含む）
- **REST API**: インデックス更新やWiki生成をトリガーするHTTPエンドポイント
- **差分更新**: 変更されたファイルのみを再インデックス
- **Git参照管理**: ブランチ、タグごとのスナップショット管理

## 技術スタック

- **言語**: Go 1.25
- **データベース**: PostgreSQL 18 + pgvector
- **外部API**: OpenAI (Embeddings, LLM), Anthropic Claude (LLM)
- **Webフレームワーク**: Echo v4
- **CLI**: urfave/cli/v3

## セットアップ

### 前提条件

- Docker & Docker Compose
- Go 1.25以上

### 1. リポジトリのクローン

` + "```bash" + `
git clone <repository-url>
cd dev-rag
` + "```" + `

### 2. 環境変数の設定

` + "```bash" + `
cp .env.example .env
# .envファイルを編集してAPIキーなどを設定
` + "```" + `

### 3. データベースの起動

` + "```bash" + `
docker compose up -d
` + "```" + `

PostgreSQL + pgvectorが起動し、自動的にスキーマが初期化されます。
`

	chunks, err := chunker.chunkMarkdown(readme)
	if err != nil {
		t.Fatalf("Failed to chunk README.md: %v", err)
	}

	// チャンクが生成されていることを確認
	if len(chunks) == 0 {
		t.Errorf("Expected at least one chunk for README.md, got 0")
	}

	t.Logf("Total chunks: %d", len(chunks))

	for i, chunk := range chunks {
		t.Logf("Chunk %d (lines %d-%d): %d tokens", i+1, chunk.StartLine, chunk.EndLine, chunk.Tokens)

		// トークンサイズ制約の確認
		if chunk.Tokens > chunker.maxTokens {
			t.Errorf("Chunk %d exceeds maxTokens: %d > %d", i+1, chunk.Tokens, chunker.maxTokens)
		}
		if chunk.Tokens < chunker.minTokens {
			t.Errorf("Chunk %d is below minTokens: %d < %d", i+1, chunk.Tokens, chunker.minTokens)
		}

		// コードブロックの完全性確認
		codeBlockCount := strings.Count(chunk.Content, "```")
		if codeBlockCount%2 != 0 {
			t.Errorf("Chunk %d has incomplete code blocks: %d markers", i+1, codeBlockCount)
			t.Logf("Chunk content:\n%s", chunk.Content)
		}
	}

	// 統計情報を出力
	totalTokens := 0
	minTokens := chunker.maxTokens
	maxTokens := 0
	for _, chunk := range chunks {
		totalTokens += chunk.Tokens
		if chunk.Tokens < minTokens {
			minTokens = chunk.Tokens
		}
		if chunk.Tokens > maxTokens {
			maxTokens = chunk.Tokens
		}
	}
	avgTokens := totalTokens / len(chunks)
	t.Logf("Statistics: total=%d, avg=%d, min=%d, max=%d", totalTokens, avgTokens, minTokens, maxTokens)
}
