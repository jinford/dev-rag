package commands

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/quality"
	"github.com/stretchr/testify/assert"
)

func TestExportStaleChunksToJSON(t *testing.T) {
	// テスト用の古いチャンク
	chunks := []models.ChunkFreshness{
		{
			ChunkID:       uuid.New(),
			FilePath:      "/path/to/file1.go",
			ChunkKey:      "file1.go#L1-L10",
			GitCommitHash: "abc123",
			LatestCommit:  "def456",
			FreshnessDays: 45,
			IsStale:       true,
			LastUpdated:   time.Now().AddDate(0, 0, -45),
		},
		{
			ChunkID:       uuid.New(),
			FilePath:      "/path/to/file2.go",
			ChunkKey:      "file2.go#L1-L10",
			GitCommitHash: "ghi789",
			LatestCommit:  "def456",
			FreshnessDays: 50,
			IsStale:       true,
			LastUpdated:   time.Now().AddDate(0, 0, -50),
		},
	}

	// 一時ファイルに書き出し
	tmpFile := "/tmp/test_stale_chunks.json"
	defer os.Remove(tmpFile)

	err := exportStaleChunksToJSON(chunks, tmpFile)
	assert.NoError(t, err)

	// ファイルが存在することを確認
	_, err = os.Stat(tmpFile)
	assert.NoError(t, err)

	// ファイルを読み込んで内容を確認
	data, err := os.ReadFile(tmpFile)
	assert.NoError(t, err)

	var loadedChunks []models.ChunkFreshness
	err = json.Unmarshal(data, &loadedChunks)
	assert.NoError(t, err)

	assert.Len(t, loadedChunks, 2)
	assert.Equal(t, chunks[0].FilePath, loadedChunks[0].FilePath)
	assert.Equal(t, chunks[1].FilePath, loadedChunks[1].FilePath)
}

func TestDisplayFreshnessReport(t *testing.T) {
	// テスト用の鮮度レポート
	report := &models.FreshnessReport{
		TotalChunks:          100,
		StaleChunks:          10,
		AverageFreshnessDays: 25.5,
		FreshnessThreshold:   30,
		StaleChunkDetails: []models.ChunkFreshness{
			{
				ChunkID:       uuid.New(),
				FilePath:      "/path/to/file1.go",
				ChunkKey:      "file1.go#L1-L10",
				FreshnessDays: 45,
				IsStale:       true,
				LastUpdated:   time.Now().AddDate(0, 0, -45),
			},
		},
		GeneratedAt: time.Now(),
	}

	// パニックが発生しないことを確認
	assert.NotPanics(t, func() {
		displayFreshnessReport(report)
	})
}

func TestDisplayStaleChunksTable(t *testing.T) {
	chunks := []models.ChunkFreshness{
		{
			ChunkID:       uuid.New(),
			FilePath:      "/path/to/file1.go",
			ChunkKey:      "file1.go#L1-L10",
			FreshnessDays: 45,
			IsStale:       true,
			LastUpdated:   time.Now().AddDate(0, 0, -45),
		},
		{
			ChunkID:       uuid.New(),
			FilePath:      "/path/to/file2.go",
			ChunkKey:      "file2.go#L1-L10",
			FreshnessDays: 50,
			IsStale:       true,
			LastUpdated:   time.Now().AddDate(0, 0, -50),
		},
	}

	// パニックが発生しないことを確認
	assert.NotPanics(t, func() {
		displayStaleChunksTable(chunks)
	})
}

func TestDisplayStaleChunksTable_Empty(t *testing.T) {
	// 空のスライス
	chunks := []models.ChunkFreshness{}

	// パニックが発生しないことを確認
	assert.NotPanics(t, func() {
		displayStaleChunksTable(chunks)
	})
}

func TestDisplayReindexActionsTable(t *testing.T) {
	actions := []quality.ReindexAction{
		{
			FilePath:      "/path/to/file1.go",
			ChunkIDs:      []uuid.UUID{uuid.New(), uuid.New()},
			Reason:        "stale_chunks_detected",
			ThresholdDays: 30,
			CreatedAt:     time.Now(),
		},
		{
			FilePath:      "/path/to/file2.go",
			ChunkIDs:      []uuid.UUID{uuid.New()},
			Reason:        "stale_chunks_detected",
			ThresholdDays: 30,
			CreatedAt:     time.Now(),
		},
	}

	// パニックが発生しないことを確認
	assert.NotPanics(t, func() {
		displayReindexActionsTable(actions)
	})
}

func TestDisplayReindexActionsTable_Empty(t *testing.T) {
	// 空のスライス
	actions := []quality.ReindexAction{}

	// パニックが発生しないことを確認
	assert.NotPanics(t, func() {
		displayReindexActionsTable(actions)
	})
}
