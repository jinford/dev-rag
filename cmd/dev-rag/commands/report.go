package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/jinford/dev-rag/pkg/quality"
	"github.com/jinford/dev-rag/pkg/repository"
	"github.com/jinford/dev-rag/pkg/sqlc"
	"github.com/urfave/cli/v3"
)

// ReportGenerateAction はHTMLレポートを生成するコマンドのアクション
// 品質ダッシュボードの作成
func ReportGenerateAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")
	output := cmd.String("output")
	openBrowser := cmd.Bool("open")
	freshnessThreshold := cmd.Int("freshness-threshold")

	// デフォルトの出力ファイル名
	if output == "" {
		output = "quality_report.html"
	}

	// 出力ファイルの絶対パスを取得
	absOutput, err := filepath.Abs(output)
	if err != nil {
		return fmt.Errorf("出力パスの解決に失敗: %w", err)
	}

	// 出力ディレクトリが存在することを確認
	outputDir := filepath.Dir(absOutput)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("出力ディレクトリの作成に失敗: %w", err)
	}

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	// リポジトリとサービスの初期化
	queries := sqlc.New(appCtx.Database.Pool)
	qualityRepo := repository.NewQualityRepositoryR(queries)
	indexRepo := repository.NewIndexRepositoryR(queries)

	// メトリクス計算機の初期化
	metricsCalc := quality.NewQualityMetricsCalculator(qualityRepo)
	metricsAdapter := quality.NewMetricsCalculatorAdapter(metricsCalc)

	// 鮮度計算機の初期化
	if freshnessThreshold == 0 {
		freshnessThreshold = 30 // デフォルト30日
	}
	freshnessMonitor := quality.NewFreshnessMonitor(indexRepo, ".", freshnessThreshold)
	freshnessAdapter := quality.NewFreshnessMonitorAdapter(freshnessMonitor, freshnessThreshold)

	// レポート生成器の初期化
	reportGen := quality.NewReportGenerator(metricsAdapter, freshnessAdapter)

	// HTMLレポートの生成
	fmt.Printf("HTMLレポートを生成中...\n")
	if err := reportGen.GenerateHTMLReport(absOutput); err != nil {
		return fmt.Errorf("HTMLレポートの生成に失敗: %w", err)
	}

	fmt.Printf("✓ HTMLレポートを生成しました: %s\n", absOutput)

	// ブラウザで開く
	if openBrowser {
		if err := openInBrowser(absOutput); err != nil {
			fmt.Fprintf(os.Stderr, "警告: ブラウザを開けませんでした: %v\n", err)
			fmt.Printf("次のコマンドで手動で開いてください:\n  open %s\n", absOutput)
		} else {
			fmt.Printf("✓ ブラウザでレポートを開きました\n")
		}
	}

	return nil
}

// openInBrowser はブラウザでファイルを開きます（OS依存）
func openInBrowser(filepath string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("open", filepath)
	case "linux":
		// Linuxではxdg-openを試す
		cmd = exec.Command("xdg-open", filepath)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", filepath)
	default:
		return fmt.Errorf("未サポートのOS: %s", runtime.GOOS)
	}

	return cmd.Start()
}
