package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/urfave/cli/v3"

	"github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
	"github.com/jinford/dev-rag/pkg/indexer"
	"github.com/jinford/dev-rag/pkg/indexer/chunker"
	"github.com/jinford/dev-rag/pkg/indexer/detector"
	"github.com/jinford/dev-rag/pkg/indexer/llm"
	"github.com/jinford/dev-rag/pkg/indexer/provider"
	gitprovider "github.com/jinford/dev-rag/pkg/indexer/provider/git"
	"github.com/jinford/dev-rag/pkg/indexer/summarizer"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/repository"
	"github.com/jinford/dev-rag/pkg/repository/txprovider"
	"github.com/jinford/dev-rag/pkg/wiki/analyzer"
	"github.com/jinford/dev-rag/pkg/wiki/security"
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

	// Gitソースインデックス処理を実行
	if err := executeGitIndexing(ctx, appCtx, repoURL, product, ref, forceInit); err != nil {
		slog.Error("Gitソースインデックス処理に失敗しました", "error", err)
		return err
	}

	slog.Info("Gitソースインデックス処理が完了しました")
	return nil
}

// executeGitIndexing はGitリポジトリのインデックス化とWiki要約生成を実行する
func executeGitIndexing(ctx context.Context, appCtx *AppContext, repoURL, productName, ref string, forceInit bool) error {
	// 1. 必要なコンポーネントを初期化
	queries := sqlc.New(appCtx.Database.Pool)

	// リポジトリの初期化
	sourceRepo := repository.NewSourceRepositoryR(queries)
	indexRepo := repository.NewIndexRepositoryR(queries)
	txProvider := txprovider.NewTransactionProvider(appCtx.Database.Pool)

	// Chunkerの初期化
	chunkr, err := chunker.NewChunker()
	if err != nil {
		return fmt.Errorf("Chunker初期化に失敗: %w", err)
	}

	// Embedderを再利用
	emb := appCtx.Embedder

	detect := detector.NewContentTypeDetector()

	// GitClientの初期化
	gitClient := gitprovider.NewGitClient(
		appCtx.Config.Git.SSHKeyPath,
		appCtx.Config.Git.SSHPassword,
	)

	// TokenCounterの初期化
	tokenCounter, err := llm.NewTokenCounter()
	if err != nil {
		return fmt.Errorf("TokenCounter初期化に失敗: %w", err)
	}

	// FileSummaryServiceの初期化（必須）
	// LLMモデル名は設定から取得
	fileSummaryService := summarizer.NewFileSummaryService(
		appCtx.LLMClient, // indexer/llm.LLMClient を使用
		tokenCounter,
		emb,
		txProvider,
		slog.Default(),
		appCtx.Config.OpenAI.LLMModel, // 設定から取得したモデル名
	)

	// Indexerの作成
	idx, err := indexer.NewIndexer(
		sourceRepo,
		indexRepo,
		txProvider,
		chunkr,
		emb,
		detect,
		gitClient,
		slog.Default(),
	)
	if err != nil {
		return fmt.Errorf("Indexer作成に失敗: %w", err)
	}

	// FileSummaryServiceを設定（必須）
	idx.SetFileSummaryService(fileSummaryService)

	// GitProviderの作成と登録
	gitCloneBaseDir := appCtx.Config.Git.CloneDir
	defaultBranch := appCtx.Config.Git.DefaultBranch
	if ref != "" {
		defaultBranch = ref
	}

	gitProvider := gitprovider.NewProvider(gitClient, gitCloneBaseDir, defaultBranch)
	idx.RegisterProvider(gitProvider)

	// 2. インデックス化を実行
	slog.Info("インデックス化を開始します", "url", repoURL, "product", productName)

	params := provider.IndexParams{
		Identifier:  repoURL,
		ProductName: productName,
		ForceInit:   forceInit,
		Options: map[string]interface{}{
			"ref": ref,
		},
	}

	result, err := idx.IndexSource(ctx, models.SourceTypeGit, params)
	if err != nil {
		return fmt.Errorf("インデックス化に失敗: %w", err)
	}

	slog.Info("インデックス化が完了しました",
		"snapshotID", result.SnapshotID,
		"processedFiles", result.ProcessedFiles,
		"totalChunks", result.TotalChunks,
		"duration", result.Duration,
	)

	// 3. Wiki要約生成を実行
	snapshotID, err := uuid.Parse(result.SnapshotID)
	if err != nil {
		return fmt.Errorf("snapshotIDのパースに失敗: %w", err)
	}

	// ソースIDを取得（プロダクト名とソース名から）
	sourceName := gitProvider.ExtractSourceName(repoURL)
	source, err := queries.GetSourceByName(ctx, sourceName)
	if err != nil {
		return fmt.Errorf("ソース取得に失敗: %w", err)
	}

	// pgtype.UUIDをuuid.UUIDに変換
	var sourceID uuid.UUID
	if err := sourceID.UnmarshalBinary(source.ID.Bytes[:]); err != nil {
		return fmt.Errorf("sourceIDの変換に失敗: %w", err)
	}

	slog.Info("Wiki要約生成を開始します", "sourceID", sourceID, "snapshotID", snapshotID)

	// RepositoryAnalyzerを作成
	securityFilter := security.NewFilter()
	repositoryAnalyzer := analyzer.NewRepositoryAnalyzer(
		appCtx.Database.Pool,
		appCtx.WikiLLMClient,
		appCtx.Embedder,
		securityFilter,
	)

	// リポジトリ解析とWiki要約生成を実行
	if err := repositoryAnalyzer.AnalyzeRepository(ctx, sourceID, snapshotID); err != nil {
		return fmt.Errorf("Wiki要約生成に失敗しました: %w", err)
	}

	slog.Info("Wiki要約生成が完了しました")

	return nil
}
