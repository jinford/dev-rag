package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/sqlc"
)

// SourceRepositoryR はソース集約に対する読み取り専用のデータベース操作を提供します
// 集約: Source（ルート）+ SourceSnapshot + GitRef
type SourceRepositoryR struct {
	q sqlc.Querier
}

// NewSourceRepositoryR は新しい読み取り専用リポジトリを作成します
func NewSourceRepositoryR(q sqlc.Querier) *SourceRepositoryR {
	return &SourceRepositoryR{q: q}
}

// SourceRepositoryRW は SourceRepositoryR を埋め込み、書き込み操作を提供します
type SourceRepositoryRW struct {
	*SourceRepositoryR
}

// NewSourceRepositoryRW は読み書き可能なリポジトリを作成します
func NewSourceRepositoryRW(q sqlc.Querier) *SourceRepositoryRW {
	return &SourceRepositoryRW{SourceRepositoryR: NewSourceRepositoryR(q)}
}

// SourceWithStats はソースと統計情報を含む構造体です
type SourceWithStats struct {
	models.Source
	LastIndexedAt *time.Time `json:"lastIndexedAt,omitempty"`
}

// === Source操作 ===

// CreateIfNotExists は名前でソースを検索し、存在しなければ作成します（冪等）
func (rw *SourceRepositoryRW) CreateIfNotExists(ctx context.Context, name string, sourceType models.SourceType, productID uuid.UUID, metadata models.SourceMetadata) (*models.Source, error) {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	sqlcSource, err := rw.q.CreateSourceIfNotExists(ctx, sqlc.CreateSourceIfNotExistsParams{
		Name:       name,
		SourceType: string(sourceType),
		ProductID:  UUIDToPgtype(productID),
		Metadata:   metadataJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create source: %w", err)
	}

	source := &models.Source{
		ID:         PgtypeToUUID(sqlcSource.ID),
		ProductID:  PgtypeToUUID(sqlcSource.ProductID),
		Name:       sqlcSource.Name,
		SourceType: models.SourceType(sqlcSource.SourceType),
		Metadata:   metadata,
		CreatedAt:  PgtypeToTime(sqlcSource.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcSource.UpdatedAt),
	}

	return source, nil
}

// GetByID はIDでソースを取得します
func (r *SourceRepositoryR) GetByID(ctx context.Context, id uuid.UUID) (*models.Source, error) {
	sqlcSource, err := r.q.GetSource(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("source not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	var metadata models.SourceMetadata
	if err := json.Unmarshal(sqlcSource.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	source := &models.Source{
		ID:         PgtypeToUUID(sqlcSource.ID),
		ProductID:  PgtypeToUUID(sqlcSource.ProductID),
		Name:       sqlcSource.Name,
		SourceType: models.SourceType(sqlcSource.SourceType),
		Metadata:   metadata,
		CreatedAt:  PgtypeToTime(sqlcSource.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcSource.UpdatedAt),
	}

	return source, nil
}

// GetByName は名前でソースを取得します
func (r *SourceRepositoryR) GetByName(ctx context.Context, name string) (*models.Source, error) {
	sqlcSource, err := r.q.GetSourceByName(ctx, name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("source not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	var metadata models.SourceMetadata
	if err := json.Unmarshal(sqlcSource.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	source := &models.Source{
		ID:         PgtypeToUUID(sqlcSource.ID),
		ProductID:  PgtypeToUUID(sqlcSource.ProductID),
		Name:       sqlcSource.Name,
		SourceType: models.SourceType(sqlcSource.SourceType),
		Metadata:   metadata,
		CreatedAt:  PgtypeToTime(sqlcSource.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcSource.UpdatedAt),
	}

	return source, nil
}

// ListByProductID はプロダクトIDでソースを一覧取得します
func (r *SourceRepositoryR) ListByProductID(ctx context.Context, productID uuid.UUID) ([]*models.Source, error) {
	sqlcSources, err := r.q.ListSourcesByProduct(ctx, UUIDToPgtype(productID))
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}

	sources := make([]*models.Source, 0, len(sqlcSources))
	for _, sqlcSource := range sqlcSources {
		var metadata models.SourceMetadata
		if err := json.Unmarshal(sqlcSource.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		source := &models.Source{
			ID:         PgtypeToUUID(sqlcSource.ID),
			ProductID:  PgtypeToUUID(sqlcSource.ProductID),
			Name:       sqlcSource.Name,
			SourceType: models.SourceType(sqlcSource.SourceType),
			Metadata:   metadata,
			CreatedAt:  PgtypeToTime(sqlcSource.CreatedAt),
			UpdatedAt:  PgtypeToTime(sqlcSource.UpdatedAt),
		}
		sources = append(sources, source)
	}

	return sources, nil
}

// === SourceSnapshot操作 ===

// CreateSnapshot はスナップショットを作成します
func (rw *SourceRepositoryRW) CreateSnapshot(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*models.SourceSnapshot, error) {
	sqlcSnapshot, err := rw.q.CreateSourceSnapshot(ctx, sqlc.CreateSourceSnapshotParams{
		SourceID:          UUIDToPgtype(sourceID),
		VersionIdentifier: versionIdentifier,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	snapshot := &models.SourceSnapshot{
		ID:                PgtypeToUUID(sqlcSnapshot.ID),
		SourceID:          PgtypeToUUID(sqlcSnapshot.SourceID),
		VersionIdentifier: sqlcSnapshot.VersionIdentifier,
		Indexed:           sqlcSnapshot.Indexed,
		IndexedAt:         PgtypeToTimePtr(sqlcSnapshot.IndexedAt),
		CreatedAt:         PgtypeToTime(sqlcSnapshot.CreatedAt),
	}

	return snapshot, nil
}

// GetSnapshotByVersion はバージョン識別子でスナップショットを取得します
func (r *SourceRepositoryR) GetSnapshotByVersion(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*models.SourceSnapshot, error) {
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

	snapshot := &models.SourceSnapshot{
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
func (r *SourceRepositoryR) GetLatestIndexedSnapshot(ctx context.Context, sourceID uuid.UUID) (*models.SourceSnapshot, error) {
	sqlcSnapshot, err := r.q.GetLatestIndexedSnapshot(ctx, UUIDToPgtype(sourceID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get latest indexed snapshot: %w", err)
	}

	snapshot := &models.SourceSnapshot{
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
func (rw *SourceRepositoryRW) MarkSnapshotIndexed(ctx context.Context, snapshotID uuid.UUID) error {
	_, err := rw.q.MarkSnapshotIndexed(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return fmt.Errorf("failed to mark snapshot as indexed: %w", err)
	}
	return nil
}

// === GitRef操作 ===

// UpsertGitRef はGit参照を作成または更新します
func (rw *SourceRepositoryRW) UpsertGitRef(ctx context.Context, sourceID uuid.UUID, refName string, snapshotID uuid.UUID) (*models.GitRef, error) {
	sqlcRef, err := rw.q.CreateGitRef(ctx, sqlc.CreateGitRefParams{
		SourceID:   UUIDToPgtype(sourceID),
		RefName:    refName,
		SnapshotID: UUIDToPgtype(snapshotID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert git ref: %w", err)
	}

	ref := &models.GitRef{
		ID:         PgtypeToUUID(sqlcRef.ID),
		SourceID:   PgtypeToUUID(sqlcRef.SourceID),
		RefName:    sqlcRef.RefName,
		SnapshotID: PgtypeToUUID(sqlcRef.SnapshotID),
		CreatedAt:  PgtypeToTime(sqlcRef.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcRef.UpdatedAt),
	}

	return ref, nil
}

// GetGitRefByName は名前でGit参照を取得します
func (r *SourceRepositoryR) GetGitRefByName(ctx context.Context, sourceID uuid.UUID, refName string) (*models.GitRef, error) {
	sqlcRef, err := r.q.GetGitRefByName(ctx, sqlc.GetGitRefByNameParams{
		SourceID: UUIDToPgtype(sourceID),
		RefName:  refName,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("git ref not found: %s/%s", sourceID, refName)
		}
		return nil, fmt.Errorf("failed to get git ref: %w", err)
	}

	ref := &models.GitRef{
		ID:         PgtypeToUUID(sqlcRef.ID),
		SourceID:   PgtypeToUUID(sqlcRef.SourceID),
		RefName:    sqlcRef.RefName,
		SnapshotID: PgtypeToUUID(sqlcRef.SnapshotID),
		CreatedAt:  PgtypeToTime(sqlcRef.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcRef.UpdatedAt),
	}

	return ref, nil
}

// ListGitRefsBySource はソースのGit参照一覧を取得します
func (r *SourceRepositoryR) ListGitRefsBySource(ctx context.Context, sourceID uuid.UUID) ([]*models.GitRef, error) {
	sqlcRefs, err := r.q.ListGitRefsBySource(ctx, UUIDToPgtype(sourceID))
	if err != nil {
		return nil, fmt.Errorf("failed to list git refs: %w", err)
	}

	refs := make([]*models.GitRef, 0, len(sqlcRefs))
	for _, sqlcRef := range sqlcRefs {
		ref := &models.GitRef{
			ID:         PgtypeToUUID(sqlcRef.ID),
			SourceID:   PgtypeToUUID(sqlcRef.SourceID),
			RefName:    sqlcRef.RefName,
			SnapshotID: PgtypeToUUID(sqlcRef.SnapshotID),
			CreatedAt:  PgtypeToTime(sqlcRef.CreatedAt),
			UpdatedAt:  PgtypeToTime(sqlcRef.UpdatedAt),
		}
		refs = append(refs, ref)
	}

	return refs, nil
}

// ListSnapshotsBySource はソースのスナップショット一覧を取得します
func (r *SourceRepositoryR) ListSnapshotsBySource(ctx context.Context, sourceID uuid.UUID) ([]*models.SourceSnapshot, error) {
	sqlcSnapshots, err := r.q.ListSourceSnapshotsBySource(ctx, UUIDToPgtype(sourceID))
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	snapshots := make([]*models.SourceSnapshot, 0, len(sqlcSnapshots))
	for _, sqlcSnapshot := range sqlcSnapshots {
		snapshot := &models.SourceSnapshot{
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
