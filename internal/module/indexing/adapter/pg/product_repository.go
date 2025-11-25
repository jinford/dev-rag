package pg

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// ProductRepository はプロダクト集約の永続化アダプターです
type ProductRepository struct {
	q sqlc.Querier
}

// NewProductRepository は新しいプロダクトリポジトリを作成します
func NewProductRepository(q sqlc.Querier) *ProductRepository {
	return &ProductRepository{q: q}
}

// 読み取り操作の実装

var _ domain.ProductReader = (*ProductRepository)(nil)

// GetByID はIDでプロダクトを取得します
func (r *ProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	product, err := r.q.GetProduct(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return convertSQLCProduct(product), nil
}

// GetByName は名前でプロダクトを取得します
func (r *ProductRepository) GetByName(ctx context.Context, name string) (*domain.Product, error) {
	product, err := r.q.GetProductByName(ctx, name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return convertSQLCProduct(product), nil
}

// List はすべてのプロダクトを取得します
func (r *ProductRepository) List(ctx context.Context) ([]*domain.Product, error) {
	products, err := r.q.ListProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	result := make([]*domain.Product, 0, len(products))
	for _, p := range products {
		result = append(result, convertSQLCProduct(p))
	}

	return result, nil
}

// GetListWithStats は統計情報付きのプロダクト一覧を取得します
func (r *ProductRepository) GetListWithStats(ctx context.Context) ([]*domain.ProductWithStats, error) {
	rows, err := r.q.ListProductsWithStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list products with stats: %w", err)
	}

	products := make([]*domain.ProductWithStats, 0, len(rows))
	for _, row := range rows {
		product := &domain.ProductWithStats{
			ID:          PgtypeToUUID(row.ID),
			Name:        row.Name,
			Description: PgtextToStringPtr(row.Description),
			CreatedAt:   PgtypeToTime(row.CreatedAt),
			UpdatedAt:   PgtypeToTime(row.UpdatedAt),
			SourceCount: int(row.SourceCount),
		}

		// NULL可能なフィールドを処理
		if lastIndexed, ok := row.LastIndexedAt.(pgtype.Timestamp); ok {
			product.LastIndexedAt = PgtypeToTimePtr(lastIndexed)
		}
		if wikiGenerated, ok := row.WikiGeneratedAt.(pgtype.Timestamp); ok {
			product.WikiGeneratedAt = PgtypeToTimePtr(wikiGenerated)
		}

		products = append(products, product)
	}

	return products, nil
}

// 書き込み操作の実装

var _ domain.ProductWriter = (*ProductRepository)(nil)

// CreateIfNotExists は名前でプロダクトを検索し、存在しなければ作成します（冪等）
func (r *ProductRepository) CreateIfNotExists(ctx context.Context, name string, description *string) (*domain.Product, error) {
	// まず既存のプロダクトを検索
	existing, err := r.q.GetProductByName(ctx, name)
	if err == nil {
		return convertSQLCProduct(existing), nil
	}
	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing product: %w", err)
	}

	// 存在しない場合は作成
	product, err := r.q.CreateProduct(ctx, sqlc.CreateProductParams{
		Name:        name,
		Description: StringPtrToPgtext(description),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	return convertSQLCProduct(product), nil
}

// Update はプロダクト情報を更新します
func (r *ProductRepository) Update(ctx context.Context, id uuid.UUID, name string, description *string) (*domain.Product, error) {
	product, err := r.q.UpdateProduct(ctx, sqlc.UpdateProductParams{
		ID:          UUIDToPgtype(id),
		Name:        name,
		Description: StringPtrToPgtext(description),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found: %s", id)
		}
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	return convertSQLCProduct(product), nil
}

// Delete はプロダクトを削除します（内部API専用）
func (r *ProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.q.DeleteProduct(ctx, UUIDToPgtype(id))
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}

	return nil
}

// === Private helpers ===

func convertSQLCProduct(row sqlc.Product) *domain.Product {
	return &domain.Product{
		ID:          PgtypeToUUID(row.ID),
		Name:        row.Name,
		Description: PgtextToStringPtr(row.Description),
		CreatedAt:   PgtypeToTime(row.CreatedAt),
		UpdatedAt:   PgtypeToTime(row.UpdatedAt),
	}
}
