package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jinford/dev-rag/internal/module/wiki/adapter/pg/sqlc"
	"github.com/jinford/dev-rag/internal/module/wiki/domain"
	pgvector "github.com/pgvector/pgvector-go"
)

// === Wiki Metadata Repository ===

// wikiMetadataRepository は domain.WikiMetadataRepository の実装です
type wikiMetadataRepository struct {
	q sqlc.Querier
}

// NewWikiMetadataRepository は domain.WikiMetadataRepository を返します
func NewWikiMetadataRepository(q sqlc.Querier) domain.WikiMetadataRepository {
	return &wikiMetadataRepository{q: q}
}

// Upsert はWikiメタデータを登録・更新します（domain ポート実装）
func (r *wikiMetadataRepository) Upsert(ctx context.Context, productID uuid.UUID, outputPath string, fileCount int) (*domain.WikiMetadata, error) {
	sqlcMetadata, err := r.q.CreateWikiMetadata(ctx, sqlc.CreateWikiMetadataParams{
		ProductID:   UUIDToPgtype(productID),
		OutputPath:  outputPath,
		FileCount:   int32(fileCount),
		GeneratedAt: TimestampToPgtype(time.Now()),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert wiki metadata: %w", err)
	}

	metadata := &domain.WikiMetadata{
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
func (r *wikiMetadataRepository) GetByProductID(ctx context.Context, productID uuid.UUID) (*domain.WikiMetadata, error) {
	sqlcMetadata, err := r.q.GetWikiMetadataByProduct(ctx, UUIDToPgtype(productID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("wiki metadata not found for product: %s", productID)
		}
		return nil, fmt.Errorf("failed to get wiki metadata: %w", err)
	}

	metadata := &domain.WikiMetadata{
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
func (r *wikiMetadataRepository) Delete(ctx context.Context, productID uuid.UUID) error {
	// まず取得してIDを確認
	metadata, err := r.q.GetWikiMetadataByProduct(ctx, UUIDToPgtype(productID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("wiki metadata not found for product: %s", productID)
		}
		return fmt.Errorf("failed to get wiki metadata: %w", err)
	}

	// IDで削除
	err = r.q.DeleteWikiMetadata(ctx, metadata.ID)
	if err != nil {
		return fmt.Errorf("failed to delete wiki metadata: %w", err)
	}

	return nil
}

// === File Summary Repository ===

// fileSummaryRepository は domain.FileSummaryRepository の実装です
type fileSummaryRepository struct {
	q sqlc.Querier
}

// NewFileSummaryRepository は domain.FileSummaryRepository を返します
func NewFileSummaryRepository(q sqlc.Querier) domain.FileSummaryRepository {
	return &fileSummaryRepository{q: q}
}

// Upsert はファイルサマリーをUPSERTします（冪等性保証・domain ポート実装）
func (r *fileSummaryRepository) Upsert(
	ctx context.Context,
	fileID uuid.UUID,
	summary string,
	embedding []float32,
	metadataJSON []byte,
) (*domain.FileSummary, error) {
	// pgvectorベクトルに変換
	vec := pgvector.NewVector(embedding)

	fileSummary, err := r.q.UpsertFileSummary(ctx, sqlc.UpsertFileSummaryParams{
		FileID:    UUIDToPgtype(fileID),
		Summary:   summary,
		Embedding: vec,
		Metadata:  metadataJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert file summary: %w", err)
	}

	return convertSQLCFileSummary(fileSummary), nil
}

func convertSQLCFileSummary(row sqlc.FileSummary) *domain.FileSummary {
	// pgvectorからfloat32スライスに変換
	embedding := make([]float32, len(row.Embedding.Slice()))
	for i, v := range row.Embedding.Slice() {
		embedding[i] = v
	}

	return &domain.FileSummary{
		ID:        PgtypeToUUID(row.ID),
		FileID:    PgtypeToUUID(row.FileID),
		Summary:   row.Summary,
		Embedding: embedding,
		Metadata:  row.Metadata,
		CreatedAt: PgtypeToTime(row.CreatedAt),
		UpdatedAt: PgtypeToTime(row.UpdatedAt),
	}
}

// === Legacy Support (for backward compatibility) ===

// WikiRepositoryR はWikiメタデータの読み取り専用データベース操作を提供します（レガシー）
// 集約: WikiMetadata（プロダクトに関連するが独立したライフサイクル）
type WikiRepositoryR struct {
	q sqlc.Querier
}

// NewWikiRepositoryR は新しい読み取り専用リポジトリを作成します（レガシー）
func NewWikiRepositoryR(q sqlc.Querier) *WikiRepositoryR {
	return &WikiRepositoryR{q: q}
}

// WikiRepositoryRW は WikiRepositoryR を埋め込み、書き込み操作を提供します（レガシー）
type WikiRepositoryRW struct {
	*WikiRepositoryR
}

// NewWikiRepositoryRW は読み書き可能なリポジトリを作成します（レガシー）
func NewWikiRepositoryRW(q sqlc.Querier) *WikiRepositoryRW {
	return &WikiRepositoryRW{WikiRepositoryR: NewWikiRepositoryR(q)}
}

// UpsertFileSummary はファイルサマリーをUPSERTします（レガシー互換性のため）
func (rw *WikiRepositoryRW) UpsertFileSummary(
	ctx context.Context,
	fileID uuid.UUID,
	summary string,
	embedding []float32,
	metadataJSON []byte,
) (*domain.FileSummary, error) {
	// 新しい FileSummaryRepository に委譲
	repo := NewFileSummaryRepository(rw.q)
	return repo.Upsert(ctx, fileID, summary, embedding, metadataJSON)
}
