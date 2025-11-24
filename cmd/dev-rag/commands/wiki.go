package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/urfave/cli/v3"

	indexingsqlc "github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
	wikisqlc "github.com/jinford/dev-rag/internal/module/wiki/adapter/pg/sqlc"
	"github.com/jinford/dev-rag/pkg/search"
	"github.com/jinford/dev-rag/pkg/wiki/generator"
)

// WikiGenerateAction はプロダクト単位でWikiを生成するコマンドのアクション
func WikiGenerateAction(ctx context.Context, cmd *cli.Command) error {
	product := cmd.String("product")
	out := cmd.String("out")
	configFile := cmd.String("config")
	envFile := cmd.String("env")

	slog.Info("Wiki生成を開始",
		"product", product,
		"out", out,
		"config", configFile,
	)

	// 設定ファイルの優先順位: --config > --env > デフォルト
	cfgPath := envFile
	if configFile != "" {
		cfgPath = configFile
	}

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, cfgPath)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	// 出力ディレクトリの決定: --out > WIKI_OUTPUT_DIR > デフォルト
	outputDir := out
	if outputDir == "" {
		outputDir = appCtx.Config.WikiOutputDir
		slog.Info("出力ディレクトリが未指定のため、設定値を使用します", "outputDir", outputDir)
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
	// 1. プロダクトIDを取得
	indexingQueries := indexingsqlc.New(appCtx.Database.Pool)
	wikiQueries := wikisqlc.New(appCtx.Database.Pool)

	product, err := indexingQueries.GetProductByName(ctx, productName)
	if err != nil {
		return fmt.Errorf("プロダクト取得に失敗: %w", err)
	}

	// pgtype.UUIDをuuid.UUIDに変換
	var productID uuid.UUID
	if err := productID.UnmarshalBinary(product.ID.Bytes[:]); err != nil {
		return fmt.Errorf("productIDの変換に失敗: %w", err)
	}

	slog.Info("プロダクトを取得しました", "productID", productID, "productName", product.Name)

	// 2. 必要なコンポーネントを初期化
	// Searcherの初期化（AppContextのEmbedderを再利用）
	searcher := search.NewSearcher(appCtx.Database, appCtx.Embedder)
	searcher.SetLogger(slog.Default())

	// WikiGeneratorの作成（AppContextのWikiLLMClientを使用）
	wikiGen := generator.NewWikiGenerator(
		indexingQueries,
		wikiQueries,
		appCtx.WikiLLMClient,
		searcher,
	)

	// 3. アーキテクチャページ生成
	slog.Info("アーキテクチャページを生成します", "outputDir", outputDir)

	if err := wikiGen.GenerateArchitecturePage(ctx, productID, outputDir); err != nil {
		return fmt.Errorf("アーキテクチャページ生成に失敗: %w", err)
	}

	slog.Info("アーキテクチャページの生成が完了しました")

	// 4. ディレクトリページ生成
	// プロダクトの全ソースを取得
	var pgProductID uuid.UUID
	copy(pgProductID[:], product.ID.Bytes[:])

	var pgtypeProductID pgtype.UUID
	if err := pgtypeProductID.Scan(pgProductID); err != nil {
		return fmt.Errorf("productIDの変換に失敗: %w", err)
	}

	sources, err := indexingQueries.ListSourcesByProduct(ctx, pgtypeProductID)
	if err != nil {
		return fmt.Errorf("ソース一覧取得に失敗: %w", err)
	}

	if len(sources) == 0 {
		slog.Warn("ソースが見つかりません", "product", productName)
		return nil
	}

	// 各ソースのディレクトリページを生成
	for _, source := range sources {
		// pgtype.UUIDをuuid.UUIDに変換
		var sourceID uuid.UUID
		if err := sourceID.UnmarshalBinary(source.ID.Bytes[:]); err != nil {
			slog.Warn("sourceIDの変換に失敗しました", "error", err)
			continue
		}

		// 最新スナップショットを取得
		snapshot, err := indexingQueries.GetLatestIndexedSnapshot(ctx, source.ID)
		if err != nil {
			slog.Warn("最新スナップショットが見つかりません", "sourceID", sourceID, "error", err)
			continue
		}

		// ディレクトリサマリー一覧を取得
		dirSummaries, err := wikiQueries.ListDirectorySummariesBySnapshot(ctx, snapshot.ID)
		if err != nil {
			slog.Warn("ディレクトリサマリー取得に失敗しました", "snapshotID", snapshot.ID, "error", err)
			continue
		}

		slog.Info("ディレクトリページを生成します",
			"sourceID", sourceID,
			"sourceName", source.Name,
			"directoriesCount", len(dirSummaries),
		)

		// 各ディレクトリのページを生成
		for _, dirSummary := range dirSummaries {
			if err := wikiGen.GenerateDirectoryPage(ctx, sourceID, dirSummary.Path, outputDir); err != nil {
				slog.Warn("ディレクトリページ生成に失敗しました",
					"path", dirSummary.Path,
					"error", err,
				)
				// エラーがあっても続行
				continue
			}
		}

		slog.Info("ディレクトリページの生成が完了しました", "sourceID", sourceID)
	}

	return nil
}
