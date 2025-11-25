package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// ProductService は製品管理のユースケースを提供します
type ProductService struct {
	productRepo domain.ProductReader
	log         *slog.Logger
}

// NewProductService は新しいProductServiceを作成します
func NewProductService(productRepo domain.ProductReader, log *slog.Logger) *ProductService {
	return &ProductService{
		productRepo: productRepo,
		log:         log,
	}
}

// GetProduct は製品を取得します
func (s *ProductService) GetProduct(ctx context.Context, productID uuid.UUID) (*domain.Product, error) {
	if productID == uuid.Nil {
		return nil, fmt.Errorf("product ID is required")
	}

	product, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		s.log.Error("Failed to get product",
			"productID", productID,
			"error", err,
		)
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return product, nil
}

// GetProductByName は名前で製品を取得します
func (s *ProductService) GetProductByName(ctx context.Context, name string) (*domain.Product, error) {
	if name == "" {
		return nil, fmt.Errorf("product name is required")
	}

	product, err := s.productRepo.GetByName(ctx, name)
	if err != nil {
		s.log.Error("Failed to get product by name",
			"name", name,
			"error", err,
		)
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return product, nil
}

// ListProducts は製品一覧を取得します
func (s *ProductService) ListProducts(ctx context.Context) ([]*domain.Product, error) {
	products, err := s.productRepo.List(ctx)
	if err != nil {
		s.log.Error("Failed to list products", "error", err)
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	s.log.Info("Products listed successfully", "count", len(products))

	return products, nil
}

// ListProductsWithStats は製品一覧を統計情報と共に取得します
func (s *ProductService) ListProductsWithStats(ctx context.Context) ([]*domain.ProductWithStats, error) {
	products, err := s.productRepo.GetListWithStats(ctx)
	if err != nil {
		s.log.Error("Failed to list products with stats", "error", err)
		return nil, fmt.Errorf("failed to list products with stats: %w", err)
	}

	s.log.Info("Products with stats listed successfully", "count", len(products))

	return products, nil
}
