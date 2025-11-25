package coverage

import (
	"context"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/application"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	indexingpg "github.com/jinford/dev-rag/internal/module/indexing/adapter/pg"
)

// CoverageBuilder はカバレッジマップの構築を提供します
// 内部的にapplication.CoverageServiceに委譲します
type CoverageBuilder struct {
	coverageService *application.CoverageService
}

// NewCoverageBuilder は新しいCoverageBuilderを作成します
func NewCoverageBuilder(indexRepo *indexingpg.IndexRepositoryR) *CoverageBuilder {
	return &CoverageBuilder{
		coverageService: application.NewCoverageService(indexRepo, nil),
	}
}

// BuildCoverageMap はスナップショットIDからカバレッジマップを構築します
// application.CoverageServiceに委譲します
func (cb *CoverageBuilder) BuildCoverageMap(ctx context.Context, snapshotID uuid.UUID, snapshotVersion string) (*domain.CoverageMap, error) {
	return cb.coverageService.BuildCoverageMap(ctx, snapshotID, snapshotVersion)
}

// ExportToJSON はカバレッジマップをJSON形式でエクスポートします
// application.CoverageServiceに委譲します
func (cb *CoverageBuilder) ExportToJSON(coverageMap *domain.CoverageMap) ([]byte, error) {
	return cb.coverageService.ExportToJSON(coverageMap)
}
