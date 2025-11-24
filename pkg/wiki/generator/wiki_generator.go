package generator

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	indexingsqlc "github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
	wikisqlc "github.com/jinford/dev-rag/internal/module/wiki/adapter/pg/sqlc"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/search"
	"github.com/jinford/dev-rag/pkg/wiki"
)

// WikiGenerator はMarkdown形式のWikiページを生成するエンジン
type WikiGenerator struct {
	indexingDB    *indexingsqlc.Queries
	wikiDB        *wikisqlc.Queries
	llm           wiki.LLMClient
	searcher      *search.Searcher
	promptBuilder *PromptBuilder
}

// NewWikiGenerator は新しいWikiGeneratorを作成する
func NewWikiGenerator(
	indexingDB *indexingsqlc.Queries,
	wikiDB *wikisqlc.Queries,
	llm wiki.LLMClient,
	searcher *search.Searcher,
) *WikiGenerator {
	return &WikiGenerator{
		indexingDB:    indexingDB,
		wikiDB:        wikiDB,
		llm:           llm,
		searcher:      searcher,
		promptBuilder: NewPromptBuilder(),
	}
}

// GenerateArchitecturePage はarchitecture.mdページを生成
func (g *WikiGenerator) GenerateArchitecturePage(
	ctx context.Context,
	productID uuid.UUID,
	outputDir string,
) error {
	log.Printf("アーキテクチャページの生成を開始します (productID: %s)", productID)

	// 1. プロダクトの全ソースを取得
	var pgProductID pgtype.UUID
	if err := pgProductID.Scan(productID); err != nil {
		return fmt.Errorf("failed to convert productID: %w", err)
	}

	sources, err := g.indexingDB.ListSourcesByProduct(ctx, pgProductID)
	if err != nil {
		return fmt.Errorf("failed to list sources: %w", err)
	}

	if len(sources) == 0 {
		return fmt.Errorf("no sources found for product: %s", productID)
	}

	log.Printf("ソース数: %d", len(sources))

	// 2. 各ソースの最新スナップショットを取得
	var snapshots []pgtype.UUID
	for _, source := range sources {
		snapshot, err := g.indexingDB.GetLatestIndexedSnapshot(ctx, source.ID)
		if err != nil {
			log.Printf("警告: ソース %s の最新スナップショットが見つかりません: %v", source.ID, err)
			continue
		}
		snapshots = append(snapshots, snapshot.ID)
	}

	if len(snapshots) == 0 {
		return fmt.Errorf("no snapshots found for product: %s", productID)
	}

	log.Printf("スナップショット数: %d", len(snapshots))

	// 3. アーキテクチャ要約を取得（優先）
	var architectureSummaries []string
	summaryTypes := []string{"overview", "tech_stack", "data_flow", "components"}

	for _, snapshotID := range snapshots {
		for _, summaryType := range summaryTypes {
			summary, err := g.wikiDB.GetArchitectureSummary(ctx, wikisqlc.GetArchitectureSummaryParams{
				SnapshotID:  snapshotID,
				SummaryType: summaryType,
			})
			if err == nil && summary != "" {
				architectureSummaries = append(architectureSummaries, fmt.Sprintf("## %s\n%s", summaryType, summary))
				log.Printf("アーキテクチャ要約を取得しました: %s (%d文字)", summaryType, len(summary))
			} else {
				log.Printf("警告: アーキテクチャ要約が見つかりません: %s (snapshot: %s)", summaryType, snapshotID)
			}
		}
	}

	if len(architectureSummaries) == 0 {
		log.Printf("警告: アーキテクチャ要約が見つかりません。RAG検索結果のみを使用します")
	}

	// 4. 疑似クエリでRAG検索（補足情報）
	pseudoQuery := "システムアーキテクチャ、コンポーネント間の依存関係、データフロー、技術的な設計判断を説明する"

	searchParams := search.SearchParams{
		ProductID: &productID,
		Query:     pseudoQuery,
		Limit:     25,
	}

	searchResult, err := g.searcher.SearchByProduct(ctx, searchParams)
	if err != nil {
		log.Printf("警告: RAG検索に失敗しました: %v", err)
		searchResult = &search.Result{Chunks: []*models.SearchResult{}}
	} else {
		log.Printf("RAG検索結果: %d件", len(searchResult.Chunks))
	}

	// 5. プロンプト構築（階層的コンテキスト）
	prompt := g.promptBuilder.BuildArchitecturePrompt(
		architectureSummaries, // 最優先コンテキスト
		searchResult.Chunks,   // 補足コンテキスト
		pseudoQuery,
	)

	log.Printf("プロンプトを構築しました (%d文字)", len(prompt))

	// 6. LLM生成
	markdown, err := g.llm.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("LLM generation failed: %w", err)
	}

	log.Printf("Wikiページを生成しました (%d文字)", len(markdown))

	// 7. ファイル出力
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, "architecture.md")
	if err := os.WriteFile(outputPath, []byte(markdown), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	log.Printf("アーキテクチャページを出力しました: %s", outputPath)

	return nil
}

