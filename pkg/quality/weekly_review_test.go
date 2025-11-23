package quality

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WeeklyReviewMockQualityRepository はテスト用のモックリポジトリです
type WeeklyReviewMockQualityRepository struct {
	notes []*models.QualityNote
}

func (m *WeeklyReviewMockQualityRepository) ListQualityNotesByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*models.QualityNote, error) {
	// 期間内のノートをフィルタ
	var filtered []*models.QualityNote
	for _, note := range m.notes {
		if note.CreatedAt.After(startDate) && note.CreatedAt.Before(endDate) {
			filtered = append(filtered, note)
		}
	}
	return filtered, nil
}

func TestPrepareWeeklyReview(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tmpDir := t.TempDir()

	// CODEOWNERSファイルを作成
	codeownersPath := filepath.Join(tmpDir, "CODEOWNERS")
	content := `* @default-owner
*.go @go-team
`
	err := os.WriteFile(codeownersPath, []byte(content), 0644)
	require.NoError(t, err)

	// モックの品質ノートを作成
	startDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 22, 0, 0, 0, 0, time.UTC)

	mockRepo := &WeeklyReviewMockQualityRepository{
		notes: []*models.QualityNote{
			{
				ID:           uuid.New(),
				NoteID:       "QN-2024-001",
				Severity:     models.QualitySeverityHigh,
				NoteText:     "Missing documentation",
				LinkedFiles:  []string{"README.md"},
				LinkedChunks: []string{"chunk-1"},
				Reviewer:     "reviewer1",
				Status:       models.QualityStatusOpen,
				CreatedAt:    time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			},
			{
				ID:           uuid.New(),
				NoteID:       "QN-2024-002",
				Severity:     models.QualitySeverityCritical,
				NoteText:     "Outdated API reference",
				LinkedFiles:  []string{"api/v1/users.go"},
				LinkedChunks: []string{"chunk-2"},
				Reviewer:     "reviewer2",
				Status:       models.QualityStatusOpen,
				CreatedAt:    time.Date(2024, 1, 18, 14, 0, 0, 0, time.UTC),
			},
		},
	}

	// モックのGitLogParserを作成
	gitParser := &MockGitLogParser{
		commits: []GitCommit{
			{
				Hash:         "abc123",
				FilesChanged: []string{"README.md", "main.go"},
				MergedAt:     time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
				Author:       "John Doe",
				Message:      "Update documentation",
			},
		},
	}

	// サービスを作成
	coParser := NewCodeownersParser()
	service := &WeeklyReviewService{
		qualityRepo: mockRepo,
		gitParser:   gitParser,
		coParser:    coParser,
	}

	// PrepareWeeklyReview を実行
	result, err := service.PrepareWeeklyReview(context.Background(), tmpDir, startDate, endDate)
	require.NoError(t, err)

	// 検証
	assert.Equal(t, startDate, result.WeekRange.StartDate)
	assert.Equal(t, endDate, result.WeekRange.EndDate)
	assert.Len(t, result.QualityNotes, 2)
	assert.Len(t, result.RecentChanges, 1)
	assert.Len(t, result.CodeownersLookup, 2)

	// QualityNotesの検証
	assert.Equal(t, "QN-2024-001", result.QualityNotes[0].NoteID)
	assert.Equal(t, models.QualitySeverityHigh, result.QualityNotes[0].Severity)

	// RecentChangesの検証
	assert.Equal(t, "abc123", result.RecentChanges[0].Hash)
	assert.Equal(t, []string{"README.md", "main.go"}, result.RecentChanges[0].FilesChanged)

	// CodeownersLookupの検証
	assert.Equal(t, []string{"@default-owner"}, result.CodeownersLookup["*"])
	assert.Equal(t, []string{"@go-team"}, result.CodeownersLookup["*.go"])
}

func TestToJSON(t *testing.T) {
	startDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 22, 0, 0, 0, 0, time.UTC)

	data := &WeeklyReviewData{
		WeekRange: WeekRange{
			StartDate: startDate,
			EndDate:   endDate,
		},
		QualityNotes: []*models.QualityNote{
			{
				ID:           uuid.New(),
				NoteID:       "QN-2024-001",
				Severity:     models.QualitySeverityHigh,
				NoteText:     "Test note",
				LinkedFiles:  []string{"test.go"},
				LinkedChunks: []string{"chunk-1"},
				Reviewer:     "reviewer1",
				Status:       models.QualityStatusOpen,
				CreatedAt:    time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			},
		},
		RecentChanges: []RecentChange{
			{
				Hash:         "abc123",
				FilesChanged: []string{"test.go"},
				MergedAt:     time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
				Author:       "John Doe",
				Message:      "Add feature",
			},
		},
		CodeownersLookup: map[string][]string{
			"*":    {"@default-owner"},
			"*.go": {"@go-team"},
		},
	}

	// ToJSON を実行
	jsonStr, err := data.ToJSON()
	require.NoError(t, err)

	// JSON文字列が正しく生成されることを確認
	assert.Contains(t, jsonStr, "weekRange")
	assert.Contains(t, jsonStr, "qualityNotes")
	assert.Contains(t, jsonStr, "recentChanges")
	assert.Contains(t, jsonStr, "codeownersLookup")
	assert.Contains(t, jsonStr, "QN-2024-001")
	assert.Contains(t, jsonStr, "abc123")
}
