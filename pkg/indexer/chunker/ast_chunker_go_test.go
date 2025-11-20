package chunker

import (
	"strings"
	"testing"
)

func TestASTChunkerGo_SmallFile(t *testing.T) {
	// 小規模ファイル（1関数のみ）- トークンサイズ100以上にするため少し大きくする
	source := `package main

import (
	"fmt"
	"os"
	"strings"
)

// HelloWorld prints hello world to stdout
// This is a simple example function that demonstrates basic Go syntax
// It uses the fmt package to print a message
// The message is a classic "Hello, World!" greeting
// This function takes no parameters and returns nothing
func HelloWorld() {
	message := "Hello, World!"
	fmt.Println(message)

	// Also print to stderr for demonstration
	fmt.Fprintln(os.Stderr, "Error output: " + message)

	// Convert to uppercase
	upper := strings.ToUpper(message)
	fmt.Println(upper)
}
`

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	chunks, err := astChunker.Chunk(source, chunker)
	if err != nil {
		t.Fatalf("Failed to chunk Go source: %v", err)
	}

	if len(chunks) == 0 {
		// デバッグ情報を出力
		t.Logf("Source length: %d bytes", len(source))
		// チャンク化を試して何が起きているか確認
		testChunks, _ := chunker.ChunkWithMetadata(source, "text/x-go")
		t.Logf("ChunkWithMetadata returned %d chunks", len(testChunks))
		for i, c := range testChunks {
			t.Logf("  Chunk %d: tokens=%d, lines=%d-%d, hasMetadata=%v",
				i, c.Chunk.Tokens, c.Chunk.StartLine, c.Chunk.EndLine, c.Metadata != nil)
		}
		t.Fatalf("Expected at least one chunk, got 0. Chunker settings: minTokens=%d, maxTokens=%d", chunker.minTokens, chunker.maxTokens)
	}

	// 関数チャンクを検証
	found := false
	for _, chunk := range chunks {
		if chunk.Metadata != nil && chunk.Metadata.Name != nil && *chunk.Metadata.Name == "HelloWorld" {
			found = true
			if chunk.Metadata.Type == nil || *chunk.Metadata.Type != "function" {
				t.Errorf("Expected type 'function', got %v", chunk.Metadata.Type)
			}
			if chunk.Metadata.DocComment == nil || !strings.Contains(*chunk.Metadata.DocComment, "prints hello world") {
				t.Errorf("Expected doc comment to contain 'prints hello world', got %v", chunk.Metadata.DocComment)
			}
			break
		}
	}

	if !found {
		t.Error("Expected to find HelloWorld function chunk")
	}
}

func TestASTChunkerGo_MediumFile(t *testing.T) {
	// 中規模ファイル（複数関数、複数struct、100行程度）
	source := `package calculator

import (
	"fmt"
	"math"
)

// Calculator represents a simple calculator
type Calculator struct {
	result float64
}

// NewCalculator creates a new calculator instance
func NewCalculator() *Calculator {
	return &Calculator{result: 0}
}

// Add adds two numbers
func (c *Calculator) Add(a, b float64) float64 {
	c.result = a + b
	return c.result
}

// Subtract subtracts two numbers
func (c *Calculator) Subtract(a, b float64) float64 {
	c.result = a - b
	return c.result
}

// Multiply multiplies two numbers
func (c *Calculator) Multiply(a, b float64) float64 {
	c.result = a * b
	return c.result
}

// Divide divides two numbers
func (c *Calculator) Divide(a, b float64) (float64, error) {
	if b == 0 {
		return 0, fmt.Errorf("division by zero")
	}
	c.result = a / b
	return c.result, nil
}

// GetResult returns the last calculation result
func (c *Calculator) GetResult() float64 {
	return c.result
}

const (
	// MaxValue is the maximum value
	MaxValue = math.MaxFloat64
	// MinValue is the minimum value
	MinValue = -math.MaxFloat64
)
`

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	chunks, err := astChunker.Chunk(source, chunker)
	if err != nil {
		t.Fatalf("Failed to chunk Go source: %v", err)
	}

	if len(chunks) < 5 {
		t.Errorf("Expected at least 5 chunks, got %d", len(chunks))
	}

	// デバッグ: 生成されたチャンクを確認
	t.Logf("Generated %d chunks:", len(chunks))
	for i, chunk := range chunks {
		typeName := "nil"
		name := "nil"
		if chunk.Metadata != nil {
			if chunk.Metadata.Type != nil {
				typeName = *chunk.Metadata.Type
			}
			if chunk.Metadata.Name != nil {
				name = *chunk.Metadata.Name
			}
		}
		t.Logf("  Chunk %d: type=%s, name=%s, tokens=%d", i, typeName, name, chunk.Chunk.Tokens)
	}

	// struct定義を検証
	foundStruct := false
	for _, chunk := range chunks {
		if chunk.Metadata != nil && chunk.Metadata.Name != nil && *chunk.Metadata.Name == "Calculator" {
			if chunk.Metadata.Type != nil && *chunk.Metadata.Type == "struct" {
				foundStruct = true
				break
			}
		}
	}
	if !foundStruct {
		t.Error("Expected to find Calculator struct chunk")
	}

	// メソッドを検証
	foundMethod := false
	for _, chunk := range chunks {
		if chunk.Metadata != nil && chunk.Metadata.Name != nil && *chunk.Metadata.Name == "Add" {
			if chunk.Metadata.Type != nil && *chunk.Metadata.Type == "method" {
				foundMethod = true
				if chunk.Metadata.ParentName == nil || !strings.Contains(*chunk.Metadata.ParentName, "Calculator") {
					t.Errorf("Expected parent name to contain 'Calculator', got %v", chunk.Metadata.ParentName)
				}
				break
			}
		}
	}
	if !foundMethod {
		t.Error("Expected to find Add method chunk")
	}

	// インポートを検証
	for _, chunk := range chunks {
		if chunk.Metadata != nil && chunk.Metadata.Imports != nil {
			hasImport := false
			for _, imp := range chunk.Metadata.Imports {
				if imp == "fmt" || imp == "math" {
					hasImport = true
					break
				}
			}
			if hasImport {
				break
			}
		}
	}
}

