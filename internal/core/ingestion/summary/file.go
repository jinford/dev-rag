package summary

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/core/ingestion"
)

// FileSummarizer はファイル単位の要約を生成する
type FileSummarizer struct {
	ingestionRepo ingestion.Repository
	summaryRepo   Repository
	llm           LLMClient
	embedder      Embedder
	hasher        *Hasher
	logger        *slog.Logger
	concurrency   int
}

// NewFileSummarizer は新しいFileSummarizerを作成
func NewFileSummarizer(
	ingestionRepo ingestion.Repository,
	summaryRepo Repository,
	llm LLMClient,
	embedder Embedder,
	logger *slog.Logger,
) *FileSummarizer {
	return &FileSummarizer{
		ingestionRepo: ingestionRepo,
		summaryRepo:   summaryRepo,
		llm:           llm,
		embedder:      embedder,
		hasher:        NewHasher(),
		logger:        logger,
		concurrency:   5, // 並列度
	}
}

// GenerateForSnapshot はスナップショット内の全ファイル要約を生成（差分更新）
func (s *FileSummarizer) GenerateForSnapshot(ctx context.Context, snapshotID uuid.UUID) error {
	// 1. スナップショット内のファイル一覧を取得
	files, err := s.ingestionRepo.ListFilesBySnapshot(ctx, snapshotID)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	// ゼロ除算対策: ファイルが0件の場合は早期リターン
	if len(files) == 0 {
		s.logger.Info("no files to summarize", "snapshot_id", snapshotID)
		return nil
	}

	s.logger.Info("starting file summary generation",
		"snapshot_id", snapshotID,
		"file_count", len(files))

	// 2. 並列処理用のチャネルとWaitGroup
	type task struct {
		file *ingestion.File
	}
	taskCh := make(chan task, len(files))
	var wg sync.WaitGroup

	// エラー収集
	var mu sync.Mutex
	var errors []error
	successCount := 0

	// ワーカー起動
	for i := 0; i < s.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range taskCh {
				summary, changed, err := s.GenerateIfChanged(ctx, snapshotID, t.file)
				mu.Lock()
				if err != nil {
					errors = append(errors, fmt.Errorf("file %s: %w", t.file.Path, err))
					s.logger.Warn("failed to generate file summary",
						"path", t.file.Path,
						"error", err)
				} else {
					successCount++
					if changed {
						s.logger.Debug("generated file summary",
							"path", t.file.Path,
							"summary_id", summary.ID)
					} else {
						s.logger.Debug("file summary unchanged",
							"path", t.file.Path)
					}
				}
				mu.Unlock()
			}
		}()
	}

	// タスク投入
	for _, file := range files {
		taskCh <- task{file: file}
	}
	close(taskCh)

	// 完了待ち
	wg.Wait()

	// 30%以上失敗したらエラー
	failureRate := float64(len(errors)) / float64(len(files))
	if failureRate > 0.3 {
		return fmt.Errorf("too many failures: %d/%d files failed (%.1f%%)",
			len(errors), len(files), failureRate*100)
	}

	s.logger.Info("completed file summary generation",
		"snapshot_id", snapshotID,
		"success", successCount,
		"failed", len(errors))

	return nil
}

// GenerateIfChanged はハッシュが変更されたファイルのみ要約を生成
func (s *FileSummarizer) GenerateIfChanged(ctx context.Context, snapshotID uuid.UUID, file *ingestion.File) (*Summary, bool, error) {
	// source_hashを計算
	sourceHash := s.hasher.HashFileSource(file.ContentHash)

	// 既存の要約を確認
	existingOpt, err := s.summaryRepo.GetFileSummary(ctx, snapshotID, file.Path)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get existing summary: %w", err)
	}

	if existingOpt.IsPresent() {
		existing := existingOpt.MustGet()
		if existing.SourceHash == sourceHash {
			// source_hashが同じなら更新不要
			return existing, false, nil
		}
	}

	// 要約を生成
	summary, err := s.Generate(ctx, snapshotID, file)
	if err != nil {
		return nil, false, err
	}

	return summary, true, nil
}

// Generate は単一ファイルの要約を生成
func (s *FileSummarizer) Generate(ctx context.Context, snapshotID uuid.UUID, file *ingestion.File) (*Summary, error) {
	// 1. ファイルのチャンクを取得
	chunks, err := s.ingestionRepo.ListChunksByFile(ctx, file.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list chunks: %w", err)
	}

	// 2. チャンクの内容を結合
	var builder strings.Builder
	for _, chunk := range chunks {
		builder.WriteString(chunk.Content)
		builder.WriteString("\n")
	}
	content := builder.String()

	// 3. プロンプトを構築
	prompt := s.buildPrompt(file, content)

	// 4. LLMで要約を生成
	summaryContent, err := s.llm.GenerateCompletion(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	// 5. Embeddingを生成
	embedding, err := s.embedder.Embed(ctx, summaryContent)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// 6. ハッシュを計算
	sourceHash := s.hasher.HashFileSource(file.ContentHash)
	contentHash := s.hasher.HashContent(summaryContent)

	// 7. メタデータを構築
	metadata := map[string]any{
		"llm_model":      "default",
		"embedder_model": s.embedder.ModelName(),
		"chunk_count":    len(chunks),
	}
	if file.Language != nil {
		metadata["language"] = *file.Language
	}

	// 8. 要約を作成
	summary := &Summary{
		ID:          uuid.New(),
		SnapshotID:  snapshotID,
		SummaryType: SummaryTypeFile,
		TargetPath:  file.Path,
		Content:     summaryContent,
		ContentHash: contentHash,
		SourceHash:  sourceHash,
		Metadata:    metadata,
	}

	// 9. DBに保存（既存があれば更新、なければ作成）
	existingOpt, err := s.summaryRepo.GetFileSummary(ctx, snapshotID, file.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing summary: %w", err)
	}

	var saved *Summary
	if existingOpt.IsPresent() {
		existing := existingOpt.MustGet()
		// 既存の要約を更新
		existing.Content = summaryContent
		existing.ContentHash = contentHash
		existing.SourceHash = sourceHash
		existing.Metadata = metadata
		if err := s.summaryRepo.UpdateSummary(ctx, existing); err != nil {
			return nil, fmt.Errorf("failed to update summary: %w", err)
		}
		saved = existing
	} else {
		// 新規作成
		saved, err = s.summaryRepo.CreateSummary(ctx, summary)
		if err != nil {
			return nil, fmt.Errorf("failed to create summary: %w", err)
		}
	}

	// 10. Embeddingを保存
	err = s.summaryRepo.UpsertSummaryEmbedding(ctx, &SummaryEmbedding{
		SummaryID: saved.ID,
		Vector:    embedding,
		Model:     s.embedder.ModelName(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding: %w", err)
	}

	return saved, nil
}

// buildPrompt はファイル要約用のプロンプトを構築
func (s *FileSummarizer) buildPrompt(file *ingestion.File, content string) string {
	language := "unknown"
	if file.Language != nil {
		language = *file.Language
	}

	// コンテンツが長すぎる場合は切り詰め
	maxContentLen := 8000
	if len(content) > maxContentLen {
		content = content[:maxContentLen] + "\n... (truncated)"
	}

	return fmt.Sprintf(`以下のファイルの要約を作成してください。

パス: %s
言語: %s

内容:
%s

要件:
- 2-3文で目的を説明
- 主要な関数/型を列挙
- 日本語、200字以内`, file.Path, language, content)
}
