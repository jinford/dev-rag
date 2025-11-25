package testing

import (
	"context"

	"github.com/jinford/dev-rag/internal/module/indexing/application"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// MockIndexer はテスト用のモックIndexerです
type MockIndexer struct {
	IndexSourceFunc func(ctx context.Context, sourceType domain.SourceType, params domain.IndexParams) (*application.IndexResult, error)
}

// IndexSource はIndexSourceのモック実装です
func (m *MockIndexer) IndexSource(ctx context.Context, sourceType domain.SourceType, params domain.IndexParams) (*application.IndexResult, error) {
	if m.IndexSourceFunc != nil {
		return m.IndexSourceFunc(ctx, sourceType, params)
	}
	return nil, nil
}
