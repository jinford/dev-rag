package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
)

// TestActionModel は Action 構造体が正しく定義されていることを確認します
func TestActionModel(t *testing.T) {
	now := time.Now()
	id := uuid.New()

	action := &models.Action{
		ID:                 id,
		ActionID:           "ACT-2025-001",
		PromptVersion:      "1.0",
		Priority:           models.ActionPriorityP1,
		ActionType:         models.ActionTypeReindex,
		Title:              "ファイルの再インデックス",
		Description:        "特定のファイルを再インデックスする必要があります",
		LinkedFiles:        []string{"src/main.go", "src/handler.go"},
		OwnerHint:          "backend-team",
		AcceptanceCriteria: "全てのファイルが正常にインデックスされること",
		Status:             models.ActionStatusOpen,
		CreatedAt:          now,
		CompletedAt:        nil,
	}

	assert.Equal(t, id, action.ID)
	assert.Equal(t, "ACT-2025-001", action.ActionID)
	assert.Equal(t, "1.0", action.PromptVersion)
	assert.Equal(t, models.ActionPriorityP1, action.Priority)
	assert.Equal(t, models.ActionTypeReindex, action.ActionType)
	assert.Equal(t, "ファイルの再インデックス", action.Title)
	assert.Equal(t, 2, len(action.LinkedFiles))
	assert.Equal(t, "backend-team", action.OwnerHint)
	assert.Equal(t, models.ActionStatusOpen, action.Status)
	assert.Equal(t, now, action.CreatedAt)
	assert.Nil(t, action.CompletedAt)
	assert.True(t, action.IsPending())
	assert.False(t, action.IsCompleted())
	assert.True(t, action.IsP1())
}

// TestActionPriorityEnum は ActionPriority の列挙値をテストします
func TestActionPriorityEnum(t *testing.T) {
	tests := []struct {
		name     string
		priority models.ActionPriority
		expected string
	}{
		{"P1", models.ActionPriorityP1, "P1"},
		{"P2", models.ActionPriorityP2, "P2"},
		{"P3", models.ActionPriorityP3, "P3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.priority))
		})
	}
}

// TestActionTypeEnum は ActionType の列挙値をテストします
func TestActionTypeEnum(t *testing.T) {
	tests := []struct {
		name       string
		actionType models.ActionType
		expected   string
	}{
		{"reindex", models.ActionTypeReindex, "reindex"},
		{"doc_fix", models.ActionTypeDocFix, "doc_fix"},
		{"test_update", models.ActionTypeTestUpdate, "test_update"},
		{"investigate", models.ActionTypeInvestigate, "investigate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.actionType))
		})
	}
}

// TestActionStatusEnum は ActionStatus の列挙値をテストします
func TestActionStatusEnum(t *testing.T) {
	tests := []struct {
		name     string
		status   models.ActionStatus
		expected string
	}{
		{"open", models.ActionStatusOpen, "open"},
		{"noop", models.ActionStatusNoop, "noop"},
		{"completed", models.ActionStatusCompleted, "completed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

// TestActionHelperMethods は Action のヘルパーメソッドをテストします
func TestActionHelperMethods(t *testing.T) {
	tests := []struct {
		name          string
		status        models.ActionStatus
		priority      models.ActionPriority
		expectPending bool
		expectComplete bool
		expectNoop    bool
		expectP1      bool
	}{
		{
			name:           "Open P1",
			status:         models.ActionStatusOpen,
			priority:       models.ActionPriorityP1,
			expectPending:  true,
			expectComplete: false,
			expectNoop:     false,
			expectP1:       true,
		},
		{
			name:           "Completed P2",
			status:         models.ActionStatusCompleted,
			priority:       models.ActionPriorityP2,
			expectPending:  false,
			expectComplete: true,
			expectNoop:     false,
			expectP1:       false,
		},
		{
			name:           "Noop P3",
			status:         models.ActionStatusNoop,
			priority:       models.ActionPriorityP3,
			expectPending:  false,
			expectComplete: false,
			expectNoop:     true,
			expectP1:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := &models.Action{
				Status:   tt.status,
				Priority: tt.priority,
			}

			assert.Equal(t, tt.expectPending, action.IsPending())
			assert.Equal(t, tt.expectComplete, action.IsCompleted())
			assert.Equal(t, tt.expectNoop, action.IsNoop())
			assert.Equal(t, tt.expectP1, action.IsP1())
		})
	}
}
