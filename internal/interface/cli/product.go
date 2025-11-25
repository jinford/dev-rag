package cli

import (
	"context"
	"log/slog"

	"github.com/urfave/cli/v3"
)

// ProductListAction はプロダクト一覧を表示するコマンドのアクション
func ProductListAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")

	slog.Info("プロダクト一覧表示を開始")

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	// TODO: プロダクト一覧取得の実装
	slog.Info("プロダクト一覧取得は未実装です")

	return nil
}

// ProductShowAction はプロダクト詳細を表示するコマンドのアクション
func ProductShowAction(ctx context.Context, cmd *cli.Command) error {
	name := cmd.String("name")
	envFile := cmd.String("env")

	slog.Info("プロダクト詳細表示を開始", "name", name)

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	// TODO: プロダクト詳細取得の実装
	slog.Info("プロダクト詳細取得は未実装です")

	return nil
}
