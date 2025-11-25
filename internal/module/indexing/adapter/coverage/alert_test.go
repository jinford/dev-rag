package coverage

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultAlertConfig(t *testing.T) {
	config := DefaultAlertConfig()

	assert.True(t, config.EnableMissingReadmeAlert)
	assert.Equal(t, 10, config.ADRTotalThreshold)
	assert.Equal(t, 5, config.ADRIndexedThreshold)
	assert.Equal(t, 20.0, config.TestCoverageThreshold)
}

func TestGenerateTestCoverageAlert(t *testing.T) {
	tests := []struct {
		name           string
		coverageMap    *domain.CoverageMap
		config         *AlertConfig
		expectedAlerts bool
	}{
		{
			name: "テストカバレッジが閾値以上の場合アラートなし",
			coverageMap: &domain.CoverageMap{
				SnapshotID:      uuid.New().String(),
				SnapshotVersion: "test-v1",
				DomainCoverages: []domain.DomainCoverage{
					{
						Domain:       "tests",
						TotalFiles:   100,
						IndexedFiles: 50,
						CoverageRate: 50.0,
					},
				},
				GeneratedAt: time.Now(),
			},
			config: &AlertConfig{
				TestCoverageThreshold: 20.0,
			},
			expectedAlerts: false,
		},
		{
			name: "テストカバレッジが閾値未満の場合アラート生成",
			coverageMap: &domain.CoverageMap{
				SnapshotID:      uuid.New().String(),
				SnapshotVersion: "test-v1",
				DomainCoverages: []domain.DomainCoverage{
					{
						Domain:       "tests",
						TotalFiles:   100,
						IndexedFiles: 10,
						CoverageRate: 10.0,
					},
				},
				GeneratedAt: time.Now(),
			},
			config: &AlertConfig{
				TestCoverageThreshold: 20.0,
			},
			expectedAlerts: true,
		},
		{
			name: "testsドメインが存在しない場合アラートなし",
			coverageMap: &domain.CoverageMap{
				SnapshotID:      uuid.New().String(),
				SnapshotVersion: "test-v1",
				DomainCoverages: []domain.DomainCoverage{
					{
						Domain:       "code",
						TotalFiles:   100,
						IndexedFiles: 80,
						CoverageRate: 80.0,
					},
				},
				GeneratedAt: time.Now(),
			},
			config: &AlertConfig{
				TestCoverageThreshold: 20.0,
			},
			expectedAlerts: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert := domain.GenerateTestCoverageAlert(tt.coverageMap, tt.config)

			if tt.expectedAlerts {
				require.NotNil(t, alert)
				assert.Equal(t, domain.AlertSeverityWarning, alert.Severity)
				assert.Equal(t, "tests", alert.Domain)
				assert.NotNil(t, alert.Details)
				assert.Contains(t, alert.Message, "テストコードのカバレッジ率が低すぎます")

				// Details の検証
				details, ok := alert.Details.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, tt.config.TestCoverageThreshold, details["threshold"])
			} else {
				assert.Nil(t, alert)
			}
		})
	}
}

func TestAlertSeverity(t *testing.T) {
	// AlertSeverityの定数が正しく定義されているかテスト
	assert.Equal(t, domain.AlertSeverity("warning"), domain.AlertSeverityWarning)
	assert.Equal(t, domain.AlertSeverity("error"), domain.AlertSeverityError)
}

func TestAlert_Structure(t *testing.T) {
	// Alertの構造体テスト
	alert := domain.Alert{
		Severity: domain.AlertSeverityWarning,
		Message:  "テストメッセージ",
		Domain:   "tests",
		Details: map[string]interface{}{
			"key": "value",
		},
		GeneratedAt: time.Now(),
	}

	assert.Equal(t, domain.AlertSeverityWarning, alert.Severity)
	assert.Equal(t, "テストメッセージ", alert.Message)
	assert.Equal(t, "tests", alert.Domain)
	assert.NotNil(t, alert.Details)
	assert.NotZero(t, alert.GeneratedAt)
}

