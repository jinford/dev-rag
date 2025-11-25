package chunker

import (
	"strings"
	"testing"
)

// TestCountTokens は countTokens メソッドの基本動作を確認します
func TestCountTokens(t *testing.T) {
	chunker, err := NewDefaultChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	tests := []struct {
		name     string
		text     string
		minToken int
		maxToken int
	}{
		{
			name:     "英語のシンプルなテキスト",
			text:     "Hello, world!",
			minToken: 1,
			maxToken: 10,
		},
		{
			name:     "日本語のテキスト",
			text:     "こんにちは、世界！",
			minToken: 1,
			maxToken: 20,
		},
		{
			name:     "日本語と英語の混在テキスト",
			text:     "Hello, こんにちは! This is a test. これはテストです。",
			minToken: 5,
			maxToken: 30,
		},
		{
			name:     "長い英語テキスト",
			text:     strings.Repeat("This is a test sentence. ", 10),
			minToken: 40,
			maxToken: 70,
		},
		{
			name:     "長い日本語テキスト",
			text:     strings.Repeat("これはテストの文章です。", 10),
			minToken: 40,
			maxToken: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := chunker.countTokens(tt.text)
			if count < tt.minToken || count > tt.maxToken {
				t.Errorf("Token count %d is out of expected range [%d, %d] for text: %s",
					count, tt.minToken, tt.maxToken, tt.text)
			}
			t.Logf("Text: %s\nToken count: %d", tt.text, count)
		})
	}
}

// TestBoundaryValues は 100トークンと1600トークンの境界値をテストします
func TestBoundaryValues(t *testing.T) {
	chunker, err := NewDefaultChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	// 100トークン前後のテキストを生成
	// 英語の場合、約130語で100トークン程度になる（"word "は1トークン）
	text100 := strings.Repeat("word ", 130)
	count100 := chunker.countTokens(text100)
	t.Logf("100トークン境界テスト: %d tokens", count100)
	if count100 < 90 || count100 > 140 {
		t.Errorf("Expected around 100-130 tokens, got %d", count100)
	}

	// 1600トークン前後のテキストを生成
	// 英語の場合、約2000語で1600トークン程度になる
	text1600 := strings.Repeat("word ", 2100)
	count1600 := chunker.countTokens(text1600)
	t.Logf("1600トークン境界テスト: %d tokens", count1600)
	if count1600 < 1500 || count1600 > 2200 {
		t.Errorf("Expected around 1600 tokens, got %d", count1600)
	}
}

// TestMinTokensFilter は minTokens より小さいチャンクがフィルタされることを確認します
func TestMinTokensFilter(t *testing.T) {
	chunker, err := NewDefaultChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	// 100トークン未満の短いテキスト
	shortText := "This is a very short text."
	lines := []string{shortText}

	chunk := chunker.createChunk(lines, 1, 1)
	if chunk != nil {
		t.Errorf("Expected nil for text with less than minTokens, but got a chunk with %d tokens", chunk.Tokens)
	}
	t.Logf("Short text (%d tokens) was correctly filtered", chunker.countTokens(shortText))
}

// TestMaxTokensChunking は maxTokens を超えるテキストが正しく分割されることを確認します
func TestMaxTokensChunking(t *testing.T) {
	chunker, err := NewDefaultChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	// 1600トークンを超える長いテキスト（約2500トークン）
	// chunkPlainTextは改行で分割するため、改行を含むテキストを作成
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "This is a test sentence that will be used to create a very long text.")
	}
	longText := strings.Join(lines, "\n")

	chunks, err := chunker.chunkPlainText(longText)
	if err != nil {
		t.Fatalf("Failed to chunk text: %v", err)
	}

	// 各チャンクがmaxTokens以下であることを確認
	for i, chunk := range chunks {
		t.Logf("Chunk %d: %d tokens (lines %d-%d)", i+1, chunk.Tokens, chunk.StartLine, chunk.EndLine)
		if chunk.Tokens > chunker.maxTokens {
			t.Errorf("Chunk %d exceeds maxTokens: %d > %d", i+1, chunk.Tokens, chunker.maxTokens)
		}
		if chunk.Tokens < chunker.minTokens {
			t.Errorf("Chunk %d is below minTokens: %d < %d", i+1, chunk.Tokens, chunker.minTokens)
		}
	}

	// 複数のチャンクに分割されていることを確認
	if len(chunks) < 2 {
		t.Errorf("Expected at least 2 chunks for long text, got %d", len(chunks))
	}
}

