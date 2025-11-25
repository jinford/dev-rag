package coverage

import (
	"context"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/application"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	indexingpg "github.com/jinford/dev-rag/internal/module/indexing/adapter/pg"
)

// AlertConfig はアラート条件の設定を表します（後方互換性のため残す）
// 新規実装では domain.AlertConfig を使用してください
type AlertConfig = domain.AlertConfig

// DefaultAlertConfig はデフォルトのアラート設定を返します（後方互換性のため残す）
// 新規実装では domain.DefaultAlertConfig() を使用してください
func DefaultAlertConfig() *AlertConfig {
	return domain.DefaultAlertConfig()
}

// AlertGenerator はカバレッジに基づいてアラートを生成します
// 内部的にapplication.CoverageServiceに委譲します
type AlertGenerator struct {
	coverageService *application.CoverageService
}

// NewAlertGenerator は新しいAlertGeneratorを作成します
func NewAlertGenerator(indexRepo *indexingpg.IndexRepositoryR, config *AlertConfig) *AlertGenerator {
	cfg := &application.CoverageServiceConfig{
		AlertConfig: config,
	}
	return &AlertGenerator{
		coverageService: application.NewCoverageService(indexRepo, cfg),
	}
}

// NewAlertGeneratorWithDefaults はデフォルト設定でAlertGeneratorを作成します
func NewAlertGeneratorWithDefaults(indexRepo *indexingpg.IndexRepositoryR) *AlertGenerator {
	return NewAlertGenerator(indexRepo, DefaultAlertConfig())
}

// Config は現在のアラート設定を返します
func (ag *AlertGenerator) Config() *AlertConfig {
	// application層のConfigを返す
	return ag.coverageService.Config().AlertConfig
}

// GenerateAlerts はカバレッジマップからアラートを生成します
// application.CoverageServiceに委譲します
func (ag *AlertGenerator) GenerateAlerts(ctx context.Context, snapshotID uuid.UUID, coverageMap *domain.CoverageMap) ([]domain.Alert, error) {
	return ag.coverageService.GenerateAlerts(ctx, snapshotID, coverageMap)
}