// GenerateDirectoryPage はdirectories/<directory>.mdページを生成
func (g *WikiGenerator) GenerateDirectoryPage(
	ctx context.Context,
	sourceID uuid.UUID,
	directoryPath string,
	outputDir string,
) error {
	log.Printf("ディレクトリページの生成を開始します (sourceID: %s, path: %s)", sourceID, directoryPath)

	// 1. 最新スナップショットを取得
	var pgSourceID pgtype.UUID
	if err := pgSourceID.Scan(sourceID); err != nil {
		return fmt.Errorf("failed to convert sourceID: %w", err)
	}

	snapshot, err := g.indexingDB.GetLatestIndexedSnapshot(ctx, pgSourceID)
	if err != nil {
		return fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	log.Printf("スナップショットID: %s", snapshot.ID)

	// 2. ディレクトリ要約を取得（優先）
	var summaryContent string
	dirSummary, err := g.wikiDB.GetDirectorySummaryByPath(ctx, wikisqlc.GetDirectorySummaryByPathParams{
		SnapshotID: snapshot.ID,
		Path:       directoryPath,
	})
	if err != nil {
		log.Printf("警告: ディレクトリ要約が見つかりません: %v", err)
		summaryContent = ""
	} else {
		summaryContent = dirSummary
		log.Printf("ディレクトリ要約を取得しました (%d文字)", len(summaryContent))
	}

	// 3. 疑似クエリでRAG検索（パスフィルタ適用）
	pseudoQuery := fmt.Sprintf("%sの責務、実装詳細、関連ファイル、処理フローを説明する", directoryPath)

	searchParams := search.SearchParams{
		SourceID:   &sourceID,
		Query:      pseudoQuery,
		Limit:      15,
		PathPrefix: directoryPath, // パスプレフィックス
	}

	searchResult, err := g.searcher.SearchBySource(ctx, searchParams)
	if err != nil {
		log.Printf("警告: RAG検索に失敗しました: %v", err)
		searchResult = &search.Result{Chunks: []*models.SearchResult{}}
	} else {
		log.Printf("RAG検索結果: %d件", len(searchResult.Chunks))
	}

	// 4. プロンプト構築
	prompt := g.promptBuilder.BuildDirectoryPrompt(
		summaryContent,        // 優先コンテキスト
		searchResult.Chunks,   // 補足コンテキスト
		pseudoQuery,
	)

	log.Printf("プロンプトを構築しました (%d文字)", len(prompt))

	// 5. LLM生成
	markdown, err := g.llm.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("LLM generation failed: %w", err)
	}

	log.Printf("Wikiページを生成しました (%d文字)", len(markdown))

	// 6. ファイル出力
	directoriesDir := filepath.Join(outputDir, "directories")
	if err := os.MkdirAll(directoriesDir, 0755); err != nil {
		return fmt.Errorf("failed to create directories directory: %w", err)
	}

	// ディレクトリパスをファイル名に変換（/を_に置換）
	dirName := strings.ReplaceAll(directoryPath, "/", "_")
	if dirName == "." {
		dirName = "root"
	}
	outputPath := filepath.Join(directoriesDir, dirName+".md")

	if err := os.WriteFile(outputPath, []byte(markdown), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	log.Printf("ディレクトリページを出力しました: %s", outputPath)

	return nil
}
