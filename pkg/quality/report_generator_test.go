package quality

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportGenerator_GenerateHTMLReport(t *testing.T) {
	// モックの準備
	mockMetricsCalc := &MockMetricsCalculator{
		metrics: &models.QualityMetrics{
			TotalNotes:    10,
			OpenNotes:     4,
			ResolvedNotes: 6,
			BySeverity: map[models.QualitySeverity]int{
				models.QualitySeverityCritical: 2,
				models.QualitySeverityHigh:     3,
				models.QualitySeverityMedium:   3,
				models.QualitySeverityLow:      2,
			},
			RecentTrend: []models.QualityTrendPoint{
				{
					Date:          time.Date(2024, 11, 18, 0, 0, 0, 0, time.UTC),
					OpenCount:     5,
					ResolvedCount: 3,
				},
				{
					Date:          time.Date(2024, 11, 19, 0, 0, 0, 0, time.UTC),
					OpenCount:     4,
					ResolvedCount: 6,
				},
			},
			AverageFreshnessDays: 15.5,
			StaleChunkCount:      3,
			FreshnessThreshold:   30,
			GeneratedAt:          time.Now(),
		},
	}

	mockFreshnessCalc := &MockFreshnessCalculator{
		report: &models.FreshnessReport{
			TotalChunks:          100,
			StaleChunks:          3,
			AverageFreshnessDays: 15.5,
			FreshnessThreshold:   30,
			StaleChunkDetails: []models.ChunkFreshness{
				{
					FilePath:      "pkg/quality/report_generator.go",
					ChunkKey:      "func1",
					FreshnessDays: 35,
					IsStale:       true,
					LastUpdated:   time.Date(2024, 10, 15, 0, 0, 0, 0, time.UTC),
				},
			},
			GeneratedAt: time.Now(),
		},
	}

	rg := NewReportGenerator(mockMetricsCalc, mockFreshnessCalc)

	// 一時ファイルに出力
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "report.html")

	err := rg.GenerateHTMLReport(outputPath)
	require.NoError(t, err)

	// ファイルが作成されたことを確認
	_, err = os.Stat(outputPath)
	require.NoError(t, err)

	// ファイルの内容を読み込み
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	html := string(content)

	// HTMLの基本構造を確認
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "品質ダッシュボード")
	assert.Contains(t, html, "chart.js") // Chart.jsのCDNリンク

	// メトリクスの値が含まれているか確認
	assert.Contains(t, html, "10") // TotalNotes
	assert.Contains(t, html, "4")  // OpenNotes
	assert.Contains(t, html, "6")  // ResolvedNotes

	// チャートのキャンバスが含まれているか確認
	assert.Contains(t, html, "severityChart")
	assert.Contains(t, html, "freshnessChart")
	assert.Contains(t, html, "trendChart")

	// 鮮度詳細テーブルが含まれているか確認
	assert.Contains(t, html, "pkg/quality/report_generator.go")
	assert.Contains(t, html, "func1")
}

func TestReportGenerator_PrepareReportData(t *testing.T) {
	mockMetricsCalc := &MockMetricsCalculator{}
	mockFreshnessCalc := &MockFreshnessCalculator{}
	rg := NewReportGenerator(mockMetricsCalc, mockFreshnessCalc)

	metrics := &models.QualityMetrics{
		TotalNotes:    5,
		OpenNotes:     2,
		ResolvedNotes: 3,
		BySeverity: map[models.QualitySeverity]int{
			models.QualitySeverityCritical: 1,
			models.QualitySeverityHigh:     2,
			models.QualitySeverityMedium:   1,
			models.QualitySeverityLow:      1,
		},
		RecentTrend: []models.QualityTrendPoint{
			{
				Date:          time.Date(2024, 11, 20, 0, 0, 0, 0, time.UTC),
				OpenCount:     2,
				ResolvedCount: 3,
			},
		},
		AverageFreshnessDays: 10.0,
		StaleChunkCount:      1,
		FreshnessThreshold:   30,
		GeneratedAt:          time.Now(),
	}

	freshnessReport := &models.FreshnessReport{
		TotalChunks:          50,
		StaleChunks:          1,
		AverageFreshnessDays: 10.0,
		FreshnessThreshold:   30,
		StaleChunkDetails: []models.ChunkFreshness{
			{
				FilePath:      "test.go",
				ChunkKey:      "testFunc",
				FreshnessDays: 35,
				IsStale:       true,
				LastUpdated:   time.Now(),
			},
		},
		GeneratedAt: time.Now(),
	}

	reportData, err := rg.prepareReportData(metrics, freshnessReport)
	require.NoError(t, err)
	require.NotNil(t, reportData)

	// 基本フィールドの確認
	assert.NotEmpty(t, reportData.GeneratedAt)
	assert.Equal(t, metrics, reportData.QualityMetrics)
	assert.Equal(t, freshnessReport, reportData.FreshnessReport)

	// JSONデータの確認
	assert.Contains(t, reportData.SeverityData, "critical")
	assert.Contains(t, reportData.SeverityData, "high")
	assert.Contains(t, reportData.TrendData, "2024-11-20")
	assert.NotEmpty(t, reportData.FreshnessDistData)
}

