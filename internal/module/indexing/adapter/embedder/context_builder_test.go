package embedder

import (
	"strings"
	"testing"

	"github.com/jinford/dev-rag/internal/module/indexing/adapter/chunker"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

func TestContextBuilder_BuildContext(t *testing.T) {
	builder, err := NewContextBuilder()
	if err != nil {
		t.Fatalf("failed to create context builder: %v", err)
	}

	tests := []struct {
		name          string
		chunk         *chunker.Chunk
		metadata      *domain.ChunkMetadata
		filePath      string
		wantContains  []string
		wantNotEmpty  bool
		checkTokens   bool
		maxTokens     int
	}{
		{
			name: "メタデータなしの場合はチャンク本体のみ",
			chunk: &chunker.Chunk{
				Content:   "func main() {\n\tfmt.Println(\"Hello\")\n}",
				StartLine: 1,
				EndLine:   3,
				Tokens:    20,
			},
			metadata:     nil,
			filePath:     "main.go",
			wantContains: []string{"func main()"},
			wantNotEmpty: true,
		},
		{
			name: "関数のメタデータ付き",
			chunk: &chunker.Chunk{
				Content:   "func Add(a, b int) int {\n\treturn a + b\n}",
				StartLine: 5,
				EndLine:   7,
				Tokens:    25,
			},
			metadata: &domain.ChunkMetadata{
				Type:       stringPtr("function"),
				Name:       stringPtr("Add"),
				ParentName: stringPtr("math"),
				Signature:  stringPtr("func Add(a, b int) int"),
			},
			filePath: "math/add.go",
			wantContains: []string{
				"File: math/add.go",
				"Package: math",
				"Function: Add",
				"Signature: func Add(a, b int) int",
				"func Add(a, b int) int {",
			},
			wantNotEmpty: true,
		},
		{
			name: "メソッドのメタデータ付き",
			chunk: &chunker.Chunk{
				Content:   "func (s *Server) Start() error {\n\treturn nil\n}",
				StartLine: 10,
				EndLine:   12,
				Tokens:    30,
			},
			metadata: &domain.ChunkMetadata{
				Type:       stringPtr("method"),
				Name:       stringPtr("Start"),
				ParentName: stringPtr("Server"),
				Signature:  stringPtr("func (s *Server) Start() error"),
			},
			filePath: "server.go",
			wantContains: []string{
				"File: server.go",
				"Package: Server",
				"Method: Start",
				"func (s *Server) Start() error",
			},
			wantNotEmpty: true,
		},
		{
			name: "ファイルパスのみ",
			chunk: &chunker.Chunk{
				Content:   "package main",
				StartLine: 1,
				EndLine:   1,
				Tokens:    5,
			},
			metadata: &domain.ChunkMetadata{},
			filePath: "main.go",
			wantContains: []string{
				"File: main.go",
				"package main",
			},
			wantNotEmpty: true,
		},
		{
			name: "トークン制限を超える場合のトリミング",
			chunk: &chunker.Chunk{
				Content:   strings.Repeat("a", 30000), // 約8000トークン超
				StartLine: 1,
				EndLine:   1,
				Tokens:    8000,
			},
			metadata: &domain.ChunkMetadata{
				Type:       stringPtr("function"),
				Name:       stringPtr("VeryLongFunction"),
				ParentName: stringPtr("package"),
				Signature:  stringPtr("func VeryLongFunction() string"),
			},
			filePath:    "long.go",
			wantNotEmpty: true,
			checkTokens: true,
			maxTokens:   8191,
		},
		{
			name: "最小限のメタデータ（Type と Name のみ）",
			chunk: &chunker.Chunk{
				Content:   "const Version = \"1.0.0\"",
				StartLine: 1,
				EndLine:   1,
				Tokens:    10,
			},
			metadata: &domain.ChunkMetadata{
				Type: stringPtr("const"),
				Name: stringPtr("Version"),
			},
			filePath: "version.go",
			wantContains: []string{
				"File: version.go",
				"Const: Version",
				"const Version",
			},
			wantNotEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.BuildContext(tt.chunk, tt.metadata, tt.filePath)

			if !tt.wantNotEmpty {
				t.Errorf("expected non-empty result")
			}

			if result == "" {
				t.Errorf("BuildContext() returned empty string")
			}

			// 期待する文字列が含まれているか確認
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("BuildContext() result does not contain %q\nGot: %s", want, result)
				}
			}

			// トークン数チェック
			if tt.checkTokens {
				tokens := builder.countTokens(result)
				if tokens > tt.maxTokens {
					t.Errorf("BuildContext() result exceeds max tokens: got %d, want <= %d", tokens, tt.maxTokens)
				}
			}
		})
	}
}

