package quality

import (
	"context"

	"github.com/jinford/dev-rag/pkg/models"
)

// FreshnessMonitorAdapter は FreshnessMonitor を FreshnessCalculatorInterface に適合させるアダプターです
type FreshnessMonitorAdapter struct {
	monitor   *FreshnessMonitor
	threshold int
}

// NewFreshnessMonitorAdapter は新しい FreshnessMonitorAdapter を作成します
func NewFreshnessMonitorAdapter(monitor *FreshnessMonitor, threshold int) *FreshnessMonitorAdapter {
	return &FreshnessMonitorAdapter{
		monitor:   monitor,
		threshold: threshold,
	}
}

// CalculateFreshness はチャンク鮮度を計算します
func (a *FreshnessMonitorAdapter) CalculateFreshness(threshold int) (*models.FreshnessReport, error) {
	return a.monitor.GenerateFreshnessReport(context.Background(), threshold)
}