func TestReportGenerator_CalculateFreshnessDistribution(t *testing.T) {
	mockMetricsCalc := &MockMetricsCalculator{}
	mockFreshnessCalc := &MockFreshnessCalculator{}
	rg := NewReportGenerator(mockMetricsCalc, mockFreshnessCalc)

	report := &models.FreshnessReport{
		StaleChunkDetails: []models.ChunkFreshness{
			{FreshnessDays: 5},  // 0-7 days
			{FreshnessDays: 10}, // 8-14 days
			{FreshnessDays: 20}, // 15-30 days
			{FreshnessDays: 35}, // 30+ days
			{FreshnessDays: 7},  // 0-7 days
			{FreshnessDays: 40}, // 30+ days
		},
	}

	distribution := rg.calculateFreshnessDistribution(report)

	assert.Equal(t, 2, distribution["0-7 days"])
	assert.Equal(t, 1, distribution["8-14 days"])
	assert.Equal(t, 1, distribution["15-30 days"])
	assert.Equal(t, 2, distribution["30+ days"])
}

func TestReportGenerator_RenderHTML(t *testing.T) {
	mockMetricsCalc := &MockMetricsCalculator{}
	mockFreshnessCalc := &MockFreshnessCalculator{}
	rg := NewReportGenerator(mockMetricsCalc, mockFreshnessCalc)

	reportData := &ReportData{
		GeneratedAt: "2024-11-20 10:00:00",
		QualityMetrics: &models.QualityMetrics{
			TotalNotes:           5,
			OpenNotes:            2,
			ResolvedNotes:        3,
			AverageFreshnessDays: 10.5,
			BySeverity: map[models.QualitySeverity]int{
				models.QualitySeverityCritical: 1,
			},
		},
		FreshnessReport: &models.FreshnessReport{
			TotalChunks:        50,
			StaleChunks:        1,
			FreshnessThreshold: 30,
			StaleChunkDetails: []models.ChunkFreshness{
				{
					FilePath:      "test.go",
					ChunkKey:      "func1",
					FreshnessDays: 35,
					LastUpdated:   time.Date(2024, 10, 15, 0, 0, 0, 0, time.UTC),
				},
			},
		},
		SeverityData:      `{"critical":1,"high":0,"medium":0,"low":0}`,
		TrendData:         `[{"date":"2024-11-20T00:00:00Z","openCount":2,"resolvedCount":3}]`,
		FreshnessDistData: `{"0-7 days":0,"8-14 days":0,"15-30 days":0,"30+ days":1}`,
	}

	html, err := rg.renderHTML(reportData)
	require.NoError(t, err)
	assert.NotEmpty(t, html)

	// HTMLの基本構造を確認
	assert.True(t, strings.HasPrefix(html, "<!DOCTYPE html>"))
	assert.Contains(t, html, "品質ダッシュボード")
	assert.Contains(t, html, "2024-11-20 10:00:00")

	// メトリクス値の確認
	assert.Contains(t, html, "5")  // TotalNotes
	assert.Contains(t, html, "2")  // OpenNotes
	assert.Contains(t, html, "3")  // ResolvedNotes
	assert.Contains(t, html, "10.5") // AverageFreshnessDays

	// チャートデータの確認
	assert.Contains(t, html, "severityChart")
	assert.Contains(t, html, "freshnessChart")
	assert.Contains(t, html, "trendChart")

	// 鮮度テーブルの確認
	assert.Contains(t, html, "test.go")
	assert.Contains(t, html, "func1")
	assert.Contains(t, html, "35")
}

func TestReportGenerator_EmptyStaleChunks(t *testing.T) {
	mockMetricsCalc := &MockMetricsCalculator{
		metrics: &models.QualityMetrics{
			TotalNotes:           0,
			OpenNotes:            0,
			ResolvedNotes:        0,
			AverageFreshnessDays: 5.0,
			StaleChunkCount:      0,
			BySeverity: map[models.QualitySeverity]int{
				models.QualitySeverityCritical: 0,
				models.QualitySeverityHigh:     0,
				models.QualitySeverityMedium:   0,
				models.QualitySeverityLow:      0,
			},
			RecentTrend:        []models.QualityTrendPoint{},
			FreshnessThreshold: 30,
			GeneratedAt:        time.Now(),
		},
	}

	mockFreshnessCalc := &MockFreshnessCalculator{
		report: &models.FreshnessReport{
			TotalChunks:          100,
			StaleChunks:          0,
			AverageFreshnessDays: 5.0,
			FreshnessThreshold:   30,
			StaleChunkDetails:    []models.ChunkFreshness{},
			GeneratedAt:          time.Now(),
		},
	}

	rg := NewReportGenerator(mockMetricsCalc, mockFreshnessCalc)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "report.html")

	err := rg.GenerateHTMLReport(outputPath)
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	html := string(content)

	// 古いチャンクがない場合のメッセージを確認
	assert.Contains(t, html, "すべてのチャンクが最新です")
}
