package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jinford/dev-rag/pkg/models"
)

// SourceRepository はソース集約のデータベース操作を提供します
// 集約: Source（ルート）+ SourceSnapshot + GitRef
type SourceRepository struct {
	pool *pgxpool.Pool
}

// NewSourceRepository は新しいSourceRepositoryを作成します
func NewSourceRepository(pool *pgxpool.Pool) *SourceRepository {
	return &SourceRepository{pool: pool}
}

// SourceWithStats はソースと統計情報を含む構造体です
type SourceWithStats struct {
	models.Source
	LastIndexedAt *time.Time `json:"lastIndexedAt,omitempty"`
}

// === Source操作 ===

// CreateIfNotExists は名前でソースを検索し、存在しなければ作成します（冪等）
func (r *SourceRepository) CreateIfNotExists(ctx context.Context, name string, sourceType models.SourceType, productID *uuid.UUID, metadata models.SourceMetadata) (*models.Source, error) {
	query := `
		INSERT INTO sources (name, source_type, product_id, metadata)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name)
		DO UPDATE SET
			source_type = EXCLUDED.source_type,
			product_id = EXCLUDED.product_id,
			metadata = EXCLUDED.metadata,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, product_id, name, source_type, metadata, created_at, updated_at
	`

	var source models.Source
	err := r.pool.QueryRow(ctx, query, name, sourceType, productID, metadata).Scan(
		&source.ID,
		&source.ProductID,
		&source.Name,
		&source.SourceType,
		&source.Metadata,
		&source.CreatedAt,
		&source.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create source: %w", err)
	}

	return &source, nil
}

