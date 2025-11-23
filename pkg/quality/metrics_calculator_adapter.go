package quality

import (
	"context"

	"github.com/jinford/dev-rag/pkg/models"
)

// MetricsCalculatorAdapter は QualityMetricsCalculator を MetricsCalculatorInterface に適合させるアダプターです
type MetricsCalculatorAdapter struct {
	calculator *QualityMetricsCalculator
}

// NewMetricsCalculatorAdapter は新しい MetricsCalculatorAdapter を作成します
func NewMetricsCalculatorAdapter(calculator *QualityMetricsCalculator) *MetricsCalculatorAdapter {
	return &MetricsCalculatorAdapter{
		calculator: calculator,
	}
}

// CalculateMetrics は品質メトリクスを計算します（パラメータなし版）
func (a *MetricsCalculatorAdapter) CalculateMetrics() (*models.QualityMetrics, error) {
	return a.calculator.CalculateMetrics(context.Background(), nil, nil)
}
