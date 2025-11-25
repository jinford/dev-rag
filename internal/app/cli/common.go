package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jinford/dev-rag/internal/platform/config"
	"github.com/jinford/dev-rag/internal/platform/container"
	"github.com/jinford/dev-rag/internal/platform/logger"
)

// AppContext はコマンド実行に必要な共通コンテキストを保持する
type AppContext struct {
	Container *container.ServiceContainer // 新アーキテクチャ用コンテナ
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
	cont, err := container.NewContainer(ctx, appLogger, cfg)
	if err != nil {
		return nil, fmt.Errorf("コンテナの初期化に失敗: %w", err)
	}

	return &AppContext{
		Container: cont,
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
		return ac.Container.Logger()
	}
	return slog.Default()
}
