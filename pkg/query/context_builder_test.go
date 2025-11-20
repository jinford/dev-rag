package query

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestBuildContextWithHierarchy(t *testing.T) {
	// テストデータの準備
	parentChunk := &models.Chunk{
		ID:        uuid.New(),
		Content:   "parent content",
		StartLine: 1,
		EndLine:   5,
	}

	childChunk1 := &models.Chunk{
		ID:        uuid.New(),
		Content:   "child content 1",
		StartLine: 11,
		EndLine:   15,
	}

	childChunk2 := &models.Chunk{
		ID:        uuid.New(),
		Content:   "child content 2",
		StartLine: 16,
		EndLine:   20,
	}

	results := []*EnhancedSearchResult{
		{
			SearchResult: &models.SearchResult{
				ChunkID:   uuid.New(),
				FilePath:  "test.go",
				StartLine: 6,
				EndLine:   10,
				Content:   "main content",
				Score:     0.9,
			},
			ParentChunk: parentChunk,
			ChildChunks: []*models.Chunk{childChunk1, childChunk2},
		},
	}

	builder := NewContextBuilder(8000)
	context := builder.BuildContextWithHierarchy(results)

	// 親チャンク、メインチャンク、子チャンクの内容が含まれることを確認
	assert.Contains(t, context, "parent content")
	assert.Contains(t, context, "main content")
	assert.Contains(t, context, "child content 1")
	assert.Contains(t, context, "child content 2")

	// 構造のマーカーが含まれることを確認
	assert.Contains(t, context, "## Parent Context")
	assert.Contains(t, context, "## Search Result 1")
	assert.Contains(t, context, "### Sub-sections:")
}

func TestBuildContextWithHierarchy_NoHierarchy(t *testing.T) {
	// 階層情報がない場合
	results := []*EnhancedSearchResult{
		{
			SearchResult: &models.SearchResult{
				ChunkID:   uuid.New(),
				FilePath:  "test.go",
				StartLine: 1,
				EndLine:   10,
				Content:   "main content",
				Score:     0.9,
			},
		},
	}

	builder := NewContextBuilder(8000)
	context := builder.BuildContextWithHierarchy(results)

	// メインチャンクの内容のみが含まれることを確認
	assert.Contains(t, context, "main content")
	assert.NotContains(t, context, "## Parent Context")
	assert.NotContains(t, context, "### Sub-sections:")
}

func TestTruncateToTokenLimit(t *testing.T) {
	builder := NewContextBuilder(100) // 100トークン = 400文字

	// 短いコンテキスト（切り詰め不要）
	shortContext := "This is a short context."
	truncated := builder.TruncateToTokenLimit(shortContext)
	assert.Equal(t, shortContext, truncated)

	// 長いコンテキスト（切り詰め必要）
	longContext := make([]byte, 1000)
	for i := range longContext {
		longContext[i] = 'a'
	}
	truncated = builder.TruncateToTokenLimit(string(longContext))
	assert.Less(t, len(truncated), len(longContext))
	assert.Contains(t, truncated, "... (truncated)")
}

func TestEstimateTokenCount(t *testing.T) {
	builder := NewContextBuilder(8000)

	// 400文字 = 約100トークン
	content := make([]byte, 400)
	for i := range content {
		content[i] = 'a'
	}

	estimatedTokens := builder.EstimateTokenCount(string(content))
	assert.Equal(t, 100, estimatedTokens)
}

func TestBuildSimpleContext(t *testing.T) {
	results := []*EnhancedSearchResult{
		{
			SearchResult: &models.SearchResult{
				ChunkID:   uuid.New(),
				FilePath:  "test.go",
				StartLine: 1,
				EndLine:   10,
				Content:   "content 1",
				Score:     0.9,
			},
		},
		{
			SearchResult: &models.SearchResult{
				ChunkID:   uuid.New(),
				FilePath:  "test2.go",
				StartLine: 1,
				EndLine:   10,
				Content:   "content 2",
				Score:     0.8,
			},
		},
	}

	builder := NewContextBuilder(8000)
	context := builder.BuildSimpleContext(results)

	// 両方のコンテンツが含まれることを確認
	assert.Contains(t, context, "content 1")
	assert.Contains(t, context, "content 2")
	assert.Contains(t, context, "## Search Result 1")
	assert.Contains(t, context, "## Search Result 2")
}

func TestBuildContextWithMetadata(t *testing.T) {
	results := []*EnhancedSearchResult{
		{
			SearchResult: &models.SearchResult{
				ChunkID:   uuid.New(),
				FilePath:  "test.go",
				StartLine: 1,
				EndLine:   10,
				Content:   "main content",
				Score:     0.9,
			},
		},
	}

	builder := NewContextBuilder(8000)
	context := builder.BuildContextWithMetadata(results)

	// メタデータが含まれることを確認
	assert.Contains(t, context, "test.go")
	assert.Contains(t, context, "0.9000")
	assert.Contains(t, context, "main content")
}

func TestBuildCompactContext(t *testing.T) {
	results := []*EnhancedSearchResult{
		{
			SearchResult: &models.SearchResult{
				ChunkID:   uuid.New(),
				FilePath:  "test.go",
				StartLine: 1,
				EndLine:   10,
				Content:   "content 1",
				Score:     0.9,
			},
		},
	}

	builder := NewContextBuilder(8000)
	context := builder.BuildCompactContext(results)

	// コンパクトな形式でコンテンツが含まれることを確認
	assert.Contains(t, context, "content 1")
	assert.Contains(t, context, "test.go")
	assert.Contains(t, context, "[1]")
}
