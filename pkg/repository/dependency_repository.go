package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/sqlc"
)

// === Dependency操作 ===

// CreateDependency は依存関係を作成します
func (rw *IndexRepositoryRW) CreateDependency(ctx context.Context, fromChunkID, toChunkID uuid.UUID, depType, symbol string) error {
	return rw.q.CreateDependency(ctx, sqlc.CreateDependencyParams{
		FromChunkID: UUIDToPgtype(fromChunkID),
		ToChunkID:   UUIDToPgtype(toChunkID),
		DepType:     depType,
		Symbol:      StringToNullableText(symbol),
	})
}

// GetDependenciesByChunk は指定されたチャンクの依存関係を取得します
func (r *IndexRepositoryR) GetDependenciesByChunk(ctx context.Context, chunkID uuid.UUID) ([]sqlc.ChunkDependency, error) {
	rows, err := r.q.GetDependenciesByChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies: %w", err)
	}

	return rows, nil
}

// GetDependenciesByChunkAndType は指定されたチャンクと依存タイプで依存関係を取得します
func (r *IndexRepositoryR) GetDependenciesByChunkAndType(ctx context.Context, chunkID uuid.UUID, depType string) ([]sqlc.ChunkDependency, error) {
	rows, err := r.q.GetDependenciesByChunkAndType(ctx, sqlc.GetDependenciesByChunkAndTypeParams{
		FromChunkID: UUIDToPgtype(chunkID),
		DepType:     depType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies by type: %w", err)
	}

	return rows, nil
}

// GetIncomingDependenciesByChunk は指定されたチャンクへの被依存関係を取得します
func (r *IndexRepositoryR) GetIncomingDependenciesByChunk(ctx context.Context, chunkID uuid.UUID) ([]sqlc.ChunkDependency, error) {
	rows, err := r.q.GetIncomingDependenciesByChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		return nil, fmt.Errorf("failed to get incoming dependencies: %w", err)
	}

	return rows, nil
}

// DeleteDependenciesByChunk は指定されたチャンクに関連する全ての依存関係を削除します
func (rw *IndexRepositoryRW) DeleteDependenciesByChunk(ctx context.Context, chunkID uuid.UUID) error {
	return rw.q.DeleteDependenciesByChunk(ctx, UUIDToPgtype(chunkID))
}
