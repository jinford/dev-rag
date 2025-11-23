package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/quality"
	"github.com/jinford/dev-rag/pkg/repository"
	"github.com/jinford/dev-rag/pkg/sqlc"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v3"
)

// FreshnessCheckAction はインデックス鮮度をチェックするコマンドのアクション
// インデックス鮮度の監視
func FreshnessCheckAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")
	threshold := cmd.Int("threshold")
	repoPath := cmd.String("repo-path")

	if threshold <= 0 {
		threshold = 30 // デフォルトは30日
	}

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	indexRepo := repository.NewIndexRepositoryR(sqlc.New(appCtx.Database.Pool))
	monitor := quality.NewFreshnessMonitor(indexRepo, repoPath, threshold)

	// 鮮度レポートを生成
	report, err := monitor.GenerateFreshnessReport(ctx, threshold)
	if err != nil {
		return fmt.Errorf("鮮度レポートの生成に失敗: %w", err)
	}

	// レポートを表示
	displayFreshnessReport(report)

	return nil
}

// FreshnessAlertAction は古いチャンクのアラートを表示するコマンドのアクション
func FreshnessAlertAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")
	threshold := cmd.Int("threshold")
	repoPath := cmd.String("repo-path")
	exportFile := cmd.String("export")

	if threshold <= 0 {
		threshold = 30 // デフォルトは30日
	}

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	indexRepo := repository.NewIndexRepositoryR(sqlc.New(appCtx.Database.Pool))
	monitor := quality.NewFreshnessMonitor(indexRepo, repoPath, threshold)

	// 古いチャンクを検出
	staleChunks, err := monitor.DetectStaleChunks(ctx, threshold)
	if err != nil {
		return fmt.Errorf("古いチャンクの検出に失敗: %w", err)
	}

	if len(staleChunks) == 0 {
		fmt.Println("✓ 古いチャンクは見つかりませんでした")
		return nil
	}

	// アラートを表示
	fmt.Printf("\n⚠ %d 個の古いチャンクが見つかりました（閾値: %d日）\n\n", len(staleChunks), threshold)

	// JSON形式でエクスポート
	if exportFile != "" {
		return exportStaleChunksToJSON(staleChunks, exportFile)
	}

	// テーブル形式で表示
	displayStaleChunksTable(staleChunks)

	return nil
}

// FreshnessReindexAction は自動再インデックスアクションを生成するコマンドのアクション
// 自動再インデックストリガー
func FreshnessReindexAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")
	threshold := cmd.Int("threshold")
	repoPath := cmd.String("repo-path")
	dryRun := cmd.Bool("dry-run")

	if threshold <= 0 {
		threshold = 30 // デフォルトは30日
	}

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	indexRepo := repository.NewIndexRepositoryR(sqlc.New(appCtx.Database.Pool))
	monitor := quality.NewFreshnessMonitor(indexRepo, repoPath, threshold)

	// 古いチャンクを検出
	staleChunks, err := monitor.DetectStaleChunks(ctx, threshold)
	if err != nil {
		return fmt.Errorf("古いチャンクの検出に失敗: %w", err)
	}

	if len(staleChunks) == 0 {
		fmt.Println("✓ 古いチャンクは見つかりませんでした。再インデックスは不要です。")
		return nil
	}

	// 再インデックスアクションを生成
	actions, err := monitor.GenerateReindexActions(ctx, staleChunks)
	if err != nil {
		return fmt.Errorf("再インデックスアクションの生成に失敗: %w", err)
	}

	if dryRun {
		fmt.Printf("\n[DRY RUN] %d 個の再インデックスアクションを生成しました\n\n", len(actions))
		displayReindexActionsTable(actions)
		return nil
	}

	// TODO: action_backlog に登録する実装
	fmt.Printf("\n✓ %d 個の再インデックスアクションを生成しました\n", len(actions))
	displayReindexActionsTable(actions)

	return nil
}

// displayFreshnessReport は鮮度レポートをテーブル形式で表示します
func displayFreshnessReport(report *models.FreshnessReport) {
	fmt.Println("\n=== インデックス鮮度レポート ===")
	fmt.Printf("生成日時: %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05"))

	table := tablewriter.NewWriter(os.Stdout)
	table.Header("メトリクス", "値")

	table.Append("総チャンク数", fmt.Sprintf("%d", report.TotalChunks))
	table.Append("古いチャンク数", fmt.Sprintf("%d", report.StaleChunks))
	table.Append("平均鮮度（日数）", fmt.Sprintf("%.1f", report.AverageFreshnessDays))
	table.Append("鮮度閾値（日数）", fmt.Sprintf("%d", report.FreshnessThreshold))

	if report.TotalChunks > 0 {
		staleRate := float64(report.StaleChunks) / float64(report.TotalChunks) * 100
		table.Append("古いチャンクの割合", fmt.Sprintf("%.1f%%", staleRate))
	}

	table.Render()

	// 古いチャンクの詳細
	if len(report.StaleChunkDetails) > 0 {
		fmt.Println("\n=== 古いチャンクの詳細 ===")
		displayStaleChunksTable(report.StaleChunkDetails)
	}
}

// displayStaleChunksTable は古いチャンクをテーブル形式で表示します
func displayStaleChunksTable(chunks []models.ChunkFreshness) {
	if len(chunks) == 0 {
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header("ファイルパス", "チャンクキー", "鮮度（日数）", "最終更新日")

	for _, chunk := range chunks {
		table.Append(
			chunk.FilePath,
			chunk.ChunkKey,
			fmt.Sprintf("%d", chunk.FreshnessDays),
			chunk.LastUpdated.Format("2006-01-02"),
		)
	}

	table.Render()
}

// displayReindexActionsTable は再インデックスアクションをテーブル形式で表示します
func displayReindexActionsTable(actions []quality.ReindexAction) {
	if len(actions) == 0 {
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header("ファイルパス", "チャンク数", "理由", "閾値（日数）")

	for _, action := range actions {
		table.Append(
			action.FilePath,
			fmt.Sprintf("%d", len(action.ChunkIDs)),
			action.Reason,
			fmt.Sprintf("%d", action.ThresholdDays),
		)
	}

	table.Render()
}

// exportStaleChunksToJSON は古いチャンクをJSON形式でエクスポートします
func exportStaleChunksToJSON(chunks []models.ChunkFreshness, filename string) error {
	data, err := json.MarshalIndent(chunks, "", "  ")
	if err != nil {
		return fmt.Errorf("JSONエンコードに失敗: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("ファイル書き込みに失敗: %w", err)
	}

	fmt.Printf("✓ 古いチャンク情報を %s にエクスポートしました\n", filename)
	return nil
}
