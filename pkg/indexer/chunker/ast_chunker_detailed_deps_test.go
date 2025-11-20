package chunker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractImportsDetailed(t *testing.T) {
	content := `package main

import (
	"fmt"
	"net/http"
	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
)

func main() {
	fmt.Println("Hello")
}
`

	chunker, err := NewChunker()
	assert.NoError(t, err)

	astChunker := NewASTChunkerGo()
	result := astChunker.ChunkWithMetrics(content, chunker)

	assert.True(t, result.ParseSuccess)
	assert.NotEmpty(t, result.Chunks)

	// すべてのチャンクをログに出力
	for i, chunk := range result.Chunks {
		if chunk.Metadata != nil && chunk.Metadata.Name != nil {
			t.Logf("Chunk[%d]: Type=%v, Name=%v, Level=%d, StandardImports=%v, ExternalImports=%v",
				i, chunk.Metadata.Type, *chunk.Metadata.Name, chunk.Metadata.Level,
				chunk.Metadata.StandardImports, chunk.Metadata.ExternalImports)
		}
	}

	// 最初のチャンクのメタデータを確認
	// ファイルサマリーチャンクの次の関数チャンクを探す（レベル2の関数チャンクを探す）
	var mainFuncChunk *ChunkWithMetadata
	for _, chunk := range result.Chunks {
		if chunk.Metadata != nil && chunk.Metadata.Name != nil &&
			*chunk.Metadata.Name == "main" && chunk.Metadata.Level == 2 {
			mainFuncChunk = chunk
			break
		}
	}

	assert.NotNil(t, mainFuncChunk)
	assert.NotNil(t, mainFuncChunk.Metadata)

	// デバッグ情報を出力
	t.Logf("StandardImports: %v", mainFuncChunk.Metadata.StandardImports)
	t.Logf("ExternalImports: %v", mainFuncChunk.Metadata.ExternalImports)
	t.Logf("All Imports: %v", mainFuncChunk.Metadata.Imports)

	// 標準ライブラリのインポートを確認
	assert.Contains(t, mainFuncChunk.Metadata.StandardImports, "fmt")
	assert.Contains(t, mainFuncChunk.Metadata.StandardImports, "net/http")

	// 外部依存のインポートを確認
	assert.Contains(t, mainFuncChunk.Metadata.ExternalImports, "github.com/google/uuid")
	assert.Contains(t, mainFuncChunk.Metadata.ExternalImports, "github.com/jinford/dev-rag/pkg/models")
}

func TestExtractTypeDependencies(t *testing.T) {
	content := `package main

type User struct {
	ID   string
	Name string
}

func CreateUser(name string) *User {
	return &User{Name: name}
}

func ProcessUser(u *User) error {
	return nil
}
`

	chunker, err := NewChunker()
	assert.NoError(t, err)

	astChunker := NewASTChunkerGo()
	result := astChunker.ChunkWithMetrics(content, chunker)

	assert.True(t, result.ParseSuccess)

	// CreateUser関数のチャンクを探す
	var createUserChunk *ChunkWithMetadata
	for _, chunk := range result.Chunks {
		if chunk.Metadata != nil && chunk.Metadata.Name != nil && *chunk.Metadata.Name == "CreateUser" {
			createUserChunk = chunk
			break
		}
	}

	assert.NotNil(t, createUserChunk)
	// User型への依存があることを確認
	// ポインタ型の場合は "*User" として抽出される
	hasUserDep := false
	for _, dep := range createUserChunk.Metadata.TypeDependencies {
		if dep == "User" || dep == "*User" {
			hasUserDep = true
			break
		}
	}
	assert.True(t, hasUserDep, "CreateUser should have dependency on User type")

	// ProcessUser関数のチャンクを探す
	var processUserChunk *ChunkWithMetadata
	for _, chunk := range result.Chunks {
		if chunk.Metadata != nil && chunk.Metadata.Name != nil && *chunk.Metadata.Name == "ProcessUser" {
			processUserChunk = chunk
			break
		}
	}

	assert.NotNil(t, processUserChunk)
	// User型への依存があることを確認
	hasUserDep = false
	for _, dep := range processUserChunk.Metadata.TypeDependencies {
		if dep == "User" || dep == "*User" {
			hasUserDep = true
			break
		}
	}
	assert.True(t, hasUserDep, "ProcessUser should have dependency on User type")
}

func TestIsStandardLibrary(t *testing.T) {
	astChunker := NewASTChunkerGo()

	// 標準ライブラリ
	assert.True(t, astChunker.isStandardLibrary("fmt"))
	assert.True(t, astChunker.isStandardLibrary("net/http"))
	assert.True(t, astChunker.isStandardLibrary("encoding/json"))
	assert.True(t, astChunker.isStandardLibrary("io"))
	assert.True(t, astChunker.isStandardLibrary("golang.org/x/net/context"))

	// 外部依存
	assert.False(t, astChunker.isStandardLibrary("github.com/google/uuid"))
	assert.False(t, astChunker.isStandardLibrary("github.com/jinford/dev-rag/pkg/models"))
	assert.False(t, astChunker.isStandardLibrary("gopkg.in/yaml.v2"))
}

func TestIsBuiltinType(t *testing.T) {
	// 組み込み型
	assert.True(t, isBuiltinType("string"))
	assert.True(t, isBuiltinType("int"))
	assert.True(t, isBuiltinType("bool"))
	assert.True(t, isBuiltinType("error"))
	assert.True(t, isBuiltinType("float64"))

	// カスタム型
	assert.False(t, isBuiltinType("User"))
	assert.False(t, isBuiltinType("*User"))
	assert.False(t, isBuiltinType("[]User"))
}
