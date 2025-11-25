package domain

import (
	"context"

	"github.com/google/uuid"
)

// CoverageReporter はカバレッジマップの構築とエクスポートを行うインターフェース
type CoverageReporter interface {
	// Build は指定されたスナップショットのカバレッジマップを構築します
	Build(ctx context.Context, snapshotID uuid.UUID, version string) (*CoverageMap, error)

	// ExportJSON はカバレッジマップをJSON形式でエクスポートします
	ExportJSON(coverageMap *CoverageMap) ([]byte, error)
}
