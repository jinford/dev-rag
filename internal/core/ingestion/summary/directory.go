package summary

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/core/ingestion"
)

// DirectorySummarizer はディレクトリ単位の要約を生成する
type DirectorySummarizer struct {
	ingestionRepo ingestion.Repository
	summaryRepo   Repository
	llm           LLMClient
	embedder      Embedder
	hasher        *Hasher
	logger        *slog.Logger
	concurrency   int
}

// NewDirectorySummarizer は新しいDirectorySummarizerを作成
func NewDirectorySummarizer(
	ingestionRepo ingestion.Repository,
	summaryRepo Repository,
	llm LLMClient,
	embedder Embedder,
	logger *slog.Logger,
) *DirectorySummarizer {
	return &DirectorySummarizer{
		ingestionRepo: ingestionRepo,
		summaryRepo:   summaryRepo,
		llm:           llm,
		embedder:      embedder,
		hasher:        NewHasher(),
		logger:        logger,
		concurrency:   5,
	}
}

// GenerateForSnapshot は全ディレクトリ要約を生成（深い階層から順に処理）
func (s *DirectorySummarizer) GenerateForSnapshot(ctx context.Context, snapshotID uuid.UUID) error {
	// 1. ディレクトリ構造を構築
	dirs, err := s.buildDirectoryStructure(ctx, snapshotID)
	if err != nil {
		return fmt.Errorf("failed to build directory structure: %w", err)
	}

	if len(dirs) == 0 {
		s.logger.Info("no directories found", "snapshot_id", snapshotID)
		return nil
	}

	// 2. 深さでグループ化
	depthMap := make(map[int][]*DirectoryInfo)
	maxDepth := 0
	for _, dir := range dirs {
		depthMap[dir.Depth] = append(depthMap[dir.Depth], dir)
		if dir.Depth > maxDepth {
			maxDepth = dir.Depth
		}
	}

	s.logger.Info("starting directory summary generation",
		"snapshot_id", snapshotID,
		"directory_count", len(dirs),
		"max_depth", maxDepth)

	// 3. 深い階層から順に処理（葉→幹）
	for depth := maxDepth; depth >= 0; depth-- {
		dirsAtDepth := depthMap[depth]
		if len(dirsAtDepth) == 0 {
			continue
		}

		s.logger.Debug("processing depth", "depth", depth, "count", len(dirsAtDepth))

		if err := s.processDepth(ctx, snapshotID, dirsAtDepth); err != nil {
			return fmt.Errorf("failed to process depth %d: %w", depth, err)
		}
	}

	s.logger.Info("completed directory summary generation", "snapshot_id", snapshotID)
	return nil
}

