package search

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHierarchicalSearcher_GetChunkLabel は getChunkLabel メソッドのユニットテストです
func TestHierarchicalSearcher_GetChunkLabel(t *testing.T) {
	hs := &HierarchicalSearcher{}

	tests := []struct {
		name     string
		chunk    *models.Chunk
		expected string
	}{
		{
			name: "名前とタイプの両方がある場合",
			chunk: &models.Chunk{
				Type: strPtr("function"),
				Name: strPtr("main"),
			},
			expected: "function main",
		},
		{
			name: "名前のみがある場合",
			chunk: &models.Chunk{
				Name: strPtr("myFunction"),
			},
			expected: "myFunction",
		},
		{
			name: "タイプのみがある場合",
			chunk: &models.Chunk{
				Type: strPtr("struct"),
			},
			expected: "struct",
		},
		{
			name: "名前もタイプもない場合",
			chunk: &models.Chunk{
				StartLine: 10,
				EndLine:   20,
			},
			expected: "Chunk L10-L20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hs.getChunkLabel(tt.chunk)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHierarchicalSearcher_BuildContextFromHierarchy はコンテキスト構築のユニットテストです
func TestHierarchicalSearcher_BuildContextFromHierarchy(t *testing.T) {
	hs := &HierarchicalSearcher{}

	t.Run("親と子を含むコンテキスト", func(t *testing.T) {
		result := &HierarchicalSearchResult{
			SearchResult: &models.SearchResult{
				Content: "main function content",
			},
			Parent: &models.Chunk{
				Level:   1,
				Type:    strPtr("file_summary"),
				Name:    strPtr("test.go"),
				Content: "file summary content",
			},
			Children: []*models.Chunk{
				{
					Level:   3,
					Type:    strPtr("logic_block"),
					Name:    strPtr("initialization"),
					Content: "initialization logic",
				},
			},
		}

		context := hs.BuildContextFromHierarchy(result)

		assert.Contains(t, context, "親コンテキスト")
		assert.Contains(t, context, "メインコンテンツ")
		assert.Contains(t, context, "詳細コンテンツ（子チャンク）")
		assert.Contains(t, context, "file summary content")
		assert.Contains(t, context, "main function content")
		assert.Contains(t, context, "initialization logic")
	})

	t.Run("祖先を含むコンテキスト", func(t *testing.T) {
		result := &HierarchicalSearchResult{
			SearchResult: &models.SearchResult{
				Content: "logic block content",
			},
			Ancestors: []*models.Chunk{
				{
					Level:   2,
					Type:    strPtr("function"),
					Name:    strPtr("main"),
					Content: "function content",
				},
				{
					Level:   1,
					Type:    strPtr("file_summary"),
					Name:    strPtr("test.go"),
					Content: "file summary",
				},
			},
		}

		context := hs.BuildContextFromHierarchy(result)

		assert.Contains(t, context, "上位コンテキスト")
		assert.Contains(t, context, "メインコンテンツ")
		assert.Contains(t, context, "file summary")
		assert.Contains(t, context, "function content")
		assert.Contains(t, context, "logic block content")

		// 祖先は遠い順に表示されるべき
		summaryPos := indexOf(context, "file summary")
		funcPos := indexOf(context, "function content")
		assert.True(t, summaryPos < funcPos, "file summary should appear before function content")
	})

	t.Run("親も子もいない場合", func(t *testing.T) {
		result := &HierarchicalSearchResult{
			SearchResult: &models.SearchResult{
				Content: "standalone chunk content",
			},
		}

		context := hs.BuildContextFromHierarchy(result)

		assert.Contains(t, context, "メインコンテンツ")
		assert.Contains(t, context, "standalone chunk content")
		assert.NotContains(t, context, "親コンテキスト")
		assert.NotContains(t, context, "詳細コンテンツ")
	})
}

// TestHierarchicalSearchOptions は HierarchicalSearchOptions 構造体のテストです
func TestHierarchicalSearchOptions(t *testing.T) {
	tests := []struct {
		name     string
		options  HierarchicalSearchOptions
		validate func(*testing.T, HierarchicalSearchOptions)
	}{
		{
			name: "親のみを含める",
			options: HierarchicalSearchOptions{
				IncludeParent: true,
			},
			validate: func(t *testing.T, opts HierarchicalSearchOptions) {
				assert.True(t, opts.IncludeParent)
				assert.False(t, opts.IncludeChildren)
				assert.False(t, opts.IncludeAncestors)
			},
		},
		{
			name: "子のみを含める",
			options: HierarchicalSearchOptions{
				IncludeChildren: true,
			},
			validate: func(t *testing.T, opts HierarchicalSearchOptions) {
				assert.False(t, opts.IncludeParent)
				assert.True(t, opts.IncludeChildren)
				assert.False(t, opts.IncludeAncestors)
			},
		},
		{
			name: "祖先を含める（最大深度指定あり）",
			options: HierarchicalSearchOptions{
				IncludeAncestors: true,
				MaxDepth:         2,
			},
			validate: func(t *testing.T, opts HierarchicalSearchOptions) {
				assert.False(t, opts.IncludeParent)
				assert.True(t, opts.IncludeAncestors)
				assert.Equal(t, 2, opts.MaxDepth)
			},
		},
		{
			name: "すべてを含める",
			options: HierarchicalSearchOptions{
				IncludeParent:    true,
				IncludeChildren:  true,
				IncludeAncestors: true,
				MaxDepth:         0, // 無制限
			},
			validate: func(t *testing.T, opts HierarchicalSearchOptions) {
				assert.True(t, opts.IncludeParent)
				assert.True(t, opts.IncludeChildren)
				assert.True(t, opts.IncludeAncestors)
				assert.Equal(t, 0, opts.MaxDepth)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t, tt.options)
		})
	}
}

// TestHierarchicalSearchResult は HierarchicalSearchResult 構造体のテストです
func TestHierarchicalSearchResult(t *testing.T) {
	chunkID := uuid.New()
	parentID := uuid.New()
	child1ID := uuid.New()
	child2ID := uuid.New()

	result := &HierarchicalSearchResult{
		SearchResult: &models.SearchResult{
			ChunkID:   chunkID,
			FilePath:  "test.go",
			StartLine: 10,
			EndLine:   20,
			Content:   "function content",
			Score:     0.95,
		},
		Parent: &models.Chunk{
			ID:      parentID,
			Level:   1,
			Content: "parent content",
		},
		Children: []*models.Chunk{
			{
				ID:      child1ID,
				Level:   3,
				Content: "child1 content",
			},
			{
				ID:      child2ID,
				Level:   3,
				Content: "child2 content",
			},
		},
		Ancestors: []*models.Chunk{},
	}

	// 基本フィールドの確認
	assert.Equal(t, chunkID, result.ChunkID)
	assert.Equal(t, "test.go", result.FilePath)
	assert.Equal(t, 0.95, result.Score)

	// 親チャンクの確認
	require.NotNil(t, result.Parent)
	assert.Equal(t, parentID, result.Parent.ID)
	assert.Equal(t, 1, result.Parent.Level)

	// 子チャンクの確認
	require.Len(t, result.Children, 2)
	assert.Equal(t, child1ID, result.Children[0].ID)
	assert.Equal(t, child2ID, result.Children[1].ID)

	// 祖先チャンクの確認
	assert.Empty(t, result.Ancestors)
}

// ヘルパー関数
func strPtr(s string) *string {
	return &s
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
