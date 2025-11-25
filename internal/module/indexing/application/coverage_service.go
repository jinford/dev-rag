package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// CoverageServiceConfig はCoverageServiceの設定を表します
type CoverageServiceConfig struct {
	AlertConfig *domain.AlertConfig
}

// DefaultCoverageServiceConfig はデフォルトの設定を返します
func DefaultCoverageServiceConfig() CoverageServiceConfig {
	return CoverageServiceConfig{
		AlertConfig: domain.DefaultAlertConfig(),
	}
}

// CoverageService はカバレッジマップの構築とアラート生成を統合管理します
type CoverageService struct {
	config           CoverageServiceConfig
	snapshotFileRepo domain.SnapshotFileReader
}

// NewCoverageService は新しいCoverageServiceを作成します
func NewCoverageService(
	snapshotFileRepo domain.SnapshotFileReader,
	config *CoverageServiceConfig,
) *CoverageService {
	cfg := DefaultCoverageServiceConfig()
	if config != nil {
		cfg = *config
	}

	return &CoverageService{
		config:           cfg,
		snapshotFileRepo: snapshotFileRepo,
	}
}

// Config は現在のサービス設定を返します
func (s *CoverageService) Config() CoverageServiceConfig {
	return s.config
}

// BuildCoverageMap はスナップショットIDからカバレッジマップを構築します
func (s *CoverageService) BuildCoverageMap(ctx context.Context, snapshotID uuid.UUID, snapshotVersion string) (*domain.CoverageMap, error) {
	// 1. ドメイン別統計を取得（I/O）
	domainStats, err := s.snapshotFileRepo.GetDomainCoverageStats(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain coverage stats: %w", err)
	}

	// 2. 未インデックス重要ファイルを取得（I/O）
	unindexedFiles, err := s.snapshotFileRepo.GetUnindexedImportantFiles(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unindexed important files: %w", err)
	}

	// 3. 各ドメインに未インデックス重要ファイルを追加
	// （簡易実装: 全ドメインに追加。実際の実装では、各ファイルのドメインを取得してフィルタリングする）
	if len(unindexedFiles) > 0 {
		for i := range domainStats {
			domainStats[i].UnindexedImportantFiles = unindexedFiles
		}
	}

	// 4. 純粋計算でカバレッジマップを構築（domain層）
	coverageMap := domain.BuildCoverageMap(snapshotID, snapshotVersion, domainStats)

	return coverageMap, nil
}

// ExportToJSON はカバレッジマップをJSON形式でエクスポートします
func (s *CoverageService) ExportToJSON(coverageMap *domain.CoverageMap) ([]byte, error) {
	jsonData, err := json.MarshalIndent(coverageMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal coverage map to JSON: %w", err)
	}
	return jsonData, nil
}

// GenerateAlerts はカバレッジマップからアラートを生成します
func (s *CoverageService) GenerateAlerts(ctx context.Context, snapshotID uuid.UUID, coverageMap *domain.CoverageMap) ([]domain.Alert, error) {
	var alerts []domain.Alert

	// 1. 重要READMEが未インデックスかチェック（I/O）
	unindexedFiles, err := s.snapshotFileRepo.GetUnindexedImportantFiles(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unindexed important files: %w", err)
	}

	// 2. README アラートを生成（純粋計算）
	readmeAlerts := domain.GenerateReadmeAlerts(unindexedFiles, s.config.AlertConfig)
	alerts = append(alerts, readmeAlerts...)

	// 3. ADRドキュメントのカバレッジをチェック（I/O）
	adrAlert, err := s.checkADRCoverage(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to check ADR coverage: %w", err)
	}
	if adrAlert != nil {
		alerts = append(alerts, *adrAlert)
	}

	// 4. テストコードのカバレッジ率をチェック（純粋計算）
	testAlert := domain.GenerateTestCoverageAlert(coverageMap, s.config.AlertConfig)
	if testAlert != nil {
		alerts = append(alerts, *testAlert)
	}

	return alerts, nil
}

// checkADRCoverage はADRドキュメントのカバレッジをチェックします
func (s *CoverageService) checkADRCoverage(ctx context.Context, snapshotID uuid.UUID) (*domain.Alert, error) {
	// 1. スナップショット配下の全ファイルを取得（I/O）
	snapshotFiles, err := s.snapshotFileRepo.GetBySnapshot(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot files: %w", err)
	}

	// 2. ADRドキュメントの総数とインデックス済み数をカウント（純粋計算）
	totalADRs := 0
	indexedADRs := 0

	for _, sf := range snapshotFiles {
		if domain.IsADRDocument(sf.FilePath) {
			totalADRs++
			if sf.Indexed {
				indexedADRs++
			}
		}
	}

	// 3. アラート生成（純粋計算）
	return domain.GenerateADRCoverageAlert(totalADRs, indexedADRs, s.config.AlertConfig), nil
}