// buildDirectoryStructure はファイル一覧からディレクトリ構造を構築
func (s *DirectorySummarizer) buildDirectoryStructure(ctx context.Context, snapshotID uuid.UUID) ([]*DirectoryInfo, error) {
	// ファイル一覧を取得
	files, err := s.ingestionRepo.ListFilesBySnapshot(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	// ディレクトリごとにファイルをグループ化
	dirFiles := make(map[string][]*FileInfo)
	allDirs := make(map[string]bool)

	for _, file := range files {
		dir := filepath.Dir(file.Path)
		if dir == "." {
			dir = ""
		}

		// FileInfoに変換
		fileInfo := &FileInfo{
			ID:          file.ID,
			SnapshotID:  snapshotID,
			Path:        file.Path,
			ContentHash: file.ContentHash,
			Language:    file.Language,
		}
		dirFiles[dir] = append(dirFiles[dir], fileInfo)

		// 親ディレクトリも登録
		current := dir
		for current != "" && current != "." {
			allDirs[current] = true
			current = filepath.Dir(current)
			if current == "." {
				current = ""
			}
		}
		allDirs[""] = true // ルートディレクトリ
	}

	// DirectoryInfoを構築
	dirs := make([]*DirectoryInfo, 0, len(allDirs))
	for dirPath := range allDirs {
		depth := 0
		if dirPath != "" {
			depth = strings.Count(dirPath, string(filepath.Separator)) + 1
		}

		var parentPath *string
		if dirPath != "" {
			parent := filepath.Dir(dirPath)
			if parent == "." {
				parent = ""
			}
			parentPath = &parent
		}

		// サブディレクトリを特定
		var subdirs []string
		for otherDir := range allDirs {
			if otherDir == dirPath {
				continue
			}

			parent := filepath.Dir(otherDir)
			if parent == "." {
				parent = ""
			}

			if parent == dirPath {
				subdirs = append(subdirs, otherDir)
			}
		}
		sort.Strings(subdirs)

		dirs = append(dirs, &DirectoryInfo{
			Path:           dirPath,
			Depth:          depth,
			ParentPath:     parentPath,
			Files:          dirFiles[dirPath],
			Subdirectories: subdirs,
		})
	}

	return dirs, nil
}

// processDepth は指定深さのディレクトリを並列処理
func (s *DirectorySummarizer) processDepth(ctx context.Context, snapshotID uuid.UUID, dirs []*DirectoryInfo) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	// セマフォで並列度制御
	sem := make(chan struct{}, s.concurrency)

	for _, dir := range dirs {
		wg.Add(1)
		sem <- struct{}{}

		go func(d *DirectoryInfo) {
			defer wg.Done()
			defer func() { <-sem }()

			_, changed, err := s.GenerateIfChanged(ctx, snapshotID, d)
			mu.Lock()
			if err != nil {
				errors = append(errors, fmt.Errorf("directory %s: %w", d.Path, err))
				s.logger.Warn("failed to generate directory summary",
					"path", d.Path,
					"error", err)
			} else if changed {
				s.logger.Debug("generated directory summary", "path", d.Path)
			}
			mu.Unlock()
		}(dir)
	}

	wg.Wait()

	if len(errors) > 0 && float64(len(errors))/float64(len(dirs)) > 0.3 {
		return fmt.Errorf("too many failures: %d/%d", len(errors), len(dirs))
	}

	return nil
}

// GenerateIfChanged はハッシュが変更されたディレクトリのみ要約を生成
func (s *DirectorySummarizer) GenerateIfChanged(ctx context.Context, snapshotID uuid.UUID, dir *DirectoryInfo) (*Summary, bool, error) {
	// 配下のファイル要約を取得
	var fileSummaryHashes []string
	for _, file := range dir.Files {
		summaryOpt, err := s.summaryRepo.GetFileSummary(ctx, snapshotID, file.Path)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get file summary for %s: %w", file.Path, err)
		}
		if summaryOpt.IsAbsent() {
			s.logger.Debug("file summary not found, skipping",
				"path", file.Path,
				"directory", dir.Path)
			continue
		}
		fileSummaryHashes = append(fileSummaryHashes, summaryOpt.MustGet().ContentHash)
	}

	// サブディレクトリ要約を取得
	var subdirSummaryHashes []string
	for _, subdirPath := range dir.Subdirectories {
		summaryOpt, err := s.summaryRepo.GetDirectorySummary(ctx, snapshotID, subdirPath)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get directory summary for %s: %w", subdirPath, err)
		}
		if summaryOpt.IsAbsent() {
			s.logger.Debug("subdirectory summary not found, skipping",
				"path", subdirPath,
				"directory", dir.Path)
			continue
		}
		subdirSummaryHashes = append(subdirSummaryHashes, summaryOpt.MustGet().ContentHash)
	}

	// source_hashを計算
	sourceHash := s.hasher.HashDirectorySource(fileSummaryHashes, subdirSummaryHashes)

	// 既存の要約を確認
	existingOpt, err := s.summaryRepo.GetDirectorySummary(ctx, snapshotID, dir.Path)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get existing summary: %w", err)
	}
	if existingOpt.IsPresent() {
		existing := existingOpt.MustGet()
		if existing.SourceHash == sourceHash {
			return existing, false, nil
		}
	}

	// 要約を生成
	summary, err := s.Generate(ctx, snapshotID, dir, fileSummaryHashes, subdirSummaryHashes, sourceHash)
	if err != nil {
		return nil, false, err
	}

	return summary, true, nil
}

