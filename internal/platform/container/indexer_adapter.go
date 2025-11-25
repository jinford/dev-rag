package container

import (
	"context"

	"github.com/jinford/dev-rag/internal/module/indexing/adapter/indexer"
	indexingapp "github.com/jinford/dev-rag/internal/module/indexing/application"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// indexerAdapter は adapter 層の Indexer を application 層の Indexer インターフェースに適応させます
type indexerAdapter struct {
	indexer *indexer.Indexer
}

// newIndexerAdapter は新しいindexerAdapterを作成します
func newIndexerAdapter(idx *indexer.Indexer) *indexerAdapter {
	return &indexerAdapter{
		indexer: idx,
	}
}

// IndexSource は adapter 層の IndexSource を呼び出し、結果を application 層の型に変換します
func (a *indexerAdapter) IndexSource(ctx context.Context, sourceType domain.SourceType, params domain.IndexParams) (*indexingapp.IndexResult, error) {
	result, err := a.indexer.IndexSource(ctx, sourceType, params)
	if err != nil {
		return nil, err
	}

	// adapter 層の IndexResult を application 層の IndexResult に変換
	return &indexingapp.IndexResult{
		SnapshotID:        result.SnapshotID,
		VersionIdentifier: result.VersionIdentifier,
		ProcessedFiles:    result.ProcessedFiles,
		TotalChunks:       result.TotalChunks,
		Duration:          result.Duration,
	}, nil
}