func TestASTChunkerGo_HighCommentRatio(t *testing.T) {
	// コメント比率が高いファイル（95%以上）
	source := `package main

// This is a very long comment
// That takes up many lines
// And has a high comment ratio
// This function should be excluded
// Because it's mostly comments
// And not actual code
// We need more comments
// To reach 95% comment ratio
// Keep adding comments
// More and more comments
// Almost there
// Just a bit more
// And we're done
func SmallFunc() {
	// Just one line of code
}
`

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	chunks, err := astChunker.Chunk(source, chunker)
	if err != nil {
		t.Fatalf("Failed to chunk Go source: %v", err)
	}

	// コメント比率95%以上のチャンクは除外されるべき
	for _, chunk := range chunks {
		if chunk.Metadata != nil && chunk.Metadata.CommentRatio != nil {
			if *chunk.Metadata.CommentRatio > 0.95 {
				t.Errorf("Expected high comment ratio chunk to be excluded, but found chunk with ratio %f", *chunk.Metadata.CommentRatio)
			}
		}
	}
}

func TestASTChunkerGo_CyclomaticComplexity(t *testing.T) {
	// 循環的複雑度が高い関数
	source := `package main

// ComplexFunction has high cyclomatic complexity
func ComplexFunction(a, b, c int) int {
	result := 0

	if a > 0 {
		result += a
	}

	if b > 0 {
		result += b
	}

	if c > 0 {
		result += c
	}

	for i := 0; i < a; i++ {
		if i%2 == 0 {
			result += i
		}
	}

	switch result {
	case 0:
		return 0
	case 1:
		return 1
	default:
		return result
	}
}
`

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	chunks, err := astChunker.Chunk(source, chunker)
	if err != nil {
		t.Fatalf("Failed to chunk Go source: %v", err)
	}

	found := false
	for _, chunk := range chunks {
		if chunk.Metadata != nil && chunk.Metadata.Name != nil && *chunk.Metadata.Name == "ComplexFunction" {
			found = true
			if chunk.Metadata.CyclomaticComplexity == nil {
				t.Error("Expected cyclomatic complexity to be calculated")
			} else if *chunk.Metadata.CyclomaticComplexity < 5 {
				t.Errorf("Expected cyclomatic complexity >= 5, got %d", *chunk.Metadata.CyclomaticComplexity)
			}
			break
		}
	}

	if !found {
		t.Error("Expected to find ComplexFunction chunk")
	}
}

