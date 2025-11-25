package indexing

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
	"github.com/jinford/dev-rag/internal/module/wiki/domain"
)

// IndexingDataReader は indexing モジュールのデータを読み取る adapter 実装です
type IndexingDataReader struct {
	q sqlc.Querier
}

// NewIndexingDataReader は新しい IndexingDataReader を作成します
func NewIndexingDataReader(q sqlc.Querier) domain.IndexingDataReader {
	return &IndexingDataReader{q: q}
}

// Ensure IndexingDataReader implements domain.IndexingDataReader
var _ domain.IndexingDataReader = (*IndexingDataReader)(nil)

// ListSourcesByProduct はプロダクトに属するソース一覧を取得します
func (r *IndexingDataReader) ListSourcesByProduct(ctx context.Context, productID uuid.UUID) ([]*domain.SourceInfo, error) {
	var pgProductID PgtypeUUID
	if err := pgProductID.Scan(productID); err != nil {
		return nil, fmt.Errorf("failed to convert productID: %w", err)
	}

	sources, err := r.q.ListSourcesByProduct(ctx, pgProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}

	result := make([]*domain.SourceInfo, len(sources))
	for i, src := range sources {
		var id uuid.UUID
		if err := id.UnmarshalBinary(src.ID.Bytes[:]); err != nil {
			return nil, fmt.Errorf("failed to convert source ID: %w", err)
		}

		result[i] = &domain.SourceInfo{
			ID:   id,
			Name: src.Name,
		}
	}

	return result, nil
}

// GetLatestIndexedSnapshot は最新のインデックス済みスナップショットを取得します
func (r *IndexingDataReader) GetLatestIndexedSnapshot(ctx context.Context, sourceID uuid.UUID) (*domain.SnapshotInfo, error) {
	var pgSourceID PgtypeUUID
	if err := pgSourceID.Scan(sourceID); err != nil {
		return nil, fmt.Errorf("failed to convert sourceID: %w", err)
	}

	snapshot, err := r.q.GetLatestIndexedSnapshot(ctx, pgSourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	var id uuid.UUID
	if err := id.UnmarshalBinary(snapshot.ID.Bytes[:]); err != nil {
		return nil, fmt.Errorf("failed to convert snapshot ID: %w", err)
	}

	var srcID uuid.UUID
	if err := srcID.UnmarshalBinary(snapshot.SourceID.Bytes[:]); err != nil {
		return nil, fmt.Errorf("failed to convert source ID: %w", err)
	}

	result := &domain.SnapshotInfo{
		ID:                id,
		SourceID:          srcID,
		VersionIdentifier: snapshot.VersionIdentifier,
		Indexed:           snapshot.Indexed,
		CreatedAt:         snapshot.CreatedAt.Time,
	}

	if snapshot.IndexedAt.Valid {
		result.IndexedAt = &snapshot.IndexedAt.Time
	}

	return result, nil
}

// ListFilesBySnapshot はスナップショット配下のファイル一覧を取得します
func (r *IndexingDataReader) ListFilesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*domain.FileInfo, error) {
	var pgSnapshotID PgtypeUUID
	if err := pgSnapshotID.Scan(snapshotID); err != nil {
		return nil, fmt.Errorf("failed to convert snapshotID: %w", err)
	}

	files, err := r.q.ListFilesBySnapshot(ctx, pgSnapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	result := make([]*domain.FileInfo, len(files))
	for i, file := range files {
		var id uuid.UUID
		if err := id.UnmarshalBinary(file.ID.Bytes[:]); err != nil {
			return nil, fmt.Errorf("failed to convert file ID: %w", err)
		}

		result[i] = &domain.FileInfo{
			FileID:   id,
			Path:     file.Path,
			Size:     file.Size,
			Language: file.Language.String,
			Domain:   file.Domain.String,
			Hash:     file.ContentHash,
		}
	}

	return result, nil
}
