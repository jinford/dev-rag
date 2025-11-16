package commands

import (
	"context"
	"log/slog"

	"github.com/urfave/cli/v3"
)

// SourceListAction はソース一覧を表示するコマンドのアクション
func SourceListAction(ctx context.Context, cmd *cli.Command) error {
	product := cmd.String("product")
	envFile := cmd.String("env")

	slog.Info("ソース一覧表示を開始", "product", product)

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	// TODO: ソース一覧取得の実装
	slog.Info("ソース一覧取得は未実装です")

	return nil
}

// SourceShowAction はソース詳細を表示するコマンドのアクション
func SourceShowAction(ctx context.Context, cmd *cli.Command) error {
	name := cmd.String("name")
	envFile := cmd.String("env")

	slog.Info("ソース詳細表示を開始", "name", name)

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	// TODO: ソース詳細取得の実装
	slog.Info("ソース詳細取得は未実装です")

	return nil
}

// SourceIndexGitAction はGitソースをインデックス化するコマンドのアクション
func SourceIndexGitAction(ctx context.Context, cmd *cli.Command) error {
	repoURL := cmd.String("url")
	product := cmd.String("product")
	ref := cmd.String("ref")
	forceInit := cmd.Bool("force-init")
	envFile := cmd.String("env")

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	slog.Info("Gitソースインデックス処理を開始",
		"url", repoURL,
		"product", product,
		"ref", ref,
		"forceInit", forceInit,
	)

	// TODO: Gitソースインデックス処理の実装
	slog.Info("Gitソースインデックス処理は未実装です")

	return nil
}
