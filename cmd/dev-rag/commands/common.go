package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jinford/dev-rag/internal/platform/config"
	"github.com/jinford/dev-rag/internal/platform/container"
	"github.com/jinford/dev-rag/internal/platform/logger"
	"github.com/jinford/dev-rag/pkg/db"
	"github.com/jinford/dev-rag/pkg/indexer/embedder"
	"github.com/jinford/dev-rag/pkg/indexer/llm"
	"github.com/jinford/dev-rag/pkg/wiki"
)

// AppContext はコマンド実行に必要な共通コンテキストを保持する
type AppContext struct {
	Config        *config.Config
	Database      *db.DB
	LLMClient     llm.LLMClient
	WikiLLMClient wiki.LLMClient // Wiki生成用のLLMクライアント
	Embedder      *embedder.Embedder
	Container     *container.Container // Platform Container（新しいアーキテクチャ）
}

// NewAppContext は設定ファイルを読み込み、DBに接続して AppContext を作成する
func NewAppContext(ctx context.Context, envFile string) (*AppContext, error) {
	// 設定の読み込み（platform層を使用）
	cfg, err := config.Load(envFile)
	if err != nil {
		return nil, fmt.Errorf("設定の読み込みに失敗: %w", err)
	}

	// ロガーの初期化（platform層を使用）
	appLogger := logger.New(logger.DefaultConfig())

	// コンテナの初期化（platform層を使用）
	cont, err := container.New(ctx, appLogger, cfg)
	if err != nil {
		return nil, fmt.Errorf("コンテナの初期化に失敗: %w", err)
	}

	// 既存のAPIとの互換性のため、pkg/db.DBを作成
	database := &db.DB{Pool: cont.Database.Pool}

	return &AppContext{
		Config:        cfg,
		Database:      database,
		LLMClient:     cont.LLMClient,
		WikiLLMClient: cont.WikiLLMClient,
		Embedder:      cont.Embedder,
		Container:     cont,
	}, nil
}

// Close はAppContextが保持するリソースをクリーンアップする
func (ac *AppContext) Close() {
	if ac.Container != nil {
		ac.Container.Close()
	}
}

// Logger はAppContextのロガーを返す
func (ac *AppContext) Logger() *slog.Logger {
	if ac.Container != nil {
		return ac.Container.Logger
	}
	return slog.Default()
}