func TestNewAlertGenerator(t *testing.T) {
	// デフォルト設定でのインスタンス生成
	ag := NewAlertGeneratorWithDefaults(nil)
	assert.NotNil(t, ag)
	assert.NotNil(t, ag.Config())
	assert.Equal(t, DefaultAlertConfig().ADRTotalThreshold, ag.Config().ADRTotalThreshold)

	// カスタム設定でのインスタンス生成
	customConfig := &AlertConfig{
		EnableMissingReadmeAlert: false,
		ADRTotalThreshold:        20,
		ADRIndexedThreshold:      10,
		TestCoverageThreshold:    30.0,
	}
	ag2 := NewAlertGenerator(nil, customConfig)
	assert.NotNil(t, ag2)
	assert.Equal(t, customConfig, ag2.Config())
	assert.False(t, ag2.Config().EnableMissingReadmeAlert)
	assert.Equal(t, 20, ag2.Config().ADRTotalThreshold)
}

func TestAlertPrinter_Print(t *testing.T) {
	// AlertPrinterのテスト
	// 標準出力のキャプチャは難しいので、パニックしないことのみ確認

	// アラートなしの場合
	t.Run("アラートなしの場合", func(t *testing.T) {
		printer := NewAlertPrinter(&mockWriter{})
		assert.NotPanics(t, func() {
			printer.Print([]domain.Alert{})
		})
	})

	// アラートありの場合
	t.Run("アラートありの場合", func(t *testing.T) {
		alerts := []domain.Alert{
			{
				Severity:    domain.AlertSeverityWarning,
				Message:     "テストアラート1",
				Domain:      "tests",
				GeneratedAt: time.Now(),
			},
			{
				Severity:    domain.AlertSeverityError,
				Message:     "テストアラート2",
				Domain:      "architecture",
				GeneratedAt: time.Now(),
			},
		}

		printer := NewAlertPrinter(&mockWriter{})
		assert.NotPanics(t, func() {
			printer.Print(alerts)
		})
	})
}

// mockWriter はテスト用のio.Writer実装
type mockWriter struct {
	content []byte
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	m.content = append(m.content, p...)
	return len(p), nil
}

func TestAlertConfig_CustomThresholds(t *testing.T) {
	// カスタム閾値の設定テスト
	config := &AlertConfig{
		EnableMissingReadmeAlert: true,
		ADRTotalThreshold:        15,
		ADRIndexedThreshold:      8,
		TestCoverageThreshold:    25.0,
	}

	assert.True(t, config.EnableMissingReadmeAlert)
	assert.Equal(t, 15, config.ADRTotalThreshold)
	assert.Equal(t, 8, config.ADRIndexedThreshold)
	assert.Equal(t, 25.0, config.TestCoverageThreshold)
}

// モックテスト: 実際のrepository層を使わないテスト
func TestGenerateTestCoverageAlert_EdgeCases(t *testing.T) {
	config := DefaultAlertConfig()

	t.Run("カバレッジ率が0%の場合", func(t *testing.T) {
		coverageMap := &domain.CoverageMap{
			DomainCoverages: []domain.DomainCoverage{
				{
					Domain:       "tests",
					TotalFiles:   100,
					IndexedFiles: 0,
					CoverageRate: 0.0,
				},
			},
		}

		alert := domain.GenerateTestCoverageAlert(coverageMap, config)
		assert.NotNil(t, alert)
		assert.Equal(t, domain.AlertSeverityWarning, alert.Severity)
	})

	t.Run("カバレッジ率が100%の場合", func(t *testing.T) {
		coverageMap := &domain.CoverageMap{
			DomainCoverages: []domain.DomainCoverage{
				{
					Domain:       "tests",
					TotalFiles:   100,
					IndexedFiles: 100,
					CoverageRate: 100.0,
				},
			},
		}

		alert := domain.GenerateTestCoverageAlert(coverageMap, config)
		assert.Nil(t, alert)
	})

	t.Run("複数ドメインが存在する場合", func(t *testing.T) {
		coverageMap := &domain.CoverageMap{
			DomainCoverages: []domain.DomainCoverage{
				{
					Domain:       "code",
					TotalFiles:   200,
					IndexedFiles: 180,
					CoverageRate: 90.0,
				},
				{
					Domain:       "tests",
					TotalFiles:   50,
					IndexedFiles: 5,
					CoverageRate: 10.0,
				},
				{
					Domain:       "architecture",
					TotalFiles:   30,
					IndexedFiles: 25,
					CoverageRate: 83.33,
				},
			},
		}

		alert := domain.GenerateTestCoverageAlert(coverageMap, config)
		assert.NotNil(t, alert)
		assert.Equal(t, "tests", alert.Domain)
	})
}

