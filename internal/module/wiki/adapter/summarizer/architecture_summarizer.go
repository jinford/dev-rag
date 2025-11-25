package summarizer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"

	"github.com/jinford/dev-rag/internal/module/wiki/adapter/pg/sqlc"
	llmdomain "github.com/jinford/dev-rag/internal/module/llm/domain"
	wikipg "github.com/jinford/dev-rag/internal/module/wiki/adapter/pg"
	"github.com/jinford/dev-rag/internal/module/wiki/domain"
)

// architectureSummarizer は domain.ArchitectureSummarizer の実装です。
type architectureSummarizer struct {
	pool           *pgxpool.Pool
	llm            domain.LLMClient
	embedder       llmdomain.Embedder
	securityFilter domain.SecurityFilter
}

// NewArchitectureSummarizer は domain.ArchitectureSummarizer を実装した新しい Summarizer を作成します。
func NewArchitectureSummarizer(
	pool *pgxpool.Pool,
	llm domain.LLMClient,
	embedder llmdomain.Embedder,
	securityFilter domain.SecurityFilter,
) domain.ArchitectureSummarizer {
	return &architectureSummarizer{
		pool:           pool,
		llm:            llm,
		embedder:       embedder,
		securityFilter: securityFilter,
	}
}

// SummarizeArchitecture は単一タイプのアーキテクチャ要約を生成します（domain ポート実装）。
func (s *architectureSummarizer) SummarizeArchitecture(
	ctx context.Context,
	structure *domain.RepoStructure,
	summaryType string,
) (*domain.ArchitectureSummaryResult, error) {
	// トランザクション開始
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// sqlc.Queriesをトランザクションでラップ
	queries := sqlc.New(tx)

	// Directory Summaryを全て取得（重要: ディレクトリ構造ではなく、既に生成された要約を使う）
	// コミット済みのディレクトリ要約を読み込む
	directorySummaries, err := s.collectAllDirectorySummaries(ctx, queries, structure.SnapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to collect directory summaries: %w", err)
	}

	// プロンプト構築（Directory Summaryを元に）
	prompt := s.buildPrompt(structure, directorySummaries, summaryType)

	// セキュリティチェック
	if s.securityFilter.ContainsSensitiveInfo(prompt) {
		prompt = s.securityFilter.MaskSensitiveInfo(prompt)
	}

	// LLMで要約生成（リトライ付き）
	summary, err := s.llm.GenerateWithRetry(ctx, prompt, 3)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Embedding生成（リトライ付き）
	embedding, err := s.llm.CreateEmbeddingWithRetry(ctx, summary, 3)
	if err != nil {
		return nil, fmt.Errorf("embedding creation failed: %w", err)
	}

	// メタデータ構築（Embedder設定から取得）
	metadata := map[string]interface{}{
		"model":              "text-embedding-3-small", // 固定値に変更
		"dim":                s.embedder.Dimension(),
		"generated_at":       time.Now().Format(time.RFC3339),
		"file_count":         len(structure.Files),
		"directory_count":    len(structure.Directories),
		"llm_model":          s.llm.GetModelName(), // LLMから取得
		"prompt_version":     "3.0",                // トークンベース + 階層的集約
		"aggregation_source": "directory_summaries",
	}
	metadataJSON, _ := json.Marshal(metadata)

	// architecture_summariesテーブルにUPSERT（冪等性保証）
	_, err = queries.UpsertArchitectureSummary(ctx, sqlc.UpsertArchitectureSummaryParams{
		SnapshotID:  wikipg.UUIDToPgtype(structure.SnapshotID),
		SummaryType: summaryType,
		Summary:     summary,
		Embedding:   pgvector.NewVector(embedding),
		Metadata:    metadataJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert architecture summary: %w", err)
	}

	log.Printf("architecture summary generated: type=%s, snapshot_id=%s", summaryType, structure.SnapshotID)

	// コミット
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 結果を返す
	return &domain.ArchitectureSummaryResult{
		SnapshotID:  structure.SnapshotID,
		SummaryType: summaryType,
		Summary:     summary,
		Embedding:   embedding,
		Metadata:    metadata,
	}, nil
}

// GenerateSummaries は複数種類のアーキテクチャ要約を生成する（レガシーメソッド、後で削除予定）
func (s *architectureSummarizer) GenerateSummaries(
	ctx context.Context,
	structure *domain.RepoStructure,
) error {
	// 複数種類の要約を生成
	summaryTypes := []string{"overview", "tech_stack", "data_flow", "components"}

	for _, summaryType := range summaryTypes {
		if _, err := s.SummarizeArchitecture(ctx, structure, summaryType); err != nil {
			return fmt.Errorf("failed to generate %s summary: %w", summaryType, err)
		}
	}

	return nil
}

// collectAllDirectorySummaries は全てのディレクトリ要約を取得する
func (s *architectureSummarizer) collectAllDirectorySummaries(
	ctx context.Context,
	queries *sqlc.Queries,
	snapshotID uuid.UUID,
) (string, error) {
	// directory_summariesテーブルから全てのディレクトリ要約を取得
	const maxContextTokens = 8000 // トークンベースで管理
	var summaries []string
	totalTokens := 0

	// ディレクトリを取得（深さでソート）
	rows, err := queries.ListDirectorySummariesBySnapshot(ctx, wikipg.UUIDToPgtype(snapshotID))
	if err != nil {
		return "", fmt.Errorf("failed to list directory summaries: %w", err)
	}

	for _, row := range rows {
		// ディレクトリ要約を整形
		summaryText := fmt.Sprintf("## %s (深さ: %d)\n%s\n", row.Path, row.Depth, row.Summary)

		// トークン数を推定（文字数 / 4 で概算）
		estimatedTokens := len(summaryText) / 4

		// コンテキスト長チェック（安全マージン20%）
		if totalTokens+estimatedTokens > int(float64(maxContextTokens)*0.8) {
			log.Printf("warning: context limit reached, truncating at %d directories", len(summaries))
			summaries = append(summaries, fmt.Sprintf("... (残り %d ディレクトリは省略されました)", len(rows)-len(summaries)))
			break
		}

		summaries = append(summaries, summaryText)
		totalTokens += estimatedTokens
	}

	if len(summaries) == 0 {
		return "", fmt.Errorf("no directory summaries found for snapshot_id=%s (total rows: %d)", snapshotID, len(rows))
	}

	return strings.Join(summaries, "\n\n"), nil
}

// buildPrompt はアーキテクチャ要約生成用のプロンプトを構築する
func (s *architectureSummarizer) buildPrompt(
	structure *domain.RepoStructure,
	directorySummariesContent string,
	summaryType string,
) string {
	// プロンプトテンプレートの詳細は以下のドキュメントを参照
	// docs/architecture-wiki-prompt-template.md - セクションA: ArchitectureSummarizer用プロンプト

	// ディレクトリ要約を統合して、summary_type（overview/tech_stack/data_flow/components）に応じた
	// アーキテクチャレベルの要約を生成する
	// 実装の詳細はプロンプトテンプレートドキュメントを参照

	return buildArchitectureSummaryPrompt(structure, directorySummariesContent, summaryType)
}