// TestJapaneseMixedTokenCount は日本語・英語混在テキストでのトークンカウントを確認します
func TestJapaneseMixedTokenCount(t *testing.T) {
	chunker, err := NewDefaultChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	tests := []struct {
		name string
		text string
	}{
		{
			name: "日本語と英語の混在 - 短文",
			text: "Go言語（Golang）はGoogleが開発したプログラミング言語です。It is known for its simplicity and efficiency.",
		},
		{
			name: "日本語と英語の混在 - コード例",
			text: `
package main

import "fmt"

// Hello は挨拶を表示します
func Hello() {
    fmt.Println("こんにちは、世界！")
    fmt.Println("Hello, World!")
}
`,
		},
		{
			name: "日本語が多い混在テキスト",
			text: `
このシステムは、ソースコードとドキュメントをインデックス化し、semantic searchを可能にするRAGシステムです。
The system uses OpenAI embeddings for semantic understanding.
各チャンクは100〜1600トークンの範囲で生成され、適切なメタデータが付与されます。
This ensures efficient retrieval and high-quality context for LLM queries.
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := chunker.countTokens(tt.text)
			t.Logf("Text: %s\nToken count: %d", tt.text, count)

			// トークンカウントが正の数であることを確認
			if count <= 0 {
				t.Errorf("Token count should be positive, got %d", count)
			}
		})
	}
}

// TestTrimToTokenLimit は TrimToTokenLimit メソッドの動作を確認します
func TestTrimToTokenLimit(t *testing.T) {
	chunker, err := NewDefaultChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	tests := []struct {
		name      string
		text      string
		maxTokens int
	}{
		{
			name:      "トークン制限内のテキスト",
			text:      "This is a short text.",
			maxTokens: 100,
		},
		{
			name:      "トークン制限を超えるテキスト",
			text:      strings.Repeat("This is a test sentence. ", 50),
			maxTokens: 50,
		},
		{
			name:      "日本語テキストのトリミング",
			text:      strings.Repeat("これはテストの文章です。", 20),
			maxTokens: 30,
		},
		{
			name:      "1600トークン制限",
			text:      strings.Repeat("word ", 2000),
			maxTokens: 1600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trimmed := chunker.TrimToTokenLimit(tt.text, tt.maxTokens)
			trimmedCount := chunker.countTokens(trimmed)
			originalCount := chunker.countTokens(tt.text)

			t.Logf("Original tokens: %d, Max tokens: %d, Trimmed tokens: %d",
				originalCount, tt.maxTokens, trimmedCount)

			// トリミング後のトークン数が制限以下であることを確認
			if trimmedCount > tt.maxTokens {
				t.Errorf("Trimmed text still exceeds maxTokens: %d > %d", trimmedCount, tt.maxTokens)
			}

			// 元のテキストが制限内の場合、変更されていないことを確認
			if originalCount <= tt.maxTokens && trimmed != tt.text {
				t.Errorf("Text within limit should not be modified")
			}

			// トリミングが必要な場合、トリミングされていることを確認
			if originalCount > tt.maxTokens && trimmedCount >= originalCount {
				t.Errorf("Text should be trimmed when exceeding limit")
			}
		})
	}
}

// TestChunkerSettings は Chunker の設定値を確認します
func TestChunkerSettings(t *testing.T) {
	chunker, err := NewDefaultChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	// 設定値の確認
	if chunker.maxTokens != 1600 {
		t.Errorf("Expected maxTokens to be 1600, got %d", chunker.maxTokens)
	}
	if chunker.minTokens != 100 {
		t.Errorf("Expected minTokens to be 100, got %d", chunker.minTokens)
	}
	if chunker.targetTokens != 800 {
		t.Errorf("Expected targetTokens to be 800, got %d", chunker.targetTokens)
	}
	if chunker.overlap != 200 {
		t.Errorf("Expected overlap to be 200, got %d", chunker.overlap)
	}

	t.Logf("Chunker settings: maxTokens=%d, minTokens=%d, targetTokens=%d, overlap=%d",
		chunker.maxTokens, chunker.minTokens, chunker.targetTokens, chunker.overlap)
}
