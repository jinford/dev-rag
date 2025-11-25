package pg

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	
)

// DependencyRepository は依存関係の永続化アダプターです
type DependencyRepository struct {
	q sqlc.Querier
}

// NewDependencyRepository は新しい依存関係リポジトリを作成します
func NewDependencyRepository(q sqlc.Querier) *DependencyRepository {
	return &DependencyRepository{q: q}
}

// 読み取り操作の実装

var _ domain.DependencyReader = (*DependencyRepository)(nil)

// GetByChunk は指定されたチャンクの依存関係を取得します
func (r *DependencyRepository) GetByChunk(ctx context.Context, chunkID uuid.UUID) ([]*domain.ChunkDependency, error) {
	rows, err := r.q.GetDependenciesByChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies: %w", err)
	}

	deps := make([]*domain.ChunkDependency, 0, len(rows))
	for _, row := range rows {
		deps = append(deps, convertSQLCDependency(row))
	}

	return deps, nil
}

// GetByChunkAndType は指定されたチャンクと依存タイプで依存関係を取得します
func (r *DependencyRepository) GetByChunkAndType(ctx context.Context, chunkID uuid.UUID, depType string) ([]*domain.ChunkDependency, error) {
	rows, err := r.q.GetDependenciesByChunkAndType(ctx, sqlc.GetDependenciesByChunkAndTypeParams{
		FromChunkID: UUIDToPgtype(chunkID),
		DepType:     depType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies by type: %w", err)
	}

	deps := make([]*domain.ChunkDependency, 0, len(rows))
	for _, row := range rows {
		deps = append(deps, convertSQLCDependency(row))
	}

	return deps, nil
}

// GetIncomingByChunk は指定されたチャンクへの被依存関係を取得します
func (r *DependencyRepository) GetIncomingByChunk(ctx context.Context, chunkID uuid.UUID) ([]*domain.ChunkDependency, error) {
	rows, err := r.q.GetIncomingDependenciesByChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		return nil, fmt.Errorf("failed to get incoming dependencies: %w", err)
	}

	deps := make([]*domain.ChunkDependency, 0, len(rows))
	for _, row := range rows {
		deps = append(deps, convertSQLCDependency(row))
	}

	return deps, nil
}

// 書き込み操作の実装

var _ domain.DependencyWriter = (*DependencyRepository)(nil)

// Create は依存関係を作成します
func (r *DependencyRepository) Create(ctx context.Context, fromChunkID, toChunkID uuid.UUID, depType, symbol string) error {
	return r.q.CreateDependency(ctx, sqlc.CreateDependencyParams{
		FromChunkID: UUIDToPgtype(fromChunkID),
		ToChunkID:   UUIDToPgtype(toChunkID),
		DepType:     depType,
		Symbol:      StringToNullableText(symbol),
	})
}

// DeleteByChunk は指定されたチャンクに関連する全ての依存関係を削除します
func (r *DependencyRepository) DeleteByChunk(ctx context.Context, chunkID uuid.UUID) error {
	return r.q.DeleteDependenciesByChunk(ctx, UUIDToPgtype(chunkID))
}

// === Private helpers ===

func convertSQLCDependency(row sqlc.ChunkDependency) *domain.ChunkDependency {
	return &domain.ChunkDependency{
		ID:          PgtypeToUUID(row.ID),
		FromChunkID: PgtypeToUUID(row.FromChunkID),
		ToChunkID:   PgtypeToUUID(row.ToChunkID),
		DepType:     row.DepType,
		Symbol:      PgtextToStringPtr(row.Symbol),
		CreatedAt:   PgtypeToTime(row.CreatedAt),
	}
}
