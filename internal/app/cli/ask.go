package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/urfave/cli/v3"

	coreask "github.com/jinford/dev-rag/internal/core/ask"
)

// AskAction は質問応答コマンドのアクション
func AskAction(ctx context.Context, cmd *cli.Command) error {
	// フラグの取得
	product := cmd.String("product")
	showSources := cmd.Bool("show-sources")
	envFile := cmd.String("env")

	// 質問文の取得
	question := cmd.Args().First()
	if question == "" {
		return fmt.Errorf("質問文を指定してください")
	}

	slog.Info("質問応答を開始",
		"product", product,
		"question", question,
		"showSources", showSources,
	)

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	// 質問応答処理を実行
	result, err := executeAsk(ctx, appCtx, product, question)
	if err != nil {
		slog.Error("質問応答に失敗しました", "error", err)
		return err
	}

	// 結果出力
	fmt.Println(result.Answer)

	// --show-sourcesフラグが指定されている場合、参照ソースも出力
	if showSources && len(result.Sources) > 0 {
		fmt.Println("\n--- 参照ソース ---")
		for i, source := range result.Sources {
			fmt.Printf("[%d] %s (L%d-L%d) スコア: %.4f\n",
				i+1,
				source.FilePath,
				source.StartLine,
				source.EndLine,
				source.Score,
			)
		}
	}

	slog.Info("質問応答が完了しました")
	return nil
}

// executeAsk は質問応答処理を実行する
func executeAsk(ctx context.Context, appCtx *AppContext, productName, question string) (*coreask.AskResult, error) {
	repo := appCtx.Container.IngestionRepo

	// 1. プロダクト名からプロダクトを取得
	slog.Info("プロダクトを取得します", "product", productName)
	product, err := repo.GetProductByName(ctx, productName)
	if err != nil {
		return nil, fmt.Errorf("プロダクト取得に失敗: %w", err)
	}

	slog.Info("プロダクトを取得しました", "productID", product.ID, "productName", product.Name)

	// 2. AskParamsを構築
	params := coreask.AskParams{
		ProductID:    &product.ID,
		Query:        question,
		ChunkLimit:   10, // デフォルト値
		SummaryLimit: 5,  // デフォルト値
	}

	// 3. AskServiceで質問応答を実行
	slog.Info("質問応答を実行します",
		"productID", product.ID,
		"productName", product.Name,
		"query", question,
	)

	result, err := appCtx.Container.AskService.Ask(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("質問応答に失敗: %w", err)
	}

	slog.Info("質問応答処理完了",
		"productName", product.Name,
		"answerLength", len(result.Answer),
		"sources", len(result.Sources),
	)

	return result, nil
}
