package coverage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/repository"
)

// AlertConfig はアラート条件の設定を表します
type AlertConfig struct {
	// 重要READMEの未インデックスアラート
	EnableMissingReadmeAlert bool

	// ADRドキュメント数の閾値
	ADRTotalThreshold   int // ADRドキュメントがこの数以上ある場合にチェック
	ADRIndexedThreshold int // インデックス済みADRドキュメントがこの数未満の場合にアラート

	// テストコードのカバレッジ率の閾値
	TestCoverageThreshold float64 // テストコードのカバレッジ率がこの値未満の場合にアラート
}

// DefaultAlertConfig はデフォルトのアラート設定を返します
func DefaultAlertConfig() *AlertConfig {
	return &AlertConfig{
		EnableMissingReadmeAlert: true,
		ADRTotalThreshold:        10,
		ADRIndexedThreshold:      5,
		TestCoverageThreshold:    20.0,
	}
}

// AlertGenerator はカバレッジに基づいてアラートを生成します
type AlertGenerator struct {
	indexRepo *repository.IndexRepositoryR
	config    *AlertConfig
}

// NewAlertGenerator は新しいAlertGeneratorを作成します
func NewAlertGenerator(indexRepo *repository.IndexRepositoryR, config *AlertConfig) *AlertGenerator {
	if config == nil {
		config = DefaultAlertConfig()
	}
	return &AlertGenerator{
		indexRepo: indexRepo,
		config:    config,
	}
}

// Config は現在のアラート設定を返します
func (ag *AlertGenerator) Config() *AlertConfig {
	return ag.config
}

// GenerateAlerts はカバレッジマップからアラートを生成します
func (ag *AlertGenerator) GenerateAlerts(ctx context.Context, snapshotID uuid.UUID, coverageMap *models.CoverageMap) ([]models.Alert, error) {
	var alerts []models.Alert

	// 1. 重要READMEが未インデックスかチェック
	if ag.config.EnableMissingReadmeAlert {
		readmeAlerts, err := ag.checkMissingImportantReadmes(ctx, snapshotID)
		if err != nil {
			return nil, fmt.Errorf("failed to check missing readmes: %w", err)
		}
		alerts = append(alerts, readmeAlerts...)
	}

	// 2. ADRドキュメントのカバレッジをチェック
	adrAlerts, err := ag.checkADRCoverage(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to check ADR coverage: %w", err)
	}
	alerts = append(alerts, adrAlerts...)

	// 3. テストコードのカバレッジ率をチェック
	testAlerts := ag.checkTestCoverage(coverageMap)
	alerts = append(alerts, testAlerts...)

	return alerts, nil
}

