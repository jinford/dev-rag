package query

import (
	"context"
	"testing"

	"github.com/google/uuid"
	searchdomain "github.com/jinford/dev-rag/internal/module/search/domain"
	"github.com/stretchr/testify/assert"
)

func TestEnrichSearchResultsWithHierarchy_NoOptions(t *testing.T) {
	// Optionsがnilの場合、階層情報を含まない基本結果を返す
	baseResults := []*searchdomain.SearchResult{
		{
			ChunkID:   uuid.New(),
			FilePath:  "test.go",
			StartLine: 1,
			EndLine:   10,
			Content:   "test content",
			Score:     0.9,
		},
	}

	// モックRepositoryは不要（階層情報を取得しないため）
	querier := &Querier{repo: nil}

	enhanced, err := querier.EnrichSearchResultsWithHierarchy(context.Background(), baseResults, nil)
	assert.NoError(t, err)
	assert.Len(t, enhanced, 1)
	assert.Nil(t, enhanced[0].ParentChunk)
	assert.Nil(t, enhanced[0].ChildChunks)
}

func TestEnrichSearchResultsWithHierarchy_IncludeParentFalse(t *testing.T) {
	// IncludeParent=false, IncludeChildren=falseの場合、階層情報を取得しない
	baseResults := []*searchdomain.SearchResult{
		{
			ChunkID:   uuid.New(),
			FilePath:  "test.go",
			StartLine: 1,
			EndLine:   10,
			Content:   "test content",
			Score:     0.9,
		},
	}

	options := &SearchOptions{
		IncludeParent:   false,
		IncludeChildren: false,
	}

	querier := &Querier{repo: nil}

	enhanced, err := querier.EnrichSearchResultsWithHierarchy(context.Background(), baseResults, options)
	assert.NoError(t, err)
	assert.Len(t, enhanced, 1)
	assert.Nil(t, enhanced[0].ParentChunk)
	assert.Nil(t, enhanced[0].ChildChunks)
}
