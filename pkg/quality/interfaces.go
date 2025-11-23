package quality

import (
	"context"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
)

// MetricsCalculatorInterface は品質メトリクスを計算するインターフェースです
type MetricsCalculatorInterface interface {
	CalculateMetrics() (*models.QualityMetrics, error)
}

// FreshnessCalculatorInterface はチャンク鮮度を計算するインターフェースです
type FreshnessCalculatorInterface interface {
	CalculateFreshness(threshold int) (*models.FreshnessReport, error)
}

// ActionBacklogRepositoryInterface はアクションバックログのリポジトリインターフェースです
type ActionBacklogRepositoryInterface interface {
	CreateAction(ctx context.Context, action *models.Action) (*models.Action, error)
	GetActionByID(ctx context.Context, id uuid.UUID) (*models.Action, error)
	GetActionByActionID(ctx context.Context, actionID string) (*models.Action, error)
	ListActions(ctx context.Context, filter *models.ActionFilter) ([]*models.Action, error)
	UpdateActionStatus(ctx context.Context, id uuid.UUID, status string) error
	CompleteAction(ctx context.Context, id uuid.UUID) error
}

// LLMClientInterface はLLMクライアントのインターフェースです
type LLMClientInterface interface {
	GenerateText(ctx context.Context, prompt string) (string, error)
}
