package quality

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
	"github.com/robfig/cron/v3"
)

// WeeklyReviewJobConfig は週次レビュージョブの設定です
type WeeklyReviewJobConfig struct {
	CronSchedule string    // Cron形式のスケジュール（例: "0 9 * * 1" = 毎週月曜9:00）
	WeekRange    int       // レビュー対象の週数（例: 1 = 過去1週間）
	Notifier     Notifier  // 通知先
}

// WeeklyReviewJob は週次レビューを自動実行するジョブです
type WeeklyReviewJob struct {
	config            *WeeklyReviewJobConfig
	weeklyReview      *WeeklyReview
	actionGenerator   *ActionGenerator
	actionRepo        ActionBacklogRepositoryInterface
	cron              *cron.Cron
	logger            *slog.Logger
}

// NewWeeklyReviewJob は新しいWeeklyReviewJobを作成します
func NewWeeklyReviewJob(
	config *WeeklyReviewJobConfig,
	weeklyReview *WeeklyReview,
	actionGenerator *ActionGenerator,
	actionRepo ActionBacklogRepositoryInterface,
	logger *slog.Logger,
) *WeeklyReviewJob {
	if logger == nil {
		logger = slog.Default()
	}

	return &WeeklyReviewJob{
		config:          config,
		weeklyReview:    weeklyReview,
		actionGenerator: actionGenerator,
		actionRepo:      actionRepo,
		cron:            cron.New(),
		logger:          logger,
	}
}

// Start はスケジューラーを起動します
func (j *WeeklyReviewJob) Start() error {
	_, err := j.cron.AddFunc(j.config.CronSchedule, func() {
		if err := j.Run(context.Background()); err != nil {
			j.logger.Error("週次レビュージョブの実行に失敗しました", "error", err)
		}
	})
	if err != nil {
		return fmt.Errorf("cron ジョブの登録に失敗: %w", err)
	}

	j.cron.Start()
	j.logger.Info("週次レビュージョブを開始しました", "schedule", j.config.CronSchedule)

	return nil
}

// Stop はスケジューラーを停止します
func (j *WeeklyReviewJob) Stop() {
	j.cron.Stop()
	j.logger.Info("週次レビュージョブを停止しました")
}

// Run は週次レビューを実行します（手動実行可能）
func (j *WeeklyReviewJob) Run(ctx context.Context) error {
	j.logger.Info("週次レビューを開始します")

	// レビュー対象期間を計算
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -7*j.config.WeekRange)

	j.logger.Info("レビュー対象期間",
		"start", startDate.Format("2006-01-02"),
		"end", endDate.Format("2006-01-02"))

	// 週次レビューデータを準備
	reviewData, err := j.weeklyReview.PrepareWeeklyReviewData(ctx, startDate, endDate)
	if err != nil {
		return fmt.Errorf("週次レビューデータの準備に失敗: %w", err)
	}

	j.logger.Info("週次レビューデータを準備しました",
		"notes_count", len(reviewData.QualityNotes))

	// 品質ノートが0件の場合はスキップ
	if len(reviewData.QualityNotes) == 0 {
		j.logger.Info("品質ノートが0件のため、アクション生成をスキップします")
		if j.config.Notifier != nil {
			return j.config.Notifier.Notify([]models.Action{})
		}
		return nil
	}

	// アクションを生成
	actions, err := j.actionGenerator.GenerateActions(ctx, reviewData)
	if err != nil {
		return fmt.Errorf("アクションの生成に失敗: %w", err)
	}

	j.logger.Info("アクションを生成しました", "count", len(actions))

	// 生成されたアクションをバックログに登録
	createdActions := []models.Action{}
	for _, action := range actions {
		// noopステータスのアクションはスキップ
		if action.Status == "noop" {
			j.logger.Info("noopアクションをスキップ", "action_id", action.ActionID, "title", action.Title)
			continue
		}

		createdAction, err := j.actionRepo.CreateAction(ctx, &action)
		if err != nil {
			j.logger.Error("アクションの作成に失敗しました",
				"action_id", action.ActionID,
				"error", err)
			continue
		}

		createdActions = append(createdActions, *createdAction)
		j.logger.Info("アクションを作成しました",
			"action_id", createdAction.ActionID,
			"priority", createdAction.Priority,
			"type", createdAction.ActionType)
	}

	j.logger.Info("アクションをバックログに登録しました", "created_count", len(createdActions))

	// 通知を送信
	if j.config.Notifier != nil {
		if err := j.config.Notifier.Notify(createdActions); err != nil {
			j.logger.Error("通知の送信に失敗しました", "error", err)
			return fmt.Errorf("通知の送信に失敗: %w", err)
		}
		j.logger.Info("通知を送信しました")
	}

	j.logger.Info("週次レビューが完了しました")

	return nil
}

// WaitForSchedule はスケジューラーが停止するまで待機します（ブロッキング）
func (j *WeeklyReviewJob) WaitForSchedule() {
	// シグナルを待つなど、適切な実装を行う
	select {}
}
