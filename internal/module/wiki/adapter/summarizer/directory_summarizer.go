package summarizer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"

	"github.com/jinford/dev-rag/internal/module/wiki/adapter/pg/sqlc"
	llmdomain "github.com/jinford/dev-rag/internal/module/llm/domain"
	wikipg "github.com/jinford/dev-rag/internal/module/wiki/adapter/pg"
	"github.com/jinford/dev-rag/internal/module/wiki/domain"
)

const (
	// maxContextTokens はプロンプトに含めるコンテキストの最大トークン数
	maxContextTokens = 8000
)

// directorySummarizer は domain.DirectorySummarizer の実装です。
type directorySummarizer struct {
	pool           *pgxpool.Pool
	llm            domain.LLMClient
	embedder       llmdomain.Embedder
	securityFilter domain.SecurityFilter
}

// NewDirectorySummarizer は domain.DirectorySummarizer を実装した新しい Summarizer を作成します。
func NewDirectorySummarizer(
	pool *pgxpool.Pool,
	llm domain.LLMClient,
	embedder llmdomain.Embedder,
	securityFilter domain.SecurityFilter,
) domain.DirectorySummarizer {
	return &directorySummarizer{
		pool:           pool,
		llm:            llm,
		embedder:       embedder,
		securityFilter: securityFilter,
	}
}