// checkMissingImportantReadmes は重要READMEが未インデックスかチェックします
func (ag *AlertGenerator) checkMissingImportantReadmes(ctx context.Context, snapshotID uuid.UUID) ([]models.Alert, error) {
	var alerts []models.Alert

	// 重要ファイル（README）を取得
	unindexedFiles, err := ag.indexRepo.GetUnindexedImportantFiles(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	// リポジトリルートのREADMEをチェック
	var missingRootReadmes []string
	var missingDirReadmes []string

	for _, file := range unindexedFiles {
		lowerFile := strings.ToLower(file)
		if strings.HasSuffix(lowerFile, "readme.md") || strings.HasSuffix(lowerFile, "readme") {
			// ルートのREADMEか判定（パスに/が含まれていない）
			if !strings.Contains(file, "/") || file == "README.md" || file == "readme.md" {
				missingRootReadmes = append(missingRootReadmes, file)
			} else {
				missingDirReadmes = append(missingDirReadmes, file)
			}
		}
	}

	// ルートREADMEの未インデックスアラート（重要度: error）
	if len(missingRootReadmes) > 0 {
		alerts = append(alerts, models.Alert{
			Severity: models.AlertSeverityError,
			Message:  fmt.Sprintf("リポジトリルートの重要なREADMEファイルがインデックスされていません: %s", strings.Join(missingRootReadmes, ", ")),
			Domain:   "architecture",
			Details: map[string]interface{}{
				"missing_files": missingRootReadmes,
				"file_count":    len(missingRootReadmes),
			},
			GeneratedAt: time.Now(),
		})
	}

	// 主要ディレクトリのREADMEの未インデックスアラート（重要度: warning）
	if len(missingDirReadmes) > 0 {
		alerts = append(alerts, models.Alert{
			Severity: models.AlertSeverityWarning,
			Message:  fmt.Sprintf("主要ディレクトリのREADMEファイルがインデックスされていません: %s", strings.Join(missingDirReadmes, ", ")),
			Domain:   "architecture",
			Details: map[string]interface{}{
				"missing_files": missingDirReadmes,
				"file_count":    len(missingDirReadmes),
			},
			GeneratedAt: time.Now(),
		})
	}

	return alerts, nil
}

// checkADRCoverage はADRドキュメントのカバレッジをチェックします
func (ag *AlertGenerator) checkADRCoverage(ctx context.Context, snapshotID uuid.UUID) ([]models.Alert, error) {
	var alerts []models.Alert

	// ADRドキュメントの総数とインデックス済み数を取得
	totalADRs, indexedADRs, err := ag.countADRDocuments(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	// ADRドキュメントが閾値以上あるのにインデックス数が少ない場合
	if totalADRs >= ag.config.ADRTotalThreshold && indexedADRs < ag.config.ADRIndexedThreshold {
		alerts = append(alerts, models.Alert{
			Severity: models.AlertSeverityWarning,
			Message: fmt.Sprintf(
				"ADRドキュメントが%d件ありますが、%d件しかインデックス化されていません（閾値: %d件以上）",
				totalADRs, indexedADRs, ag.config.ADRIndexedThreshold,
			),
			Domain: "architecture",
			Details: map[string]interface{}{
				"total_adrs":   totalADRs,
				"indexed_adrs": indexedADRs,
				"threshold":    ag.config.ADRIndexedThreshold,
			},
			GeneratedAt: time.Now(),
		})
	}

	return alerts, nil
}

// countADRDocuments はADRドキュメントの総数とインデックス済み数を取得します
func (ag *AlertGenerator) countADRDocuments(ctx context.Context, snapshotID uuid.UUID) (int, int, error) {
	// この実装は簡易版です。実際にはrepository層でADRドキュメントを検出するメソッドを追加する必要があります
	// ここでは、GetUnindexedImportantFilesを使用してADR関連のファイルをカウントします

	unindexedFiles, err := ag.indexRepo.GetUnindexedImportantFiles(ctx, snapshotID)
	if err != nil {
		return 0, 0, err
	}

	// ADRドキュメントの数をカウント
	totalADRs := 0
	unindexedADRs := 0

	for _, file := range unindexedFiles {
		lowerFile := strings.ToLower(file)
		// ADRドキュメントのパターン: /docs/adr/, /docs/design/, /docs/decisions/
		if strings.Contains(lowerFile, "/adr/") ||
			strings.Contains(lowerFile, "/design/") ||
			strings.Contains(lowerFile, "/decisions/") {
			totalADRs++
			unindexedADRs++
		}
	}

	// インデックス済みADR数 = 総ADR数 - 未インデックスADR数
	// ただし、この簡易実装では総ADR数を正確に取得できないため、未インデックスADR数のみを使用
	// 実際の実装では、snapshot_filesテーブルから総数を取得する必要があります
	indexedADRs := 0
	if totalADRs > unindexedADRs {
		indexedADRs = totalADRs - unindexedADRs
	}

	return totalADRs, indexedADRs, nil
}

// checkTestCoverage はテストコードのカバレッジ率をチェックします
func (ag *AlertGenerator) checkTestCoverage(coverageMap *models.CoverageMap) []models.Alert {
	var alerts []models.Alert

	// testsドメインのカバレッジを取得
	for _, dc := range coverageMap.DomainCoverages {
		if dc.Domain == "tests" {
			if dc.CoverageRate < ag.config.TestCoverageThreshold {
				alerts = append(alerts, models.Alert{
					Severity: models.AlertSeverityWarning,
					Message: fmt.Sprintf(
						"テストコードのカバレッジ率が低すぎます: %.2f%% (閾値: %.2f%%以上)",
						dc.CoverageRate, ag.config.TestCoverageThreshold,
					),
					Domain: "tests",
					Details: map[string]interface{}{
						"coverage_rate":    dc.CoverageRate,
						"threshold":        ag.config.TestCoverageThreshold,
						"total_files":      dc.TotalFiles,
						"indexed_files":    dc.IndexedFiles,
						"unindexed_files":  dc.TotalFiles - dc.IndexedFiles,
					},
					GeneratedAt: time.Now(),
				})
			}
			break
		}
	}

	return alerts
}

// PrintAlerts はアラートを標準出力に表示します
func (ag *AlertGenerator) PrintAlerts(alerts []models.Alert) {
	if len(alerts) == 0 {
		fmt.Println("✓ カバレッジアラートはありません")
		return
	}

	fmt.Printf("\n⚠ カバレッジアラート: %d件\n", len(alerts))
	fmt.Println(strings.Repeat("=", 80))

	for i, alert := range alerts {
		severitySymbol := "⚠"
		if alert.Severity == models.AlertSeverityError {
			severitySymbol = "✗"
		}

		fmt.Printf("\n[%d] %s [%s] %s\n", i+1, severitySymbol, alert.Severity, alert.Message)
		if alert.Domain != "" {
			fmt.Printf("    ドメイン: %s\n", alert.Domain)
		}
		if alert.Details != nil {
			fmt.Printf("    詳細: %v\n", alert.Details)
		}
		fmt.Printf("    生成日時: %s\n", alert.GeneratedAt.Format(time.RFC3339))
	}

	fmt.Println(strings.Repeat("=", 80))
}
