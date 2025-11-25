package cli

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/urfave/cli/v3"

	"github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
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
	// 1. インデックス化を実行
	slog.Info("インデックス化を開始します", "url", repoURL, "product", productName)

	params := domain.IndexParams{
		Identifier:  repoURL,
		ProductName: productName,
		ForceInit:   forceInit,
		Options: map[string]interface{}{
			"ref": ref,
		},
	}

	// Application層のIndexServiceを使用
	result, err := appCtx.Container.IndexService.IndexSource(ctx, domain.SourceTypeGit, params)
	if err != nil {
		return err
	}

	slog.Info("インデックス化が完了しました",
		"snapshotID", result.SnapshotID,
		"processedFiles", result.ProcessedFiles,
		"totalChunks", result.TotalChunks,
		"duration", result.Duration,
	)

	// 2. ソース名からソースIDを取得
	// TODO: より良い方法として、IndexServiceからSourceIDを返すようにする
	// 暫定的に、GitProviderのExtractSourceName相当のロジックでソース名を抽出し、
	// 直接リポジトリから取得する
	sourceName := extractSourceNameFromURL(repoURL)

	// sqlcクエリを使用してソースを取得
	queries := sqlc.New(appCtx.Container.Database().Pool)
	sourceRow, err := queries.GetSourceByName(ctx, sourceName)
	if err != nil {
		slog.Error("ソース取得に失敗", "error", err)
		return err
	}

	// pgtype.UUIDをuuid.UUIDに変換
	var sourceID uuid.UUID
	if err := sourceID.UnmarshalBinary(sourceRow.ID.Bytes[:]); err != nil {
		slog.Error("sourceIDの変換に失敗", "error", err)
		return err
	}

	// 3. スナップショットIDをパース
	snapshotID, err := uuid.Parse(result.SnapshotID)
	if err != nil {
		slog.Error("snapshotIDのパースに失敗", "error", err)
		return err
	}

	// 4. Wiki要約生成を実行
	slog.Info("Wiki要約生成を開始します", "sourceID", sourceID, "snapshotID", snapshotID)

	// Application層のWikiServiceを使用
	if err := appCtx.Container.WikiService.GenerateWiki(ctx, sourceID, snapshotID); err != nil {
		return err
	}

	slog.Info("Wiki要約生成が完了しました")

	return nil
}

// extractSourceNameFromURL はGitリポジトリURLからソース名を抽出します
// これはgitprovider.Provider.ExtractSourceNameの簡易版です
func extractSourceNameFromURL(repoURL string) string {
	// 簡易実装: URLの最後の部分を取得
	// 例: https://github.com/org/repo.git -> repo
	parts := splitURL(repoURL)
	if len(parts) == 0 {
		return repoURL
	}
	name := parts[len(parts)-1]
	// .git サフィックスを削除
	if len(name) > 4 && name[len(name)-4:] == ".git" {
		name = name[:len(name)-4]
	}
	return name
}

// splitURL はURLをスラッシュで分割します
func splitURL(url string) []string {
	var parts []string
	var current string
	for _, c := range url {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