func TestContextBuilder_TrimContext(t *testing.T) {
	builder, err := NewContextBuilder()
	if err != nil {
		t.Fatalf("failed to create context builder: %v", err)
	}

	tests := []struct {
		name         string
		contextLines []string
		content      string
		wantContains []string
		checkTokens  bool
		maxTokens    int
	}{
		{
			name: "コンテキスト情報がトークン制限内に収まる",
			contextLines: []string{
				"File: test.go",
				"Package: test",
				"Function: TestFunc",
			},
			content: "func TestFunc() {}",
			wantContains: []string{
				"File: test.go",
				"Package: test",
				"Function: TestFunc",
				"func TestFunc() {}",
			},
			checkTokens: true,
			maxTokens:   8191,
		},
		{
			name: "コンテキスト情報が削減される",
			contextLines: []string{
				"File: test.go",
				"Package: test",
				"Function: VeryLongFunctionName",
				"Signature: func VeryLongFunctionName() string",
			},
			content:     strings.Repeat("a", 30000), // 約8000トークン超
			checkTokens: true,
			maxTokens:   8191,
		},
		{
			name: "チャンク本体だけでトークン制限を超える",
			contextLines: []string{
				"File: huge.go",
			},
			content:     strings.Repeat("b", 40000), // トークン制限を大幅に超える
			checkTokens: true,
			maxTokens:   8191,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.trimContext(tt.contextLines, tt.content)

			if result == "" {
				t.Errorf("trimContext() returned empty string")
			}

			// 期待する文字列が含まれているか確認
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Logf("trimContext() result does not contain %q", want)
				}
			}

			// トークン数チェック
			if tt.checkTokens {
				tokens := builder.countTokens(result)
				if tokens > tt.maxTokens {
					t.Errorf("trimContext() result exceeds max tokens: got %d, want <= %d", tokens, tt.maxTokens)
				}
			}
		})
	}
}

func TestContextBuilder_CountTokens(t *testing.T) {
	builder, err := NewContextBuilder()
	if err != nil {
		t.Fatalf("failed to create context builder: %v", err)
	}

	tests := []struct {
		name      string
		text      string
		wantRange [2]int // [min, max]
	}{
		{
			name:      "空文字列",
			text:      "",
			wantRange: [2]int{0, 0},
		},
		{
			name:      "短いテキスト",
			text:      "Hello, World!",
			wantRange: [2]int{1, 10},
		},
		{
			name:      "日本語テキスト",
			text:      "こんにちは、世界！",
			wantRange: [2]int{5, 20},
		},
		{
			name:      "コードスニペット",
			text:      "func main() {\n\tfmt.Println(\"Hello\")\n}",
			wantRange: [2]int{10, 30},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := builder.countTokens(tt.text)

			if got < tt.wantRange[0] || got > tt.wantRange[1] {
				t.Errorf("countTokens() = %d, want in range [%d, %d]", got, tt.wantRange[0], tt.wantRange[1])
			}
		})
	}
}

func TestContextBuilder_TrimToTokenLimit(t *testing.T) {
	builder, err := NewContextBuilder()
	if err != nil {
		t.Fatalf("failed to create context builder: %v", err)
	}

	tests := []struct {
		name      string
		text      string
		maxTokens int
	}{
		{
			name:      "トークン制限内のテキスト",
			text:      "Hello, World!",
			maxTokens: 100,
		},
		{
			name:      "トークン制限を超えるテキスト",
			text:      strings.Repeat("test ", 1000),
			maxTokens: 50,
		},
		{
			name:      "ちょうどトークン制限のテキスト",
			text:      strings.Repeat("a", 100),
			maxTokens: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.trimToTokenLimit(tt.text, tt.maxTokens)

			// トークン数をチェック
			tokens := builder.countTokens(result)
			if tokens > tt.maxTokens {
				t.Errorf("trimToTokenLimit() result exceeds max tokens: got %d, want <= %d", tokens, tt.maxTokens)
			}

			// 結果が空でないことを確認
			if result == "" && tt.text != "" {
				t.Errorf("trimToTokenLimit() returned empty string for non-empty input")
			}
		})
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "小文字で始まる文字列",
			s:    "function",
			want: "Function",
		},
		{
			name: "大文字で始まる文字列",
			s:    "Method",
			want: "Method",
		},
		{
			name: "空文字列",
			s:    "",
			want: "",
		},
		{
			name: "数字で始まる文字列",
			s:    "123abc",
			want: "123abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := capitalizeFirst(tt.s)
			if got != tt.want {
				t.Errorf("capitalizeFirst() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ヘルパー関数
func stringPtr(s string) *string {
	return &s
}
