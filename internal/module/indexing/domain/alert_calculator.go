package domain

import (
	"fmt"
	"strings"
	"time"
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

// GenerateReadmeAlerts は未インデックスREADMEファイルのアラートを生成します（純粋計算）
func GenerateReadmeAlerts(unindexedFiles []string, config *AlertConfig) []Alert {
	if !config.EnableMissingReadmeAlert {
		return nil
	}

	var alerts []Alert
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
		alerts = append(alerts, Alert{
			Severity: AlertSeverityError,
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
		alerts = append(alerts, Alert{
			Severity: AlertSeverityWarning,
			Message:  fmt.Sprintf("主要ディレクトリのREADMEファイルがインデックスされていません: %s", strings.Join(missingDirReadmes, ", ")),
			Domain:   "architecture",
			Details: map[string]interface{}{
				"missing_files": missingDirReadmes,
				"file_count":    len(missingDirReadmes),
			},
			GeneratedAt: time.Now(),
		})
	}

	return alerts
}

// GenerateADRCoverageAlert はADRドキュメントカバレッジのアラートを生成します（純粋計算）
func GenerateADRCoverageAlert(totalADRs, indexedADRs int, config *AlertConfig) *Alert {
	// ADRドキュメントが閾値以上あるのにインデックス数が少ない場合
	if totalADRs >= config.ADRTotalThreshold && indexedADRs < config.ADRIndexedThreshold {
		return &Alert{
			Severity: AlertSeverityWarning,
			Message: fmt.Sprintf(
				"ADRドキュメントが%d件ありますが、%d件しかインデックス化されていません（閾値: %d件以上）",
				totalADRs, indexedADRs, config.ADRIndexedThreshold,
			),
			Domain: "architecture",
			Details: map[string]interface{}{
				"total_adrs":   totalADRs,
				"indexed_adrs": indexedADRs,
				"threshold":    config.ADRIndexedThreshold,
			},
			GeneratedAt: time.Now(),
		}
	}
	return nil
}

// GenerateTestCoverageAlert はテストコードカバレッジのアラートを生成します（純粋計算）
func GenerateTestCoverageAlert(coverageMap *CoverageMap, config *AlertConfig) *Alert {
	// testsドメインのカバレッジを取得
	for _, dc := range coverageMap.DomainCoverages {
		if dc.Domain == "tests" {
			if dc.CoverageRate < config.TestCoverageThreshold {
				return &Alert{
					Severity: AlertSeverityWarning,
					Message: fmt.Sprintf(
						"テストコードのカバレッジ率が低すぎます: %.2f%% (閾値: %.2f%%以上)",
						dc.CoverageRate, config.TestCoverageThreshold,
					),
					Domain: "tests",
					Details: map[string]interface{}{
						"coverage_rate":   dc.CoverageRate,
						"threshold":       config.TestCoverageThreshold,
						"total_files":     dc.TotalFiles,
						"indexed_files":   dc.IndexedFiles,
						"unindexed_files": dc.TotalFiles - dc.IndexedFiles,
					},
					GeneratedAt: time.Now(),
				}
			}
			break
		}
	}
	return nil
}

// IsADRDocument はファイルパスがADRドキュメントかどうかを判定します（純粋計算）
func IsADRDocument(filePath string) bool {
	lowerPath := strings.ToLower(filePath)
	return strings.Contains(lowerPath, "/adr/") ||
		strings.Contains(lowerPath, "/design/") ||
		strings.Contains(lowerPath, "/decisions/")
}
