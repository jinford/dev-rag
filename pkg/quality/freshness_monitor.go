package quality

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/repository"
)

// FreshnessMonitor はインデックスの鮮度を監視するサービスです
// インデックス鮮度の監視
type FreshnessMonitor struct {
	indexRepo   *repository.IndexRepositoryR
	repoPath    string
	defaultThreshold int // デフォルトの鮮度閾値（日数）
}

// NewFreshnessMonitor は新しい FreshnessMonitor を作成します
func NewFreshnessMonitor(indexRepo *repository.IndexRepositoryR, repoPath string, defaultThreshold int) *FreshnessMonitor {
	if defaultThreshold <= 0 {
		defaultThreshold = 30 // デフォルトは30日
	}
	return &FreshnessMonitor{
		indexRepo:   indexRepo,
		repoPath:    repoPath,
		defaultThreshold: defaultThreshold,
	}
}

// getLatestCommitHash は最新のコミットハッシュを取得します
func (m *FreshnessMonitor) getLatestCommitHash(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", m.repoPath, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get latest commit: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// getCommitDate はコミットの日時を取得します
func (m *FreshnessMonitor) getCommitDate(ctx context.Context, commitHash string) (time.Time, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", m.repoPath, "show", "-s", "--format=%ci", commitHash)
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get commit date: %w", err)
	}

	dateStr := strings.TrimSpace(string(output))
	// Git の日付形式: "2024-01-01 12:00:00 +0900"
	commitDate, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse commit date: %w", err)
	}

	return commitDate, nil
}

// CalculateChunkFreshness はチャンクの鮮度を計算します
func (m *FreshnessMonitor) CalculateChunkFreshness(ctx context.Context, chunkID uuid.UUID, thresholdDays int) (*models.ChunkFreshness, error) {
	// チャンク情報を取得
	chunk, err := m.indexRepo.GetChunkByID(ctx, chunkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk: %w", err)
	}

	if chunk.GitCommitHash == nil || *chunk.GitCommitHash == "" {
		return nil, fmt.Errorf("chunk has no git commit hash")
	}

	// ファイル情報を取得
	file, err := m.indexRepo.GetFileByID(ctx, chunk.FileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	// 最新コミットを取得
	latestCommit, err := m.getLatestCommitHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest commit: %w", err)
	}

	// チャンクのコミット日時を取得
	chunkCommitDate, err := m.getCommitDate(ctx, *chunk.GitCommitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk commit date: %w", err)
	}

	// 最新コミットの日時を取得
	latestCommitDate, err := m.getCommitDate(ctx, latestCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest commit date: %w", err)
	}

	// 鮮度を計算（日数）
	freshnessDays := int(latestCommitDate.Sub(chunkCommitDate).Hours() / 24)
	isStale := freshnessDays > thresholdDays

	return &models.ChunkFreshness{
		ChunkID:       chunk.ID,
		FilePath:      file.Path,
		ChunkKey:      chunk.ChunkKey,
		GitCommitHash: *chunk.GitCommitHash,
		LatestCommit:  latestCommit,
		FreshnessDays: freshnessDays,
		IsStale:       isStale,
		LastUpdated:   chunkCommitDate,
	}, nil
}

// DetectStaleChunks は古いチャンクを検出します
func (m *FreshnessMonitor) DetectStaleChunks(ctx context.Context, thresholdDays int) ([]models.ChunkFreshness, error) {
	if thresholdDays <= 0 {
		thresholdDays = m.defaultThreshold
	}

	// 最新コミットを取得
	latestCommit, err := m.getLatestCommitHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest commit: %w", err)
	}

	latestCommitDate, err := m.getCommitDate(ctx, latestCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest commit date: %w", err)
	}

	// 閾値日時を計算
	thresholdDate := latestCommitDate.AddDate(0, 0, -thresholdDays)

	// 古い可能性のあるチャンクを検出
	// 注: 実際の実装ではsqlcで生成されたGetStaleChunksクエリを使用すべき
	// ここでは仮実装として、簡易的なロジックを記述

	staleChunks := make([]models.ChunkFreshness, 0)

	// 実装のヒント: sqlcで生成されたGetStaleChunksメソッドを呼び出す
	// rows, err := m.indexRepo.q.GetStaleChunks(ctx, int32(thresholdDays))

	// 現時点では空のスライスを返す（sqlc生成後に実装）
	_ = thresholdDate // 未使用変数の警告を避ける

	return staleChunks, nil
}

// GenerateFreshnessReport は鮮度レポートを生成します
func (m *FreshnessMonitor) GenerateFreshnessReport(ctx context.Context, thresholdDays int) (*models.FreshnessReport, error) {
	if thresholdDays <= 0 {
		thresholdDays = m.defaultThreshold
	}

	staleChunks, err := m.DetectStaleChunks(ctx, thresholdDays)
	if err != nil {
		return nil, fmt.Errorf("failed to detect stale chunks: %w", err)
	}

	// 鮮度の平均を計算
	totalFreshness := 0
	for _, chunk := range staleChunks {
		totalFreshness += chunk.FreshnessDays
	}

	avgFreshness := 0.0
	if len(staleChunks) > 0 {
		avgFreshness = float64(totalFreshness) / float64(len(staleChunks))
	}

	return &models.FreshnessReport{
		TotalChunks:          0, // TODO: 実装時に総チャンク数を取得
		StaleChunks:          len(staleChunks),
		AverageFreshnessDays: avgFreshness,
		FreshnessThreshold:   thresholdDays,
		StaleChunkDetails:    staleChunks,
		GeneratedAt:          time.Now(),
	}, nil
}

// GenerateReindexActions は古いチャンクに対する再インデックスアクションを生成します
// 自動再インデックストリガー
func (m *FreshnessMonitor) GenerateReindexActions(ctx context.Context, staleChunks []models.ChunkFreshness) ([]ReindexAction, error) {
	actions := make([]ReindexAction, 0)

	// ファイルパス別にグループ化
	fileGroups := make(map[string][]models.ChunkFreshness)
	for _, chunk := range staleChunks {
		fileGroups[chunk.FilePath] = append(fileGroups[chunk.FilePath], chunk)
	}

	// ファイルごとにアクションを生成
	for filePath, chunks := range fileGroups {
		action := ReindexAction{
			FilePath:      filePath,
			ChunkIDs:      make([]uuid.UUID, 0, len(chunks)),
			Reason:        "stale_chunks_detected",
			ThresholdDays: m.defaultThreshold,
			CreatedAt:     time.Now(),
		}

		for _, chunk := range chunks {
			action.ChunkIDs = append(action.ChunkIDs, chunk.ChunkID)
		}

		actions = append(actions, action)
	}

	return actions, nil
}

// ReindexAction は再インデックスアクションを表します
type ReindexAction struct {
	FilePath      string      `json:"filePath"`
	ChunkIDs      []uuid.UUID `json:"chunkIDs"`
	Reason        string      `json:"reason"`
	ThresholdDays int         `json:"thresholdDays"`
	CreatedAt     time.Time   `json:"createdAt"`
}
