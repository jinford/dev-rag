package commands

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jinford/dev-rag/pkg/quality"
	"github.com/jinford/dev-rag/pkg/repository"
	"github.com/jinford/dev-rag/pkg/sqlc"
	"github.com/urfave/cli/v3"
)

// ReviewRunAction は週次レビューを手動実行するコマンドのアクション
// Phase 4タスク10: 週次レビューの自動化
func ReviewRunAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")
	weekRange := cmd.Int("week-range")
	notifyFile := cmd.String("notify-file")
	repoPath := cmd.String("repo-path")

	if weekRange == 0 {
		weekRange = 1 // デフォルト1週間
	}

	slog.Info("週次レビューを実行します", "weekRange", weekRange, "repoPath", repoPath)

	// AppContextの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return fmt.Errorf("AppContextの初期化に失敗: %w", err)
	}
	defer appCtx.Close()

	// リポジトリの初期化
	queries := sqlc.New(appCtx.Database.Pool)
	qualityRepo := repository.NewQualityRepositoryR(queries)
	actionRepo := repository.NewActionRepositoryRW(queries)

	// サービスの初期化
	gitParser := quality.NewGitLogParser()
	coParser := quality.NewCodeownersParser()
	weeklyReviewService := quality.NewWeeklyReviewService(qualityRepo, gitParser, coParser)
	actionGenerator := quality.NewActionGenerator(appCtx.LLMClient)

	// レビュー期間の計算
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -weekRange*7)

	slog.Info("レビュー期間", "startDate", startDate.Format("2006-01-02"), "endDate", endDate.Format("2006-01-02"))

	// 週次レビューデータの準備
	reviewData, err := weeklyReviewService.PrepareWeeklyReview(ctx, repoPath, startDate, endDate)
	if err != nil {
		return fmt.Errorf("週次レビューデータの準備に失敗: %w", err)
	}

	slog.Info("週次レビューデータを取得", "qualityNotes", len(reviewData.QualityNotes), "recentChanges", len(reviewData.RecentChanges))

	// アクション生成データに変換
	actionGenData := quality.FromWeeklyReviewData(reviewData)

	// アクションの生成
	actions, err := actionGenerator.GenerateActions(ctx, actionGenData)
	if err != nil {
		return fmt.Errorf("アクションの生成に失敗: %w", err)
	}

	slog.Info("アクションを生成", "count", len(actions))

	// アクションをDBに保存
	for i := range actions {
		_, err := actionRepo.CreateAction(ctx, &actions[i])
		if err != nil {
			slog.Error("アクションの保存に失敗", "error", err, "actionID", actions[i].ActionID)
			continue
		}
		slog.Info("アクションを保存", "actionID", actions[i].ActionID)
	}

	// 通知
	var notifier quality.Notifier
	if notifyFile != "" {
		notifier = quality.NewMultiNotifier(
			quality.NewStandardOutputNotifier(),
			quality.NewFileNotifier(notifyFile),
		)
	} else {
		notifier = quality.NewStandardOutputNotifier()
	}

	if err := notifier.Notify(actions); err != nil {
		return fmt.Errorf("通知に失敗: %w", err)
	}

	slog.Info("週次レビューが正常に完了しました")
	return nil
}

// ReviewScheduleAction は週次レビューをスケジュール実行するコマンドのアクション
// Phase 4タスク10: 週次レビューの自動化
func ReviewScheduleAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")
	cronSchedule := cmd.String("cron")
	weekRange := cmd.Int("week-range")
	notifyFile := cmd.String("notify-file")
	repoPath := cmd.String("repo-path")

	if cronSchedule == "" {
		cronSchedule = "0 9 * * 1" // デフォルト: 毎週月曜9:00
	}

	if weekRange == 0 {
		weekRange = 1 // デフォルト1週間
	}

	// Cron形式のスケジュール設定はまだ実装していません
	// 現時点では単に「スケジュール設定を受け取った」ことをログに出力するだけ
	slog.Info("週次レビューのスケジュール設定",
		"cronSchedule", cronSchedule,
		"weekRange", weekRange,
		"notifyFile", notifyFile,
		"repoPath", repoPath,
		"envFile", envFile)

	return fmt.Errorf("週次レビューのスケジュール実行機能は現在未実装です。\n" +
		"cron等の外部スケジューラから `dev-rag review run` コマンドを定期実行することを推奨します")
}
