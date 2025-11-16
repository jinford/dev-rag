package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/sqlc"
)

// WikiRepositoryR はWikiメタデータの読み取り専用データベース操作を提供します
// 集約: WikiMetadata（プロダクトに関連するが独立したライフサイクル）
type WikiRepositoryR struct {
	q sqlc.Querier
}

// NewWikiRepositoryR は新しい読み取り専用リポジトリを作成します
func NewWikiRepositoryR(q sqlc.Querier) *WikiRepositoryR {
	return &WikiRepositoryR{q: q}
}

// WikiRepositoryRW は WikiRepositoryR を埋め込み、書き込み操作を提供します
type WikiRepositoryRW struct {
	*WikiRepositoryR
}

// NewWikiRepositoryRW は読み書き可能なリポジトリを作成します
func NewWikiRepositoryRW(q sqlc.Querier) *WikiRepositoryRW {
	return &WikiRepositoryRW{WikiRepositoryR: NewWikiRepositoryR(q)}
}

// UpsertMetadata はWikiメタデータを登録・更新します
func (rw *WikiRepositoryRW) UpsertMetadata(ctx context.Context, productID uuid.UUID, outputPath string, fileCount int) (*models.WikiMetadata, error) {
	sqlcMetadata, err := rw.q.CreateWikiMetadata(ctx, sqlc.CreateWikiMetadataParams{
		ProductID:   UUIDToPgtype(productID),
		OutputPath:  outputPath,
		FileCount:   int32(fileCount),
		GeneratedAt: TimestampToPgtype(time.Now()),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert wiki metadata: %w", err)
	}

	metadata := &models.WikiMetadata{
		ID:          PgtypeToUUID(sqlcMetadata.ID),
		ProductID:   PgtypeToUUID(sqlcMetadata.ProductID),
		OutputPath:  sqlcMetadata.OutputPath,
		FileCount:   int(sqlcMetadata.FileCount),
		GeneratedAt: PgtypeToTime(sqlcMetadata.GeneratedAt),
		CreatedAt:   PgtypeToTime(sqlcMetadata.CreatedAt),
	}

	return metadata, nil
}

// GetByProductID はプロダクトIDでWikiメタデータを取得します
func (r *WikiRepositoryR) GetByProductID(ctx context.Context, productID uuid.UUID) (*models.WikiMetadata, error) {
	sqlcMetadata, err := r.q.GetWikiMetadataByProduct(ctx, UUIDToPgtype(productID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("wiki metadata not found for product: %s", productID)
		}
		return nil, fmt.Errorf("failed to get wiki metadata: %w", err)
	}

	metadata := &models.WikiMetadata{
		ID:          PgtypeToUUID(sqlcMetadata.ID),
		ProductID:   PgtypeToUUID(sqlcMetadata.ProductID),
		OutputPath:  sqlcMetadata.OutputPath,
		FileCount:   int(sqlcMetadata.FileCount),
		GeneratedAt: PgtypeToTime(sqlcMetadata.GeneratedAt),
		CreatedAt:   PgtypeToTime(sqlcMetadata.CreatedAt),
	}

	return metadata, nil
}

// Delete はWikiメタデータを削除します（プロダクトIDで削除）
func (rw *WikiRepositoryRW) Delete(ctx context.Context, productID uuid.UUID) error {
	// まず取得してIDを確認
	metadata, err := rw.q.GetWikiMetadataByProduct(ctx, UUIDToPgtype(productID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("wiki metadata not found for product: %s", productID)
		}
		return fmt.Errorf("failed to get wiki metadata: %w", err)
	}

	// IDで削除
	err = rw.q.DeleteWikiMetadata(ctx, metadata.ID)
	if err != nil {
		return fmt.Errorf("failed to delete wiki metadata: %w", err)
	}

	return nil
}
