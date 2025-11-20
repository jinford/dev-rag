package importance

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/indexer/dependency"
	gitprovider "github.com/jinford/dev-rag/pkg/indexer/provider/git"
	"github.com/jinford/dev-rag/pkg/repository"
)

// ServiceConfig はServiceの設定を表します
type ServiceConfig struct {
	// EditFrequencyDays は編集頻度を集計する日数（デフォルト: 90日）
	EditFrequencyDays int
	// Weights はスコア計算の重み設定（nilの場合はデフォルト値）
	Weights *ScoreWeights
}

// DefaultServiceConfig はデフォルトの設定を返します
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		EditFrequencyDays: 90,
		Weights:           nil, // nilの場合はDefaultWeights()が使用される
	}
}

// Service は重要度スコアの計算・保存を統合管理します
type Service struct {
	config     ServiceConfig
	repo       *repository.IndexRepositoryRW
	gitClient  *gitprovider.GitClient
}

// NewService は新しいServiceを作成します
func NewService(repo *repository.IndexRepositoryRW, gitClient *gitprovider.GitClient, config *ServiceConfig) *Service {
	cfg := DefaultServiceConfig()
	if config != nil {
		cfg = *config
	}

	return &Service{
		config:    cfg,
		repo:      repo,
		gitClient: gitClient,
	}
}

// CalculateAndSaveScores は依存グラフとGitリポジトリから重要度スコアを計算し、DBに保存します
func (s *Service) CalculateAndSaveScores(ctx context.Context, graph *dependency.Graph, repoPath, ref string) error {
	// 1. Git履歴から編集頻度を取得
	since := time.Now().AddDate(0, 0, -s.config.EditFrequencyDays)
	gitEditFreqs, err := s.gitClient.GetFileEditFrequencies(ctx, repoPath, ref, since)
	if err != nil {
		return fmt.Errorf("failed to get file edit frequencies: %w", err)
	}

	// 2. editHistory形式に変換
	editHistory := make(map[string]*FileEditHistory)
	for filePath, freq := range gitEditFreqs {
		editHistory[filePath] = &FileEditHistory{
			FilePath:   freq.FilePath,
			EditCount:  freq.EditCount,
			LastEdited: freq.LastEdited,
		}
	}

	// 3. Calculator を作成
	calculator := NewCalculator(graph, editHistory, s.config.Weights)

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

	if err := s.repo.BatchUpdateChunkImportanceScores(ctx, scoreMap); err != nil {
		return fmt.Errorf("failed to save scores: %w", err)
	}

	return nil
}

// GetScoreDetails は単一チャンクのスコア詳細を取得します（デバッグ用）
func (s *Service) GetScoreDetails(ctx context.Context, graph *dependency.Graph, repoPath, ref string, chunkID uuid.UUID) (*ChunkScore, error) {
	// 1. Git履歴から編集頻度を取得
	since := time.Now().AddDate(0, 0, -s.config.EditFrequencyDays)
	gitEditFreqs, err := s.gitClient.GetFileEditFrequencies(ctx, repoPath, ref, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get file edit frequencies: %w", err)
	}

	// 2. editHistory形式に変換
	editHistory := make(map[string]*FileEditHistory)
	for filePath, freq := range gitEditFreqs {
		editHistory[filePath] = &FileEditHistory{
			FilePath:   freq.FilePath,
			EditCount:  freq.EditCount,
			LastEdited: freq.LastEdited,
		}
	}

	// 3. Calculator を作成
	calculator := NewCalculator(graph, editHistory, s.config.Weights)

	// 4. スコアを計算
	return calculator.Calculate(ctx, chunkID)
}
