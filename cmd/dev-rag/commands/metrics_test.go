package commands

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestExportMetricsToJSON(t *testing.T) {
	// テスト用のメトリクス
	metrics := &models.QualityMetrics{
		TotalNotes:    10,
		OpenNotes:     6,
		ResolvedNotes: 4,
		BySeverity: map[models.QualitySeverity]int{
			models.QualitySeverityCritical: 2,
			models.QualitySeverityHigh:     3,
			models.QualitySeverityMedium:   3,
			models.QualitySeverityLow:      2,
		},
		AverageFreshnessDays: 25.5,
		StaleChunkCount:      5,
		FreshnessThreshold:   30,
		GeneratedAt:          time.Now(),
	}

	// 一時ファイルに書き出し
	tmpFile := "/tmp/test_metrics.json"
	defer os.Remove(tmpFile)

	err := exportMetricsToJSON(metrics, tmpFile)
	assert.NoError(t, err)

	// ファイルが存在することを確認
	_, err = os.Stat(tmpFile)
	assert.NoError(t, err)

	// ファイルを読み込んで内容を確認
	data, err := os.ReadFile(tmpFile)
	assert.NoError(t, err)

	var loadedMetrics models.QualityMetrics
	err = json.Unmarshal(data, &loadedMetrics)
	assert.NoError(t, err)

	assert.Equal(t, metrics.TotalNotes, loadedMetrics.TotalNotes)
	assert.Equal(t, metrics.OpenNotes, loadedMetrics.OpenNotes)
	assert.Equal(t, metrics.ResolvedNotes, loadedMetrics.ResolvedNotes)
	assert.Equal(t, metrics.AverageFreshnessDays, loadedMetrics.AverageFreshnessDays)
	assert.Equal(t, metrics.StaleChunkCount, loadedMetrics.StaleChunkCount)
}

func TestDisplayMetricsTable(t *testing.T) {
	// テスト用のメトリクス
	metrics := &models.QualityMetrics{
		TotalNotes:    10,
		OpenNotes:     6,
		ResolvedNotes: 4,
		BySeverity: map[models.QualitySeverity]int{
			models.QualitySeverityCritical: 2,
			models.QualitySeverityHigh:     3,
			models.QualitySeverityMedium:   3,
			models.QualitySeverityLow:      2,
		},
		RecentTrend: []models.QualityTrendPoint{
			{
				Date:          time.Now().AddDate(0, 0, -2),
				OpenCount:     8,
				ResolvedCount: 2,
			},
			{
				Date:          time.Now().AddDate(0, 0, -1),
				OpenCount:     7,
				ResolvedCount: 3,
			},
		},
		AverageFreshnessDays: 25.5,
		StaleChunkCount:      5,
		FreshnessThreshold:   30,
		GeneratedAt:          time.Now(),
	}

	// パニックが発生しないことを確認
	assert.NotPanics(t, func() {
		displayMetricsTable(metrics)
	})
}

func TestDisplayMetricsTable_EmptyMetrics(t *testing.T) {
	// 空のメトリクス
	metrics := &models.QualityMetrics{
		BySeverity:  make(map[models.QualitySeverity]int),
		GeneratedAt: time.Now(),
	}

	// パニックが発生しないことを確認
	assert.NotPanics(t, func() {
		displayMetricsTable(metrics)
	})
}
