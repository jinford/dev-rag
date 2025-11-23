package quality

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWeeklyReviewJob_Run(t *testing.T) {
	// モックの準備
	mockQualityRepo := &MockQualityRepository{
		notes: []*models.QualityNote{
			{
				NoteID:      "QN-2025-001",
				Severity:    models.QualitySeverityCritical,
				NoteText:    "critical issue",
				LinkedFiles: []string{"pkg/quality/report_generator.go"},
				Reviewer:    "test-reviewer",
				Status:      models.QualityStatusOpen,
				CreatedAt:   time.Now().AddDate(0, 0, -3),
			},
		},
	}

	mockCodeownersParser := &MockCodeownersParser{
		owners: map[string]string{
			"pkg/quality/report_generator.go": "team-backend",
		},
	}

	mockGitParser := &MockGitParser{
		changes: []GitChange{
			{
				CommitHash: "abc123",
				Date:       time.Now().AddDate(0, 0, -1),
				Author:     "test-author",
				Message:    "fix: bug fix",
				Files:      []string{"pkg/quality/report_generator.go"},
			},
		},
	}

	mockLLMClient := &MockLLMClient{
		response: `[
			{
				"prompt_version": "1.0",
				"priority": "P1",
				"action_type": "reindex",
				"title": "pkg/quality をreindex",
				"description": "critical issue が報告されています",
				"linked_files": ["pkg/quality/report_generator.go"],
				"owner_hint": "team-backend",
				"acceptance_criteria": "インデックスが最新になっていること",
				"status": "open"
			}
		]`,
	}

	mockActionRepo := &MockActionBacklogRepository{
		actions: []*models.Action{},
	}

	weeklyReview := NewWeeklyReview(
		mockQualityRepo,
		mockCodeownersParser,
		mockGitParser,
		"/test/repo",
	)

	actionGenerator := NewActionGenerator(mockLLMClient)

	notifier := NewStandardOutputNotifier()

	config := &WeeklyReviewJobConfig{
		CronSchedule: "0 9 * * 1", // 毎週月曜9:00
		WeekRange:    1,            // 過去1週間
		Notifier:     notifier,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	job := NewWeeklyReviewJob(config, weeklyReview, actionGenerator, mockActionRepo, logger)

	// ジョブを実行
	err := job.Run(context.Background())
	require.NoError(t, err)

	// アクションが作成されたことを確認
	assert.Len(t, mockActionRepo.actions, 1)
	assert.Equal(t, models.ActionPriorityP1, mockActionRepo.actions[0].Priority)
	assert.Equal(t, models.ActionTypeReindex, mockActionRepo.actions[0].ActionType)
	assert.Equal(t, "team-backend", mockActionRepo.actions[0].OwnerHint)
}

func TestWeeklyReviewJob_Run_NoNotes(t *testing.T) {
	// 品質ノートが0件の場合のテスト
	mockQualityRepo := &MockQualityRepository{
		notes: []*models.QualityNote{},
	}

	mockCodeownersParser := &MockCodeownersParser{
		owners: map[string]string{},
	}

	mockGitParser := &MockGitParser{
		changes: []GitChange{},
	}

	mockLLMClient := &MockLLMClient{}

	mockActionRepo := &MockActionBacklogRepository{
		actions: []*models.Action{},
	}

	weeklyReview := NewWeeklyReview(
		mockQualityRepo,
		mockCodeownersParser,
		mockGitParser,
		"/test/repo",
	)

	actionGenerator := NewActionGenerator(mockLLMClient)

	notifier := NewStandardOutputNotifier()

	config := &WeeklyReviewJobConfig{
		CronSchedule: "0 9 * * 1",
		WeekRange:    1,
		Notifier:     notifier,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	job := NewWeeklyReviewJob(config, weeklyReview, actionGenerator, mockActionRepo, logger)

	// ジョブを実行
	err := job.Run(context.Background())
	require.NoError(t, err)

	// アクションが作成されていないことを確認
	assert.Len(t, mockActionRepo.actions, 0)
}

func TestWeeklyReviewJob_Run_NoopActions(t *testing.T) {
	// noopアクションはバックログに登録されないことを確認
	mockQualityRepo := &MockQualityRepository{
		notes: []*models.QualityNote{
			{
				NoteID:      "QN-2025-001",
				Severity:    models.QualitySeverityLow,
				NoteText:    "low priority issue",
				LinkedFiles: []string{"test.go"},
				Reviewer:    "test-reviewer",
				Status:      models.QualityStatusOpen,
				CreatedAt:   time.Now().AddDate(0, 0, -3),
			},
		},
	}

	mockCodeownersParser := &MockCodeownersParser{
		owners: map[string]string{},
	}

	mockGitParser := &MockGitParser{
		changes: []GitChange{
			{
				CommitHash: "abc123",
				Date:       time.Now().AddDate(0, 0, -1),
				Author:     "test-author",
				Message:    "fix: already fixed",
				Files:      []string{"test.go"},
			},
		},
	}

	mockLLMClient := &MockLLMClient{
		response: `[
			{
				"prompt_version": "1.0",
				"priority": "P3",
				"action_type": "investigate",
				"title": "Already fixed",
				"description": "This issue was already fixed in abc123",
				"linked_files": ["test.go"],
				"owner_hint": "unassigned",
				"acceptance_criteria": "N/A",
				"status": "noop"
			}
		]`,
	}

	mockActionRepo := &MockActionBacklogRepository{
		actions: []*models.Action{},
	}

	weeklyReview := NewWeeklyReview(
		mockQualityRepo,
		mockCodeownersParser,
		mockGitParser,
		"/test/repo",
	)

	actionGenerator := NewActionGenerator(mockLLMClient)

	notifier := NewStandardOutputNotifier()

	config := &WeeklyReviewJobConfig{
		CronSchedule: "0 9 * * 1",
		WeekRange:    1,
		Notifier:     notifier,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	job := NewWeeklyReviewJob(config, weeklyReview, actionGenerator, mockActionRepo, logger)

	// ジョブを実行
	err := job.Run(context.Background())
	require.NoError(t, err)

	// noopアクションは登録されないことを確認
	assert.Len(t, mockActionRepo.actions, 0)
}

func TestWeeklyReviewJob_StartStop(t *testing.T) {
	// スケジューラーの起動・停止のテスト
	mockQualityRepo := &MockQualityRepository{notes: []*models.QualityNote{}}
	mockCodeownersParser := &MockCodeownersParser{owners: map[string]string{}}
	mockGitParser := &MockGitParser{changes: []GitChange{}}
	mockLLMClient := &MockLLMClient{}
	mockActionRepo := &MockActionBacklogRepository{actions: []*models.Action{}}

	weeklyReview := NewWeeklyReview(mockQualityRepo, mockCodeownersParser, mockGitParser, "/test/repo")
	actionGenerator := NewActionGenerator(mockLLMClient)
	notifier := NewStandardOutputNotifier()

	config := &WeeklyReviewJobConfig{
		CronSchedule: "* * * * *", // 毎分実行（テスト用）
		WeekRange:    1,
		Notifier:     notifier,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	job := NewWeeklyReviewJob(config, weeklyReview, actionGenerator, mockActionRepo, logger)

	// スケジューラーを起動
	err := job.Start()
	require.NoError(t, err)

	// 少し待機
	time.Sleep(100 * time.Millisecond)

	// スケジューラーを停止
	job.Stop()

	// エラーなく起動・停止できたことを確認
	assert.NoError(t, err)
}
