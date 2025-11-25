package summary

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// ArchitectureSummarizer はアーキテクチャ全体の要約を生成する
type ArchitectureSummarizer struct {
	summaryRepo Repository
	llm         LLMClient
	embedder    Embedder
	hasher      *Hasher
	logger      *slog.Logger
}

// NewArchitectureSummarizer は新しいArchitectureSummarizerを作成
func NewArchitectureSummarizer(
	summaryRepo Repository,
	llm LLMClient,
	embedder Embedder,
	logger *slog.Logger,
) *ArchitectureSummarizer {
	return &ArchitectureSummarizer{
		summaryRepo: summaryRepo,
		llm:         llm,
		embedder:    embedder,
		hasher:      NewHasher(),
		logger:      logger,
	}
}

// Generate は4種類のアーキテクチャ要約を生成
func (s *ArchitectureSummarizer) Generate(ctx context.Context, snapshotID uuid.UUID) error {
	// 1. 全ディレクトリ要約を取得
	dirSummaries, err := s.summaryRepo.ListDirectorySummariesBySnapshot(ctx, snapshotID)
	if err != nil {
		return fmt.Errorf("failed to list directory summaries: %w", err)
	}

	if len(dirSummaries) == 0 {
		s.logger.Warn("no directory summaries found", "snapshot_id", snapshotID)
		return nil
	}

	// 2. source_hashを計算
	var dirHashes []string
	for _, ds := range dirSummaries {
		dirHashes = append(dirHashes, ds.ContentHash)
	}
	sourceHash := s.hasher.HashArchitectureSource(dirHashes)

	// 3. 既存の要約を確認（全4種類の要約をチェック）
	archTypes := []ArchType{
		ArchTypeOverview,
		ArchTypeTechStack,
		ArchTypeDataFlow,
		ArchTypeComponents,
	}

	allExist := true
	for _, archType := range archTypes {
		existing, err := s.summaryRepo.GetArchitectureSummary(ctx, snapshotID, archType)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("failed to check existing summary for %s: %w", archType, err)
		}
		if existing == nil || existing.SourceHash != sourceHash {
			allExist = false
			break
		}
	}

	if allExist {
		s.logger.Info("architecture summaries unchanged", "snapshot_id", snapshotID)
		return nil
	}

	s.logger.Info("generating architecture summaries", "snapshot_id", snapshotID)

	// 4. 各種類の要約を生成
	for _, archType := range archTypes {
		if err := s.generateOne(ctx, snapshotID, archType, dirSummaries, sourceHash); err != nil {
			return fmt.Errorf("failed to generate %s: %w", archType, err)
		}
		s.logger.Debug("generated architecture summary", "type", archType)
	}

	s.logger.Info("completed architecture summary generation", "snapshot_id", snapshotID)
	return nil
}

// generateOne は1種類のアーキテクチャ要約を生成
func (s *ArchitectureSummarizer) generateOne(
	ctx context.Context,
	snapshotID uuid.UUID,
	archType ArchType,
	dirSummaries []*Summary,
	sourceHash string,
) error {
	// 1. プロンプトを構築
	prompt := s.buildPrompt(archType, dirSummaries)

	// 2. LLMで要約を生成
	summaryContent, err := s.llm.GenerateCompletion(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// 3. Embeddingを生成
	embedding, err := s.embedder.Embed(ctx, summaryContent)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// 4. content_hashを計算
	contentHash := s.hasher.HashContent(summaryContent)

	// 5. メタデータを構築
	metadata := map[string]any{
		"directory_count": len(dirSummaries),
	}

	// 6. 要約を作成
	at := archType
	summary := &Summary{
		ID:          uuid.New(),
		SnapshotID:  snapshotID,
		SummaryType: SummaryTypeArchitecture,
		TargetPath:  "", // アーキテクチャ要約はパスなし
		ArchType:    &at,
		Content:     summaryContent,
		ContentHash: contentHash,
		SourceHash:  sourceHash,
		Metadata:    metadata,
	}

	// 7. DBに保存（既存があれば更新、なければ作成）
	existing, err := s.summaryRepo.GetArchitectureSummary(ctx, snapshotID, archType)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("failed to check existing summary: %w", err)
	}

	var saved *Summary
	if existing != nil {
		// 既存の要約を更新
		existing.Content = summaryContent
		existing.ContentHash = contentHash
		existing.SourceHash = sourceHash
		existing.Metadata = metadata
		if err := s.summaryRepo.UpdateSummary(ctx, existing); err != nil {
			return fmt.Errorf("failed to update summary: %w", err)
		}
		saved = existing
	} else {
		// 新規作成
		saved, err = s.summaryRepo.CreateSummary(ctx, summary)
		if err != nil {
			return fmt.Errorf("failed to create summary: %w", err)
		}
	}

	// 8. Embeddingを保存
	err = s.summaryRepo.UpsertSummaryEmbedding(ctx, &SummaryEmbedding{
		SummaryID: saved.ID,
		Vector:    embedding,
		Model:     s.embedder.ModelName(),
	})
	if err != nil {
		return fmt.Errorf("failed to create embedding: %w", err)
	}

	return nil
}

// buildPrompt はアーキテクチャ要約用のプロンプトを構築
func (s *ArchitectureSummarizer) buildPrompt(archType ArchType, dirSummaries []*Summary) string {
	// ディレクトリ要約をテキスト化
	var dirTexts []string
	for _, ds := range dirSummaries {
		path := ds.TargetPath
		if path == "" {
			path = "(root)"
		}
		dirTexts = append(dirTexts, fmt.Sprintf("- %s: %s", path, ds.Content))
	}
	sort.Strings(dirTexts)
	dirSummaryText := strings.Join(dirTexts, "\n")

	// 統計情報
	fileCount := 0
	for _, ds := range dirSummaries {
		if fc, ok := ds.Metadata["file_count"].(int); ok {
			fileCount += fc
		}
	}

	// 種類ごとのプロンプト
	switch archType {
	case ArchTypeOverview:
		return fmt.Sprintf(`以下のリポジトリの概要を作成してください。

統計:
- ディレクトリ数: %d
- 総ファイル数: %d

ディレクトリ要約:
%s

要件:
- システムの目的を説明
- 主要機能を3-5点列挙
- 日本語、500字以内`, len(dirSummaries), fileCount, dirSummaryText)

	case ArchTypeTechStack:
		return fmt.Sprintf(`以下のリポジトリの技術スタックをまとめてください。

ディレクトリ要約:
%s

要件:
- 使用言語
- フレームワーク・ライブラリ
- データベース・ストレージ
- 外部サービス
- 日本語、400字以内`, dirSummaryText)

	case ArchTypeDataFlow:
		return fmt.Sprintf(`以下のリポジトリのデータフローをまとめてください。

ディレクトリ要約:
%s

要件:
- エントリーポイント
- 処理フロー（3-5ステップ）
- データの永続化方法
- 日本語、400字以内`, dirSummaryText)

	case ArchTypeComponents:
		return fmt.Sprintf(`以下のリポジトリの主要コンポーネントをまとめてください。

ディレクトリ要約:
%s

要件:
- 主要コンポーネント（3-6個）
- 各コンポーネントの役割
- コンポーネント間の関係
- 日本語、500字以内`, dirSummaryText)

	default:
		return fmt.Sprintf(`以下のリポジトリの%sについてまとめてください。

ディレクトリ要約:
%s

日本語、400字以内`, archType, dirSummaryText)
	}
}
