package domain

import (
	"time"

	"github.com/google/uuid"
)

// BuildCoverageMap は統計からカバレッジマップを構築します（純粋計算）
func BuildCoverageMap(snapshotID uuid.UUID, snapshotVersion string, domainStats []*DomainCoverage) *CoverageMap {
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

	// []*DomainCoverage を []DomainCoverage に変換
	coverages := make([]DomainCoverage, len(domainStats))
	for i, ds := range domainStats {
		coverages[i] = *ds
	}

	return &CoverageMap{
		SnapshotID:        snapshotID.String(),
		SnapshotVersion:   snapshotVersion,
		TotalFiles:        totalFiles,
		TotalIndexedFiles: totalIndexedFiles,
		TotalChunks:       totalChunks,
		OverallCoverage:   overallCoverage,
		DomainCoverages:   coverages,
		GeneratedAt:       time.Now(),
	}
}
