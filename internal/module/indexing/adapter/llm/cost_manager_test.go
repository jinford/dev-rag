package llm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestPricingConfig() *PricingConfig {
	return &PricingConfig{
		Models: map[string]ModelPricing{
			"gpt-4o-mini": {
				InputPricePer1kTokens:  0.00015,
				OutputPricePer1kTokens: 0.0006,
				Provider:               "openai",
				Description:            "Test model",
			},
			"gpt-4o": {
				InputPricePer1kTokens:  0.0025,
				OutputPricePer1kTokens: 0.010,
				Provider:               "openai",
				Description:            "Test model 2",
			},
		},
		DefaultModel: "gpt-4o-mini",
	}
}

func TestNewCostManager(t *testing.T) {
	// テスト用の一時設定ファイルを作成
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "llm_pricing.yaml")

	configContent := `
models:
  gpt-4o-mini:
    input_price_per_1k_tokens: 0.00015
    output_price_per_1k_tokens: 0.0006
    provider: openai
    description: "Test model"
default_model: gpt-4o-mini
cost_limits:
  daily_max_cost: 10.0
  warning_threshold: 5.0
  enable_alerts: true
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// CostManagerを作成
	cm, err := NewCostManager(configPath)
	require.NoError(t, err)
	require.NotNil(t, cm)
	assert.Equal(t, "gpt-4o-mini", cm.config.DefaultModel)
	assert.Equal(t, 10.0, cm.config.CostLimits.DailyMaxCost)
}

func TestNewCostManagerWithConfig(t *testing.T) {
	config := createTestPricingConfig()
	cm := NewCostManagerWithConfig(config)

	require.NotNil(t, cm)
	assert.Equal(t, config, cm.config)
}

func TestCostManager_CalculateCost(t *testing.T) {
	config := createTestPricingConfig()
	cm := NewCostManagerWithConfig(config)

	tests := []struct {
		name          string
		model         string
		usage         TokenUsage
		expectedCost  float64
		expectedError bool
	}{
		{
			name:  "gpt-4o-mini basic",
			model: "gpt-4o-mini",
			usage: TokenUsage{
				PromptTokens:   1000,
				ResponseTokens: 500,
			},
			// (1000 * 0.00015 / 1000) + (500 * 0.0006 / 1000) = 0.00015 + 0.0003 = 0.00045
			expectedCost: 0.00045,
		},
		{
			name:  "gpt-4o higher cost",
			model: "gpt-4o",
			usage: TokenUsage{
				PromptTokens:   2000,
				ResponseTokens: 1000,
			},
			// (2000 * 0.0025 / 1000) + (1000 * 0.010 / 1000) = 0.005 + 0.01 = 0.015
			expectedCost: 0.015,
		},
		{
			name:          "unknown model",
			model:         "unknown-model",
			usage:         TokenUsage{PromptTokens: 100, ResponseTokens: 50},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost, err := cm.CalculateCost(tt.model, tt.usage)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.InDelta(t, tt.expectedCost, cost, 0.0001)
			}
		})
	}
}

func TestCostManager_RecordUsage(t *testing.T) {
	config := createTestPricingConfig()
	config.CostLimits.EnableAlerts = false // アラートを無効化
	cm := NewCostManagerWithConfig(config)

	usage := TokenUsage{
		PromptTokens:   1000,
		ResponseTokens: 500,
		TotalTokens:    1500,
	}

	err := cm.RecordUsage("gpt-4o-mini", usage, "test_request")
	require.NoError(t, err)

	// 統計の確認
	assert.Greater(t, cm.GetTotalCost(), 0.0)
	assert.Equal(t, 1, cm.GetRequestCount())

	costs := cm.GetCostsByModel()
	assert.Contains(t, costs, "gpt-4o-mini")
	assert.Greater(t, costs["gpt-4o-mini"], 0.0)

	tokens := cm.GetTokensByModel()
	assert.Contains(t, tokens, "gpt-4o-mini")
	assert.Equal(t, 1000, tokens["gpt-4o-mini"].PromptTokens)
	assert.Equal(t, 500, tokens["gpt-4o-mini"].ResponseTokens)

	requests := cm.GetRequestsByType()
	assert.Equal(t, 1, requests["test_request"])
}

func TestCostManager_MultipleRecords(t *testing.T) {
	config := createTestPricingConfig()
	config.CostLimits.EnableAlerts = false
	cm := NewCostManagerWithConfig(config)

	// 複数回記録
	usage1 := TokenUsage{PromptTokens: 1000, ResponseTokens: 500, TotalTokens: 1500}
	usage2 := TokenUsage{PromptTokens: 2000, ResponseTokens: 1000, TotalTokens: 3000}

	err := cm.RecordUsage("gpt-4o-mini", usage1, "summary")
	require.NoError(t, err)

	err = cm.RecordUsage("gpt-4o", usage2, "classification")
	require.NoError(t, err)

	// 統計の確認
	assert.Equal(t, 2, cm.GetRequestCount())

	costs := cm.GetCostsByModel()
	assert.Len(t, costs, 2)
	assert.Greater(t, costs["gpt-4o-mini"], 0.0)
	assert.Greater(t, costs["gpt-4o"], 0.0)

	requests := cm.GetRequestsByType()
	assert.Equal(t, 1, requests["summary"])
	assert.Equal(t, 1, requests["classification"])
}

func TestCostManager_CostLimits(t *testing.T) {
	config := createTestPricingConfig()
	config.CostLimits.EnableAlerts = true
	config.CostLimits.DailyMaxCost = 0.001 // 非常に低い制限
	config.CostLimits.WarningThreshold = 0.0005
	cm := NewCostManagerWithConfig(config)

	// 大きな使用量でコスト制限を超える
	usage := TokenUsage{
		PromptTokens:   10000,
		ResponseTokens: 5000,
	}

	err := cm.RecordUsage("gpt-4o-mini", usage, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "daily cost limit exceeded")
}

func TestCostManager_Reset(t *testing.T) {
	config := createTestPricingConfig()
	config.CostLimits.EnableAlerts = false
	cm := NewCostManagerWithConfig(config)

	// データを記録
	usage := TokenUsage{PromptTokens: 1000, ResponseTokens: 500}
	err := cm.RecordUsage("gpt-4o-mini", usage, "test")
	require.NoError(t, err)

	assert.Greater(t, cm.GetTotalCost(), 0.0)
	assert.Equal(t, 1, cm.GetRequestCount())

	// リセット
	cm.Reset()

	assert.Equal(t, 0.0, cm.GetTotalCost())
	assert.Equal(t, 0, cm.GetRequestCount())
	assert.Empty(t, cm.GetCostsByModel())
	assert.Empty(t, cm.GetTokensByModel())
	assert.Empty(t, cm.GetRequestsByType())
}

func TestCostManager_GetModelPricing(t *testing.T) {
	config := createTestPricingConfig()
	cm := NewCostManagerWithConfig(config)

	pricing, err := cm.GetModelPricing("gpt-4o-mini")
	require.NoError(t, err)
	assert.Equal(t, 0.00015, pricing.InputPricePer1kTokens)
	assert.Equal(t, 0.0006, pricing.OutputPricePer1kTokens)

	_, err = cm.GetModelPricing("unknown-model")
	assert.Error(t, err)
}

func TestCostManager_ConcurrentAccess(t *testing.T) {
	config := createTestPricingConfig()
	config.CostLimits.EnableAlerts = false
	cm := NewCostManagerWithConfig(config)

	// 並行アクセスのテスト
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			usage := TokenUsage{PromptTokens: 100, ResponseTokens: 50}
			_ = cm.RecordUsage("gpt-4o-mini", usage, "concurrent")
			done <- true
		}()
	}

	// 全ての goroutine の完了を待つ
	for i := 0; i < 10; i++ {
		<-done
	}

	// 結果の確認
	assert.Equal(t, 10, cm.GetRequestCount())
	tokens := cm.GetTokensByModel()
	assert.Equal(t, 1000, tokens["gpt-4o-mini"].PromptTokens)
}
