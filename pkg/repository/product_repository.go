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

// ProductRepository はプロダクト集約のデータベース操作を提供します
// 集約: Product（ルートのみ）
type ProductRepository struct {
	pool *pgxpool.Pool
}

// NewProductRepository は新しいProductRepositoryを作成します
func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

// ProductWithStats はプロダクトと統計情報を含む構造体です
type ProductWithStats struct {
	models.Product
	SourceCount     int        `json:"sourceCount"`
	LastIndexedAt   *time.Time `json:"lastIndexedAt,omitempty"`
	WikiGeneratedAt *time.Time `json:"wikiGeneratedAt,omitempty"`
}

// CreateIfNotExists は名前でプロダクトを検索し、存在しなければ作成します（冪等）
func (r *ProductRepository) CreateIfNotExists(ctx context.Context, name string, description *string) (*models.Product, error) {
	// まず既存のプロダクトを検索
	existing, err := r.GetByName(ctx, name)
	if err == nil {
		return existing, nil
	}

	// 存在しない場合は作成
	query := `
		INSERT INTO products (name, description)
		VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET
			description = COALESCE(EXCLUDED.description, products.description),
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, name, description, created_at, updated_at
	`

	var product models.Product
	err = r.pool.QueryRow(ctx, query, name, description).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.CreatedAt,
		&product.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	return &product, nil
}

// Update はプロダクト情報を更新します
func (r *ProductRepository) Update(ctx context.Context, id uuid.UUID, name string, description *string) (*models.Product, error) {
	query := `
		UPDATE products
		SET name = $2, description = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
		RETURNING id, name, description, created_at, updated_at
	`

	var product models.Product
	err := r.pool.QueryRow(ctx, query, id, name, description).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.CreatedAt,
		&product.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found: %s", id)
		}
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	return &product, nil
}

// GetByID はIDでプロダクトを取得します
func (r *ProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM products
		WHERE id = $1
	`

	var product models.Product
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.CreatedAt,
		&product.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return &product, nil
}

// GetByName は名前でプロダクトを取得します
func (r *ProductRepository) GetByName(ctx context.Context, name string) (*models.Product, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM products
		WHERE name = $1
	`

	var product models.Product
	err := r.pool.QueryRow(ctx, query, name).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.CreatedAt,
		&product.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return &product, nil
}

// List はすべてのプロダクトを取得します
func (r *ProductRepository) List(ctx context.Context) ([]*models.Product, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM products
		ORDER BY name
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}
	defer rows.Close()

	var products []*models.Product
	for rows.Next() {
		var product models.Product
		if err := rows.Scan(
			&product.ID,
			&product.Name,
			&product.Description,
			&product.CreatedAt,
			&product.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, &product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating products: %w", err)
	}

	return products, nil
}

// GetListWithStats は統計情報付きのプロダクト一覧を取得します
func (r *ProductRepository) GetListWithStats(ctx context.Context) ([]*ProductWithStats, error) {
	query := `
		SELECT
			p.id,
			p.name,
			p.description,
			p.created_at,
			p.updated_at,
			COUNT(DISTINCT s.id) AS source_count,
			MAX(ss.indexed_at) AS last_indexed_at,
			MAX(wm.generated_at) AS wiki_generated_at
		FROM products p
		LEFT JOIN sources s ON p.id = s.product_id
		LEFT JOIN source_snapshots ss ON s.id = ss.source_id AND ss.indexed = TRUE
		LEFT JOIN wiki_metadata wm ON p.id = wm.product_id
		GROUP BY p.id, p.name, p.description, p.created_at, p.updated_at
		ORDER BY p.name
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list products with stats: %w", err)
	}
	defer rows.Close()

	var products []*ProductWithStats
	for rows.Next() {
		var product ProductWithStats
		if err := rows.Scan(
			&product.ID,
			&product.Name,
			&product.Description,
			&product.CreatedAt,
			&product.UpdatedAt,
			&product.SourceCount,
			&product.LastIndexedAt,
			&product.WikiGeneratedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan product with stats: %w", err)
		}
		products = append(products, &product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating products with stats: %w", err)
	}

	return products, nil
}

// Delete はプロダクトを削除します（内部API専用）
func (r *ProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM products WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("product not found: %s", id)
	}

	return nil
}
