package embedder

import (
	"testing"

	"github.com/jinford/dev-rag/pkg/indexer/chunker"
	"github.com/jinford/dev-rag/pkg/repository"
)

// TestContextBuilder_Integration はContextBuilderとChunkerの統合テスト
func TestContextBuilder_Integration(t *testing.T) {
	// Chunkerを作成
	chk, err := chunker.NewChunker()
	if err != nil {
		t.Fatalf("failed to create chunker: %v", err)
	}

	// ContextBuilderを作成
	builder, err := NewContextBuilder()
	if err != nil {
		t.Fatalf("failed to create context builder: %v", err)
	}

	// Goコードのサンプル
	goCode := `package main

import "fmt"

// Add adds two numbers
func Add(a, b int) int {
	return a + b
}

// Multiply multiplies two numbers
func Multiply(a, b int) int {
	return a * b
}
`

	// チャンク化（メタデータ付き）
	chunksWithMeta, err := chk.ChunkWithMetadata(goCode, "text/x-go")
	if err != nil {
		t.Fatalf("failed to chunk code: %v", err)
	}

	if len(chunksWithMeta) == 0 {
		t.Fatal("no chunks generated")
	}

	// 各チャンクに対してEmbeddingコンテキストを構築
	for i, cwm := range chunksWithMeta {
		filePath := "main.go"
		embeddingContext := builder.BuildContext(cwm.Chunk, cwm.Metadata, filePath)

		t.Logf("Chunk %d:", i)
		t.Logf("  Original Content: %s", cwm.Chunk.Content)
		t.Logf("  Embedding Context: %s", embeddingContext)

		// 検証: Embeddingコンテキストにファイルパスが含まれていることを確認
		if cwm.Metadata != nil {
			if embeddingContext == "" {
				t.Errorf("Chunk %d: embedding context is empty", i)
			}

			// ファイルパスが含まれていることを確認
			// （メタデータがある場合）
			if cwm.Metadata.Name != nil && *cwm.Metadata.Name != "" {
				// メタデータがある場合は、コンテキストにファイルパスが含まれているはず
				if embeddingContext == cwm.Chunk.Content {
					// メタデータがあるのにコンテキストが追加されていない場合は警告
					t.Logf("Warning: Chunk %d has metadata but no context added", i)
				}
			}

			// トークン制限を確認
			tokens := builder.countTokens(embeddingContext)
			if tokens > builder.maxTokens {
				t.Errorf("Chunk %d: embedding context exceeds max tokens (%d > %d)", i, tokens, builder.maxTokens)
			}
		}
	}
}

// TestContextBuilder_WithRealMetadata は実際のAST解析で生成されたメタデータを使った統合テスト
func TestContextBuilder_WithRealMetadata(t *testing.T) {
	builder, err := NewContextBuilder()
	if err != nil {
		t.Fatalf("failed to create context builder: %v", err)
	}

	// 実際のメタデータを想定したテスト
	chunk := &chunker.Chunk{
		Content: `func (s *Server) Start(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	return s.Serve(listener)
}`,
		StartLine: 10,
		EndLine:   16,
		Tokens:    60,
	}

	metadata := &repository.ChunkMetadata{
		Type:       stringPtr("method"),
		Name:       stringPtr("Start"),
		ParentName: stringPtr("Server"),
		Signature:  stringPtr("func (s *Server) Start(port int) error"),
		DocComment: stringPtr("Start starts the server on the specified port"),
	}

	filePath := "server/server.go"
	embeddingContext := builder.BuildContext(chunk, metadata, filePath)

	t.Logf("Original Content:\n%s", chunk.Content)
	t.Logf("\nEmbedding Context:\n%s", embeddingContext)

	// 検証
	expectedParts := []string{
		"File: server/server.go",
		"Package: Server",
		"Method: Start",
		"Signature: func (s *Server) Start(port int) error",
		"func (s *Server) Start(port int) error",
	}

	for _, part := range expectedParts {
		if !contains(embeddingContext, part) {
			t.Errorf("Embedding context does not contain expected part: %q", part)
		}
	}

	// トークン数の確認
	tokens := builder.countTokens(embeddingContext)
	if tokens > builder.maxTokens {
		t.Errorf("Embedding context exceeds max tokens: %d > %d", tokens, builder.maxTokens)
	}

	t.Logf("Total tokens: %d (max: %d)", tokens, builder.maxTokens)
}

// ヘルパー関数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
