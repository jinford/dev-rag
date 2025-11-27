package search

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/mo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubEmbedder struct{ called bool }

func (e *stubEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.called = true
	return []float32{1, 2, 3}, nil
}

type stubSearchRepo struct {
	results   []*SearchResult
	lastLimit int
}

func (r *stubSearchRepo) SearchByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*SearchResult, error) {
	r.lastLimit = limit
	return r.results, nil
}

func (r *stubSearchRepo) SearchBySource(ctx context.Context, sourceID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*SearchResult, error) {
	r.lastLimit = limit
	return r.results, nil
}

func (r *stubSearchRepo) SearchChunksBySnapshot(ctx context.Context, snapshotID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*SearchResult, error) {
	r.lastLimit = limit
	return r.results, nil
}

func (r *stubSearchRepo) SearchChunksByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*SearchResult, error) {
	r.lastLimit = limit
	return r.results, nil
}

func (r *stubSearchRepo) SearchSummariesBySnapshot(ctx context.Context, snapshotID uuid.UUID, queryVector []float32, limit int, filters SummarySearchFilter) ([]*SummarySearchResult, error) {
	return nil, nil
}

func (r *stubSearchRepo) SearchSummariesByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters SummarySearchFilter) ([]*SummarySearchResult, error) {
	return nil, nil
}

func (r *stubSearchRepo) GetChunkContext(ctx context.Context, chunkID uuid.UUID, beforeCount int, afterCount int) ([]*ChunkContext, error) {
	return nil, nil
}

func (r *stubSearchRepo) GetParentChunk(ctx context.Context, chunkID uuid.UUID) (mo.Option[*ChunkContext], error) {
	return mo.None[*ChunkContext](), nil
}

func (r *stubSearchRepo) GetChildChunks(ctx context.Context, chunkID uuid.UUID) ([]*ChunkContext, error) {
	return nil, nil
}

func (r *stubSearchRepo) GetChunkTree(ctx context.Context, rootID uuid.UUID, maxDepth int) ([]*ChunkContext, error) {
	return nil, nil
}

func TestSearchService_SearchUsesDefaultLimitAndEmbedder(t *testing.T) {
	repo := &stubSearchRepo{
		results: []*SearchResult{{
			ChunkID:   uuid.New(),
			FilePath:  "foo.go",
			StartLine: 1,
			EndLine:   5,
			Content:   "test",
			Score:     0.9,
		}},
	}
	embedder := &stubEmbedder{}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{AddSource: false}))
	svc := NewSearchService(repo, embedder, WithSearchLogger(logger))

	params := SearchParams{
		ProductID: mo.Some(uuid.New()),
		Query:     "hello",
		Limit:     0, // default should be applied
	}

	results, err := svc.Search(context.Background(), params)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 10, repo.lastLimit) // default value applied
	assert.True(t, embedder.called)
}
