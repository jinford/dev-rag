package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// IndexResult はインデックス化の結果
type IndexResult struct {
	SnapshotID        string
	VersionIdentifier string
	ProcessedFiles    int
	TotalChunks       int
	Duration          time.Duration
}

// Indexer はインデックス化を実行する interface
type Indexer interface {
	IndexSource(ctx context.Context, sourceType domain.SourceType, params domain.IndexParams) (*IndexResult, error)
}

// IndexService はインデックス化のユースケースを提供します
type IndexService struct {
	indexer Indexer
	log     *slog.Logger
}

// NewIndexService は新しいIndexServiceを作成します
func NewIndexService(indexer Indexer, log *slog.Logger) *IndexService {
	return &IndexService{
		indexer: indexer,
		log:     log,
	}
}

// IndexSource はソースをインデックス化します
func (s *IndexService) IndexSource(ctx context.Context, sourceType domain.SourceType, params domain.IndexParams) (*IndexResult, error) {
	s.log.Info("Starting index process",
		"sourceType", sourceType,
		"identifier", params.Identifier,
		"product", params.ProductName,
		"forceInit", params.ForceInit,
	)

	// バリデーション
	if params.Identifier == "" {
		return nil, fmt.Errorf("identifier is required")
	}
	if params.ProductName == "" {
		return nil, fmt.Errorf("product name is required")
	}

	// インデックス化を実行（indexer経由）
	result, err := s.indexer.IndexSource(ctx, sourceType, params)
	if err != nil {
		s.log.Error("Index process failed",
			"sourceType", sourceType,
			"identifier", params.Identifier,
			"error", err,
		)
		return nil, fmt.Errorf("failed to index source: %w", err)
	}

	s.log.Info("Index process completed",
		"snapshotID", result.SnapshotID,
		"processedFiles", result.ProcessedFiles,
		"totalChunks", result.TotalChunks,
		"duration", result.Duration,
	)

	return result, nil
}

// ReindexSource は再インデックス化を実行します
func (s *IndexService) ReindexSource(ctx context.Context, sourceType domain.SourceType, params domain.IndexParams) (*IndexResult, error) {
	s.log.Info("Starting reindex process",
		"sourceType", sourceType,
		"identifier", params.Identifier,
		"product", params.ProductName,
	)

	// ForceInitをtrueに設定して再インデックス化
	params.ForceInit = true

	return s.IndexSource(ctx, sourceType, params)
}
