package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// ImportanceServiceConfig はImportanceServiceの設定を表します
type ImportanceServiceConfig struct {
	// EditFrequencyDays は編集頻度を集計する日数（デフォルト: 90日）
	EditFrequencyDays int
	// Weights はスコア計算の重み設定（nilの場合はデフォルト値）
	Weights *domain.ScoreWeights
}

// DefaultImportanceServiceConfig はデフォルトの設定を返します
func DefaultImportanceServiceConfig() ImportanceServiceConfig {
	return ImportanceServiceConfig{
		EditFrequencyDays: 90,
		Weights:           nil, // nilの場合はDefaultWeights()が使用される
	}
}

// ImportanceService は重要度スコアの計算・保存を統合管理します
type ImportanceService struct {
	config              ImportanceServiceConfig
	chunkRepo           domain.ChunkWriter
	gitHistoryProvider  domain.GitHistoryProvider
	depGraphProvider    domain.DependencyGraphProvider
}

// NewImportanceService は新しいImportanceServiceを作成します
func NewImportanceService(
	chunkRepo domain.ChunkWriter,
	gitHistoryProvider domain.GitHistoryProvider,
	depGraphProvider domain.DependencyGraphProvider,
	config *ImportanceServiceConfig,
) *ImportanceService {
	cfg := DefaultImportanceServiceConfig()
	if config != nil {
		cfg = *config
	}

	return &ImportanceService{
		config:             cfg,
		chunkRepo:          chunkRepo,
		gitHistoryProvider: gitHistoryProvider,
		depGraphProvider:   depGraphProvider,
	}
}

// CalculateAndSaveScores は依存グラフとGitリポジトリから重要度スコアを計算し、DBに保存します
func (s *ImportanceService) CalculateAndSaveScores(ctx context.Context, snapshotID uuid.UUID, repoPath, ref string) error {
	// 1. 依存グラフを読み込む
	graph, err := s.depGraphProvider.LoadGraphBySnapshot(ctx, snapshotID)
	if err != nil {
		return fmt.Errorf("failed to load dependency graph: %w", err)
	}

	// 2. Git履歴から編集頻度を取得
	since := time.Now().AddDate(0, 0, -s.config.EditFrequencyDays)
	editHistory, err := s.gitHistoryProvider.GetFileEditFrequencies(ctx, repoPath, ref, since)
	if err != nil {
		return fmt.Errorf("failed to get file edit frequencies: %w", err)
	}

	// 3. ImportanceCalculator を作成
	calculator := domain.NewImportanceCalculator(graph, editHistory, s.config.Weights)

	// 4. 全チャンクのスコアを計算
	scores, err := calculator.CalculateAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to calculate scores: %w", err)
	}

	// 5. スコアをDBに保存
	scoreMap := make(map[uuid.UUID]float64)
	for chunkID, score := range scores {
		scoreMap[chunkID] = score.FinalScore
	}

	if err := s.chunkRepo.BatchUpdateImportanceScores(ctx, scoreMap); err != nil {
		return fmt.Errorf("failed to save scores: %w", err)
	}

	return nil
}

// GetScoreDetails は単一チャンクのスコア詳細を取得します（デバッグ用）
func (s *ImportanceService) GetScoreDetails(ctx context.Context, snapshotID uuid.UUID, repoPath, ref string, chunkID uuid.UUID) (*domain.ChunkScore, error) {
	// 1. 依存グラフを読み込む
	graph, err := s.depGraphProvider.LoadGraphBySnapshot(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to load dependency graph: %w", err)
	}

	// 2. Git履歴から編集頻度を取得
	since := time.Now().AddDate(0, 0, -s.config.EditFrequencyDays)
	editHistory, err := s.gitHistoryProvider.GetFileEditFrequencies(ctx, repoPath, ref, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get file edit frequencies: %w", err)
	}

	// 3. ImportanceCalculator を作成
	calculator := domain.NewImportanceCalculator(graph, editHistory, s.config.Weights)

	// 4. スコアを計算
	return calculator.Calculate(ctx, chunkID)
}
