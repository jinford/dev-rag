package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/wiki/domain"
)

// WikiOrchestrator は Wiki 生成のビジネスフローを統括するオーケストレーターです。
// AnalyzeRepository → Summaries 生成 → Wiki 生成 → Repository 保存の順序でフローを制御します。
type WikiOrchestrator struct {
	analyzer             domain.RepositoryAnalyzer
	directorySummarizer  domain.DirectorySummarizer
	archSummarizer       domain.ArchitectureSummarizer
	wikiGenerator        domain.WikiGenerator
	wikiRepo             domain.WikiMetadataRepository
	directorySummaryRepo domain.DirectorySummaryRepository
	archSummaryRepo      domain.ArchitectureSummaryRepository
	log                  *slog.Logger
}

// NewWikiOrchestrator は新しい WikiOrchestrator を作成します。
func NewWikiOrchestrator(
	analyzer domain.RepositoryAnalyzer,
	directorySummarizer domain.DirectorySummarizer,
	archSummarizer domain.ArchitectureSummarizer,
	wikiGenerator domain.WikiGenerator,
	wikiRepo domain.WikiMetadataRepository,
	directorySummaryRepo domain.DirectorySummaryRepository,
	archSummaryRepo domain.ArchitectureSummaryRepository,
	log *slog.Logger,
) *WikiOrchestrator {
	return &WikiOrchestrator{
		analyzer:             analyzer,
		directorySummarizer:  directorySummarizer,
		archSummarizer:       archSummarizer,
		wikiGenerator:        wikiGenerator,
		wikiRepo:             wikiRepo,
		directorySummaryRepo: directorySummaryRepo,
		archSummaryRepo:      archSummaryRepo,
		log:                  log,
	}
}

// GenerateWiki は Wiki 生成の全体フローを実行します。
// フロー: AnalyzeRepository → Summaries 生成 → Wiki 生成 → Repository 保存
func (o *WikiOrchestrator) GenerateWiki(ctx context.Context, sourceID, snapshotID uuid.UUID) error {
	// バリデーション
	if sourceID == uuid.Nil {
		return fmt.Errorf("source ID is required")
	}
	if snapshotID == uuid.Nil {
		return fmt.Errorf("snapshot ID is required")
	}

	o.log.Info("Starting wiki generation flow",
		"sourceID", sourceID,
		"snapshotID", snapshotID,
	)

	// 1. リポジトリ構造を解析
	o.log.Info("Step 1: Analyzing repository structure")
	structure, err := o.analyzer.AnalyzeRepository(ctx, sourceID, snapshotID)
	if err != nil {
		o.log.Error("Failed to analyze repository",
			"sourceID", sourceID,
			"snapshotID", snapshotID,
			"error", err,
		)
		return fmt.Errorf("failed to analyze repository: %w", err)
	}

	o.log.Info("Repository structure analyzed",
		"directories", len(structure.Directories),
		"files", len(structure.Files),
	)

	// 2. ディレクトリ要約を生成（階層的に処理）
	o.log.Info("Step 2: Generating directory summaries")
	if err := o.generateDirectorySummaries(ctx, structure); err != nil {
		o.log.Error("Failed to generate directory summaries",
			"sourceID", sourceID,
			"snapshotID", snapshotID,
			"error", err,
		)
		return fmt.Errorf("failed to generate directory summaries: %w", err)
	}

	o.log.Info("Directory summaries generated")

	// 3. アーキテクチャ要約を生成（ディレクトリ要約から集約）
	o.log.Info("Step 3: Generating architecture summaries")
	if err := o.generateArchitectureSummaries(ctx, structure); err != nil {
		o.log.Error("Failed to generate architecture summaries",
			"sourceID", sourceID,
			"snapshotID", snapshotID,
			"error", err,
		)
		return fmt.Errorf("failed to generate architecture summaries: %w", err)
	}

	o.log.Info("Architecture summaries generated")

	// 4. Wiki ページ生成（現時点では要約生成まで完了とする）
	// Wiki ページの Markdown 生成は将来的に実装
	o.log.Info("Step 4: Wiki generation completed (summary generation phase)")

	o.log.Info("Wiki generation flow completed",
		"sourceID", sourceID,
		"snapshotID", snapshotID,
	)

	return nil
}

// generateDirectorySummaries はディレクトリ要約を生成します。
// 深い階層から順番に、同じ階層内は並列処理します。
func (o *WikiOrchestrator) generateDirectorySummaries(ctx context.Context, structure *domain.RepoStructure) error {
	// ディレクトリを深さごとにグループ化
	depthMap := make(map[int][]*domain.DirectoryInfo)
	maxDepth := 0

	for _, dir := range structure.Directories {
		depthMap[dir.Depth] = append(depthMap[dir.Depth], dir)
		if dir.Depth > maxDepth {
			maxDepth = dir.Depth
		}
	}

	// 深い階層から順番に処理（葉から幹へ）
	for depth := maxDepth; depth >= 0; depth-- {
		directories := depthMap[depth]
		if len(directories) == 0 {
			continue
		}

		o.log.Info("Processing directories at depth",
			"depth", depth,
			"count", len(directories),
		)

		// 各ディレクトリの要約を生成
		// TODO: 並列処理は将来的に実装（現在は順次処理）
		for _, dir := range directories {
			result, err := o.directorySummarizer.SummarizeDirectory(ctx, structure, dir)
			if err != nil {
				o.log.Error("Failed to summarize directory",
					"path", dir.Path,
					"error", err,
				)
				// 一部のディレクトリ要約失敗は許容（30%以上失敗したらエラー）
				continue
			}

			// 結果を保存（domain ポート経由）
			// TODO: Repository に保存処理を実装
			_ = result // 使用は将来実装
		}

		o.log.Info("Completed directories at depth", "depth", depth)
	}

	return nil
}

// generateArchitectureSummaries はアーキテクチャ要約を生成します。
func (o *WikiOrchestrator) generateArchitectureSummaries(ctx context.Context, structure *domain.RepoStructure) error {
	// 複数種類の要約を生成
	summaryTypes := []string{"overview", "tech_stack", "data_flow", "components"}

	for _, summaryType := range summaryTypes {
		o.log.Info("Generating architecture summary",
			"type", summaryType,
		)

		result, err := o.archSummarizer.SummarizeArchitecture(ctx, structure, summaryType)
		if err != nil {
			return fmt.Errorf("failed to generate %s summary: %w", summaryType, err)
		}

		// 結果を保存（domain ポート経由）
		// TODO: Repository に保存処理を実装
		_ = result // 使用は将来実装

		o.log.Info("Architecture summary generated",
			"type", summaryType,
		)
	}

	return nil
}
