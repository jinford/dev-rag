package quality

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStandardOutputNotifier_Notify(t *testing.T) {
	notifier := NewStandardOutputNotifier()

	actions := []models.Action{
		{
			ActionID:           "ACT-2025-001",
			Priority:           "P1",
			ActionType:         "reindex",
			Title:              "pkg/quality をreindex",
			Description:        "quality パッケージが古くなっています",
			OwnerHint:          "team-backend",
			Status:             "open",
			AcceptanceCriteria: "インデックスが最新コミットになっていること",
		},
	}

	err := notifier.Notify(actions)
	assert.NoError(t, err)
}

func TestStandardOutputNotifier_NotifyEmpty(t *testing.T) {
	notifier := NewStandardOutputNotifier()

	actions := []models.Action{}

	err := notifier.Notify(actions)
	assert.NoError(t, err)
}

func TestFileNotifier_Notify(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "weekly_review.txt")

	notifier := NewFileNotifier(filePath)

	actions := []models.Action{
		{
			ActionID:           "ACT-2025-001",
			Priority:           "P1",
			ActionType:         "reindex",
			Title:              "pkg/quality をreindex",
			Description:        "quality パッケージが古くなっています",
			OwnerHint:          "team-backend",
			Status:             "open",
			AcceptanceCriteria: "インデックスが最新コミットになっていること",
			LinkedFiles:        []string{"pkg/quality/report_generator.go"},
		},
	}

	err := notifier.Notify(actions)
	require.NoError(t, err)

	// ファイルが作成されたことを確認
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	// ファイルの内容を確認
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "週次レビュー結果")
	assert.Contains(t, contentStr, "ACT-2025-001")
	assert.Contains(t, contentStr, "pkg/quality をreindex")
	assert.Contains(t, contentStr, "team-backend")
	assert.Contains(t, contentStr, "pkg/quality/report_generator.go")
}

func TestFileNotifier_NotifyEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "weekly_review.txt")

	notifier := NewFileNotifier(filePath)

	actions := []models.Action{}

	err := notifier.Notify(actions)
	require.NoError(t, err)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "生成されたアクションはありません")
}

func TestFileNotifier_NotifyAppend(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "weekly_review.txt")

	notifier := NewFileNotifier(filePath)

	// 1回目の通知
	actions1 := []models.Action{
		{
			ActionID:   "ACT-2025-001",
			Priority:   "P1",
			ActionType: "reindex",
			Title:      "First action",
		},
	}

	err := notifier.Notify(actions1)
	require.NoError(t, err)

	// 2回目の通知（追記される）
	actions2 := []models.Action{
		{
			ActionID:   "ACT-2025-002",
			Priority:   "P2",
			ActionType: "doc_fix",
			Title:      "Second action",
		},
	}

	err = notifier.Notify(actions2)
	require.NoError(t, err)

	// ファイルの内容を確認
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "ACT-2025-001")
	assert.Contains(t, contentStr, "First action")
	assert.Contains(t, contentStr, "ACT-2025-002")
	assert.Contains(t, contentStr, "Second action")
}

func TestMultiNotifier_Notify(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "weekly_review.txt")

	stdoutNotifier := NewStandardOutputNotifier()
	fileNotifier := NewFileNotifier(filePath)

	multiNotifier := NewMultiNotifier(stdoutNotifier, fileNotifier)

	actions := []models.Action{
		{
			ActionID:   "ACT-2025-001",
			Priority:   "P1",
			ActionType: "reindex",
			Title:      "Test action",
		},
	}

	err := multiNotifier.Notify(actions)
	require.NoError(t, err)

	// ファイルにも書き込まれていることを確認
	_, err = os.Stat(filePath)
	require.NoError(t, err)
}

func TestMultiNotifier_NotifyWithError(t *testing.T) {
	// 無効なパスを指定してエラーを発生させる
	invalidNotifier := NewFileNotifier("/invalid/path/weekly_review.txt")
	stdoutNotifier := NewStandardOutputNotifier()

	multiNotifier := NewMultiNotifier(stdoutNotifier, invalidNotifier)

	actions := []models.Action{
		{
			ActionID:   "ACT-2025-001",
			Priority:   "P1",
			ActionType: "reindex",
			Title:      "Test action",
		},
	}

	err := multiNotifier.Notify(actions)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "一部の通知に失敗しました")
}
