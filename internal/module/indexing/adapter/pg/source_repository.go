package pg

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	
)

// SourceRepository はソース集約の永続化アダプターです
type SourceRepository struct {
	q sqlc.Querier
}

// NewSourceRepository は新しいソースリポジトリを作成します
func NewSourceRepository(q sqlc.Querier) *SourceRepository {
	return &SourceRepository{q: q}
}

// 読み取り操作の実装

var _ domain.SourceReader = (*SourceRepository)(nil)

// GetByID はIDでソースを取得します
func (r *SourceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Source, error) {
	sqlcSource, err := r.q.GetSource(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("source not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	var metadata domain.SourceMetadata
	if err := json.Unmarshal(sqlcSource.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	source := &domain.Source{
		ID:         PgtypeToUUID(sqlcSource.ID),
		ProductID:  PgtypeToUUID(sqlcSource.ProductID),
		Name:       sqlcSource.Name,
		SourceType: domain.SourceType(sqlcSource.SourceType),
		Metadata:   metadata,
		CreatedAt:  PgtypeToTime(sqlcSource.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcSource.UpdatedAt),
	}

	return source, nil
}

// GetByName は名前でソースを取得します
func (r *SourceRepository) GetByName(ctx context.Context, name string) (*domain.Source, error) {
	sqlcSource, err := r.q.GetSourceByName(ctx, name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("source not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	var metadata domain.SourceMetadata
	if err := json.Unmarshal(sqlcSource.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	source := &domain.Source{
		ID:         PgtypeToUUID(sqlcSource.ID),
		ProductID:  PgtypeToUUID(sqlcSource.ProductID),
		Name:       sqlcSource.Name,
		SourceType: domain.SourceType(sqlcSource.SourceType),
		Metadata:   metadata,
		CreatedAt:  PgtypeToTime(sqlcSource.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcSource.UpdatedAt),
	}

	return source, nil
}

// ListByProductID はプロダクトIDでソースを一覧取得します
func (r *SourceRepository) ListByProductID(ctx context.Context, productID uuid.UUID) ([]*domain.Source, error) {
	sqlcSources, err := r.q.ListSourcesByProduct(ctx, UUIDToPgtype(productID))
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}

	sources := make([]*domain.Source, 0, len(sqlcSources))
	for _, sqlcSource := range sqlcSources {
		var metadata domain.SourceMetadata
		if err := json.Unmarshal(sqlcSource.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		source := &domain.Source{
			ID:         PgtypeToUUID(sqlcSource.ID),
			ProductID:  PgtypeToUUID(sqlcSource.ProductID),
			Name:       sqlcSource.Name,
			SourceType: domain.SourceType(sqlcSource.SourceType),
			Metadata:   metadata,
			CreatedAt:  PgtypeToTime(sqlcSource.CreatedAt),
			UpdatedAt:  PgtypeToTime(sqlcSource.UpdatedAt),
		}
		sources = append(sources, source)
	}

	return sources, nil
}

// GetSnapshotByVersion はバージョン識別子でスナップショットを取得します
func (r *SourceRepository) GetSnapshotByVersion(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*domain.SourceSnapshot, error) {
	sqlcSnapshot, err := r.q.GetSourceSnapshotByVersion(ctx, sqlc.GetSourceSnapshotByVersionParams{
		SourceID:          UUIDToPgtype(sourceID),
		VersionIdentifier: versionIdentifier,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("snapshot not found: %s@%s", sourceID, versionIdentifier)
		}
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	snapshot := &domain.SourceSnapshot{
		ID:                PgtypeToUUID(sqlcSnapshot.ID),
		SourceID:          PgtypeToUUID(sqlcSnapshot.SourceID),
		VersionIdentifier: sqlcSnapshot.VersionIdentifier,
		Indexed:           sqlcSnapshot.Indexed,
		IndexedAt:         PgtypeToTimePtr(sqlcSnapshot.IndexedAt),
		CreatedAt:         PgtypeToTime(sqlcSnapshot.CreatedAt),
	}

	return snapshot, nil
}

// GetLatestIndexedSnapshot は直近でインデックス済みのスナップショットを取得します
func (r *SourceRepository) GetLatestIndexedSnapshot(ctx context.Context, sourceID uuid.UUID) (*domain.SourceSnapshot, error) {
	sqlcSnapshot, err := r.q.GetLatestIndexedSnapshot(ctx, UUIDToPgtype(sourceID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("no indexed snapshot found for source: %s", sourceID)
		}
		return nil, fmt.Errorf("failed to get latest indexed snapshot: %w", err)
	}

	snapshot := &domain.SourceSnapshot{
		ID:                PgtypeToUUID(sqlcSnapshot.ID),
		SourceID:          PgtypeToUUID(sqlcSnapshot.SourceID),
		VersionIdentifier: sqlcSnapshot.VersionIdentifier,
		Indexed:           sqlcSnapshot.Indexed,
		IndexedAt:         PgtypeToTimePtr(sqlcSnapshot.IndexedAt),
		CreatedAt:         PgtypeToTime(sqlcSnapshot.CreatedAt),
	}

	return snapshot, nil
}

// ListSnapshotsBySource はソースのスナップショット一覧を取得します
func (r *SourceRepository) ListSnapshotsBySource(ctx context.Context, sourceID uuid.UUID) ([]*domain.SourceSnapshot, error) {
	sqlcSnapshots, err := r.q.ListSourceSnapshotsBySource(ctx, UUIDToPgtype(sourceID))
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	snapshots := make([]*domain.SourceSnapshot, 0, len(sqlcSnapshots))
	for _, sqlcSnapshot := range sqlcSnapshots {
		snapshot := &domain.SourceSnapshot{
			ID:                PgtypeToUUID(sqlcSnapshot.ID),
			SourceID:          PgtypeToUUID(sqlcSnapshot.SourceID),
			VersionIdentifier: sqlcSnapshot.VersionIdentifier,
			Indexed:           sqlcSnapshot.Indexed,
			IndexedAt:         PgtypeToTimePtr(sqlcSnapshot.IndexedAt),
			CreatedAt:         PgtypeToTime(sqlcSnapshot.CreatedAt),
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// 書き込み操作の実装

var _ domain.SourceWriter = (*SourceRepository)(nil)

// CreateIfNotExists は名前でソースを検索し、存在しなければ作成します（冪等）
func (r *SourceRepository) CreateIfNotExists(ctx context.Context, name string, sourceType domain.SourceType, productID uuid.UUID, metadata domain.SourceMetadata) (*domain.Source, error) {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	sqlcSource, err := r.q.CreateSourceIfNotExists(ctx, sqlc.CreateSourceIfNotExistsParams{
		Name:       name,
		SourceType: string(sourceType),
		ProductID:  UUIDToPgtype(productID),
		Metadata:   metadataJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create source: %w", err)
	}

	source := &domain.Source{
		ID:         PgtypeToUUID(sqlcSource.ID),
		ProductID:  PgtypeToUUID(sqlcSource.ProductID),
		Name:       sqlcSource.Name,
		SourceType: domain.SourceType(sqlcSource.SourceType),
		Metadata:   metadata,
		CreatedAt:  PgtypeToTime(sqlcSource.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcSource.UpdatedAt),
	}

	return source, nil
}

// CreateSnapshot はスナップショットを作成します
func (r *SourceRepository) CreateSnapshot(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*domain.SourceSnapshot, error) {
	sqlcSnapshot, err := r.q.CreateSourceSnapshot(ctx, sqlc.CreateSourceSnapshotParams{
		SourceID:          UUIDToPgtype(sourceID),
		VersionIdentifier: versionIdentifier,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	snapshot := &domain.SourceSnapshot{
		ID:                PgtypeToUUID(sqlcSnapshot.ID),
		SourceID:          PgtypeToUUID(sqlcSnapshot.SourceID),
		VersionIdentifier: sqlcSnapshot.VersionIdentifier,
		Indexed:           sqlcSnapshot.Indexed,
		IndexedAt:         PgtypeToTimePtr(sqlcSnapshot.IndexedAt),
		CreatedAt:         PgtypeToTime(sqlcSnapshot.CreatedAt),
	}

	return snapshot, nil
}

// MarkSnapshotIndexed はスナップショットをインデックス済みとしてマークします
func (r *SourceRepository) MarkSnapshotIndexed(ctx context.Context, snapshotID uuid.UUID) error {
	_, err := r.q.MarkSnapshotIndexed(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return fmt.Errorf("failed to mark snapshot as indexed: %w", err)
	}
	return nil
}
