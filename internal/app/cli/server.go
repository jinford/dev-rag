package cli

import (
	"context"
	"log/slog"

	"github.com/urfave/cli/v3"
)

// ServerStartAction はHTTPサーバを起動するコマンドのアクション
func ServerStartAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	// TODO: HTTPサーバの起動
	slog.Info("HTTPサーバは未実装です")

	return nil
}
