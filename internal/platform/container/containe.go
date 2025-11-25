package container

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/pkoukk/tiktoken-go"

	coreindexing "github.com/jinford/dev-rag/internal/core/indexing"
	"github.com/jinford/dev-rag/internal/core/indexing/chunker"
	coresearch "github.com/jinford/dev-rag/internal/core/search"
	corewiki "github.com/jinford/dev-rag/internal/core/wiki"
	"github.com/jinford/dev-rag/internal/infra/git"
	"github.com/jinford/dev-rag/internal/infra/openai"
	"github.com/jinford/dev-rag/internal/infra/postgres"
	indexsqlc "github.com/jinford/dev-rag/internal/infra/postgres/sqlc"
	"github.com/jinford/dev-rag/internal/platform/config"
	"github.com/jinford/dev-rag/internal/platform/database"
)

// ServiceContainer は新アーキテクチャ(core/infra/pkg)の依存関係を保持する。
// 既存の container.New とは独立に動作し、移行期間の併存を前提とする。
type ServiceContainer struct {
	IndexService  *coreindexing.IndexService
	SearchService *coresearch.SearchService
	WikiService   *corewiki.WikiService

	logger   *slog.Logger
	database *database.Database
}

// NewContainer は設定とロガーからコンテナを生成する。
func NewContainer(ctx context.Context, logger *slog.Logger, cfg *config.Config) (*ServiceContainer, error) {
	db, err := database.New(ctx, database.ConnectionParams{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		return nil, fmt.Errorf("データベース初期化に失敗しました: %w", err)
	}

	return NewContainerWithDB(logger, cfg, db)
}

// NewContainerWithDB は既存の Database を受け取りコンテナを生成する。
func NewContainerWithDB(logger *slog.Logger, cfg *config.Config, db *database.Database) (*ServiceContainer, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Embedder (OpenAI)
	embedder := openai.NewEmbedder(cfg.OpenAI.APIKey, cfg.OpenAI.EmbeddingModel, cfg.OpenAI.EmbeddingDimension)

	// SourceProvider (Git)
	gitClient := git.NewClient(cfg.Git.SSHKeyPath, cfg.Git.SSHPassword)
	sourceProvider := git.NewProvider(gitClient, cfg.Git.CloneDir, cfg.Git.DefaultBranch)

	// Chunker / Detector / TokenCounter
	defaultChunker, err := chunker.NewDefaultChunker()
	if err != nil {
		return nil, fmt.Errorf("Chunker 初期化に失敗しました: %w", err)
	}
	chunkerFactory := &defaultChunkerFactory{base: defaultChunker}
	langDetector := &languageDetectorAdapter{detector: coreindexing.NewContentTypeDetector()}
	tokenCounter, err := newTokenCounter()
	if err != nil {
		return nil, fmt.Errorf("TokenCounter 初期化に失敗しました: %w", err)
	}

	// Repository (PostgreSQL)
	indexQueries := indexsqlc.New(db.Pool)
	indexRepo := postgres.NewRepository(indexQueries)

	// IndexService
	indexService := coreindexing.NewIndexService(coreindexing.IndexServiceConfig{
		Repository:     indexRepo,
		SourceProvider: sourceProvider,
		Embedder:       embedder,
		LLMClient:      nil,
		ChunkerFactory: chunkerFactory,
		LanguageDetect: langDetector,
		TokenCounter:   tokenCounter,
		ChunkerConfig:  chunker.DefaultChunkerConfig(),
		Logger:         logger,
	})

	// SearchService（新コア用リポジトリ）
	searchQueries := indexsqlc.New(db.Pool)
	searchRepo := postgres.NewSearchRepository(searchQueries)
	searchService := coresearch.NewSearchService(searchRepo, embedder)

	// WikiService（現状はスタブ実装）
	wikiService := corewiki.NewWikiService(&wikiRepositoryStub{}, &wikiLLMStub{})

	return &ServiceContainer{
		IndexService:  indexService,
		SearchService: searchService,
		WikiService:   wikiService,
		logger:        logger,
		database:      db,
	}, nil
}

// Close は内部リソースを解放する。
func (c *ServiceContainer) Close() {
	if c != nil && c.database != nil {
		c.database.Close()
	}
}

// Logger はロガーを返す。
func (c *ServiceContainer) Logger() *slog.Logger {
	if c == nil || c.logger == nil {
		return slog.Default()
	}
	return c.logger
}

// Database はデータベースを返す。
func (c *ServiceContainer) Database() *database.Database {
	if c == nil {
		return nil
	}
	return c.database
}

// --- アダプタ群 ---

// languageDetectorAdapter は ContentTypeDetector を新しい LanguageDetector に適合させる。
type languageDetectorAdapter struct {
	detector *coreindexing.ContentTypeDetector
}

func (a *languageDetectorAdapter) DetectLanguage(path string, content []byte) (string, error) {
	if a.detector == nil {
		return "text/plain", nil
	}
	return a.detector.DetectContentType(path, content), nil
}

// defaultChunkerFactory は単一の DefaultChunker を使い回すファクトリ。
type defaultChunkerFactory struct {
	base *chunker.DefaultChunker
}

func (f *defaultChunkerFactory) GetChunker(language string) (chunker.Chunker, error) {
	return &defaultChunkerAdapter{
		base:        f.base,
		contentType: language,
	}, nil
}

// defaultChunkerAdapter は DefaultChunker を Chunker インターフェースに適合させる。
type defaultChunkerAdapter struct {
	base        *chunker.DefaultChunker
	contentType string
}

func (c *defaultChunkerAdapter) Chunk(ctx context.Context, path string, content string) ([]*chunker.ChunkResult, error) {
	chunksWithMeta, err := c.base.ChunkWithMetadata(content, c.contentType)
	if err != nil {
		return nil, err
	}

	results := make([]*chunker.ChunkResult, 0, len(chunksWithMeta))
	for _, cwm := range chunksWithMeta {
		results = append(results, &chunker.ChunkResult{
			Content:   cwm.Chunk.Content,
			StartLine: cwm.Chunk.StartLine,
			EndLine:   cwm.Chunk.EndLine,
			Tokens:    cwm.Chunk.Tokens,
			Metadata:  cwm.Metadata,
		})
	}
	return results, nil
}

// tokenCounter は tiktoken を利用した TokenCounter 実装。
type tokenCounter struct {
	encoding *tiktoken.Tiktoken
}

func newTokenCounter() (*tokenCounter, error) {
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to load tiktoken encoding: %w", err)
	}
	return &tokenCounter{encoding: enc}, nil
}

