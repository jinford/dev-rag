package quality

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/indexer/llm"
	"github.com/jinford/dev-rag/pkg/models"
)

// MockQualityRepository は QualityRepositoryR のモック（テスト用）
type MockQualityRepository struct {
	notes []*models.QualityNote
}

func (m *MockQualityRepository) ListQualityNotes(ctx context.Context, filter *models.QualityNoteFilter) ([]*models.QualityNote, error) {
	return m.notes, nil
}

func (m *MockQualityRepository) GetQualityNoteByID(ctx context.Context, id uuid.UUID) (*models.QualityNote, error) {
	for _, note := range m.notes {
		if note.ID == id {
			return note, nil
		}
	}
	return nil, nil
}

func (m *MockQualityRepository) GetQualityNoteByNoteID(ctx context.Context, noteID string) (*models.QualityNote, error) {
	for _, note := range m.notes {
		if note.NoteID == noteID {
			return note, nil
		}
	}
	return nil, nil
}

func (m *MockQualityRepository) ListQualityNotesBySeverity(ctx context.Context, severity models.QualitySeverity) ([]*models.QualityNote, error) {
	var result []*models.QualityNote
	for _, note := range m.notes {
		if note.Severity == severity {
			result = append(result, note)
		}
	}
	return result, nil
}

func (m *MockQualityRepository) ListQualityNotesByStatus(ctx context.Context, status models.QualityStatus) ([]*models.QualityNote, error) {
	var result []*models.QualityNote
	for _, note := range m.notes {
		if note.Status == status {
			result = append(result, note)
		}
	}
	return result, nil
}

func (m *MockQualityRepository) ListQualityNotesByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*models.QualityNote, error) {
	var result []*models.QualityNote
	for _, note := range m.notes {
		if (note.CreatedAt.After(startDate) || note.CreatedAt.Equal(startDate)) &&
			(note.CreatedAt.Before(endDate) || note.CreatedAt.Equal(endDate)) {
			result = append(result, note)
		}
	}
	return result, nil
}

func (m *MockQualityRepository) GetRecentQualityNotes(ctx context.Context) ([]*models.QualityNote, error) {
	return m.notes, nil
}

// MockCodeownersParser は CodeownersParser のモック
type MockCodeownersParser struct {
	owners map[string]string
}

func (m *MockCodeownersParser) GetOwner(filePath string) string {
	if owner, ok := m.owners[filePath]; ok {
		return owner
	}
	return "unassigned"
}

// MockGitParser は GitParser のモック
type MockGitParser struct {
	changes []GitChange
}

func (m *MockGitParser) GetRecentChanges(since time.Time) ([]GitChange, error) {
	var result []GitChange
	for _, change := range m.changes {
		if change.Date.After(since) || change.Date.Equal(since) {
			result = append(result, change)
		}
	}
	return result, nil
}

// MockLLMClient は LLM クライアントのモック
type MockLLMClient struct {
	response string
}

func (m *MockLLMClient) GenerateCompletion(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	return llm.CompletionResponse{
		Content:       m.response,
		TokensUsed:    100,
		PromptVersion: "1.0",
		Model:         "gpt-4o-mini",
	}, nil
}

// MockActionBacklogRepository は ActionBacklogRepository のモック
type MockActionBacklogRepository struct {
	actions []*models.Action
}

func (m *MockActionBacklogRepository) CreateAction(ctx context.Context, action *models.Action) (*models.Action, error) {
	// ActionIDが設定されていない場合は生成
	if action.ActionID == "" {
		action.ActionID = "ACT-TEST-001"
	}
	// IDが設定されていない場合は生成
	if action.ID == uuid.Nil {
		action.ID = uuid.New()
	}
	m.actions = append(m.actions, action)
	return action, nil
}

func (m *MockActionBacklogRepository) GetActionByID(ctx context.Context, id uuid.UUID) (*models.Action, error) {
	for _, action := range m.actions {
		if action.ID == id {
			return action, nil
		}
	}
	return nil, nil
}

func (m *MockActionBacklogRepository) GetActionByActionID(ctx context.Context, actionID string) (*models.Action, error) {
	for _, action := range m.actions {
		if action.ActionID == actionID {
			return action, nil
		}
	}
	return nil, nil
}

func (m *MockActionBacklogRepository) ListActions(ctx context.Context, filter *models.ActionFilter) ([]*models.Action, error) {
	return m.actions, nil
}

func (m *MockActionBacklogRepository) UpdateActionStatus(ctx context.Context, id uuid.UUID, status string) error {
	for _, action := range m.actions {
		if action.ID == id {
			action.Status = models.ActionStatus(status)
			return nil
		}
	}
	return nil
}

func (m *MockActionBacklogRepository) CompleteAction(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	for _, action := range m.actions {
		if action.ID == id {
			action.Status = "completed"
			action.CompletedAt = &now
			return nil
		}
	}
	return nil
}

// MockMetricsCalculator は MetricsCalculator のモック
type MockMetricsCalculator struct {
	metrics *models.QualityMetrics
}

func (m *MockMetricsCalculator) CalculateMetrics() (*models.QualityMetrics, error) {
	return m.metrics, nil
}

// MockFreshnessCalculator は FreshnessCalculator のモック
type MockFreshnessCalculator struct {
	report *models.FreshnessReport
}

func (m *MockFreshnessCalculator) CalculateFreshness(threshold int) (*models.FreshnessReport, error) {
	return m.report, nil
}
