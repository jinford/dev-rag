package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// ModelPricing はモデルごとの価格情報
type ModelPricing struct {
	InputPricePer1kTokens  float64 `yaml:"input_price_per_1k_tokens"`
	OutputPricePer1kTokens float64 `yaml:"output_price_per_1k_tokens"`
	Provider               string  `yaml:"provider"`
	Description            string  `yaml:"description"`
}

// PricingConfig は価格設定ファイルの構造
type PricingConfig struct {
	Models       map[string]ModelPricing `yaml:"models"`
	DefaultModel string                  `yaml:"default_model"`
	CostLimits   struct {
		DailyMaxCost     float64 `yaml:"daily_max_cost"`
		WarningThreshold float64 `yaml:"warning_threshold"`
		EnableAlerts     bool    `yaml:"enable_alerts"`
	} `yaml:"cost_limits"`
}

// CostManager はLLM APIのコストを管理する
type CostManager struct {
	mu             sync.RWMutex
	config         *PricingConfig
	totalCost      float64
	costsByModel   map[string]float64
	tokensByModel  map[string]TokenUsage
	requestCount   int
	requestsByType map[string]int
}

// NewCostManager は新しいCostManagerを作成する
func NewCostManager(configPath string) (*CostManager, error) {
	// 設定ファイルがない場合はデフォルトパスを使用
	if configPath == "" {
		// プロジェクトルートのconfigディレクトリを探す
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		configPath = filepath.Join(wd, "config", "llm_pricing.yaml")
	}

	// 設定ファイルを読み込む
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pricing config: %w", err)
	}

	var config PricingConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse pricing config: %w", err)
	}

	return &CostManager{
		config:         &config,
		costsByModel:   make(map[string]float64),
		tokensByModel:  make(map[string]TokenUsage),
		requestsByType: make(map[string]int),
	}, nil
}

// NewCostManagerWithConfig は設定を直接指定してCostManagerを作成する
func NewCostManagerWithConfig(config *PricingConfig) *CostManager {
	return &CostManager{
		config:         config,
		costsByModel:   make(map[string]float64),
		tokensByModel:  make(map[string]TokenUsage),
		requestsByType: make(map[string]int),
	}
}

// CalculateCost はトークン使用量からコストを計算する
func (cm *CostManager) CalculateCost(model string, usage TokenUsage) (float64, error) {
	cm.mu.RLock()
	pricing, ok := cm.config.Models[model]
	cm.mu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("pricing not found for model: %s", model)
	}

	// 入力トークンのコスト (1000トークンあたりの価格)
	inputCost := float64(usage.PromptTokens) / 1000.0 * pricing.InputPricePer1kTokens

	// 出力トークンのコスト
	outputCost := float64(usage.ResponseTokens) / 1000.0 * pricing.OutputPricePer1kTokens

	return inputCost + outputCost, nil
}

// RecordUsage は使用量とコストを記録する
func (cm *CostManager) RecordUsage(model string, usage TokenUsage, requestType string) error {
	cost, err := cm.CalculateCost(model, usage)
	if err != nil {
		return err
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 総コストを更新
	cm.totalCost += cost

	// モデル別コストを更新
	cm.costsByModel[model] += cost

	// モデル別トークン数を更新
	existingUsage := cm.tokensByModel[model]
	existingUsage.PromptTokens += usage.PromptTokens
	existingUsage.ResponseTokens += usage.ResponseTokens
	existingUsage.TotalTokens += usage.TotalTokens
	cm.tokensByModel[model] = existingUsage

	// リクエスト数を更新
	cm.requestCount++
	cm.requestsByType[requestType]++

	// コスト制限チェック
	if cm.config.CostLimits.EnableAlerts {
		if cm.totalCost >= cm.config.CostLimits.DailyMaxCost {
			return fmt.Errorf("daily cost limit exceeded: $%.4f >= $%.4f", cm.totalCost, cm.config.CostLimits.DailyMaxCost)
		}
		if cm.totalCost >= cm.config.CostLimits.WarningThreshold && cm.totalCost < cm.config.CostLimits.DailyMaxCost {
			// 警告のみ（エラーは返さない）
			fmt.Printf("WARNING: Cost threshold reached: $%.4f >= $%.4f\n", cm.totalCost, cm.config.CostLimits.WarningThreshold)
		}
	}

	return nil
}

// GetTotalCost は総コストを返す
func (cm *CostManager) GetTotalCost() float64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.totalCost
}

// GetCostsByModel はモデル別のコストを返す
func (cm *CostManager) GetCostsByModel() map[string]float64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// コピーを返す
	result := make(map[string]float64)
	for k, v := range cm.costsByModel {
		result[k] = v
	}
	return result
}

// GetTokensByModel はモデル別のトークン使用量を返す
func (cm *CostManager) GetTokensByModel() map[string]TokenUsage {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// コピーを返す
	result := make(map[string]TokenUsage)
	for k, v := range cm.tokensByModel {
		result[k] = v
	}
	return result
}

// GetRequestCount は総リクエスト数を返す
func (cm *CostManager) GetRequestCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.requestCount
}

// GetRequestsByType はリクエストタイプ別の回数を返す
func (cm *CostManager) GetRequestsByType() map[string]int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// コピーを返す
	result := make(map[string]int)
	for k, v := range cm.requestsByType {
		result[k] = v
	}
	return result
}

// Reset は統計をリセットする
func (cm *CostManager) Reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.totalCost = 0
	cm.costsByModel = make(map[string]float64)
	cm.tokensByModel = make(map[string]TokenUsage)
	cm.requestCount = 0
	cm.requestsByType = make(map[string]int)
}

// GetModelPricing はモデルの価格情報を返す
func (cm *CostManager) GetModelPricing(model string) (ModelPricing, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	pricing, ok := cm.config.Models[model]
	if !ok {
		return ModelPricing{}, fmt.Errorf("pricing not found for model: %s", model)
	}
	return pricing, nil
}
