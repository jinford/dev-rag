package commands

import (
	"context"
	"fmt"

	"github.com/jinford/dev-rag/pkg/config"
	"github.com/jinford/dev-rag/pkg/db"
	"github.com/jinford/dev-rag/pkg/indexer/llm"
)

// AppContext はコマンド実行に必要な共通コンテキストを保持する
type AppContext struct {
	Config    *config.Config
	Database  *db.DB
	LLMClient llm.LLMClient
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

	// LLMクライアントの初期化
	llmClient, err := llm.NewOpenAIClient()
	if err != nil {
		return nil, fmt.Errorf("LLMクライアントの初期化に失敗: %w", err)
	}

	return &AppContext{
		Config:    cfg,
		Database:  database,
		LLMClient: llmClient,
	}, nil
}

// Close はAppContextが保持するリソースをクリーンアップする
func (ac *AppContext) Close() {
	if ac.Database != nil {
		ac.Database.Close()
	}
}
