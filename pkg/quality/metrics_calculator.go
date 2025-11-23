package quality

import (
	"context"
	"fmt"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/repository"
)

// QualityMetricsCalculator は品質メトリクスを計算するサービスです
// Phase 4タスク7: 品質メトリクスの定量評価
type QualityMetricsCalculator struct {
	qualityRepo *repository.QualityRepositoryR
}

// NewQualityMetricsCalculator は新しい QualityMetricsCalculator を作成します
func NewQualityMetricsCalculator(qualityRepo *repository.QualityRepositoryR) *QualityMetricsCalculator {
	return &QualityMetricsCalculator{
		qualityRepo: qualityRepo,
	}
}

// CalculateMetrics は品質メトリクスを計算します
func (c *QualityMetricsCalculator) CalculateMetrics(ctx context.Context, startDate, endDate *time.Time) (*models.QualityMetrics, error) {
	// フィルタ条件を設定
	filter := &models.QualityNoteFilter{
		StartDate: startDate,
		EndDate:   endDate,
	}

	// 全ノートを取得
	notes, err := c.qualityRepo.ListQualityNotes(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list quality notes: %w", err)
	}

	// メトリクスを初期化
	metrics := &models.QualityMetrics{
		BySeverity:  make(map[models.QualitySeverity]int),
		GeneratedAt: time.Now(),
	}

	// 集計
	for _, note := range notes {
		metrics.TotalNotes++
		metrics.BySeverity[note.Severity]++
		if note.Status == models.QualityStatusOpen {
			metrics.OpenNotes++
		} else {
			metrics.ResolvedNotes++
		}
	}

	return metrics, nil
}

// CalculateWeeklyMetrics は週次メトリクスを計算します
func (c *QualityMetricsCalculator) CalculateWeeklyMetrics(ctx context.Context, startDate, endDate time.Time) (*models.QualityMetrics, error) {
	return c.CalculateMetrics(ctx, &startDate, &endDate)
}

// CalculateMonthlyMetrics は月次メトリクスを計算します
func (c *QualityMetricsCalculator) CalculateMonthlyMetrics(ctx context.Context, year, month int) (*models.QualityMetrics, error) {
	// 月の開始と終了を計算
	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0).Add(-time.Second)

	return c.CalculateMetrics(ctx, &startDate, &endDate)
}

// CalculateTrendMetrics は時系列トレンドメトリクスを計算します
func (c *QualityMetricsCalculator) CalculateTrendMetrics(ctx context.Context, daysBack int) (*models.QualityMetrics, error) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -daysBack)

	metrics, err := c.CalculateMetrics(ctx, &startDate, &endDate)
	if err != nil {
		return nil, err
	}

	// 日別トレンドを計算
	trend := make([]models.QualityTrendPoint, 0)
	for i := 0; i <= daysBack; i++ {
		date := startDate.AddDate(0, 0, i)
		nextDate := date.AddDate(0, 0, 1)

		dailyFilter := &models.QualityNoteFilter{
			StartDate: &date,
			EndDate:   &nextDate,
		}

		notes, err := c.qualityRepo.ListQualityNotes(ctx, dailyFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to get daily notes: %w", err)
		}

		openCount := 0
		resolvedCount := 0
		for _, note := range notes {
			if note.Status == models.QualityStatusOpen {
				openCount++
			} else {
				resolvedCount++
			}
		}

		trend = append(trend, models.QualityTrendPoint{
			Date:          date,
			OpenCount:     openCount,
			ResolvedCount: resolvedCount,
		})
	}

	metrics.RecentTrend = trend
	return metrics, nil
}

// CalculateSeverityBreakdown は深刻度別の内訳を計算します
func (c *QualityMetricsCalculator) CalculateSeverityBreakdown(ctx context.Context) (map[models.QualitySeverity]int, error) {
	notes, err := c.qualityRepo.ListQualityNotes(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list quality notes: %w", err)
	}

	breakdown := make(map[models.QualitySeverity]int)
	for _, note := range notes {
		breakdown[note.Severity]++
	}

	return breakdown, nil
}

// GetResolutionRate は品質ノートの解決率を計算します
func (c *QualityMetricsCalculator) GetResolutionRate(ctx context.Context) (float64, error) {
	notes, err := c.qualityRepo.ListQualityNotes(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to list quality notes: %w", err)
	}

	if len(notes) == 0 {
		return 0, nil
	}

	resolvedCount := 0
	for _, note := range notes {
		if note.Status == models.QualityStatusResolved {
			resolvedCount++
		}
	}

	return float64(resolvedCount) / float64(len(notes)) * 100, nil
}
