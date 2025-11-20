package coverage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/repository"
)

// CoverageBuilder はカバレッジマップを構築します
type CoverageBuilder struct {
	indexRepo *repository.IndexRepositoryR
}

// NewCoverageBuilder は新しいCoverageBuilderを作成します
func NewCoverageBuilder(indexRepo *repository.IndexRepositoryR) *CoverageBuilder {
	return &CoverageBuilder{
		indexRepo: indexRepo,
	}
}

// BuildCoverageMap はスナップショットIDからカバレッジマップを構築します
func (cb *CoverageBuilder) BuildCoverageMap(ctx context.Context, snapshotID uuid.UUID, snapshotVersion string) (*models.CoverageMap, error) {
	// ドメイン別統計を取得
	domainStats, err := cb.indexRepo.GetDomainCoverageStats(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain coverage stats: %w", err)
	}

	// 未インデックス重要ファイルを各ドメインに追加
	for i := range domainStats {
		unindexedFiles, err := cb.indexRepo.GetUnindexedImportantFiles(ctx, snapshotID)
		if err != nil {
			return nil, fmt.Errorf("failed to get unindexed important files: %w", err)
		}

		// ドメイン別にフィルタリング（簡易実装: 全ドメインに追加）
		// 実際の実装では、各ファイルのドメインを取得してフィルタリングする
		if len(unindexedFiles) > 0 {
			domainStats[i].UnindexedImportantFiles = unindexedFiles
		}
	}

	// 全体統計を計算
	totalFiles := 0
	totalIndexedFiles := 0
	totalChunks := 0
	for _, ds := range domainStats {
		totalFiles += ds.TotalFiles
		totalIndexedFiles += ds.IndexedFiles
		totalChunks += ds.IndexedChunks
	}

	overallCoverage := 0.0
	if totalFiles > 0 {
		overallCoverage = float64(totalIndexedFiles) / float64(totalFiles) * 100
	}

	return &models.CoverageMap{
		SnapshotID:        snapshotID.String(),
		SnapshotVersion:   snapshotVersion,
		TotalFiles:        totalFiles,
		TotalIndexedFiles: totalIndexedFiles,
		TotalChunks:       totalChunks,
		OverallCoverage:   overallCoverage,
		DomainCoverages:   domainStats,
		GeneratedAt:       time.Now(),
	}, nil
}

// ExportToJSON はカバレッジマップをJSON形式でエクスポートします
func (cb *CoverageBuilder) ExportToJSON(coverageMap *models.CoverageMap) ([]byte, error) {
	jsonData, err := json.MarshalIndent(coverageMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal coverage map to JSON: %w", err)
	}
	return jsonData, nil
}