// Generate は単一ディレクトリの要約を生成
func (s *DirectorySummarizer) Generate(
	ctx context.Context,
	snapshotID uuid.UUID,
	dir *DirectoryInfo,
	fileSummaryHashes, subdirSummaryHashes []string,
	sourceHash string,
) (*Summary, error) {
	// 1. ファイル要約を収集
	var fileSummaries []string
	for _, file := range dir.Files {
		summaryOpt, err := s.summaryRepo.GetFileSummary(ctx, snapshotID, file.Path)
		if err != nil {
			continue
		}
		if summaryOpt.IsAbsent() {
			continue
		}
		fileSummaries = append(fileSummaries, fmt.Sprintf("- %s: %s", filepath.Base(file.Path), summaryOpt.MustGet().Content))
	}

	// 2. サブディレクトリ要約を収集
	var subdirSummaries []string
	for _, subdirPath := range dir.Subdirectories {
		summaryOpt, err := s.summaryRepo.GetDirectorySummary(ctx, snapshotID, subdirPath)
		if err != nil {
			continue
		}
		if summaryOpt.IsAbsent() {
			continue
		}
		subdirSummaries = append(subdirSummaries, fmt.Sprintf("- %s: %s", filepath.Base(subdirPath), summaryOpt.MustGet().Content))
	}

	// 3. プロンプトを構築
	prompt := s.buildPrompt(dir, fileSummaries, subdirSummaries)

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

	// 6. content_hashを計算
	contentHash := s.hasher.HashContent(summaryContent)

	// 7. メタデータを構築
	metadata := map[string]any{
		"file_count":   len(dir.Files),
		"subdir_count": len(dir.Subdirectories),
	}

	// 8. 要約を作成
	depth := dir.Depth
	summary := &Summary{
		ID:          uuid.New(),
		SnapshotID:  snapshotID,
		SummaryType: SummaryTypeDirectory,
		TargetPath:  dir.Path,
		Depth:       &depth,
		ParentPath:  dir.ParentPath,
		Content:     summaryContent,
		ContentHash: contentHash,
		SourceHash:  sourceHash,
		Metadata:    metadata,
	}

	// 9. DBに保存（既存があれば更新、なければ作成）
	existingOpt, err := s.summaryRepo.GetDirectorySummary(ctx, snapshotID, dir.Path)
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
		existing.Depth = &depth
		existing.ParentPath = dir.ParentPath
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

// buildPrompt はディレクトリ要約用のプロンプトを構築
func (s *DirectorySummarizer) buildPrompt(dir *DirectoryInfo, fileSummaries, subdirSummaries []string) string {
	path := dir.Path
	if path == "" {
		path = "(root)"
	}

	fileSummaryText := "なし"
	if len(fileSummaries) > 0 {
		fileSummaryText = strings.Join(fileSummaries, "\n")
	}

	subdirSummaryText := "なし"
	if len(subdirSummaries) > 0 {
		subdirSummaryText = strings.Join(subdirSummaries, "\n")
	}

	return fmt.Sprintf(`以下のディレクトリの要約を作成してください。

パス: %s
深さ: %d
ファイル数: %d
サブディレクトリ数: %d

ファイル要約:
%s

サブディレクトリ要約:
%s

要件:
- このディレクトリの責務を説明
- 主要な機能を列挙
- 日本語、300字以内`, path, dir.Depth, len(dir.Files), len(dir.Subdirectories), fileSummaryText, subdirSummaryText)
}