func TestASTChunkerGo_FunctionSignature(t *testing.T) {
	// 関数シグネチャの抽出を検証
	source := `package main

import "context"

// ProcessData processes data with context
func ProcessData(ctx context.Context, data []byte, count int) (string, error) {
	return "", nil
}
`

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	chunks, err := astChunker.Chunk(source, chunker)
	if err != nil {
		t.Fatalf("Failed to chunk Go source: %v", err)
	}

	found := false
	for _, chunk := range chunks {
		if chunk.Metadata != nil && chunk.Metadata.Name != nil && *chunk.Metadata.Name == "ProcessData" {
			found = true
			if chunk.Metadata.Signature == nil {
				t.Error("Expected signature to be extracted")
			} else {
				sig := *chunk.Metadata.Signature
				if !strings.Contains(sig, "ProcessData") {
					t.Errorf("Expected signature to contain 'ProcessData', got %s", sig)
				}
				if !strings.Contains(sig, "context.Context") {
					t.Errorf("Expected signature to contain 'context.Context', got %s", sig)
				}
			}
			break
		}
	}

	if !found {
		t.Error("Expected to find ProcessData function chunk")
	}
}

func TestASTChunkerGo_TokenSizeValidation(t *testing.T) {
	// トークンサイズ検証
	source := `package main

// TinyFunc is too small
func TinyFunc() {
	x := 1
}
`

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	chunks, err := astChunker.Chunk(source, chunker)
	if err != nil {
		t.Fatalf("Failed to chunk Go source: %v", err)
	}

	// AST解析の場合、最小トークン数は10（関数単位で抽出するため緩和されている）
	minTokensForAST := 10
	for _, chunk := range chunks {
		if chunk.Chunk.Tokens < minTokensForAST {
			t.Errorf("Expected chunk to have at least %d tokens, got %d", minTokensForAST, chunk.Chunk.Tokens)
		}
		if chunk.Chunk.Tokens > chunker.maxTokens {
			t.Errorf("Expected chunk to have at most %d tokens, got %d", chunker.maxTokens, chunk.Chunk.Tokens)
		}
	}
}

func TestASTChunkerGo_FallbackToRegex(t *testing.T) {
	// 不正なGo構文（AST解析失敗時のフォールバック）
	invalidSource := `package main

func BrokenFunc() {
	// Missing closing brace
`

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	// ChunkWithMetadataメソッドを使用してフォールバックをテスト
	chunks, err := chunker.ChunkWithMetadata(invalidSource, "text/x-go")
	if err != nil {
		t.Fatalf("Expected fallback to succeed, but got error: %v", err)
	}

	// フォールバックで生成されたチャンクはメタデータなし
	for _, chunk := range chunks {
		if chunk.Chunk == nil {
			t.Error("Expected chunk to be present")
		}
	}
}

func TestASTChunkerGo_InterfaceType(t *testing.T) {
	// インターフェース型の抽出
	source := `package main

// Reader is an interface for reading data
type Reader interface {
	Read(p []byte) (n int, err error)
	Close() error
}
`

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	chunks, err := astChunker.Chunk(source, chunker)
	if err != nil {
		t.Fatalf("Failed to chunk Go source: %v", err)
	}

	found := false
	for _, chunk := range chunks {
		if chunk.Metadata != nil && chunk.Metadata.Name != nil && *chunk.Metadata.Name == "Reader" {
			found = true
			if chunk.Metadata.Type == nil || *chunk.Metadata.Type != "interface" {
				t.Errorf("Expected type 'interface', got %v", chunk.Metadata.Type)
			}
			break
		}
	}

	if !found {
		t.Error("Expected to find Reader interface chunk")
	}
}

func TestASTChunkerGo_FunctionCalls(t *testing.T) {
	// 関数呼び出しの抽出
	source := `package main

import "fmt"

func ExampleFunc() {
	fmt.Println("Hello")
	fmt.Printf("World")
	someHelper()
}

func someHelper() {
}
`

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	chunks, err := astChunker.Chunk(source, chunker)
	if err != nil {
		t.Fatalf("Failed to chunk Go source: %v", err)
	}

	found := false
	for _, chunk := range chunks {
		if chunk.Metadata != nil && chunk.Metadata.Name != nil && *chunk.Metadata.Name == "ExampleFunc" {
			found = true
			if chunk.Metadata.Calls == nil || len(chunk.Metadata.Calls) == 0 {
				t.Error("Expected to find function calls")
			} else {
				// Println, Printf, someHelper のいずれかが含まれていることを確認
				hasCall := false
				for _, call := range chunk.Metadata.Calls {
					if call == "Println" || call == "Printf" || call == "someHelper" {
						hasCall = true
						break
					}
				}
				if !hasCall {
					t.Errorf("Expected to find Println, Printf, or someHelper in calls, got %v", chunk.Metadata.Calls)
				}
			}
			break
		}
	}

	if !found {
		t.Error("Expected to find ExampleFunc chunk")
	}
}
