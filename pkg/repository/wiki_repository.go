package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jinford/dev-rag/pkg/models"
)

// WikiRepository はWikiメタデータのデータベース操作を提供します
// 集約: WikiMetadata（プロダクトに関連するが独立したライフサイクル）
type WikiRepository struct {
	pool *pgxpool.Pool
}

// NewWikiRepository は新しいWikiRepositoryを作成します
func NewWikiRepository(pool *pgxpool.Pool) *WikiRepository {
	return &WikiRepository{pool: pool}
}

// UpsertMetadata はWikiメタデータを登録・更新します
func (r *WikiRepository) UpsertMetadata(ctx context.Context, productID uuid.UUID, outputPath string, fileCount int) (*models.WikiMetadata, error) {
	query := `
		INSERT INTO wiki_metadata (product_id, output_path, file_count, generated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (product_id)
		DO UPDATE SET
			output_path = EXCLUDED.output_path,
			file_count = EXCLUDED.file_count,
			generated_at = CURRENT_TIMESTAMP
		RETURNING id, product_id, output_path, file_count, generated_at, created_at
	`

	var metadata models.WikiMetadata
	err := r.pool.QueryRow(ctx, query, productID, outputPath, fileCount).Scan(
		&metadata.ID,
		&metadata.ProductID,
		&metadata.OutputPath,
		&metadata.FileCount,
		&metadata.GeneratedAt,
		&metadata.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert wiki metadata: %w", err)
	}

	return &metadata, nil
}

// GetByProductID はプロダクトIDでWikiメタデータを取得します
func (r *WikiRepository) GetByProductID(ctx context.Context, productID uuid.UUID) (*models.WikiMetadata, error) {
	query := `
		SELECT id, product_id, output_path, file_count, generated_at, created_at
		FROM wiki_metadata
		WHERE product_id = $1
	`

	var metadata models.WikiMetadata
	err := r.pool.QueryRow(ctx, query, productID).Scan(
		&metadata.ID,
		&metadata.ProductID,
		&metadata.OutputPath,
		&metadata.FileCount,
		&metadata.GeneratedAt,
		&metadata.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("wiki metadata not found for product: %s", productID)
		}
		return nil, fmt.Errorf("failed to get wiki metadata: %w", err)
	}

	return &metadata, nil
}

// Delete はWikiメタデータを削除します
func (r *WikiRepository) Delete(ctx context.Context, productID uuid.UUID) error {
	query := `DELETE FROM wiki_metadata WHERE product_id = $1`

	result, err := r.pool.Exec(ctx, query, productID)
	if err != nil {
		return fmt.Errorf("failed to delete wiki metadata: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("wiki metadata not found for product: %s", productID)
	}

	return nil
}