func TestGenerateTestCoverageAlert_MessageFormat(t *testing.T) {
	// アラートメッセージのフォーマットテスト
	config := DefaultAlertConfig()

	t.Run("テストカバレッジアラートのメッセージ", func(t *testing.T) {
		coverageMap := &domain.CoverageMap{
			DomainCoverages: []domain.DomainCoverage{
				{
					Domain:       "tests",
					TotalFiles:   100,
					IndexedFiles: 15,
					CoverageRate: 15.0,
				},
			},
		}

		alert := domain.GenerateTestCoverageAlert(coverageMap, config)
		require.NotNil(t, alert)

		assert.Contains(t, alert.Message, "テストコードのカバレッジ率が低すぎます")
		assert.Contains(t, alert.Message, "15.00%")
		assert.Contains(t, alert.Message, "20.00%")

		details, ok := alert.Details.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 15.0, details["coverage_rate"])
		assert.Equal(t, 20.0, details["threshold"])
		assert.Equal(t, 100, details["total_files"])
		assert.Equal(t, 15, details["indexed_files"])
		assert.Equal(t, 85, details["unindexed_files"])
	})
}

func TestAlertSeverity_Values(t *testing.T) {
	// AlertSeverityの値が正しいことを確認
	assert.Equal(t, "warning", string(domain.AlertSeverityWarning))
	assert.Equal(t, "error", string(domain.AlertSeverityError))
}

func TestAlertConfig_Validation(t *testing.T) {
	// AlertConfigの設定値の妥当性テスト
	t.Run("負の閾値は許容されない想定", func(t *testing.T) {
		// 実際のバリデーションロジックは実装していないが、
		// 将来的に追加する場合のテストプレースホルダー
		config := &AlertConfig{
			ADRTotalThreshold:     -1,
			ADRIndexedThreshold:   -1,
			TestCoverageThreshold: -10.0,
		}
		// 現時点ではバリデーションなしなので、設定できることを確認
		assert.NotNil(t, config)
	})

	t.Run("極端に高い閾値", func(t *testing.T) {
		config := &AlertConfig{
			ADRTotalThreshold:     1000000,
			ADRIndexedThreshold:   999999,
			TestCoverageThreshold: 99.99,
		}
		assert.NotNil(t, config)
	})
}

func TestGenerateTestCoverageAlert_MultipleDomainsCase(t *testing.T) {
	// 複数のドメインが存在する場合のテスト
	config := &AlertConfig{
		EnableMissingReadmeAlert: true,
		ADRTotalThreshold:        10,
		ADRIndexedThreshold:      5,
		TestCoverageThreshold:    20.0,
	}

	coverageMap := &domain.CoverageMap{
		SnapshotID:      uuid.New().String(),
		SnapshotVersion: "test-v1",
		DomainCoverages: []domain.DomainCoverage{
			{
				Domain:       "tests",
				TotalFiles:   100,
				IndexedFiles: 10,
				CoverageRate: 10.0, // 閾値未満
			},
			{
				Domain:       "code",
				TotalFiles:   200,
				IndexedFiles: 150,
				CoverageRate: 75.0,
			},
		},
		GeneratedAt: time.Now(),
	}

	// テストカバレッジアラートのみ生成されることを確認
	alert := domain.GenerateTestCoverageAlert(coverageMap, config)
	assert.NotNil(t, alert)
	assert.Equal(t, "tests", alert.Domain)
}