// GetByID はIDでソースを取得します
func (r *SourceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Source, error) {
	query := `
		SELECT id, product_id, name, source_type, metadata, created_at, updated_at
		FROM sources
		WHERE id = $1
	`

	var source models.Source
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&source.ID,
		&source.ProductID,
		&source.Name,
		&source.SourceType,
		&source.Metadata,
		&source.CreatedAt,
		&source.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("source not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return &source, nil
}

// GetByName は名前でソースを取得します
func (r *SourceRepository) GetByName(ctx context.Context, name string) (*models.Source, error) {
	query := `
		SELECT id, product_id, name, source_type, metadata, created_at, updated_at
		FROM sources
		WHERE name = $1
	`

	var source models.Source
	err := r.pool.QueryRow(ctx, query, name).Scan(
		&source.ID,
		&source.ProductID,
		&source.Name,
		&source.SourceType,
		&source.Metadata,
		&source.CreatedAt,
		&source.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("source not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return &source, nil
}

// List はすべてのソースを取得します
func (r *SourceRepository) List(ctx context.Context) ([]*models.Source, error) {
	query := `
		SELECT id, product_id, name, source_type, metadata, created_at, updated_at
		FROM sources
		ORDER BY name
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}
	defer rows.Close()

	var sources []*models.Source
	for rows.Next() {
		var source models.Source
		if err := rows.Scan(
			&source.ID,
			&source.ProductID,
			&source.Name,
			&source.SourceType,
			&source.Metadata,
			&source.CreatedAt,
			&source.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan source: %w", err)
		}
		sources = append(sources, &source)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sources: %w", err)
	}

	return sources, nil
}

// ListByProductID はプロダクトIDに紐づくソース一覧を取得します
func (r *SourceRepository) ListByProductID(ctx context.Context, productID uuid.UUID) ([]*models.Source, error) {
	query := `
		SELECT id, product_id, name, source_type, metadata, created_at, updated_at
		FROM sources
		WHERE product_id = $1
		ORDER BY name
	`

	rows, err := r.pool.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources by product: %w", err)
	}
	defer rows.Close()

	var sources []*models.Source
	for rows.Next() {
		var source models.Source
		if err := rows.Scan(
			&source.ID,
			&source.ProductID,
			&source.Name,
			&source.SourceType,
			&source.Metadata,
			&source.CreatedAt,
			&source.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan source: %w", err)
		}
		sources = append(sources, &source)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sources: %w", err)
	}

	return sources, nil
}

// GetWithLastIndexedAt は最終インデックス日時付きソース情報を取得します
func (r *SourceRepository) GetWithLastIndexedAt(ctx context.Context, id uuid.UUID) (*SourceWithStats, error) {
	query := `
		SELECT
			s.id,
			s.product_id,
			s.name,
			s.source_type,
			s.metadata,
			s.created_at,
			s.updated_at,
			MAX(ss.indexed_at) AS last_indexed_at
		FROM sources s
		LEFT JOIN source_snapshots ss ON s.id = ss.source_id AND ss.indexed = TRUE
		WHERE s.id = $1
		GROUP BY s.id, s.product_id, s.name, s.source_type, s.metadata, s.created_at, s.updated_at
	`

	var sourceWithStats SourceWithStats
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&sourceWithStats.ID,
		&sourceWithStats.ProductID,
		&sourceWithStats.Name,
		&sourceWithStats.SourceType,
		&sourceWithStats.Metadata,
		&sourceWithStats.CreatedAt,
		&sourceWithStats.UpdatedAt,
		&sourceWithStats.LastIndexedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("source not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get source with stats: %w", err)
	}

	return &sourceWithStats, nil
}

// Delete はソースを削除します（内部API専用）
func (r *SourceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM sources WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("source not found: %s", id)
	}

	return nil
}

// === SourceSnapshot操作（ソース集約の一部） ===

// CreateSnapshot はスナップショットを作成します（同じバージョン識別子が存在する場合は既存を返す）
func (r *SourceRepository) CreateSnapshot(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*models.SourceSnapshot, error) {
	query := `
		INSERT INTO source_snapshots (source_id, version_identifier)
		VALUES ($1, $2)
		ON CONFLICT (source_id, version_identifier)
		DO UPDATE SET created_at = source_snapshots.created_at
		RETURNING id, source_id, version_identifier, indexed, indexed_at, created_at
	`

	var snapshot models.SourceSnapshot
	err := r.pool.QueryRow(ctx, query, sourceID, versionIdentifier).Scan(
		&snapshot.ID,
		&snapshot.SourceID,
		&snapshot.VersionIdentifier,
		&snapshot.Indexed,
		&snapshot.IndexedAt,
		&snapshot.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	return &snapshot, nil
}

// GetSnapshotByID はIDでスナップショットを取得します
func (r *SourceRepository) GetSnapshotByID(ctx context.Context, id uuid.UUID) (*models.SourceSnapshot, error) {
	query := `
		SELECT id, source_id, version_identifier, indexed, indexed_at, created_at
		FROM source_snapshots
		WHERE id = $1
	`

	var snapshot models.SourceSnapshot
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&snapshot.ID,
		&snapshot.SourceID,
		&snapshot.VersionIdentifier,
		&snapshot.Indexed,
		&snapshot.IndexedAt,
		&snapshot.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("snapshot not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	return &snapshot, nil
}

// GetSnapshotByVersion はソースID + バージョン識別子でスナップショットを取得します
func (r *SourceRepository) GetSnapshotByVersion(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*models.SourceSnapshot, error) {
	query := `
		SELECT id, source_id, version_identifier, indexed, indexed_at, created_at
		FROM source_snapshots
		WHERE source_id = $1 AND version_identifier = $2
	`

	var snapshot models.SourceSnapshot
	err := r.pool.QueryRow(ctx, query, sourceID, versionIdentifier).Scan(
		&snapshot.ID,
		&snapshot.SourceID,
		&snapshot.VersionIdentifier,
		&snapshot.Indexed,
		&snapshot.IndexedAt,
		&snapshot.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("snapshot not found for source %s version %s", sourceID, versionIdentifier)
		}
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	return &snapshot, nil
}

// GetLatestIndexedSnapshotByRef はソースID + 参照名で最新インデックス済みスナップショットを取得します（Git用）
func (r *SourceRepository) GetLatestIndexedSnapshotByRef(ctx context.Context, sourceID uuid.UUID, refName string) (*models.SourceSnapshot, error) {
	query := `
		SELECT ss.id, ss.source_id, ss.version_identifier, ss.indexed, ss.indexed_at, ss.created_at
		FROM source_snapshots ss
		INNER JOIN git_refs gr ON ss.id = gr.snapshot_id
		WHERE gr.source_id = $1 AND gr.ref_name = $2 AND ss.indexed = TRUE
		ORDER BY ss.indexed_at DESC
		LIMIT 1
	`

	var snapshot models.SourceSnapshot
	err := r.pool.QueryRow(ctx, query, sourceID, refName).Scan(
		&snapshot.ID,
		&snapshot.SourceID,
		&snapshot.VersionIdentifier,
		&snapshot.Indexed,
		&snapshot.IndexedAt,
		&snapshot.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // スナップショットが存在しない場合はnilを返す（エラーではない）
		}
		return nil, fmt.Errorf("failed to get latest indexed snapshot: %w", err)
	}

	return &snapshot, nil
}

// GetLatestIndexedSnapshot はソースの最新インデックス済みスナップショットを取得します（Git以外のソース用）
func (r *SourceRepository) GetLatestIndexedSnapshot(ctx context.Context, sourceID uuid.UUID) (*models.SourceSnapshot, error) {
	query := `
		SELECT id, source_id, version_identifier, indexed, indexed_at, created_at
		FROM source_snapshots
		WHERE source_id = $1 AND indexed = TRUE
		ORDER BY indexed_at DESC
		LIMIT 1
	`

	var snapshot models.SourceSnapshot
	err := r.pool.QueryRow(ctx, query, sourceID).Scan(
		&snapshot.ID,
		&snapshot.SourceID,
		&snapshot.VersionIdentifier,
		&snapshot.Indexed,
		&snapshot.IndexedAt,
		&snapshot.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // スナップショットが存在しない場合はnilを返す（エラーではない）
		}
		return nil, fmt.Errorf("failed to get latest indexed snapshot: %w", err)
	}

	return &snapshot, nil
}

// MarkSnapshotAsIndexed はスナップショットをインデックス完了としてマークします
func (r *SourceRepository) MarkSnapshotAsIndexed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE source_snapshots
		SET indexed = TRUE, indexed_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark snapshot as indexed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("snapshot not found: %s", id)
	}

	return nil
}

// === GitRef操作（ソース集約の一部） ===

// UpsertGitRef はGit参照をUpsertします（ブランチ/タグの更新）
func (r *SourceRepository) UpsertGitRef(ctx context.Context, sourceID uuid.UUID, refName string, snapshotID uuid.UUID) (*models.GitRef, error) {
	query := `
		INSERT INTO git_refs (source_id, ref_name, snapshot_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (source_id, ref_name)
		DO UPDATE SET
			snapshot_id = EXCLUDED.snapshot_id,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, source_id, ref_name, snapshot_id, created_at, updated_at
	`

	var gitRef models.GitRef
	err := r.pool.QueryRow(ctx, query, sourceID, refName, snapshotID).Scan(
		&gitRef.ID,
		&gitRef.SourceID,
		&gitRef.RefName,
		&gitRef.SnapshotID,
		&gitRef.CreatedAt,
		&gitRef.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert git ref: %w", err)
	}

	return &gitRef, nil
}

// GetGitRef はソースID + 参照名でGit参照を取得します
func (r *SourceRepository) GetGitRef(ctx context.Context, sourceID uuid.UUID, refName string) (*models.GitRef, error) {
	query := `
		SELECT id, source_id, ref_name, snapshot_id, created_at, updated_at
		FROM git_refs
		WHERE source_id = $1 AND ref_name = $2
	`

	var gitRef models.GitRef
	err := r.pool.QueryRow(ctx, query, sourceID, refName).Scan(
		&gitRef.ID,
		&gitRef.SourceID,
		&gitRef.RefName,
		&gitRef.SnapshotID,
		&gitRef.CreatedAt,
		&gitRef.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("git ref not found for source %s ref %s", sourceID, refName)
		}
		return nil, fmt.Errorf("failed to get git ref: %w", err)
	}

	return &gitRef, nil
}

// ListGitRefs はソースのGit参照一覧を取得します
func (r *SourceRepository) ListGitRefs(ctx context.Context, sourceID uuid.UUID) ([]*models.GitRef, error) {
	query := `
		SELECT id, source_id, ref_name, snapshot_id, created_at, updated_at
		FROM git_refs
		WHERE source_id = $1
		ORDER BY ref_name
	`

	rows, err := r.pool.Query(ctx, query, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list git refs: %w", err)
	}
	defer rows.Close()

	var gitRefs []*models.GitRef
	for rows.Next() {
		var gitRef models.GitRef
		if err := rows.Scan(
			&gitRef.ID,
			&gitRef.SourceID,
			&gitRef.RefName,
			&gitRef.SnapshotID,
			&gitRef.CreatedAt,
			&gitRef.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan git ref: %w", err)
		}
		gitRefs = append(gitRefs, &gitRef)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating git refs: %w", err)
	}

	return gitRefs, nil
}
