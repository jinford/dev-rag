package ast_test

import (
	"strings"
	"testing"

	"github.com/jinford/dev-rag/internal/core/indexing/chunker"
	"github.com/jinford/dev-rag/internal/core/indexing/chunker/ast"
)

func TestASTChunkerGo_WithFileSummary(t *testing.T) {
	t.Skip("ファイルサマリー機能は未実装のため、このテストをスキップします")

	defaultChunker, err := chunker.NewDefaultChunker()
	if err != nil {
		t.Fatalf("failed to create chunker: %v", err)
	}

	astChunker := ast.NewASTChunkerGo()

	testCode := `// Package calculator は計算機能を提供します。
// このパッケージは基本的な算術演算をサポートします。
package calculator

import (
\t"github.com/jinford/dev-rag/internal/core/indexing/chunker/ast"
	"fmt"
	"math"
	"github.com/example/logger"
)

// Add は2つの数値を加算します
func Add(a, b int) int {
	return a + b
}

// Subtract は2つの数値を減算します
func Subtract(a, b int) int {
	return a - b
}

// Calculator は計算機の構造体です
type Calculator struct {
	Result float64
}

// Multiply はCalculatorのメソッドです
func (c *Calculator) Multiply(a, b float64) float64 {
	result := a * b
	c.Result = result
	return result
}

// Config は設定を保持します
type Config struct {
	Precision int
}
`

	result := astChunker.ChunkWithMetrics(testCode, defaultChunker)

	// AST解析が成功することを確認
	if !result.ParseSuccess {
		t.Fatalf("AST parse should succeed, got error: %v", result.ParseError)
	}

	// チャンクが生成されることを確認
	if len(result.Chunks) == 0 {
		t.Fatalf("should generate at least one chunk")
	}

	// 最初のチャンクがファイルサマリー（Level 1）であることを確認
	summaryChunk := result.Chunks[0]
	if summaryChunk.Metadata == nil {
		t.Fatalf("first chunk should have metadata")
	}

	if summaryChunk.Metadata.Level != 1 {
		t.Errorf("first chunk should be level 1 (file summary), got: %d", summaryChunk.Metadata.Level)
	}

	if summaryChunk.Metadata.Type == nil || *summaryChunk.Metadata.Type != "file_summary" {
		t.Errorf("first chunk type should be 'file_summary', got: %v", summaryChunk.Metadata.Type)
	}

	t.Logf("File Summary Chunk:\n%s", summaryChunk.Chunk.Content)

	// サマリーチャンクの内容を検証
	summaryContent := summaryChunk.Chunk.Content

	// Language情報が含まれることを確認
	if !strings.Contains(summaryContent, "Language: Go") {
		t.Errorf("summary should contain 'Language: Go'")
	}

	// 主要コンポーネントが含まれることを確認
	if !strings.Contains(summaryContent, "Add") {
		t.Errorf("summary should contain function 'Add'")
	}

	if !strings.Contains(summaryContent, "Calculator") {
		t.Errorf("summary should contain struct 'Calculator'")
	}

	// 外部依存が含まれることを確認
	if !strings.Contains(summaryContent, "github.com/example/logger") {
		t.Errorf("summary should contain external dependency")
	}

	// トークン数が400以内であることを確認
	if summaryChunk.Chunk.Tokens > 400 {
		t.Errorf("summary tokens should be <= 400, got: %d", summaryChunk.Chunk.Tokens)
	}

	t.Logf("Summary token count: %d", summaryChunk.Chunk.Tokens)

	// 他のチャンクがLevel 2であることを確認
	for i := 1; i < len(result.Chunks); i++ {
		chunk := result.Chunks[i]
		if chunk.Metadata == nil {
			continue
		}
		if chunk.Metadata.Level != 2 {
			t.Errorf("chunk %d should be level 2, got: %d", i, chunk.Metadata.Level)
		}
	}

	t.Logf("Total chunks generated: %d", len(result.Chunks))
	for i, chunk := range result.Chunks {
		chunkType := "unknown"
		if chunk.Metadata != nil && chunk.Metadata.Type != nil {
			chunkType = *chunk.Metadata.Type
		}
		t.Logf("Chunk %d: Type=%s, Level=%d, Tokens=%d",
			i, chunkType, chunk.Metadata.Level, chunk.Chunk.Tokens)
	}
}

