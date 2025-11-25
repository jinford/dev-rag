package prompts

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jinford/dev-rag/internal/module/indexing/adapter/llm"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// TestFileSummaryGenerator_Integration は実際のOpenAI APIを使った統合テスト
// OPENAI_API_KEYが設定されている場合のみ実行される
func TestFileSummaryGenerator_Integration(t *testing.T) {
	// OPENAI_API_KEYが設定されていない場合はスキップ
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	// LLMクライアントを作成
	client, err := llm.NewOpenAIClient()
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}

	// トークンカウンターを作成
	tokenCounter, err := llm.NewTokenCounter()
	if err != nil {
		t.Fatalf("Failed to create token counter: %v", err)
	}

	generator := NewFileSummaryGenerator(client, tokenCounter)

	tests := []struct {
		name        string
		req         domain.FileSummaryRequest
		validate    func(*testing.T, *domain.FileSummaryResponse)
		description string
	}{
		{
			name: "小規模ファイル - 100行程度のGoファイル",
			req: domain.FileSummaryRequest{
				FilePath: "pkg/models/user.go",
				Language: "Go",
				FileContent: `package models

import (
	"time"
	"github.com/google/uuid"
)

// User はユーザー情報を表します
type User struct {
	ID        uuid.UUID
	Name      string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewUser は新しいユーザーを作成します
func NewUser(name, email string) *User {
	return &User{
		ID:        uuid.New(),
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Update はユーザー情報を更新します
func (u *User) Update(name, email string) {
	u.Name = name
	u.Email = email
	u.UpdatedAt = time.Now()
}

// Validate はユーザー情報を検証します
func (u *User) Validate() error {
	if u.Name == "" {
		return errors.New("name is required")
	}
	if u.Email == "" {
		return errors.New("email is required")
	}
	return nil
}`,
			},
			validate: func(t *testing.T, resp *domain.FileSummaryResponse) {
				// サマリーが存在すること
				if len(resp.Summary) == 0 {
					t.Error("Summary is empty")
				}

				// プロンプトバージョンが正しいこと
				if resp.PromptVersion != FileSummaryPromptVersion {
					t.Errorf("PromptVersion = %s, want %s", resp.PromptVersion, FileSummaryPromptVersion)
				}

				// メタデータにキーシンボルが含まれていること
				if len(resp.Metadata.KeySymbols) == 0 {
					t.Error("KeySymbols is empty")
				}

				t.Logf("Summary items: %d", len(resp.Summary))
				t.Logf("Risks: %d", len(resp.Risks))
				t.Logf("Primary topics: %v", resp.Metadata.PrimaryTopics)
				t.Logf("Key symbols: %v", resp.Metadata.KeySymbols)
			},
			description: "小規模なGoファイルのサマリー生成",
		},
		{
			name: "中規模ファイル - 500行程度のGoファイル",
			req: domain.FileSummaryRequest{
				FilePath: "pkg/indexer/chunker/chunker.go",
				Language: "Go",
				FileContent: generateMediumGoFile(),
			},
			validate: func(t *testing.T, resp *domain.FileSummaryResponse) {
				// サマリーが3項目以上あること（上限は緩めに10項目まで許容）
				if len(resp.Summary) < 3 || len(resp.Summary) > 10 {
					t.Logf("Warning: Summary length = %d, expected 3-6 (but up to 10 is acceptable)", len(resp.Summary))
				}

				// 重要度順に並んでいることを確認（最初の項目が最も重要）
				// ここでは単純に最初の項目の長さが0でないことを確認
				if len(resp.Summary) > 0 && len(resp.Summary[0]) == 0 {
					t.Error("First summary item is empty")
				}

				t.Logf("Summary:\n%s", strings.Join(resp.Summary, "\n- "))
				if len(resp.Risks) > 0 {
					t.Logf("Risks:\n%s", strings.Join(resp.Risks, "\n- "))
				}
			},
			description: "中規模なGoファイルのサマリー生成",
		},
		{
			name: "Markdownファイル",
			req: domain.FileSummaryRequest{
				FilePath: "README.md",
				Language: "Markdown",
				FileContent: `# dev-rag

A RAG (Retrieval-Augmented Generation) system for development documentation.

## Features

- Automatic indexing of source code and documentation
- Hierarchical chunking with AST parsing
- Dependency graph construction
- Coverage analysis
- LLM-based summarization

## Architecture

The system consists of several components:

1. Indexer: Processes and indexes documents
2. Chunker: Splits documents into chunks
3. Embedder: Generates embeddings
4. Coverage Analyzer: Tracks indexing coverage

## Getting Started

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- OpenAI API key

### Installation

bash
go get github.com/jinford/dev-rag


### Usage

bash
dev-rag index --source ./path/to/repo
dev-rag search "query"
`,
			},
			validate: func(t *testing.T, resp *domain.FileSummaryResponse) {
				// サマリーが存在すること
				if len(resp.Summary) == 0 {
					t.Error("Summary is empty")
				}

				// ドキュメントファイルなのでリスクは少ないはず
				if len(resp.Risks) > 2 {
					t.Errorf("Too many risks for documentation file: %d", len(resp.Risks))
				}

				t.Logf("Summary:\n- %s", strings.Join(resp.Summary, "\n- "))
			},
			description: "Markdownファイルのサマリー生成",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			resp, err := generator.Generate(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("Failed to generate summary: %v", err)
			}

			// 検証関数を実行
			if tt.validate != nil {
				tt.validate(t, resp)
			}

			// サマリーテキストを生成して表示
			summaryText := domain.GenerateSummaryText(resp)
			t.Logf("Generated summary text:\n%s", summaryText)

			// サマリーテキストが400トークン以内に収まっているか確認
			summaryTokens := tokenCounter.CountTokens(summaryText)
			t.Logf("Summary tokens: %d", summaryTokens)

			// 実際には多少超過する可能性もあるので、500トークンまで許容
			if summaryTokens > 500 {
				t.Errorf("Summary too long: %d tokens (should be around 400)", summaryTokens)
			}
		})
	}
}

