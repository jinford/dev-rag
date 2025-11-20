package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/indexer/coverage"
	"github.com/jinford/dev-rag/pkg/models"
)

// この例では、カバレッジアラート機能の基本的な使い方を示します
func main() {
	// サンプルのカバレッジマップを作成
	coverageMap := createSampleCoverageMap()

	// デフォルト設定でAlertGeneratorを作成
	// 実際の使用では、repository.IndexRepositoryRのインスタンスを渡します
	alertGen := coverage.NewAlertGenerator(nil, nil)

	fmt.Println("=== カバレッジマップの内容 ===")
	printCoverageMap(coverageMap)

	fmt.Println("\n=== アラート評価 (デフォルト設定) ===")

	// カバレッジマップからアラートを生成
	// 注意: この例ではGenerateAlertsは使用できません（context.Contextとrepositoryが必要）
	// 代わりに、テストカバレッジのアラートのみをチェックします
	fmt.Println("デフォルト設定:")
	fmt.Printf("  - README未インデックスアラート: %v\n", alertGen.Config().EnableMissingReadmeAlert)
	fmt.Printf("  - ADR総数閾値: %d件\n", alertGen.Config().ADRTotalThreshold)
	fmt.Printf("  - ADRインデックス閾値: %d件\n", alertGen.Config().ADRIndexedThreshold)
	fmt.Printf("  - テストカバレッジ閾値: %.1f%%\n", alertGen.Config().TestCoverageThreshold)

	fmt.Println("\n=== カスタム設定でのアラート評価 ===")

	// カスタム設定でAlertGeneratorを作成
	customConfig := &coverage.AlertConfig{
		EnableMissingReadmeAlert: true,
		ADRTotalThreshold:        5,   // ADRが5件以上ある場合にチェック
		ADRIndexedThreshold:      3,   // インデックス済みADRが3件未満なら警告
		TestCoverageThreshold:    30.0, // テストカバレッジが30%未満なら警告
	}
	fmt.Println("カスタム設定:")
	fmt.Printf("  - README未インデックスアラート: %v\n", customConfig.EnableMissingReadmeAlert)
	fmt.Printf("  - ADR総数閾値: %d件\n", customConfig.ADRTotalThreshold)
	fmt.Printf("  - ADRインデックス閾値: %d件\n", customConfig.ADRIndexedThreshold)
	fmt.Printf("  - テストカバレッジ閾値: %.1f%%\n", customConfig.TestCoverageThreshold)

	fmt.Println("\n実際の使用例:")
	fmt.Println("  alertGen := coverage.NewAlertGenerator(indexRepo, config)")
	fmt.Println("  alerts, err := alertGen.GenerateAlerts(ctx, snapshotID, coverageMap)")
	fmt.Println("  if err != nil { log.Fatal(err) }")
	fmt.Println("  alertGen.PrintAlerts(alerts)")
}

// createSampleCoverageMap はサンプルのカバレッジマップを作成します
func createSampleCoverageMap() *models.CoverageMap {
	return &models.CoverageMap{
		SnapshotID:        uuid.New().String(),
		SnapshotVersion:   "example-v1",
		TotalFiles:        350,
		TotalIndexedFiles: 245,
		TotalChunks:       1250,
		OverallCoverage:   70.0,
		DomainCoverages: []models.DomainCoverage{
			{
				Domain:        "code",
				TotalFiles:    200,
				IndexedFiles:  160,
				IndexedChunks: 950,
				CoverageRate:  80.0,
				UnindexedImportantFiles: []string{
					"pkg/core/README.md",
				},
			},
			{
				Domain:        "tests",
				TotalFiles:    100,
				IndexedFiles:  15, // テストカバレッジが低い
				IndexedChunks: 75,
				CoverageRate:  15.0,
				UnindexedImportantFiles: []string{
					"tests/README.md",
				},
			},
			{
				Domain:        "architecture",
				TotalFiles:    30,
				IndexedFiles:  25,
				IndexedChunks: 150,
				CoverageRate:  83.33,
				UnindexedImportantFiles: []string{
					"README.md", // ルートREADME
					"docs/adr/ADR-001.md",
					"docs/adr/ADR-005.md",
				},
			},
			{
				Domain:        "ops",
				TotalFiles:    15,
				IndexedFiles:  12,
				IndexedChunks: 60,
				CoverageRate:  80.0,
			},
			{
				Domain:        "infra",
				TotalFiles:    5,
				IndexedFiles:  3,
				IndexedChunks: 15,
				CoverageRate:  60.0,
			},
		},
		GeneratedAt: time.Now(),
	}
}

// printCoverageMap はカバレッジマップの内容を表示します
func printCoverageMap(cm *models.CoverageMap) {
	fmt.Printf("スナップショットID: %s\n", cm.SnapshotID)
	fmt.Printf("バージョン: %s\n", cm.SnapshotVersion)
	fmt.Printf("総ファイル数: %d\n", cm.TotalFiles)
	fmt.Printf("インデックス済みファイル数: %d\n", cm.TotalIndexedFiles)
	fmt.Printf("総チャンク数: %d\n", cm.TotalChunks)
	fmt.Printf("全体カバレッジ率: %.2f%%\n", cm.OverallCoverage)
	fmt.Println("\nドメイン別カバレッジ:")
	for _, dc := range cm.DomainCoverages {
		fmt.Printf("  - %s: %.2f%% (%d/%d ファイル, %d チャンク)\n",
			dc.Domain, dc.CoverageRate, dc.IndexedFiles, dc.TotalFiles, dc.IndexedChunks)
		if len(dc.UnindexedImportantFiles) > 0 {
			fmt.Printf("    未インデックス重要ファイル: %v\n", dc.UnindexedImportantFiles)
		}
	}
}
