package container

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jinford/dev-rag/internal/module/llm/adapter"
	"github.com/jinford/dev-rag/internal/module/llm/domain"
	"github.com/jinford/dev-rag/internal/platform/config"
	"github.com/jinford/dev-rag/internal/platform/database"
	"github.com/jinford/dev-rag/pkg/indexer/embedder"
	"github.com/jinford/dev-rag/pkg/indexer/llm"
	wikillm "github.com/jinford/dev-rag/pkg/wiki/llm"
	"github.com/jinford/dev-rag/pkg/wiki"
)

// Container はアプリケーション全体の依存関係を管理します
type Container struct {
	Config            *config.Config
	Logger            *slog.Logger
	Database          *database.Database
	TxProvider        *database.TransactionProvider
	LLMClient         llm.LLMClient         // 既存のindexer用LLMクライアント（互換性のため残す）
	WikiLLMClient     wiki.LLMClient        // 既存のwiki用LLMクライアント（互換性のため残す）
	Embedder          *embedder.Embedder    // 既存のembedder（互換性のため残す）
	NewLLMClient      domain.Client         // 新しいLLMクライアント
	NewEmbedder       domain.Embedder       // 新しいEmbedder
}

// New は新しいコンテナを作成し、全ての依存関係を初期化します
func New(ctx context.Context, logger *slog.Logger, cfg *config.Config) (*Container, error) {
	// データベース接続
	db, err := database.New(ctx, database.ConnectionParams{
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
	txProvider := database.NewTransactionProvider(db.Pool)

	// インデックス用LLMクライアント（既存の互換性のため）
	llmClient, err := llm.NewOpenAIClientWithModel(cfg.OpenAI.LLMModel)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Embedder（既存の互換性のため）
	emb := embedder.NewEmbedder(
		cfg.OpenAI.APIKey,
		cfg.OpenAI.EmbeddingModel,
		cfg.OpenAI.EmbeddingDimension,
	)

	// Wiki用LLMクライアント（既存の互換性のため）
	var wikiLLMBase llm.LLMClient
	switch cfg.WikiLLM.Provider {
	case "openai":
		if cfg.WikiLLM.APIKey == "" {
			return nil, fmt.Errorf("WIKI_LLM_API_KEY is not configured")
		}
		wikiLLMBase, err = llm.NewOpenAIClientWithAPIKey(cfg.WikiLLM.APIKey, cfg.WikiLLM.Model)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Wiki LLM client: %w", err)
		}
	case "anthropic":
		return nil, fmt.Errorf("Anthropic provider is not yet supported")
	default:
		return nil, fmt.Errorf("unsupported Wiki LLM provider: %s", cfg.WikiLLM.Provider)
	}

	wikiLLMClient := wikillm.NewAdapter(wikiLLMBase, emb, cfg.WikiLLM.Temperature, cfg.WikiLLM.MaxTokens)

	// 新しいLLMモジュール（internal/module/llm）の初期化
	newLLMClient, err := adapter.NewOpenAIClient(cfg.OpenAI.APIKey, cfg.OpenAI.LLMModel)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize new LLM client: %w", err)
	}

	newEmbedder, err := adapter.NewOpenAIEmbedder(
		cfg.OpenAI.APIKey,
		cfg.OpenAI.EmbeddingModel,
		cfg.OpenAI.EmbeddingDimension,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize new embedder: %w", err)
	}

	return &Container{
		Config:        cfg,
		Logger:        logger,
		Database:      db,
		TxProvider:    txProvider,
		LLMClient:     llmClient,
		WikiLLMClient: wikiLLMClient,
		Embedder:      emb,
		NewLLMClient:  newLLMClient,
		NewEmbedder:   newEmbedder,
	}, nil
}

// Close はコンテナが保持する全てのリソースをクリーンアップします
func (c *Container) Close() {
	if c.Database != nil {
		c.Database.Close()
	}
}
