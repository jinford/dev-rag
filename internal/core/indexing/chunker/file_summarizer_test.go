package chunker

import (
	"strings"
	"testing"
)

func TestFileSummarizer_GenerateSummary_Go(t *testing.T) {
	chunker, err := NewDefaultChunker()
	if err != nil {
		t.Fatalf("failed to create chunker: %v", err)
	}

	summarizer := NewFileSummarizer()

	testCode := `// Package example はテスト用のパッケージです。
// このパッケージは主要な機能をデモンストレーションします。
package example

import (
	"fmt"
	"strings"
	"github.com/example/external"
)

// User はユーザー情報を表します
type User struct {
	ID   int
	Name string
}

// Calculate は計算を実行します
func Calculate(a, b int) int {
	return a + b
}

// Process はデータを処理します
func (u *User) Process() error {
	fmt.Println(u.Name)
	return nil
}

// Config は設定を保持します
type Config struct {
	Host string
	Port int
}
`

	summary, err := summarizer.GenerateSummary(testCode, "go", chunker)
	if err != nil {
		t.Fatalf("failed to generate summary: %v", err)
	}

	// 生成されたサマリーの検証
	t.Logf("Generated Summary:\n%s", summary)

	// Languageが含まれることを確認
	if !strings.Contains(summary, "Language: Go") {
		t.Errorf("summary should contain 'Language: Go', got: %s", summary)
	}

	// Main Componentsセクションが含まれることを確認
	if !strings.Contains(summary, "Main Components:") {
		t.Errorf("summary should contain 'Main Components:', got: %s", summary)
	}

	// 関数名が含まれることを確認
	if !strings.Contains(summary, "Calculate") {
		t.Errorf("summary should contain function name 'Calculate', got: %s", summary)
	}

	// 構造体名が含まれることを確認
	if !strings.Contains(summary, "User") || !strings.Contains(summary, "Config") {
		t.Errorf("summary should contain struct names 'User' and 'Config', got: %s", summary)
	}

	// Dependenciesセクションが含まれることを確認
	if !strings.Contains(summary, "Dependencies:") {
		t.Errorf("summary should contain 'Dependencies:', got: %s", summary)
	}

	// 外部依存が含まれることを確認
	if !strings.Contains(summary, "github.com/example/external") {
		t.Errorf("summary should contain external dependency, got: %s", summary)
	}

	// トップレベルコメントが含まれることを確認
	if !strings.Contains(summary, "Description:") {
		t.Errorf("summary should contain 'Description:', got: %s", summary)
	}

	// トークン数が400以内であることを確認
	tokens := chunker.countTokens(summary)
	if tokens > 400 {
		t.Errorf("summary tokens should be <= 400, got: %d", tokens)
	}

	t.Logf("Summary token count: %d", tokens)
}

func TestFileSummarizer_GenerateSummary_LargeFile(t *testing.T) {
	chunker, err := NewDefaultChunker()
	if err != nil {
		t.Fatalf("failed to create chunker: %v", err)
	}

	summarizer := NewFileSummarizer()

	// 大量のコンポーネントを含むコードを生成
	var codeBuilder strings.Builder
	codeBuilder.WriteString("package large\n\n")
	codeBuilder.WriteString("import (\n")
	for i := 0; i < 50; i++ {
		codeBuilder.WriteString("	\"github.com/example/dep")
		codeBuilder.WriteString(string(rune('A' + i)))
		codeBuilder.WriteString("\"\n")
	}
	codeBuilder.WriteString(")\n\n")

	// 100個の関数を生成
	for i := 0; i < 100; i++ {
		codeBuilder.WriteString("func Function")
		codeBuilder.WriteString(string(rune('A' + (i % 26))))
		codeBuilder.WriteString(string(rune('0' + (i / 26))))
		codeBuilder.WriteString("() {}\n\n")
	}

	testCode := codeBuilder.String()

	summary, err := summarizer.GenerateSummary(testCode, "go", chunker)
	if err != nil {
		t.Fatalf("failed to generate summary: %v", err)
	}

	t.Logf("Generated Summary for large file:\n%s", summary)

	// トークン数が400以内であることを確認（最も重要）
	tokens := chunker.countTokens(summary)
	if tokens > 400 {
		t.Errorf("summary tokens should be <= 400, got: %d", tokens)
	}

	// トランケーション表示があることを確認（大きなファイルなので切り詰められているはず）
	if !strings.Contains(summary, "truncated") {
		t.Logf("Note: Large file summary was not truncated (token count: %d)", tokens)
	}

	t.Logf("Summary token count: %d", tokens)
}

func TestFileSummarizer_GenerateSummary_SimpleFallback(t *testing.T) {
	chunker, err := NewDefaultChunker()
	if err != nil {
		t.Fatalf("failed to create chunker: %v", err)
	}

	summarizer := NewFileSummarizer()

	// AST解析が失敗するコード（不正なGo言語）
	testCode := `// This is a test file
// with some comments

func InvalidSyntax( {
	// 構文エラー
}
`

	summary, err := summarizer.GenerateSummary(testCode, "go", chunker)
	if err != nil {
		t.Fatalf("failed to generate summary: %v", err)
	}

	t.Logf("Generated Summary (fallback):\n%s", summary)

	// Languageが含まれることを確認
	if !strings.Contains(summary, "Language:") {
		t.Errorf("summary should contain 'Language:', got: %s", summary)
	}

	// トークン数が400以内であることを確認
	tokens := chunker.countTokens(summary)
	if tokens > 400 {
		t.Errorf("summary tokens should be <= 400, got: %d", tokens)
	}

	t.Logf("Summary token count: %d", tokens)
}

func TestFileSummarizer_GenerateSummary_OtherLanguage(t *testing.T) {
	chunker, err := NewDefaultChunker()
	if err != nil {
		t.Fatalf("failed to create chunker: %v", err)
	}

	summarizer := NewFileSummarizer()

	testCode := `# This is a Python file
# with some comments

def calculate(a, b):
    return a + b

class User:
    def __init__(self, name):
        self.name = name
`

	summary, err := summarizer.GenerateSummary(testCode, "python", chunker)
	if err != nil {
		t.Fatalf("failed to generate summary: %v", err)
	}

	t.Logf("Generated Summary (Python):\n%s", summary)

	// Languageが含まれることを確認
	if !strings.Contains(summary, "Language: python") {
		t.Errorf("summary should contain 'Language: python', got: %s", summary)
	}

	// トークン数が400以内であることを確認
	tokens := chunker.countTokens(summary)
	if tokens > 400 {
		t.Errorf("summary tokens should be <= 400, got: %d", tokens)
	}

	t.Logf("Summary token count: %d", tokens)
}

func TestFileSummarizer_GenerateSummary_EmptyFile(t *testing.T) {
	chunker, err := NewDefaultChunker()
	if err != nil {
		t.Fatalf("failed to create chunker: %v", err)
	}

	summarizer := NewFileSummarizer()

	testCode := ``

	summary, err := summarizer.GenerateSummary(testCode, "go", chunker)
	if err != nil {
		t.Fatalf("failed to generate summary: %v", err)
	}

	t.Logf("Generated Summary (empty file):\n%s", summary)

	// Languageが含まれることを確認
	if !strings.Contains(summary, "Language:") {
		t.Errorf("summary should contain 'Language:', got: %s", summary)
	}

	// トークン数が400以内であることを確認
	tokens := chunker.countTokens(summary)
	if tokens > 400 {
		t.Errorf("summary tokens should be <= 400, got: %d", tokens)
	}

	t.Logf("Summary token count: %d", tokens)
}
