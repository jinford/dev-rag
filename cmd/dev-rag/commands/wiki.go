package commands

import (
	"context"
	"log/slog"

	"github.com/urfave/cli/v3"
)

// WikiGenerateAction はプロダクト単位でWikiを生成するコマンドのアクション
func WikiGenerateAction(ctx context.Context, cmd *cli.Command) error {
	product := cmd.String("product")
	out := cmd.String("out")
	config := cmd.String("config")
	envFile := cmd.String("env")

	slog.Info("Wiki生成を開始",
		"product", product,
		"out", out,
		"config", config,
	)

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	// TODO: Wiki生成処理の実装
	slog.Info("Wiki生成は未実装です")

	return nil
}
