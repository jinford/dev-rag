package application_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/application"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	testutil "github.com/jinford/dev-rag/internal/module/indexing/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductService_GetProduct_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	productID := uuid.New()
	desc := "Test product description"
	expectedProduct := testutil.TestProduct("test-product", &desc)
	expectedProduct.ID = productID

	mockRepo := &testutil.MockProductReader{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
			assert.Equal(t, productID, id)
			return expectedProduct, nil
		},
	}

	service := application.NewProductService(mockRepo, log)

	// Execute
	result, err := service.GetProduct(ctx, productID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedProduct, result)
}

func TestProductService_GetProduct_NilID(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockRepo := &testutil.MockProductReader{}
	service := application.NewProductService(mockRepo, log)

	// Execute
	result, err := service.GetProduct(ctx, uuid.Nil)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "product ID is required")
}

func TestProductService_GetProduct_RepositoryError(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	productID := uuid.New()
	expectedErr := errors.New("database error")

	mockRepo := &testutil.MockProductReader{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
			return nil, expectedErr
		},
	}

	service := application.NewProductService(mockRepo, log)

	// Execute
	result, err := service.GetProduct(ctx, productID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get product")
	assert.ErrorIs(t, err, expectedErr)
}

func TestProductService_GetProductByName_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	productName := "test-product"
	expectedProduct := testutil.TestProduct(productName, nil)

	mockRepo := &testutil.MockProductReader{
		GetByNameFunc: func(ctx context.Context, name string) (*domain.Product, error) {
			assert.Equal(t, productName, name)
			return expectedProduct, nil
		},
	}

	service := application.NewProductService(mockRepo, log)

	// Execute
	result, err := service.GetProductByName(ctx, productName)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedProduct, result)
}

func TestProductService_GetProductByName_EmptyName(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockRepo := &testutil.MockProductReader{}
	service := application.NewProductService(mockRepo, log)

	// Execute
	result, err := service.GetProductByName(ctx, "")

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "product name is required")
}

func TestProductService_ListProducts_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	desc1 := "Product 1"
	desc2 := "Product 2"
	expectedProducts := []*domain.Product{
		testutil.TestProduct("product1", &desc1),
		testutil.TestProduct("product2", &desc2),
		testutil.TestProduct("product3", nil),
	}

	mockRepo := &testutil.MockProductReader{
		ListFunc: func(ctx context.Context) ([]*domain.Product, error) {
			return expectedProducts, nil
		},
	}

	service := application.NewProductService(mockRepo, log)

	// Execute
	result, err := service.ListProducts(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedProducts, result)
	assert.Len(t, result, 3)
}

func TestProductService_ListProducts_EmptyList(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockRepo := &testutil.MockProductReader{
		ListFunc: func(ctx context.Context) ([]*domain.Product, error) {
			return []*domain.Product{}, nil
		},
	}

	service := application.NewProductService(mockRepo, log)

	// Execute
	result, err := service.ListProducts(ctx)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

func TestProductService_ListProducts_RepositoryError(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	expectedErr := errors.New("database error")

	mockRepo := &testutil.MockProductReader{
		ListFunc: func(ctx context.Context) ([]*domain.Product, error) {
			return nil, expectedErr
		},
	}

	service := application.NewProductService(mockRepo, log)

	// Execute
	result, err := service.ListProducts(ctx)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list products")
	assert.ErrorIs(t, err, expectedErr)
}

func TestProductService_ListProductsWithStats_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	product1 := testutil.TestProduct("product1", nil)
	product2 := testutil.TestProduct("product2", nil)

	expectedProducts := []*domain.ProductWithStats{
		{
			ID:              product1.ID,
			Name:            product1.Name,
			Description:     product1.Description,
			CreatedAt:       product1.CreatedAt,
			UpdatedAt:       product1.UpdatedAt,
			SourceCount:     2,
			LastIndexedAt:   nil,
			WikiGeneratedAt: nil,
		},
		{
			ID:              product2.ID,
			Name:            product2.Name,
			Description:     product2.Description,
			CreatedAt:       product2.CreatedAt,
			UpdatedAt:       product2.UpdatedAt,
			SourceCount:     1,
			LastIndexedAt:   nil,
			WikiGeneratedAt: nil,
		},
	}

	mockRepo := &testutil.MockProductReader{
		GetListWithStatsFunc: func(ctx context.Context) ([]*domain.ProductWithStats, error) {
			return expectedProducts, nil
		},
	}

	service := application.NewProductService(mockRepo, log)

	// Execute
	result, err := service.ListProductsWithStats(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedProducts, result)
	assert.Len(t, result, 2)
}

func TestProductService_ListProductsWithStats_RepositoryError(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	expectedErr := errors.New("database error")

	mockRepo := &testutil.MockProductReader{
		GetListWithStatsFunc: func(ctx context.Context) ([]*domain.ProductWithStats, error) {
			return nil, expectedErr
		},
	}

	service := application.NewProductService(mockRepo, log)

	// Execute
	result, err := service.ListProductsWithStats(ctx)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list products with stats")
	assert.ErrorIs(t, err, expectedErr)
}
