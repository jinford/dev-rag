package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/urfave/cli/v3"

	corewiki "github.com/jinford/dev-rag/internal/core/wiki"
	"github.com/samber/mo"
)

// WikiGenerateAction はプロダクト単位でWikiを生成するコマンドのアクション
func WikiGenerateAction(ctx context.Context, cmd *cli.Command) error {
	product := cmd.String("product")
	out := cmd.String("out")
	envFile := cmd.String("env")

	slog.Info("Wiki生成を開始",
		"product", product,
		"out", out,
	)

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	// 出力ディレクトリの決定
	outputDir := out
	if outputDir == "" {
		// デフォルト値を設定（環境変数から取得可能）
		outputDir = "/var/lib/dev-rag/wikis"
		slog.Info("出力ディレクトリが未指定のため、デフォルト値を使用します", "outputDir", outputDir)
	}

	// Wiki生成処理を実行
	if err := executeWikiGeneration(ctx, appCtx, product, outputDir); err != nil {
		slog.Error("Wiki生成に失敗しました", "error", err)
		return err
	}

	slog.Info("Wiki生成が完了しました")
	return nil
}

// executeWikiGeneration はプロダクト単位でWikiページを生成する
func executeWikiGeneration(ctx context.Context, appCtx *AppContext, productName, outputDir string) error {
	repo := appCtx.Container.IngestionRepo

	// 1. プロダクト名からプロダクトを取得
	slog.Info("プロダクトを取得します", "product", productName)
	productOpt, err := repo.GetProductByName(ctx, productName)
	if err != nil {
		return fmt.Errorf("プロダクト取得に失敗: %w", err)
	}
	if productOpt.IsAbsent() {
		return fmt.Errorf("プロダクトが見つかりません: %s", productName)
	}
	product := productOpt.MustGet()

	slog.Info("プロダクトを取得しました", "productID", product.ID, "productName", product.Name)

	// 2. プロダクト単位でWikiを生成
	productOutputDir := fmt.Sprintf("%s/%s", outputDir, product.Name)

	params := corewiki.GenerateParams{
		ProductID: mo.Some(product.ID),
		OutputDir: productOutputDir,
	}

	slog.Info("Wiki生成を開始します",
		"productID", product.ID,
		"productName", product.Name,
		"outputDir", productOutputDir,
	)

	if err := appCtx.Container.WikiService.Generate(ctx, params); err != nil {
		return fmt.Errorf("Wiki生成に失敗: %w", err)
	}

	slog.Info("Wiki生成処理完了", "productName", product.Name)
	return nil
}
