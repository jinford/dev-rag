package cli

import (
	"context"
	"log/slog"

	"github.com/urfave/cli/v3"

	coreingestion "github.com/jinford/dev-rag/internal/core/ingestion"
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
	generateWiki := cmd.Bool("generate-wiki")
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

	// Gitソースインデックス処理を実行
	if err := executeGitIndexing(ctx, appCtx, repoURL, product, ref, forceInit, generateWiki); err != nil {
		slog.Error("Gitソースインデックス処理に失敗しました", "error", err)
		return err
	}

	slog.Info("Gitソースインデックス処理が完了しました")
	return nil
}

// executeGitIndexing はGitリポジトリのインデックス化とWiki要約生成を実行する
func executeGitIndexing(ctx context.Context, appCtx *AppContext, repoURL, productName, ref string, forceInit bool, generateWiki bool) error {
	// 1. インデックス化を実行
	slog.Info("インデックス化を開始します", "url", repoURL, "product", productName)

	params := coreingestion.IndexParams{
		Identifier:  repoURL,
		ProductName: productName,
		ForceInit:   forceInit,
		Options: map[string]any{
			"ref": ref,
		},
	}

	// Application層のIndexServiceを使用
	result, err := appCtx.Container.IndexService.IndexSource(ctx, params)
	if err != nil {
		return err
	}

	slog.Info("インデックス化が完了しました",
		"snapshotID", result.SnapshotID,
		"processedFiles", result.ProcessedFiles,
		"totalChunks", result.TotalChunks,
		"duration", result.Duration,
	)

	// 2. Wiki生成（未実装スタブ）
	if generateWiki {
		slog.Warn("Wiki生成は新アーキテクチャでは未実装のためスキップします")
	}

	return nil
}
