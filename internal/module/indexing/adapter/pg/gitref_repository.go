package pg

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	
)

// GitRefRepository はGit参照の永続化アダプターです
type GitRefRepository struct {
	q sqlc.Querier
}

// NewGitRefRepository は新しいGit参照リポジトリを作成します
func NewGitRefRepository(q sqlc.Querier) *GitRefRepository {
	return &GitRefRepository{q: q}
}

// 読み取り操作の実装

var _ domain.GitRefReader = (*GitRefRepository)(nil)

// GetByName は名前でGit参照を取得します
func (r *GitRefRepository) GetByName(ctx context.Context, sourceID uuid.UUID, refName string) (*domain.GitRef, error) {
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

	ref := &domain.GitRef{
		ID:         PgtypeToUUID(sqlcRef.ID),
		SourceID:   PgtypeToUUID(sqlcRef.SourceID),
		RefName:    sqlcRef.RefName,
		SnapshotID: PgtypeToUUID(sqlcRef.SnapshotID),
		CreatedAt:  PgtypeToTime(sqlcRef.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcRef.UpdatedAt),
	}

	return ref, nil
}

// ListBySource はソースのGit参照一覧を取得します
func (r *GitRefRepository) ListBySource(ctx context.Context, sourceID uuid.UUID) ([]*domain.GitRef, error) {
	sqlcRefs, err := r.q.ListGitRefsBySource(ctx, UUIDToPgtype(sourceID))
	if err != nil {
		return nil, fmt.Errorf("failed to list git refs: %w", err)
	}

	refs := make([]*domain.GitRef, 0, len(sqlcRefs))
	for _, sqlcRef := range sqlcRefs {
		ref := &domain.GitRef{
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

// 書き込み操作の実装

var _ domain.GitRefWriter = (*GitRefRepository)(nil)

// Upsert はGit参照を作成または更新します
func (r *GitRefRepository) Upsert(ctx context.Context, sourceID uuid.UUID, refName string, snapshotID uuid.UUID) (*domain.GitRef, error) {
	sqlcRef, err := r.q.CreateGitRef(ctx, sqlc.CreateGitRefParams{
		SourceID:   UUIDToPgtype(sourceID),
		RefName:    refName,
		SnapshotID: UUIDToPgtype(snapshotID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert git ref: %w", err)
	}

	ref := &domain.GitRef{
		ID:         PgtypeToUUID(sqlcRef.ID),
		SourceID:   PgtypeToUUID(sqlcRef.SourceID),
		RefName:    sqlcRef.RefName,
		SnapshotID: PgtypeToUUID(sqlcRef.SnapshotID),
		CreatedAt:  PgtypeToTime(sqlcRef.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcRef.UpdatedAt),
	}

	return ref, nil
}
