package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/quality"
	"github.com/jinford/dev-rag/pkg/repository"
	"github.com/jinford/dev-rag/pkg/sqlc"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v3"
)

// MetricsQualityAction は品質メトリクスを表示するコマンドのアクション
// Phase 4タスク7: 品質メトリクスの定量評価
func MetricsQualityAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")
	exportFile := cmd.String("export")
	period := cmd.String("period")
	startDateStr := cmd.String("start-date")
	endDateStr := cmd.String("end-date")
	year := cmd.Int("year")
	month := cmd.Int("month")

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	qualityRepo := repository.NewQualityRepositoryR(sqlc.New(appCtx.Database.Pool))
	calculator := quality.NewQualityMetricsCalculator(qualityRepo)

	var metrics *models.QualityMetrics

	// 期間別にメトリクスを計算
	switch period {
	case "weekly":
		if startDateStr == "" || endDateStr == "" {
			return fmt.Errorf("週次集計には --start-date と --end-date が必要です")
		}
		startDate, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return fmt.Errorf("開始日のパースに失敗: %w", err)
		}
		endDate, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return fmt.Errorf("終了日のパースに失敗: %w", err)
		}
		metrics, err = calculator.CalculateWeeklyMetrics(ctx, startDate, endDate)
		if err != nil {
			return fmt.Errorf("週次メトリクスの計算に失敗: %w", err)
		}

	case "monthly":
		if year == 0 || month == 0 {
			return fmt.Errorf("月次集計には --year と --month が必要です")
		}
		metrics, err = calculator.CalculateMonthlyMetrics(ctx, year, month)
		if err != nil {
			return fmt.Errorf("月次メトリクスの計算に失敗: %w", err)
		}

	default:
		// デフォルトは全期間
		metrics, err = calculator.CalculateMetrics(ctx, nil, nil)
		if err != nil {
			return fmt.Errorf("メトリクスの計算に失敗: %w", err)
		}
	}

	// JSON形式でエクスポート
	if exportFile != "" {
		return exportMetricsToJSON(metrics, exportFile)
	}

	// テーブル形式で表示
	displayMetricsTable(metrics)

	return nil
}

// displayMetricsTable はメトリクスをテーブル形式で表示します
func displayMetricsTable(metrics *models.QualityMetrics) {
	fmt.Println("\n=== 品質メトリクス ===")
	fmt.Printf("生成日時: %s\n\n", metrics.GeneratedAt.Format("2006-01-02 15:04:05"))

	// サマリーテーブル
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("メトリクス", "値")

	table.Append("総ノート数", fmt.Sprintf("%d", metrics.TotalNotes))
	table.Append("未解決ノート数", fmt.Sprintf("%d", metrics.OpenNotes))
	table.Append("解決済みノート数", fmt.Sprintf("%d", metrics.ResolvedNotes))

	if metrics.TotalNotes > 0 {
		resolutionRate := float64(metrics.ResolvedNotes) / float64(metrics.TotalNotes) * 100
		table.Append("解決率", fmt.Sprintf("%.1f%%", resolutionRate))
	}

	if metrics.AverageFreshnessDays > 0 {
		table.Append("平均鮮度（日数）", fmt.Sprintf("%.1f", metrics.AverageFreshnessDays))
	}
	if metrics.StaleChunkCount > 0 {
		table.Append("古いチャンク数", fmt.Sprintf("%d", metrics.StaleChunkCount))
	}

	table.Render()

	// 深刻度別内訳
	if len(metrics.BySeverity) > 0 {
		fmt.Println("\n=== 深刻度別内訳 ===")
		severityTable := tablewriter.NewWriter(os.Stdout)
		severityTable.Header("深刻度", "件数")

		severityTable.Append("Critical", fmt.Sprintf("%d", metrics.BySeverity["critical"]))
		severityTable.Append("High", fmt.Sprintf("%d", metrics.BySeverity["high"]))
		severityTable.Append("Medium", fmt.Sprintf("%d", metrics.BySeverity["medium"]))
		severityTable.Append("Low", fmt.Sprintf("%d", metrics.BySeverity["low"]))

		severityTable.Render()
	}

	// トレンド情報
	if len(metrics.RecentTrend) > 0 {
		fmt.Println("\n=== 最近のトレンド ===")
		trendTable := tablewriter.NewWriter(os.Stdout)
		trendTable.Header("日付", "未解決", "解決済み")

		for _, point := range metrics.RecentTrend {
			trendTable.Append(
				point.Date.Format("2006-01-02"),
				fmt.Sprintf("%d", point.OpenCount),
				fmt.Sprintf("%d", point.ResolvedCount),
			)
		}

		trendTable.Render()
	}
}

// exportMetricsToJSON はメトリクスをJSON形式でエクスポートします
func exportMetricsToJSON(metrics *models.QualityMetrics, filename string) error {
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("JSONエンコードに失敗: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("ファイル書き込みに失敗: %w", err)
	}

	fmt.Printf("✓ メトリクスを %s にエクスポートしました\n", filename)
	return nil
}
