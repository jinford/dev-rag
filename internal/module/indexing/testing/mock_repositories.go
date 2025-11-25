package testing

import (
	"context"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// MockSourceReader はテスト用のモックSourceReaderです
type MockSourceReader struct {
	GetByIDFunc                 func(ctx context.Context, id uuid.UUID) (*domain.Source, error)
	GetByNameFunc               func(ctx context.Context, name string) (*domain.Source, error)
	ListByProductIDFunc         func(ctx context.Context, productID uuid.UUID) ([]*domain.Source, error)
	GetSnapshotByVersionFunc    func(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*domain.SourceSnapshot, error)
	GetLatestIndexedSnapshotFunc func(ctx context.Context, sourceID uuid.UUID) (*domain.SourceSnapshot, error)
	ListSnapshotsBySourceFunc   func(ctx context.Context, sourceID uuid.UUID) ([]*domain.SourceSnapshot, error)
}

func (m *MockSourceReader) GetByID(ctx context.Context, id uuid.UUID) (*domain.Source, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockSourceReader) GetByName(ctx context.Context, name string) (*domain.Source, error) {
	if m.GetByNameFunc != nil {
		return m.GetByNameFunc(ctx, name)
	}
	return nil, nil
}

func (m *MockSourceReader) ListByProductID(ctx context.Context, productID uuid.UUID) ([]*domain.Source, error) {
	if m.ListByProductIDFunc != nil {
		return m.ListByProductIDFunc(ctx, productID)
	}
	return nil, nil
}

func (m *MockSourceReader) GetSnapshotByVersion(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*domain.SourceSnapshot, error) {
	if m.GetSnapshotByVersionFunc != nil {
		return m.GetSnapshotByVersionFunc(ctx, sourceID, versionIdentifier)
	}
	return nil, nil
}

func (m *MockSourceReader) GetLatestIndexedSnapshot(ctx context.Context, sourceID uuid.UUID) (*domain.SourceSnapshot, error) {
	if m.GetLatestIndexedSnapshotFunc != nil {
		return m.GetLatestIndexedSnapshotFunc(ctx, sourceID)
	}
	return nil, nil
}

func (m *MockSourceReader) ListSnapshotsBySource(ctx context.Context, sourceID uuid.UUID) ([]*domain.SourceSnapshot, error) {
	if m.ListSnapshotsBySourceFunc != nil {
		return m.ListSnapshotsBySourceFunc(ctx, sourceID)
	}
	return nil, nil
}

// MockProductReader はテスト用のモックProductReaderです
type MockProductReader struct {
	GetByIDFunc          func(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	GetByNameFunc        func(ctx context.Context, name string) (*domain.Product, error)
	ListFunc             func(ctx context.Context) ([]*domain.Product, error)
	GetListWithStatsFunc func(ctx context.Context) ([]*domain.ProductWithStats, error)
}

func (m *MockProductReader) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockProductReader) GetByName(ctx context.Context, name string) (*domain.Product, error) {
	if m.GetByNameFunc != nil {
		return m.GetByNameFunc(ctx, name)
	}
	return nil, nil
}

func (m *MockProductReader) List(ctx context.Context) ([]*domain.Product, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

func (m *MockProductReader) GetListWithStats(ctx context.Context) ([]*domain.ProductWithStats, error) {
	if m.GetListWithStatsFunc != nil {
		return m.GetListWithStatsFunc(ctx)
	}
	return nil, nil
}