func TestASTChunkerGo_FileSummaryWithLargeFile(t *testing.T) {
	t.Skip("ファイルサマリー機能は未実装のため、このテストをスキップします")

	defaultChunker, err := chunker.NewDefaultChunker()
	if err != nil {
		t.Fatalf("failed to create chunker: %v", err)
	}

	astChunker := ast.NewASTChunkerGo()

	// 大量のコンポーネントを含むコードを生成
	var codeBuilder strings.Builder
	codeBuilder.WriteString("// Package large は大規模なパッケージです\n")
	codeBuilder.WriteString("package large\n\n")
	codeBuilder.WriteString("import (\n")
	for i := 0; i < 20; i++ {
		codeBuilder.WriteString("	\"github.com/example/dep")
		codeBuilder.WriteString(string(rune('A' + i)))
		codeBuilder.WriteString("\"\n")
	}
	codeBuilder.WriteString(")\n\n")

	// 50個の関数を生成
	for i := 0; i < 50; i++ {
		codeBuilder.WriteString("func Function")
		codeBuilder.WriteString(string(rune('A' + (i % 26))))
		codeBuilder.WriteString("() {}\n\n")
	}

	testCode := codeBuilder.String()

	result := astChunker.ChunkWithMetrics(testCode, defaultChunker)

	// AST解析が成功することを確認
	if !result.ParseSuccess {
		t.Fatalf("AST parse should succeed, got error: %v", result.ParseError)
	}

	// 最初のチャンクがファイルサマリーであることを確認
	if len(result.Chunks) == 0 {
		t.Fatalf("should generate at least one chunk")
	}

	summaryChunk := result.Chunks[0]
	if summaryChunk.Metadata.Level != 1 {
		t.Errorf("first chunk should be level 1, got: %d", summaryChunk.Metadata.Level)
	}

	// トークン数が400以内であることを確認（最も重要）
	if summaryChunk.Chunk.Tokens > 400 {
		t.Errorf("summary tokens should be <= 400, got: %d", summaryChunk.Chunk.Tokens)
	}

	t.Logf("Large file summary token count: %d", summaryChunk.Chunk.Tokens)
	t.Logf("Total chunks: %d", len(result.Chunks))
}

func TestASTChunkerGo_FileSummaryMinimalFile(t *testing.T) {
	t.Skip("ファイルサマリー機能は未実装のため、このテストをスキップします")

	defaultChunker, err := chunker.NewDefaultChunker()
	if err != nil {
		t.Fatalf("failed to create chunker: %v", err)
	}

	astChunker := ast.NewASTChunkerGo()

	// 最小限のGoファイル
	testCode := `package main

func main() {}
`

	result := astChunker.ChunkWithMetrics(testCode, defaultChunker)

	// AST解析が成功することを確認
	if !result.ParseSuccess {
		t.Fatalf("AST parse should succeed, got error: %v", result.ParseError)
	}

	// 最初のチャンクがファイルサマリーであることを確認
	if len(result.Chunks) == 0 {
		t.Fatalf("should generate at least one chunk")
	}

	summaryChunk := result.Chunks[0]
	if summaryChunk.Metadata.Level != 1 {
		t.Errorf("first chunk should be level 1, got: %d", summaryChunk.Metadata.Level)
	}

	t.Logf("Minimal file summary:\n%s", summaryChunk.Chunk.Content)
	t.Logf("Summary token count: %d", summaryChunk.Chunk.Tokens)
}
