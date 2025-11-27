package summary

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/core/ingestion"
)

// SummaryService は階層的要約生成のオーケストレーター
type SummaryService struct {
	fileSummarizer *FileSummarizer
	dirSummarizer  *DirectorySummarizer
	archSummarizer *ArchitectureSummarizer
	logger         *slog.Logger
}

// SummaryServiceOption は SummaryService のオプション設定
type SummaryServiceOption func(*SummaryService)

// WithSummaryLogger は SummaryService にロガーを設定する
func WithSummaryLogger(logger *slog.Logger) SummaryServiceOption {
	return func(s *SummaryService) {
		s.logger = logger
	}
}

// NewSummaryService は新しいSummaryServiceを作成
func NewSummaryService(
	ingestionRepo ingestion.Repository,
	summaryRepo Repository,
	llm LLMClient,
	embedder Embedder,
	opts ...SummaryServiceOption,
) *SummaryService {
	svc := &SummaryService{
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(svc)
	}
	if svc.logger == nil {
		svc.logger = slog.Default()
	}

	svc.fileSummarizer = NewFileSummarizer(ingestionRepo, summaryRepo, llm, embedder, svc.logger)
	svc.dirSummarizer = NewDirectorySummarizer(ingestionRepo, summaryRepo, llm, embedder, svc.logger)
	svc.archSummarizer = NewArchitectureSummarizer(summaryRepo, llm, embedder, svc.logger)

	return svc
}

// GenerateForSnapshot はスナップショットの全要約を生成（差分更新）
// 処理順序: ファイル → ディレクトリ（深→浅） → アーキテクチャ
func (s *SummaryService) GenerateForSnapshot(ctx context.Context, snapshotID uuid.UUID) error {
	s.logger.Info("starting summary generation", "snapshot_id", snapshotID)

	// 1. ファイル要約を生成
	s.logger.Info("generating file summaries...")
	if err := s.fileSummarizer.GenerateForSnapshot(ctx, snapshotID); err != nil {
		return fmt.Errorf("failed to generate file summaries: %w", err)
	}

	// 2. ディレクトリ要約を生成（深い階層から順に）
	s.logger.Info("generating directory summaries...")
	if err := s.dirSummarizer.GenerateForSnapshot(ctx, snapshotID); err != nil {
		return fmt.Errorf("failed to generate directory summaries: %w", err)
	}

	// 3. アーキテクチャ要約を生成
	s.logger.Info("generating architecture summaries...")
	if err := s.archSummarizer.Generate(ctx, snapshotID); err != nil {
		return fmt.Errorf("failed to generate architecture summaries: %w", err)
	}

	s.logger.Info("completed summary generation", "snapshot_id", snapshotID)
	return nil
}

// GenerateFileSummaries はファイル要約のみを生成
func (s *SummaryService) GenerateFileSummaries(ctx context.Context, snapshotID uuid.UUID) error {
	return s.fileSummarizer.GenerateForSnapshot(ctx, snapshotID)
}

// GenerateDirectorySummaries はディレクトリ要約のみを生成
func (s *SummaryService) GenerateDirectorySummaries(ctx context.Context, snapshotID uuid.UUID) error {
	return s.dirSummarizer.GenerateForSnapshot(ctx, snapshotID)
}

// GenerateArchitectureSummaries はアーキテクチャ要約のみを生成
func (s *SummaryService) GenerateArchitectureSummaries(ctx context.Context, snapshotID uuid.UUID) error {
	return s.archSummarizer.Generate(ctx, snapshotID)
}
