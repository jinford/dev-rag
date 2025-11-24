package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
)

// ProductRepositoryR はプロダクト集約の読み取り専用リポジトリです
// 集約: Product（ルートのみ）
type ProductRepositoryR struct {
	q sqlc.Querier
}

// NewProductRepositoryR は新しい読み取り専用リポジトリを作成します
func NewProductRepositoryR(q sqlc.Querier) *ProductRepositoryR {
	return &ProductRepositoryR{q: q}
}

// ProductRepositoryRW は ProductRepositoryR を埋め込み書き込み操作を提供します
type ProductRepositoryRW struct {
	*ProductRepositoryR
}

// NewProductRepositoryRW は読み書き可能なリポジトリを作成します
func NewProductRepositoryRW(q sqlc.Querier) *ProductRepositoryRW {
	return &ProductRepositoryRW{ProductRepositoryR: NewProductRepositoryR(q)}
}

// ProductWithStats はプロダクトと統計情報を含む構造体です
type ProductWithStats struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Description     *string    `json:"description,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	SourceCount     int        `json:"sourceCount"`
	LastIndexedAt   *time.Time `json:"lastIndexedAt,omitempty"`
	WikiGeneratedAt *time.Time `json:"wikiGeneratedAt,omitempty"`
}

// CreateIfNotExists は名前でプロダクトを検索し、存在しなければ作成します（冪等）
func (rw *ProductRepositoryRW) CreateIfNotExists(ctx context.Context, name string, description *string) (*sqlc.Product, error) {
	// まず既存のプロダクトを検索
	existing, err := rw.q.GetProductByName(ctx, name)
	if err == nil {
		return &existing, nil
	}
	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing product: %w", err)
	}

	// 存在しない場合は作成
	product, err := rw.q.CreateProduct(ctx, sqlc.CreateProductParams{
		Name:        name,
		Description: StringPtrToPgtext(description),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	return &product, nil
}

// Update はプロダクト情報を更新します
func (rw *ProductRepositoryRW) Update(ctx context.Context, id uuid.UUID, name string, description *string) (*sqlc.Product, error) {
	product, err := rw.q.UpdateProduct(ctx, sqlc.UpdateProductParams{
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

	return &product, nil
}

// GetByID はIDでプロダクトを取得します
func (r *ProductRepositoryR) GetByID(ctx context.Context, id uuid.UUID) (*sqlc.Product, error) {
	product, err := r.q.GetProduct(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return &product, nil
}

// GetByName は名前でプロダクトを取得します
func (r *ProductRepositoryR) GetByName(ctx context.Context, name string) (*sqlc.Product, error) {
	product, err := r.q.GetProductByName(ctx, name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return &product, nil
}

// List はすべてのプロダクトを取得します
func (r *ProductRepositoryR) List(ctx context.Context) ([]sqlc.Product, error) {
	products, err := r.q.ListProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	return products, nil
}

// GetListWithStats は統計情報付きのプロダクト一覧を取得します
func (r *ProductRepositoryR) GetListWithStats(ctx context.Context) ([]*ProductWithStats, error) {
	rows, err := r.q.ListProductsWithStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list products with stats: %w", err)
	}

	products := make([]*ProductWithStats, 0, len(rows))
	for _, row := range rows {
		product := &ProductWithStats{
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

// Delete はプロダクトを削除します（内部API専用）
func (rw *ProductRepositoryRW) Delete(ctx context.Context, id uuid.UUID) error {
	err := rw.q.DeleteProduct(ctx, UUIDToPgtype(id))
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}

	return nil
}
