package wiki

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/core/search"
)

// LLMClient はLLMとの通信インターフェース
type LLMClient interface {
	// GenerateCompletion はプロンプトから応答を生成する
	GenerateCompletion(ctx context.Context, prompt string) (string, error)
}

// WikiService はWiki生成のビジネスロジックを提供する
type WikiService struct {
	searchService *search.SearchService
	repo          Repository
	llm           LLMClient
	fileReader    FileReader
	logger        *slog.Logger
}

// WikiServiceOption は WikiService のオプション設定
type WikiServiceOption func(*WikiService)

// WithWikiLogger は WikiService にロガーを設定する
func WithWikiLogger(logger *slog.Logger) WikiServiceOption {
	return func(s *WikiService) {
		s.logger = logger
	}
}

// NewWikiService は新しいWikiServiceを作成する
func NewWikiService(
	searchService *search.SearchService,
	repo Repository,
	llm LLMClient,
	fileReader FileReader,
	opts ...WikiServiceOption,
) *WikiService {
	svc := &WikiService{
		searchService: searchService,
		repo:          repo,
		llm:           llm,
		fileReader:    fileReader,
		logger:        slog.Default(),
	}

	for _, opt := range opts {
		opt(svc)
	}

	if svc.logger == nil {
		svc.logger = slog.Default()
	}

	return svc
}

// Generate はWikiを生成する
func (s *WikiService) Generate(ctx context.Context, params GenerateParams) error {
	// バリデーション: ProductIDまたはSnapshotIDのいずれかが必須
	if params.ProductID.IsAbsent() && params.SnapshotID == uuid.Nil {
		return fmt.Errorf("either productID or snapshotID is required")
	}
	if params.OutputDir == "" {
		return fmt.Errorf("outputDir is required")
	}

	// OutputDirを作成
	if err := os.MkdirAll(params.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 各セクションを生成
	configs := GetSectionConfigs()
	pages := make([]*WikiPage, 0, len(configs))

	for _, config := range configs {
		page, err := s.generateSection(ctx, params, config)
		if err != nil {
			// エラーが発生しても続行可能な範囲で続行
			s.logger.Warn("failed to generate section",
				"section", config.Section,
				"error", err,
			)
			// 空のページを作成
			page = &WikiPage{
				Section:  config.Section,
				Title:    config.Title,
				FileName: config.FileName,
				Content:  fmt.Sprintf("# %s\n\nエラーが発生したため、このセクションを生成できませんでした。\n\nエラー: %v\n", config.Title, err),
			}
		}
		pages = append(pages, page)
	}

	// ファイルに書き出し
	for _, page := range pages {
		outputPath := filepath.Join(params.OutputDir, page.FileName)
		if err := os.WriteFile(outputPath, []byte(page.Content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", page.FileName, err)
		}
	}

	return nil
}

// generateSection は単一のセクションを生成する
func (s *WikiService) generateSection(ctx context.Context, params GenerateParams, config SectionConfig) (*WikiPage, error) {
	// 1. 事前定義クエリでSearchServiceを呼び出し
	summaryResults, chunkResults, err := s.searchContext(ctx, params, config.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to search context: %w", err)
	}

	// 2. プロンプト構築
	prompt := BuildSectionPrompt(config, summaryResults, chunkResults)

	// 3. LLMで生成
	content, err := s.llm.GenerateCompletion(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	// 4. WikiPageを作成
	page := &WikiPage{
		Section:  config.Section,
		Title:    config.Title,
		FileName: config.FileName,
		Content:  content,
	}

	return page, nil
}

// searchContext はクエリを使ってコンテキストを検索する
func (s *WikiService) searchContext(
	ctx context.Context,
	params GenerateParams,
	query string,
) ([]*search.SummarySearchResult, []*search.SearchResult, error) {
	// ハイブリッド検索パラメータを構築
	searchParams := search.HybridSearchParams{
		Query:        query,
		ChunkLimit:   10,
		SummaryLimit: 5,
		SummaryFilter: &search.SummarySearchFilter{
			// アーキテクチャ要約を優先
			SummaryTypes: []string{"architecture", "directory", "file"},
		},
	}

	// ProductIDが指定されている場合はプロダクト横断検索、
	// それ以外はSnapshotID検索
	if params.ProductID.IsPresent() {
		searchParams.ProductID = params.ProductID
	} else {
		searchParams.SnapshotID = params.SnapshotID
	}

	// ハイブリッド検索を実行
	result, err := s.searchService.HybridSearch(ctx, searchParams)
	if err != nil {
		return nil, nil, fmt.Errorf("hybrid search failed: %w", err)
	}

	return result.Summaries, result.Chunks, nil
}

// RegenerateSection は指定されたセクションのみを再生成する
func (s *WikiService) RegenerateSection(
	ctx context.Context,
	snapshotID uuid.UUID,
	outputDir string,
	section WikiSection,
) error {
	// バリデーション
	if snapshotID == uuid.Nil {
		return fmt.Errorf("snapshotID is required")
	}
	if outputDir == "" {
		return fmt.Errorf("outputDir is required")
	}

	// セクション設定を取得
	configs := GetSectionConfigs()
	var targetConfig *SectionConfig
	for _, config := range configs {
		if config.Section == section {
			targetConfig = &config
			break
		}
	}

	if targetConfig == nil {
		return fmt.Errorf("unknown section: %s", section)
	}

	// セクション生成用のGenerateParamsを作成
	params := GenerateParams{
		SnapshotID: snapshotID,
		OutputDir:  outputDir,
	}
	page, err := s.generateSection(ctx, params, *targetConfig)
	if err != nil {
		return fmt.Errorf("failed to generate section: %w", err)
	}

	// ファイル書き出し
	outputPath := filepath.Join(outputDir, page.FileName)
	if err := os.WriteFile(outputPath, []byte(page.Content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ReadSourceFile はスナップショット内のソースファイルを読み取る
func (s *WikiService) ReadSourceFile(ctx context.Context, snapshotID uuid.UUID, filePath string) (string, error) {
	if s.fileReader == nil {
		return "", fmt.Errorf("fileReader is not configured")
	}

	content, err := s.fileReader.ReadFile(ctx, snapshotID, filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return content, nil
}
