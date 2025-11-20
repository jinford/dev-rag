package coverage

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverageMap_Structure(t *testing.T) {
	// カバレッジマップの構造体テスト
	snapshotID := uuid.New()
	coverageMap := &models.CoverageMap{
		SnapshotID:        snapshotID.String(),
		SnapshotVersion:   "test-version-1",
		TotalFiles:        10,
		TotalIndexedFiles: 7,
		TotalChunks:       50,
		OverallCoverage:   70.0,
		DomainCoverages: []models.DomainCoverage{
			{
				Domain:        "code",
				TotalFiles:    5,
				IndexedFiles:  4,
				IndexedChunks: 30,
				CoverageRate:  80.0,
			},
			{
				Domain:        "tests",
				TotalFiles:    3,
				IndexedFiles:  2,
				IndexedChunks: 15,
				CoverageRate:  66.67,
			},
			{
				Domain:        "architecture",
				TotalFiles:    2,
				IndexedFiles:  1,
				IndexedChunks: 5,
				CoverageRate:  50.0,
			},
		},
		GeneratedAt: time.Now(),
	}

	// 検証
	assert.Equal(t, snapshotID.String(), coverageMap.SnapshotID)
	assert.Equal(t, "test-version-1", coverageMap.SnapshotVersion)
	assert.Equal(t, 10, coverageMap.TotalFiles)
	assert.Equal(t, 7, coverageMap.TotalIndexedFiles)
	assert.Equal(t, 50, coverageMap.TotalChunks)
	assert.Equal(t, 70.0, coverageMap.OverallCoverage)
	assert.Len(t, coverageMap.DomainCoverages, 3)
}

func TestCoverageMap_ExportToJSON(t *testing.T) {
	// テストデータ作成
	snapshotID := uuid.New()
	coverageMap := &models.CoverageMap{
		SnapshotID:        snapshotID.String(),
		SnapshotVersion:   "test-version-1",
		TotalFiles:        10,
		TotalIndexedFiles: 7,
		TotalChunks:       50,
		OverallCoverage:   70.0,
		DomainCoverages: []models.DomainCoverage{
			{
				Domain:           "code",
				TotalFiles:       5,
				IndexedFiles:     4,
				IndexedChunks:    30,
				CoverageRate:     80.0,
				AvgCommentRatio:  0.25,
				AvgComplexity:    4.5,
				UnindexedImportantFiles: []string{"README.md"},
			},
		},
		GeneratedAt: time.Now(),
	}

	// JSON出力
	jsonData, err := json.MarshalIndent(coverageMap, "", "  ")
	require.NoError(t, err)
	require.NotNil(t, jsonData)

	// JSONが有効かチェック
	var decoded models.CoverageMap
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, coverageMap.SnapshotID, decoded.SnapshotID)
	assert.Equal(t, coverageMap.TotalFiles, decoded.TotalFiles)
	assert.Equal(t, coverageMap.TotalIndexedFiles, decoded.TotalIndexedFiles)
	assert.Len(t, decoded.DomainCoverages, 1)

	t.Logf("Exported JSON:\n%s", string(jsonData))
}

func TestDomainCoverage_Structure(t *testing.T) {
	// ドメインカバレッジの構造体テスト
	dc := models.DomainCoverage{
		Domain:           "code",
		TotalFiles:       10,
		IndexedFiles:     8,
		IndexedChunks:    50,
		CoverageRate:     80.0,
		AvgCommentRatio:  0.3,
		AvgComplexity:    5.2,
		UnindexedImportantFiles: []string{"README.md", "CONTRIBUTING.md"},
	}

	assert.Equal(t, "code", dc.Domain)
	assert.Equal(t, 10, dc.TotalFiles)
	assert.Equal(t, 8, dc.IndexedFiles)
	assert.Equal(t, 50, dc.IndexedChunks)
	assert.Equal(t, 80.0, dc.CoverageRate)
	assert.Equal(t, 0.3, dc.AvgCommentRatio)
	assert.Equal(t, 5.2, dc.AvgComplexity)
	assert.Len(t, dc.UnindexedImportantFiles, 2)
}

func TestSnapshotFile_Structure(t *testing.T) {
	// SnapshotFileの構造体テスト
	snapshotID := uuid.New()
	domain := "code"
	skipReason := "ignored by filter"

	sf := models.SnapshotFile{
		ID:         uuid.New(),
		SnapshotID: snapshotID,
		FilePath:   "main.go",
		FileSize:   1000,
		Domain:     &domain,
		Indexed:    true,
		SkipReason: nil,
		CreatedAt:  time.Now(),
	}

	assert.NotEqual(t, uuid.Nil, sf.ID)
	assert.Equal(t, snapshotID, sf.SnapshotID)
	assert.Equal(t, "main.go", sf.FilePath)
	assert.Equal(t, int64(1000), sf.FileSize)
	assert.NotNil(t, sf.Domain)
	assert.Equal(t, "code", *sf.Domain)
	assert.True(t, sf.Indexed)
	assert.Nil(t, sf.SkipReason)

	// 未インデックスファイルのケース
	sf2 := models.SnapshotFile{
		ID:         uuid.New(),
		SnapshotID: snapshotID,
		FilePath:   "vendor/lib.go",
		FileSize:   2000,
		Domain:     &domain,
		Indexed:    false,
		SkipReason: &skipReason,
		CreatedAt:  time.Now(),
	}

	assert.False(t, sf2.Indexed)
	assert.NotNil(t, sf2.SkipReason)
	assert.Equal(t, "ignored by filter", *sf2.SkipReason)
}
