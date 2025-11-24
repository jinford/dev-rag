package commands

import (
	"context"
	"fmt"

	"github.com/jinford/dev-rag/pkg/config"
	"github.com/jinford/dev-rag/pkg/db"
	"github.com/jinford/dev-rag/pkg/indexer/embedder"
	"github.com/jinford/dev-rag/pkg/indexer/llm"
	wikillm "github.com/jinford/dev-rag/pkg/wiki/llm"
	"github.com/jinford/dev-rag/pkg/wiki"
)

// AppContext はコマンド実行に必要な共通コンテキストを保持する
type AppContext struct {
	Config        *config.Config
	Database      *db.DB
	LLMClient     llm.LLMClient
	WikiLLMClient wiki.LLMClient // Wiki生成用のLLMクライアント
	Embedder      *embedder.Embedder
}

// NewAppContext は設定ファイルを読み込み、DBに接続して AppContext を作成する
func NewAppContext(ctx context.Context, envFile string) (*AppContext, error) {
	// 設定の読み込み
	cfg, err := config.Load(envFile)
	if err != nil {
		return nil, fmt.Errorf("設定の読み込みに失敗: %w", err)
	}

	// DB接続
	database, err := db.New(ctx, db.ConnectionParams{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		return nil, fmt.Errorf("データベース接続に失敗: %w", err)
	}

	// インデックス用LLMクライアントの初期化（設定からモデル名を取得）
	llmClient, err := llm.NewOpenAIClientWithModel(cfg.OpenAI.LLMModel)
	if err != nil {
		return nil, fmt.Errorf("LLMクライアントの初期化に失敗: %w", err)
	}

	// Embedderの初期化
	emb := embedder.NewEmbedder(
		cfg.OpenAI.APIKey,
		cfg.OpenAI.EmbeddingModel,
		cfg.OpenAI.EmbeddingDimension,
	)

	// Wiki用LLMクライアントの初期化
	var wikiLLMBase llm.LLMClient
	switch cfg.WikiLLM.Provider {
	case "openai":
		// Wiki用OpenAIクライアントの初期化（Wiki用API key + モデルを使用）
		if cfg.WikiLLM.APIKey == "" {
			return nil, fmt.Errorf("WIKI_LLM_API_KEYが設定されていません")
		}
		wikiLLMBase, err = llm.NewOpenAIClientWithAPIKey(cfg.WikiLLM.APIKey, cfg.WikiLLM.Model)
		if err != nil {
			return nil, fmt.Errorf("Wiki用LLMクライアントの初期化に失敗: %w", err)
		}
	case "anthropic":
		// TODO: Anthropic用クライアントの実装
		return nil, fmt.Errorf("Anthropicプロバイダーはまだサポートされていません")
	default:
		return nil, fmt.Errorf("未対応のWiki LLMプロバイダー: %s", cfg.WikiLLM.Provider)
	}

	// Wiki用LLMクライアントアダプターの作成
	wikiLLMClient := wikillm.NewAdapter(wikiLLMBase, emb, cfg.WikiLLM.Temperature, cfg.WikiLLM.MaxTokens)

	return &AppContext{
		Config:        cfg,
		Database:      database,
		LLMClient:     llmClient,
		WikiLLMClient: wikiLLMClient,
		Embedder:      emb,
	}, nil
}

// Close はAppContextが保持するリソースをクリーンアップする
func (ac *AppContext) Close() {
	if ac.Database != nil {
		ac.Database.Close()
	}
}
