package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jinford/dev-rag/cmd/dev-rag/commands"
	"github.com/urfave/cli/v3"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 構造化ログの設定
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

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
						Action: commands.ProductListAction,
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
						Action: commands.ProductShowAction,
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
						Action: commands.SourceListAction,
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
						Action: commands.SourceShowAction,
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
						Action: commands.SourceIndexGitAction,
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
						Action: commands.WikiGenerateAction,
					},
				},
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
						Action: commands.ServerStartAction,
					},
				},
			},
			{
				Name:  "action",
				Usage: "アクションバックログ管理コマンド",
				Commands: []*cli.Command{
					{
						Name:  "list",
						Usage: "アクションリストを表示",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:  "priority",
								Usage: "優先度でフィルタ (P1/P2/P3)",
							},
							&cli.StringFlag{
								Name:  "type",
								Usage: "アクション種別でフィルタ (reindex/doc_fix/test_update/investigate)",
							},
							&cli.StringFlag{
								Name:  "status",
								Usage: "ステータスでフィルタ (open/noop/completed)",
							},
						},
						Action: commands.ActionListAction,
					},
					{
						Name:  "show",
						Usage: "特定のアクションを詳細表示",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:     "action-id",
								Usage:    "Action ID",
								Required: true,
							},
						},
						Action: commands.ActionShowAction,
					},
					{
						Name:  "complete",
						Usage: "アクションを完了にする",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:     "action-id",
								Usage:    "Action ID",
								Required: true,
							},
						},
						Action: commands.ActionCompleteAction,
					},
					{
						Name:  "export",
						Usage: "バックログをエクスポート",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:  "format",
								Usage: "出力フォーマット (json/csv)",
								Value: "json",
							},
							&cli.StringFlag{
								Name:  "output",
								Usage: "出力ファイルパス",
								Value: "actions.json",
							},
						},
						Action: commands.ActionExportAction,
					},
					{
						Name:  "create",
						Usage: "新しいアクションを手動で作成",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:     "action-id",
								Usage:    "Action ID (例: ACT-2025-001)",
								Required: true,
							},
							&cli.StringFlag{
								Name:  "prompt-version",
								Usage: "プロンプトバージョン",
								Value: "1.0",
							},
							&cli.StringFlag{
								Name:     "priority",
								Usage:    "優先度 (P1/P2/P3)",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "type",
								Usage:    "アクション種別 (reindex/doc_fix/test_update/investigate)",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "title",
								Usage:    "タイトル",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "description",
								Usage:    "詳細説明",
								Required: true,
							},
							&cli.StringFlag{
								Name:  "owner-hint",
								Usage: "担当者ヒント",
							},
							&cli.StringFlag{
								Name:     "acceptance-criteria",
								Usage:    "受け入れ基準",
								Required: true,
							},
						},
						Action: commands.ActionCreateAction,
					},
				},
			},
			{
				Name:  "metrics",
				Usage: "品質メトリクス管理コマンド",
				Commands: []*cli.Command{
					{
						Name:  "quality",
						Usage: "品質メトリクスを表示",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:  "period",
								Usage: "集計期間 (weekly/monthly/all)",
								Value: "all",
							},
							&cli.StringFlag{
								Name:  "start-date",
								Usage: "開始日 (週次集計時に必須: YYYY-MM-DD)",
							},
							&cli.StringFlag{
								Name:  "end-date",
								Usage: "終了日 (週次集計時に必須: YYYY-MM-DD)",
							},
							&cli.IntFlag{
								Name:  "year",
								Usage: "年 (月次集計時に必須)",
							},
							&cli.IntFlag{
								Name:  "month",
								Usage: "月 (月次集計時に必須)",
							},
							&cli.StringFlag{
								Name:  "export",
								Usage: "JSON形式でエクスポート (ファイルパス)",
							},
						},
						Action: commands.MetricsQualityAction,
					},
				},
			},
			{
				Name:  "freshness",
				Usage: "インデックス鮮度監視コマンド",
				Commands: []*cli.Command{
					{
						Name:  "check",
						Usage: "鮮度レポートを表示",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.IntFlag{
								Name:  "threshold",
								Usage: "鮮度閾値（日数）",
								Value: 30,
							},
							&cli.StringFlag{
								Name:  "repo-path",
								Usage: "リポジトリパス",
								Value: ".",
							},
						},
						Action: commands.FreshnessCheckAction,
					},
					{
						Name:  "alert",
						Usage: "古いチャンクをアラート",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.IntFlag{
								Name:  "threshold",
								Usage: "鮮度閾値（日数）",
								Value: 30,
							},
							&cli.StringFlag{
								Name:  "export",
								Usage: "JSON形式でエクスポート（ファイルパス）",
							},
							&cli.StringFlag{
								Name:  "repo-path",
								Usage: "リポジトリパス",
								Value: ".",
							},
						},
						Action: commands.FreshnessAlertAction,
					},
					{
						Name:  "reindex",
						Usage: "古いチャンクを再インデックス",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.IntFlag{
								Name:  "threshold",
								Usage: "鮮度閾値（日数）",
								Value: 30,
							},
							&cli.BoolFlag{
								Name:  "dry-run",
								Usage: "ドライラン（アクション生成のみ）",
							},
							&cli.StringFlag{
								Name:  "repo-path",
								Usage: "リポジトリパス",
								Value: ".",
							},
						},
						Action: commands.FreshnessReindexAction,
					},
				},
			},
			{
				Name:  "report",
				Usage: "レポート生成コマンド",
				Commands: []*cli.Command{
					{
						Name:  "generate",
						Usage: "品質ダッシュボードHTMLレポートを生成",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:  "output",
								Usage: "出力ファイルパス",
								Value: "quality_report.html",
							},
							&cli.BoolFlag{
								Name:  "open",
								Usage: "生成後にブラウザで開く",
							},
							&cli.IntFlag{
								Name:  "freshness-threshold",
								Usage: "鮮度閾値（日数）",
								Value: 30,
							},
						},
						Action: commands.ReportGenerateAction,
					},
				},
			},
			{
				Name:  "review",
				Usage: "週次レビュー管理コマンド",
				Commands: []*cli.Command{
					{
						Name:  "run",
						Usage: "週次レビューを手動実行",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.IntFlag{
								Name:  "week-range",
								Usage: "レビュー対象の週数",
								Value: 1,
							},
							&cli.StringFlag{
								Name:  "notify-file",
								Usage: "通知を記録するファイルパス",
							},
							&cli.StringFlag{
								Name:  "repo-path",
								Usage: "リポジトリパス",
								Value: ".",
							},
						},
						Action: commands.ReviewRunAction,
					},
					{
						Name:  "schedule",
						Usage: "週次レビューをスケジュール実行",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:  "cron",
								Usage: "Cron形式のスケジュール (例: 0 9 * * 1 = 毎週月曜9:00)",
								Value: "0 9 * * 1",
							},
							&cli.IntFlag{
								Name:  "week-range",
								Usage: "レビュー対象の週数",
								Value: 1,
							},
							&cli.StringFlag{
								Name:  "notify-file",
								Usage: "通知を記録するファイルパス",
							},
							&cli.StringFlag{
								Name:  "repo-path",
								Usage: "リポジトリパス",
								Value: ".",
							},
						},
						Action: commands.ReviewScheduleAction,
					},
				},
			},
			{
				Name:  "feedback",
				Usage: "品質フィードバック管理コマンド",
				Commands: []*cli.Command{
					{
						Name:  "create",
						Usage: "新しいフィードバックを記録",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.BoolFlag{
								Name:  "interactive",
								Usage: "インタラクティブモードで入力",
							},
							&cli.StringFlag{
								Name:  "note-id",
								Usage: "ビジネス識別子 (例: QN-2025-001)",
							},
							&cli.StringFlag{
								Name:  "severity",
								Usage: "深刻度 (critical/high/medium/low)",
							},
							&cli.StringFlag{
								Name:  "text",
								Usage: "問題の詳細",
							},
							&cli.StringFlag{
								Name:  "reviewer",
								Usage: "レビュー者名",
							},
							&cli.StringFlag{
								Name:  "linked-files",
								Usage: "関連ファイル (カンマ区切り)",
							},
							&cli.StringFlag{
								Name:  "linked-chunks",
								Usage: "関連チャンクID (カンマ区切り)",
							},
						},
						Action: commands.FeedbackCreateAction,
					},
					{
						Name:  "list",
						Usage: "フィードバックリストを表示",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:  "severity",
								Usage: "深刻度でフィルタ (critical/high/medium/low)",
							},
							&cli.StringFlag{
								Name:  "status",
								Usage: "ステータスでフィルタ (open/resolved)",
							},
							&cli.IntFlag{
								Name:  "days",
								Usage: "過去N日間でフィルタ",
							},
							&cli.StringFlag{
								Name:  "reviewer",
								Usage: "レビュー者でフィルタ",
							},
						},
						Action: commands.FeedbackListAction,
					},
					{
						Name:  "show",
						Usage: "特定のフィードバックを詳細表示",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:     "note-id",
								Usage:    "Note ID",
								Required: true,
							},
						},
						Action: commands.FeedbackShowAction,
					},
					{
						Name:  "resolve",
						Usage: "フィードバックを解決済みにする",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:     "note-id",
								Usage:    "Note ID",
								Required: true,
							},
						},
						Action: commands.FeedbackResolveAction,
					},
					{
						Name:  "import",
						Usage: "JSON形式から一括インポート",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "env",
								Usage: "環境変数ファイルパス",
								Value: ".env",
							},
							&cli.StringFlag{
								Name:     "file",
								Usage:    "JSONファイルパス",
								Required: true,
							},
						},
						Action: commands.FeedbackImportAction,
					},
				},
			},
		},
	}

	if err := app.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
