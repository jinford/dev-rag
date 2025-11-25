package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	appcli "github.com/jinford/dev-rag/internal/app/cli"
	"github.com/jinford/dev-rag/internal/platform/logger"
	"github.com/urfave/cli/v3"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 構造化ログの設定(platform層を使用)
	_ = logger.New(logger.DefaultConfig())

	app := &cli.Command{
		Name:  "dev-rag",
		Usage: "社内リポジトリ向け RAG 基盤および Wiki 自動生成システム",
		Commands: []*cli.Command{
			{
				Name:  "product",
				Usage: "プロダクト管理コマンド",
				Commands: []*cli.Command{
					{
						Name:  "list",
						Usage: "プロダクト一覧を表示",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
						},
						Action: appcli.ProductListAction,
					},
					{
						Name:  "show",
						Usage: "プロダクト詳細を表示",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:     "name",
								Usage:    "プロダクト名",
								Required: true,
							},
						},
						Action: appcli.ProductShowAction,
					},
				},
			},
			{
				Name:  "source",
				Usage: "ソース管理コマンド",
				Commands: []*cli.Command{
					{
						Name:  "list",
						Usage: "ソース一覧を表示",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:  "product",
								Usage: "プロダクト名（絞り込み）",
							},
						},
						Action: appcli.SourceListAction,
					},
					{
						Name:  "show",
						Usage: "ソース詳細を表示",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:     "name",
								Usage:    "ソース名",
								Required: true,
							},
						},
						Action: appcli.SourceShowAction,
					},
				},
			},
			{
				Name:  "index",
				Usage: "インデックス管理コマンド",
				Commands: []*cli.Command{
					{
						Name:  "git",
						Usage: "Gitソースをインデックス化",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:     "url",
								Usage:    "GitリポジトリURL",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "product",
								Usage:    "プロダクト名（存在しない場合は自動作成）",
								Required: true,
							},
							&cli.StringFlag{
								Name:  "ref",
								Usage: "ブランチ名またはタグ名（省略時はリモートのdefault_branch）",
							},
							&cli.BoolFlag{
								Name:  "force-init",
								Usage: "強制的にフルインデックスを実行",
							},
							&cli.BoolFlag{
								Name:  "generate-wiki",
								Usage: "インデックス完了後にWikiを自動生成",
							},
						},
						Action: appcli.SourceIndexGitAction,
					},
				},
			},
			{
				Name:  "wiki",
				Usage: "Wiki生成コマンド",
				Commands: []*cli.Command{
					{
						Name:  "generate",
						Usage: "プロダクト単位でWikiを生成",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:     "product",
								Usage:    "プロダクト名",
								Required: true,
							},
							&cli.StringFlag{
								Name:  "out",
								Usage: "出力ディレクトリ（省略時は /var/lib/dev-rag/wikis/<プロダクト名>）",
							},
							&cli.StringFlag{
								Name:  "config",
								Usage: "Wiki生成設定ファイル（省略時はデフォルト設定）",
							},
						},
						Action: appcli.WikiGenerateAction,
					},
				},
			},
			{
				Name:  "ask",
				Usage: "プロダクトに関する質問に回答",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "env",
						Usage: "環境変数ファイルパス",
						Value: ".env",
					},
					&cli.StringFlag{
						Name:     "product",
						Usage:    "プロダクト名",
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "show-sources",
						Usage: "参照したソースを表示",
						Value: false,
					},
				},
				ArgsUsage: "<質問文>",
				Action:    appcli.AskAction,
			},
			{
				Name:  "server",
				Usage: "サーバ関連コマンド",
				Commands: []*cli.Command{
					{
						Name:  "start",
						Usage: "HTTPサーバを起動",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.IntFlag{
								Name:  "port",
								Usage: "HTTPポート（省略時は環境変数またはデフォルトの8080）",
								Value: 8080,
							},
						},
						Action: appcli.ServerStartAction,
					},
				},
			},
		},
	}

	if err := app.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
