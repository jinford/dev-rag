package container

import (
	"context"
	"fmt"
	"log/slog"

	indexingpg "github.com/jinford/dev-rag/internal/module/indexing/adapter/pg"
	indexingsqlc "github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
	indexingapp "github.com/jinford/dev-rag/internal/module/indexing/application"
	searchapp "github.com/jinford/dev-rag/internal/module/search/application"
	wikiapp "github.com/jinford/dev-rag/internal/module/wiki/application"
	wikidomain "github.com/jinford/dev-rag/internal/module/wiki/domain"
	"github.com/jinford/dev-rag/internal/platform/config"
	"github.com/jinford/dev-rag/internal/platform/database"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/indexer"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/chunker"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/detector"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/embedder"
	indexingllm "github.com/jinford/dev-rag/internal/module/indexing/adapter/llm"
	llmadapter "github.com/jinford/dev-rag/internal/module/llm/adapter"
	gitprovider "github.com/jinford/dev-rag/internal/module/indexing/adapter/git"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/prompts"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/summarizer"
	searchpg "github.com/jinford/dev-rag/internal/module/search/adapter/pg"
	searchsqlc "github.com/jinford/dev-rag/internal/module/search/adapter/pg/sqlc"
	search "github.com/jinford/dev-rag/internal/module/search/adapter/searcher"
	wikillm "github.com/jinford/dev-rag/internal/module/wiki/adapter/llm"
	"github.com/jinford/dev-rag/internal/module/wiki/adapter/security"
	wikianalyzer "github.com/jinford/dev-rag/internal/module/wiki/adapter/analyzer"
	wikisummarizer "github.com/jinford/dev-rag/internal/module/wiki/adapter/summarizer"
	wikipg "github.com/jinford/dev-rag/internal/module/wiki/adapter/pg"
	wikisqlc "github.com/jinford/dev-rag/internal/module/wiki/adapter/pg/sqlc"
)

// Container はアプリケーション全体の依存関係を管理します
// 公開フィールドは各モジュールの Application Service のみを保持します
type Container struct {
	// 各モジュールの公開 Application Services
	IndexService  *indexingapp.IndexService
	WikiService   *wikiapp.WikiService
	SearchService *searchapp.SearchService

	// Platform層の基盤コンポーネント（技術詳細）
	// これらは Container 内部でのみ使用し、外部には公開しません
	config     *config.Config
	logger     *slog.Logger
	database   *database.Database
	txProvider *database.TransactionProvider
}