func (t *tokenCounter) CountTokens(text string) int {
	if t.encoding == nil {
		return 0
	}
	return len(t.encoding.Encode(text, nil, nil))
}

func (t *tokenCounter) TrimToTokenLimit(text string, maxTokens int) string {
	if t.encoding == nil {
		return text
	}
	tokens := t.encoding.Encode(text, nil, nil)
	if len(tokens) <= maxTokens {
		return text
	}
	return t.encoding.Decode(tokens[:maxTokens])
}

// wikiRepositoryStub は未実装領域を埋めるスタブ。
type wikiRepositoryStub struct{}

func (r *wikiRepositoryStub) GetWikiMetadata(ctx context.Context, productID uuid.UUID) (*corewiki.WikiMetadata, error) {
	return nil, fmt.Errorf("wiki repository is not implemented")
}
func (r *wikiRepositoryStub) CreateWikiMetadata(ctx context.Context, productID uuid.UUID, outputPath string, fileCount int) (*corewiki.WikiMetadata, error) {
	return nil, fmt.Errorf("wiki repository is not implemented")
}
func (r *wikiRepositoryStub) UpdateWikiMetadata(ctx context.Context, id uuid.UUID, fileCount int) error {
	return fmt.Errorf("wiki repository is not implemented")
}
func (r *wikiRepositoryStub) GetFileSummary(ctx context.Context, fileID uuid.UUID) (*corewiki.FileSummary, error) {
	return nil, fmt.Errorf("wiki repository is not implemented")
}
func (r *wikiRepositoryStub) ListFileSummaries(ctx context.Context, productID uuid.UUID) ([]*corewiki.FileSummary, error) {
	return nil, fmt.Errorf("wiki repository is not implemented")
}
func (r *wikiRepositoryStub) CreateFileSummary(ctx context.Context, fileID uuid.UUID, summary string, embedding []float32, metadata []byte) (*corewiki.FileSummary, error) {
	return nil, fmt.Errorf("wiki repository is not implemented")
}
func (r *wikiRepositoryStub) UpdateFileSummary(ctx context.Context, id uuid.UUID, summary string, embedding []float32, metadata []byte) error {
	return fmt.Errorf("wiki repository is not implemented")
}
func (r *wikiRepositoryStub) DeleteFileSummary(ctx context.Context, id uuid.UUID) error {
	return fmt.Errorf("wiki repository is not implemented")
}
func (r *wikiRepositoryStub) GetRepoStructure(ctx context.Context, sourceID uuid.UUID, snapshotID uuid.UUID) (*corewiki.RepoStructure, error) {
	return nil, fmt.Errorf("wiki repository is not implemented")
}
func (r *wikiRepositoryStub) GetSourceInfo(ctx context.Context, sourceID uuid.UUID) (*corewiki.SourceInfo, error) {
	return nil, fmt.Errorf("wiki repository is not implemented")
}
func (r *wikiRepositoryStub) GetSnapshotInfo(ctx context.Context, snapshotID uuid.UUID) (*corewiki.SnapshotInfo, error) {
	return nil, fmt.Errorf("wiki repository is not implemented")
}

// wikiLLMStub は WikiService 用の暫定 LLMClient。
type wikiLLMStub struct{}

func (c *wikiLLMStub) Generate(ctx context.Context, prompt string) (string, error) {
	return "", fmt.Errorf("wiki LLM client is not implemented")
}

func (c *wikiLLMStub) GenerateWithRetry(ctx context.Context, prompt string, maxRetries int) (string, error) {
	return "", fmt.Errorf("wiki LLM client is not implemented")
}
