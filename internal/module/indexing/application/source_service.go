package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// SourceService はソース管理のユースケースを提供します
type SourceService struct {
	sourceRepo domain.SourceReader
	log        *slog.Logger
}

// NewSourceService は新しいSourceServiceを作成します
func NewSourceService(sourceRepo domain.SourceReader, log *slog.Logger) *SourceService {
	return &SourceService{
		sourceRepo: sourceRepo,
		log:        log,
	}
}

// GetSource はソースを取得します
func (s *SourceService) GetSource(ctx context.Context, sourceID uuid.UUID) (*domain.Source, error) {
	if sourceID == uuid.Nil {
		return nil, fmt.Errorf("source ID is required")
	}

	source, err := s.sourceRepo.GetByID(ctx, sourceID)
	if err != nil {
		s.log.Error("Failed to get source",
			"sourceID", sourceID,
			"error", err,
		)
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return source, nil
}

// GetSourceByName は名前でソースを取得します
func (s *SourceService) GetSourceByName(ctx context.Context, name string) (*domain.Source, error) {
	if name == "" {
		return nil, fmt.Errorf("source name is required")
	}

	source, err := s.sourceRepo.GetByName(ctx, name)
	if err != nil {
		s.log.Error("Failed to get source by name",
			"name", name,
			"error", err,
		)
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return source, nil
}

// SourceFilter はソース一覧取得時のフィルター
type SourceFilter struct {
	ProductID *uuid.UUID
}

// ListSources はソース一覧を取得します
func (s *SourceService) ListSources(ctx context.Context, filter SourceFilter) ([]*domain.Source, error) {
	if filter.ProductID == nil {
		s.log.Warn("Listing all sources without product filter is not recommended")
		return nil, fmt.Errorf("product ID filter is required")
	}

	sources, err := s.sourceRepo.ListByProductID(ctx, *filter.ProductID)
	if err != nil {
		s.log.Error("Failed to list sources",
			"productID", *filter.ProductID,
			"error", err,
		)
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}

	s.log.Info("Sources listed successfully", "count", len(sources))

	return sources, nil
}

// GetLatestSnapshot は最新のインデックス済みスナップショットを取得します
func (s *SourceService) GetLatestSnapshot(ctx context.Context, sourceID uuid.UUID) (*domain.SourceSnapshot, error) {
	if sourceID == uuid.Nil {
		return nil, fmt.Errorf("source ID is required")
	}

	snapshot, err := s.sourceRepo.GetLatestIndexedSnapshot(ctx, sourceID)
	if err != nil {
		s.log.Error("Failed to get latest snapshot",
			"sourceID", sourceID,
			"error", err,
		)
		return nil, fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	return snapshot, nil
}

// ListSnapshots はソースのスナップショット一覧を取得します
func (s *SourceService) ListSnapshots(ctx context.Context, sourceID uuid.UUID) ([]*domain.SourceSnapshot, error) {
	if sourceID == uuid.Nil {
		return nil, fmt.Errorf("source ID is required")
	}

	snapshots, err := s.sourceRepo.ListSnapshotsBySource(ctx, sourceID)
	if err != nil {
		s.log.Error("Failed to list snapshots",
			"sourceID", sourceID,
			"error", err,
		)
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	s.log.Info("Snapshots listed successfully", "sourceID", sourceID, "count", len(snapshots))

	return snapshots, nil
}