// SummarizeDirectory は単一ディレクトリの要約を生成します（domain ポート実装）。
func (s *directorySummarizer) SummarizeDirectory(
	ctx context.Context,
	structure *domain.RepoStructure,
	directory *domain.DirectoryInfo,
) (*domain.DirectorySummaryResult, error) {
	// トランザクション開始
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// sqlc.Queriesをトランザクションでラップ
	queries := sqlc.New(tx)

	// ディレクトリ直下の全ファイルの要約を取得
	fileSummaries, err := s.collectAllFileSummaries(ctx, queries, structure.SnapshotID, directory.Files)
	if err != nil && len(directory.Files) > 0 {
		// ファイルがあるのに要約が取得できない場合はエラー
		return nil, fmt.Errorf("failed to collect file summaries: %w", err)
	}

	// サブディレクトリの要約を取得（階層的集約）
	subdirSummaries, err := s.collectSubdirectorySummaries(ctx, queries, structure.SnapshotID, directory.Subdirectories)
	if err != nil {
		// サブディレクトリ要約が取得できない場合は警告のみ
		log.Printf("warning: failed to collect subdirectory summaries for %s: %v", directory.Path, err)
		subdirSummaries = ""
	}

	// ファイル要約もサブディレクトリ要約もない場合はエラー
	if fileSummaries == "" && subdirSummaries == "" {
		// ファイルまたはサブディレクトリがあるのに要約がない場合はエラー
		if len(directory.Files) > 0 || len(directory.Subdirectories) > 0 {
			return nil, fmt.Errorf("directory %s has %d files and %d subdirectories but no summaries found",
				directory.Path, len(directory.Files), len(directory.Subdirectories))
		}
		// 本当に空のディレクトリの場合はスキップ
		log.Printf("info: skipping empty directory %s", directory.Path)
		return nil, nil
	}

	// プロンプト構築（ファイル要約 + サブディレクトリ要約）
	prompt := buildDirectorySummaryPrompt(directory, fileSummaries, subdirSummaries)

	// セキュリティフィルタ（プロンプト全体に適用）
	if s.securityFilter.ContainsSensitiveInfo(prompt) {
		prompt = s.securityFilter.MaskSensitiveInfo(prompt)
	}

	// LLM生成
	summary, err := s.llm.GenerateWithRetry(ctx, prompt, 3)
	if err != nil {
		return nil, err
	}

	// Embedding生成
	embedding, err := s.llm.CreateEmbeddingWithRetry(ctx, summary, 3)
	if err != nil {
		return nil, err
	}

	// メタデータ構築
	metadata := map[string]interface{}{
		"model":            "text-embedding-3-small", // 固定値に変更
		"dim":              len(embedding),
		"file_count":       len(directory.Files),
		"subdir_count":     len(directory.Subdirectories),
		"total_files":      directory.TotalFiles,
		"languages":        directory.Languages,
		"llm_model":        "gpt-4o-mini",
		"prompt_version":   "2.0",
		"aggregation_mode": "hierarchical", // 階層的集約を明示
		"generated_at":     time.Now().Format(time.RFC3339),
	}
	metadataJSON, _ := json.Marshal(metadata)

	// directory_summariesテーブルにUPSERT
	_, err = queries.UpsertDirectorySummary(ctx, sqlc.UpsertDirectorySummaryParams{
		SnapshotID: wikipg.UUIDToPgtype(structure.SnapshotID),
		Path:       directory.Path,
		ParentPath: pgtype.Text{String: directory.ParentPath, Valid: directory.ParentPath != ""},
		Depth:      int32(directory.Depth),
		Summary:    summary,
		Embedding:  pgvector.NewVector(embedding),
		Metadata:   metadataJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert directory summary: %w", err)
	}

	// コミット
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 結果を返す
	return &domain.DirectorySummaryResult{
		Path:       directory.Path,
		ParentPath: directory.ParentPath,
		Depth:      directory.Depth,
		Summary:    summary,
		Embedding:  embedding,
		Metadata:   metadata,
	}, nil
}

// GenerateSummaries はすべてのディレクトリの要約を生成する（レガシーメソッド、後で削除予定）
// 階層的に処理する（深い階層から順番に、同じ階層内は並列処理）
func (s *directorySummarizer) GenerateSummaries(
	ctx context.Context,
	structure *domain.RepoStructure,
) error {
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
	// 各階層内では並列処理、階層間では同期処理
	for depth := maxDepth; depth >= 0; depth-- {
		directories := depthMap[depth]
		if len(directories) == 0 {
			continue
		}

		log.Printf("processing directories at depth %d (%d directories)", depth, len(directories))

		// 同じ階層のディレクトリは並列処理可能
		sem := make(chan struct{}, 5) // 最大5並列
		errCh := make(chan error, len(directories))
		var wg sync.WaitGroup

		for _, directory := range directories {
			wg.Add(1)
			go func(dir *domain.DirectoryInfo) {
				defer wg.Done()
				sem <- struct{}{}        // 並列数制限
				defer func() { <-sem }() // 解放

				// 各ディレクトリで個別にトランザクション開始・コミット
				if _, err := s.SummarizeDirectory(ctx, structure, dir); err != nil {
					log.Printf("directory summary failed for %s: %v", dir.Path, err)
					errCh <- err
				}
			}(directory)
		}

		// この階層の全ディレクトリ処理完了を待つ
		wg.Wait()
		close(errCh)

		// エラー集約（この階層の30%以上失敗したら致命的とみなす）
		var errors []error
		for err := range errCh {
			errors = append(errors, err)
		}

		if len(errors) > len(directories)/3 {
			return fmt.Errorf("too many directory summary failures at depth %d: %d/%d", depth, len(errors), len(directories))
		}

		log.Printf("completed directories at depth %d (failures: %d/%d)", depth, len(errors), len(directories))
	}

	return nil
}

// collectAllFileSummaries はディレクトリ直下の全ファイルの要約を取得する
func (s *directorySummarizer) collectAllFileSummaries(
	ctx context.Context,
	queries *sqlc.Queries,
	snapshotID uuid.UUID,
	filePaths []string,
) (string, error) {
	var summaries []string
	totalTokens := 0

	for _, filePath := range filePaths {
		// file_summariesテーブルからファイル要約を取得
		summary, err := queries.GetFileSummaryByPath(ctx, sqlc.GetFileSummaryByPathParams{
			SnapshotID: wikipg.UUIDToPgtype(snapshotID),
			Path:       filePath,
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				log.Printf("warning: no file summary found for %s", filePath)
				continue
			}
			log.Printf("warning: failed to get file summary for %s: %v", filePath, err)
			continue
		}

		if summary == "" {
			continue
		}

		// ファイルサマリーを整形
		summaryText := fmt.Sprintf("## %s\n%s\n", filepath.Base(filePath), summary)

		// トークン数を推定（文字数 / 4 で概算）
		estimatedTokens := len(summaryText) / 4

		// コンテキスト長チェック（安全マージン20%）
		if totalTokens+estimatedTokens > int(float64(maxContextTokens)*0.8) {
			log.Printf("warning: context limit reached for directory, truncating at %d files", len(summaries))
			summaries = append(summaries, fmt.Sprintf("... (残り %d ファイルは省略されました)", len(filePaths)-len(summaries)))
			break
		}

		summaries = append(summaries, summaryText)
		totalTokens += estimatedTokens
	}

	if len(summaries) == 0 {
		return "", nil // エラーではなく空文字列を返す（サブディレクトリのみの場合もある）
	}

	return strings.Join(summaries, "\n\n"), nil
}

// collectSubdirectorySummaries はサブディレクトリの要約を取得する（階層的集約）
func (s *directorySummarizer) collectSubdirectorySummaries(
	ctx context.Context,
	queries *sqlc.Queries,
	snapshotID uuid.UUID,
	subdirectories []string,
) (string, error) {
	var summaries []string
	totalTokens := 0

	for _, subdirPath := range subdirectories {
		// directory_summariesテーブルからサブディレクトリ要約を取得
		summary, err := queries.GetDirectorySummaryByPath(ctx, sqlc.GetDirectorySummaryByPathParams{
			SnapshotID: wikipg.UUIDToPgtype(snapshotID),
			Path:       subdirPath,
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				log.Printf("warning: no subdirectory summary found for %s", subdirPath)
				continue
			}
			log.Printf("warning: failed to get subdirectory summary for %s: %v", subdirPath, err)
			continue
		}

		if summary == "" {
			continue
		}

		// サブディレクトリ要約を整形
		summaryText := fmt.Sprintf("### サブディレクトリ: %s\n%s\n", filepath.Base(subdirPath), summary)

		// トークン数を推定
		estimatedTokens := len(summaryText) / 4

		// コンテキスト長チェック（安全マージン20%）
		if totalTokens+estimatedTokens > int(float64(maxContextTokens)*0.8) {
			log.Printf("warning: context limit reached for subdirectories, truncating at %d subdirs", len(summaries))
			summaries = append(summaries, fmt.Sprintf("... (残り %d サブディレクトリは省略されました)", len(subdirectories)-len(summaries)))
			break
		}

		summaries = append(summaries, summaryText)
		totalTokens += estimatedTokens
	}

	if len(summaries) == 0 {
		return "", nil // エラーではなく空文字列を返す
	}

	return strings.Join(summaries, "\n\n"), nil
}