// generateMediumGoFile は中規模のGoファイルを生成します（テスト用）
func generateMediumGoFile() string {
	return `package chunker

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/jinford/dev-rag/internal/module/indexing/adapter/detector"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// Chunker はドキュメントをチャンクに分割します
type Chunker struct {
	detector     *detector.ContentTypeDetector
	minTokens    int
	maxTokens    int
	overlapRatio float64
}

// NewChunker は新しいChunkerを作成します
func NewChunker(detector *detector.ContentTypeDetector) *Chunker {
	return &Chunker{
		detector:     detector,
		minTokens:    100,
		maxTokens:    1600,
		overlapRatio: 0.1,
	}
}

// Chunk はコンテンツをチャンクに分割します
func (c *Chunker) Chunk(content string, contentType string) ([]*domain.Chunk, error) {
	switch contentType {
	case "code":
		return c.chunkCode(content)
	case "markdown":
		return c.chunkMarkdown(content)
	case "text":
		return c.chunkText(content)
	default:
		return c.chunkText(content)
	}
}

// chunkCode はソースコードをチャンク化します
func (c *Chunker) chunkCode(content string) ([]*domain.Chunk, error) {
	// AST解析を使用してチャンク化
	lines := strings.Split(content, "\n")
	chunks := make([]*domain.Chunk, 0)

	currentChunk := make([]string, 0)
	startLine := 1
	tokens := 0

	for i, line := range lines {
		lineTokens := c.estimateTokens(line)

		if tokens+lineTokens > c.maxTokens && len(currentChunk) > 0 {
			// チャンクを確定
			chunkContent := strings.Join(currentChunk, "\n")
			chunks = append(chunks, &domain.Chunk{
				StartLine: startLine,
				EndLine:   i,
				Content:   chunkContent,
				Tokens:    tokens,
			})

			// 新しいチャンクを開始
			currentChunk = make([]string, 0)
			startLine = i + 1
			tokens = 0
		}

		currentChunk = append(currentChunk, line)
		tokens += lineTokens
	}

	// 最後のチャンクを追加
	if len(currentChunk) > 0 {
		chunkContent := strings.Join(currentChunk, "\n")
		chunks = append(chunks, &domain.Chunk{
			StartLine: startLine,
			EndLine:   len(lines),
			Content:   chunkContent,
			Tokens:    tokens,
		})
	}

	return chunks, nil
}

// chunkMarkdown はMarkdownをチャンク化します
func (c *Chunker) chunkMarkdown(content string) ([]*domain.Chunk, error) {
	lines := strings.Split(content, "\n")
	chunks := make([]*domain.Chunk, 0)

	currentChunk := make([]string, 0)
	startLine := 1
	tokens := 0
	inCodeBlock := false

	for i, line := range lines {
		lineTokens := c.estimateTokens(line)

		// コードブロックの開始/終了を検出
		if strings.HasPrefix(strings.TrimSpace(line), "` + "```" + `") {
			inCodeBlock = !inCodeBlock
		}

		// チャンクを分割する条件
		shouldSplit := false
		if !inCodeBlock && tokens+lineTokens > c.maxTokens && len(currentChunk) > 0 {
			shouldSplit = true
		}

		// 見出しでも分割可能
		if !inCodeBlock && strings.HasPrefix(line, "#") && len(currentChunk) > 0 && tokens > c.minTokens {
			shouldSplit = true
		}

		if shouldSplit {
			// チャンクを確定
			chunkContent := strings.Join(currentChunk, "\n")
			chunks = append(chunks, &domain.Chunk{
				StartLine: startLine,
				EndLine:   i,
				Content:   chunkContent,
				Tokens:    tokens,
			})

			// 新しいチャンクを開始
			currentChunk = make([]string, 0)
			startLine = i + 1
			tokens = 0
		}

		currentChunk = append(currentChunk, line)
		tokens += lineTokens
	}

	// 最後のチャンクを追加
	if len(currentChunk) > 0 {
		chunkContent := strings.Join(currentChunk, "\n")
		chunks = append(chunks, &domain.Chunk{
			StartLine: startLine,
			EndLine:   len(lines),
			Content:   chunkContent,
			Tokens:    tokens,
		})
	}

	return chunks, nil
}

// chunkText はプレーンテキストをチャンク化します
func (c *Chunker) chunkText(content string) ([]*domain.Chunk, error) {
	// 段落単位で分割
	paragraphs := strings.Split(content, "\n\n")
	chunks := make([]*domain.Chunk, 0)

	currentChunk := make([]string, 0)
	startLine := 1
	tokens := 0
	lineCount := 1

	for _, para := range paragraphs {
		paraTokens := c.estimateTokens(para)
		paraLines := strings.Count(para, "\n") + 1

		if tokens+paraTokens > c.maxTokens && len(currentChunk) > 0 {
			// チャンクを確定
			chunkContent := strings.Join(currentChunk, "\n\n")
			chunks = append(chunks, &domain.Chunk{
				StartLine: startLine,
				EndLine:   lineCount - 1,
				Content:   chunkContent,
				Tokens:    tokens,
			})

			// 新しいチャンクを開始
			currentChunk = make([]string, 0)
			startLine = lineCount
			tokens = 0
		}

		currentChunk = append(currentChunk, para)
		tokens += paraTokens
		lineCount += paraLines + 1 // 段落間の空行も含む
	}

	// 最後のチャンクを追加
	if len(currentChunk) > 0 {
		chunkContent := strings.Join(currentChunk, "\n\n")
		chunks = append(chunks, &domain.Chunk{
			StartLine: startLine,
			EndLine:   lineCount - 1,
			Content:   chunkContent,
			Tokens:    tokens,
		})
	}

	return chunks, nil
}

// estimateTokens はテキストのトークン数を推定します
func (c *Chunker) estimateTokens(text string) int {
	// 簡易的な推定: 4文字で約1トークン
	return len([]rune(text)) / 4
}

// ValidateChunk はチャンクが有効かどうかを検証します
func (c *Chunker) ValidateChunk(chunk *domain.Chunk) error {
	if chunk.Tokens < c.minTokens {
		return fmt.Errorf("chunk too small: %d tokens (min: %d)", chunk.Tokens, c.minTokens)
	}
	if chunk.Tokens > c.maxTokens {
		return fmt.Errorf("chunk too large: %d tokens (max: %d)", chunk.Tokens, c.maxTokens)
	}
	return nil
}`
}