// New は新しいコンテナを作成し、全ての依存関係を初期化します
func New(ctx context.Context, logger *slog.Logger, cfg *config.Config) (*Container, error) {
	// データベース接続
	platformDB, err := database.New(ctx, database.ConnectionParams{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// トランザクションプロバイダー
	txProvider := database.NewTransactionProvider(platformDB.Pool)

	// indexing 用 Embedder（indexing の adapter を使用）
	emb := embedder.NewEmbedder(
		cfg.OpenAI.APIKey,
		cfg.OpenAI.EmbeddingModel,
		cfg.OpenAI.EmbeddingDimension,
	)

	// wiki 用 Embedder（llm の adapter を使用）
	wikiEmb, err := llmadapter.NewOpenAIEmbedder(
		cfg.OpenAI.APIKey,
		cfg.OpenAI.EmbeddingModel,
		cfg.OpenAI.EmbeddingDimension,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize wiki embedder: %w", err)
	}

	// Wiki用LLMクライアント（internal/module/llm を使用）
	var wikiLLMBase *llmadapter.OpenAIClient
	switch cfg.WikiLLM.Provider {
	case "openai":
		if cfg.WikiLLM.APIKey == "" {
			return nil, fmt.Errorf("WIKI_LLM_API_KEY is not configured")
		}
		wikiLLMBase, err = llmadapter.NewOpenAIClient(cfg.WikiLLM.APIKey, cfg.WikiLLM.Model)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Wiki LLM client: %w", err)
		}
	case "anthropic":
		return nil, fmt.Errorf("Anthropic provider is not yet supported")
	default:
		return nil, fmt.Errorf("unsupported Wiki LLM provider: %s", cfg.WikiLLM.Provider)
	}

	wikiLLMClient := wikillm.NewAdapter(wikiLLMBase, wikiEmb, cfg.WikiLLM.Temperature, cfg.WikiLLM.MaxTokens)

	// Application Servicesの初期化
	// Indexing関連 - 新しいリポジトリを使用
	queries := indexingsqlc.New(platformDB.Pool)
	sourceRepoR := indexingpg.NewSourceRepository(queries)
	indexRepoR := indexingpg.NewIndexRepositoryR(queries)

	// Indexerの作成
	chunkr, err := chunker.NewChunker()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize chunker: %w", err)
	}

	detect := detector.NewContentTypeDetector() // interface 型を返す

	gitClient := gitprovider.NewGitClient(
		cfg.Git.SSHKeyPath,
		cfg.Git.SSHPassword,
	)

	// indexing の TokenCounter を使用（後で internal/module/llm に移行検討）
	tokenCounter, err := indexingllm.NewTokenCounter()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize token counter: %w", err)
	}

	// indexing の LLMClient をラップ（後で adapter 層の整理が必要）
	indexingLLMClient, err := indexingllm.NewOpenAIClientWithModel(cfg.OpenAI.LLMModel)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize indexing LLM client: %w", err)
	}

	// ファイル要約サービスの初期化（application 層）
	fileSummaryGenerator := prompts.NewFileSummaryGenerator(indexingLLMClient, tokenCounter)
	fileSummaryRepository := summarizer.NewFileSummaryRepository(txProvider)
	fileSummaryService := indexingapp.NewFileSummaryService(
		fileSummaryGenerator,
		emb,
		fileSummaryRepository,
		logger,
		cfg.OpenAI.LLMModel,
		prompts.FileSummaryPromptVersion,
	)

	idx, err := indexer.NewIndexer(
		sourceRepoR,
		indexRepoR,
		txProvider,
		chunkr,
		emb,
		detect,
		gitClient,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize indexer: %w", err)
	}

	idx.SetFileSummaryService(fileSummaryService)

	// GitProviderの登録
	gitProvider := gitprovider.NewProvider(gitClient, cfg.Git.CloneDir, cfg.Git.DefaultBranch)
	idx.RegisterProvider(gitProvider)

	// Application Servicesの作成
	// Indexer を application interface に適応させる
	indexerAdapt := newIndexerAdapter(idx)
	indexService := indexingapp.NewIndexService(indexerAdapt, logger)

	// Wiki関連
	// セキュリティフィルタ（adapter）
	securityFilter := security.NewFilter()

	// Repository Analyzer（adapter）
	repositoryAnalyzer := wikianalyzer.NewRepositoryAnalyzer(
		platformDB.Pool,
		securityFilter,
	)

	// Directory Summarizer（adapter）
	directorySummarizer := wikisummarizer.NewDirectorySummarizer(
		platformDB.Pool,
		wikiLLMClient,
		wikiEmb,
		securityFilter,
	)

	// Architecture Summarizer（adapter）
	archSummarizer := wikisummarizer.NewArchitectureSummarizer(
		platformDB.Pool,
		wikiLLMClient,
		wikiEmb,
		securityFilter,
	)

	// Wiki Generator（adapter） - 将来実装
	// wikiGenerator := wikigenerator.NewWikiGenerator(...)

	// Wiki Repository（adapter）
	wikiQueries := wikisqlc.New(platformDB.Pool)
	wikiMetadataRepo := wikipg.NewWikiMetadataRepository(wikiQueries)

	// Repository は空実装として登録（将来実装）
	var directorySummaryRepo wikidomain.DirectorySummaryRepository
	var archSummaryRepo wikidomain.ArchitectureSummaryRepository
	var wikiGenerator wikidomain.WikiGenerator

	// WikiOrchestrator（application）
	wikiOrchestrator := wikiapp.NewWikiOrchestrator(
		repositoryAnalyzer,
		directorySummarizer,
		archSummarizer,
		wikiGenerator,
		wikiMetadataRepo,
		directorySummaryRepo,
		archSummaryRepo,
		logger,
	)

	// WikiService（application）
	wikiService := wikiapp.NewWikiService(wikiOrchestrator, logger)

	// Search関連
	searchQueries := searchsqlc.New(platformDB.Pool)
	searchRepo := searchpg.NewSearchRepository(searchQueries)
	searcher := search.NewSearcher(searchRepo, emb)
	searchService := searchapp.NewSearchService(searcher, logger)

	return &Container{
		// Application Services（公開）
		IndexService:  indexService,
		WikiService:   wikiService,
		SearchService: searchService,

		// Platform層の基盤コンポーネント（非公開）
		config:     cfg,
		logger:     logger,
		database:   platformDB,
		txProvider: txProvider,
	}, nil
}

// Close はコンテナが保持する全てのリソースをクリーンアップします
func (c *Container) Close() {
	if c.database != nil {
		c.database.Close()
	}
}

// Logger は Container のロガーを返します（互換性のため）
func (c *Container) Logger() *slog.Logger {
	return c.logger
}

// Database は Container のデータベースを返します（互換性のため）
// 注意: 将来的には各モジュールの Application Service 経由でのみアクセスすべき
func (c *Container) Database() *database.Database {
	return c.database
}
