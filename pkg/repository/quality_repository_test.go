package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
)

// TestQualityNoteModel は QualityNote 構造体が正しく定義されていることを確認します
func TestQualityNoteModel(t *testing.T) {
	now := time.Now()
	id := uuid.New()

	note := &models.QualityNote{
		ID:           id,
		NoteID:       "QN-2024-001",
		Severity:     models.QualitySeverityCritical,
		NoteText:     "重大な問題が見つかりました",
		LinkedFiles:  []string{"src/main.go", "src/handler.go"},
		LinkedChunks: []string{"chunk-001", "chunk-002"},
		Reviewer:     "reviewer@example.com",
		Status:       models.QualityStatusOpen,
		CreatedAt:    now,
		ResolvedAt:   nil,
	}

	assert.Equal(t, id, note.ID)
	assert.Equal(t, "QN-2024-001", note.NoteID)
	assert.Equal(t, models.QualitySeverityCritical, note.Severity)
	assert.Equal(t, "重大な問題が見つかりました", note.NoteText)
	assert.Equal(t, 2, len(note.LinkedFiles))
	assert.Equal(t, 2, len(note.LinkedChunks))
	assert.Equal(t, "reviewer@example.com", note.Reviewer)
	assert.Equal(t, models.QualityStatusOpen, note.Status)
	assert.Equal(t, now, note.CreatedAt)
	assert.Nil(t, note.ResolvedAt)
	assert.False(t, note.IsResolved())
	assert.True(t, note.IsCritical())
}

// TestQualitySeverityEnum は QualitySeverity の列挙値をテストします
func TestQualitySeverityEnum(t *testing.T) {
	tests := []struct {
		name     string
		severity models.QualitySeverity
		expected string
	}{
		{
			name:     "critical severity",
			severity: models.QualitySeverityCritical,
			expected: "critical",
		},
		{
			name:     "high severity",
			severity: models.QualitySeverityHigh,
			expected: "high",
		},
		{
			name:     "medium severity",
			severity: models.QualitySeverityMedium,
			expected: "medium",
		},
		{
			name:     "low severity",
			severity: models.QualitySeverityLow,
			expected: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.severity))
		})
	}
}

// TestQualityStatusEnum は QualityStatus の列挙値をテストします
func TestQualityStatusEnum(t *testing.T) {
	tests := []struct {
		name     string
		status   models.QualityStatus
		expected string
	}{
		{
			name:     "open status",
			status:   models.QualityStatusOpen,
			expected: "open",
		},
		{
			name:     "resolved status",
			status:   models.QualityStatusResolved,
			expected: "resolved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

// TestQualityNoteIsResolved は IsResolved メソッドをテストします
func TestQualityNoteIsResolved(t *testing.T) {
	tests := []struct {
		name     string
		status   models.QualityStatus
		expected bool
	}{
		{
			name:     "open note is not resolved",
			status:   models.QualityStatusOpen,
			expected: false,
		},
		{
			name:     "resolved note is resolved",
			status:   models.QualityStatusResolved,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := &models.QualityNote{
				Status: tt.status,
			}
			assert.Equal(t, tt.expected, note.IsResolved())
		})
	}
}

// TestQualityNoteIsCritical は IsCritical メソッドをテストします
func TestQualityNoteIsCritical(t *testing.T) {
	tests := []struct {
		name     string
		severity models.QualitySeverity
		expected bool
	}{
		{
			name:     "critical severity is critical",
			severity: models.QualitySeverityCritical,
			expected: true,
		},
		{
			name:     "high severity is not critical",
			severity: models.QualitySeverityHigh,
			expected: false,
		},
		{
			name:     "medium severity is not critical",
			severity: models.QualitySeverityMedium,
			expected: false,
		},
		{
			name:     "low severity is not critical",
			severity: models.QualitySeverityLow,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := &models.QualityNote{
				Severity: tt.severity,
			}
			assert.Equal(t, tt.expected, note.IsCritical())
		})
	}
}

// TestQualityNoteFilter は QualityNoteFilter 構造体をテストします
func TestQualityNoteFilter(t *testing.T) {
	severityCritical := models.QualitySeverityCritical
	statusOpen := models.QualityStatusOpen
	startDate := time.Now().AddDate(0, 0, -7)
	endDate := time.Now()
	limit := 10

	filter := &models.QualityNoteFilter{
		Severity:  &severityCritical,
		Status:    &statusOpen,
		StartDate: &startDate,
		EndDate:   &endDate,
		Limit:     &limit,
	}

	assert.NotNil(t, filter.Severity)
	assert.Equal(t, models.QualitySeverityCritical, *filter.Severity)
	assert.NotNil(t, filter.Status)
	assert.Equal(t, models.QualityStatusOpen, *filter.Status)
	assert.NotNil(t, filter.StartDate)
	assert.NotNil(t, filter.EndDate)
	assert.NotNil(t, filter.Limit)
	assert.Equal(t, 10, *filter.Limit)
}

// TestQualityNoteFilterEmpty は空のフィルタをテストします
func TestQualityNoteFilterEmpty(t *testing.T) {
	filter := &models.QualityNoteFilter{}

	assert.Nil(t, filter.Severity)
	assert.Nil(t, filter.Status)
	assert.Nil(t, filter.StartDate)
	assert.Nil(t, filter.EndDate)
	assert.Nil(t, filter.Limit)
}

// TestQualityMetrics は QualityMetrics 構造体をテストします
func TestQualityMetrics(t *testing.T) {
	now := time.Now()

	metrics := &models.QualityMetrics{
		TotalNotes:    100,
		OpenNotes:     30,
		ResolvedNotes: 70,
		BySeverity: map[models.QualitySeverity]int{
			models.QualitySeverityCritical: 5,
			models.QualitySeverityHigh:     15,
			models.QualitySeverityMedium:   50,
			models.QualitySeverityLow:      30,
		},
		RecentTrend: []models.QualityTrendPoint{
			{
				Date:          now.AddDate(0, 0, -7),
				OpenCount:     40,
				ResolvedCount: 60,
			},
			{
				Date:          now,
				OpenCount:     30,
				ResolvedCount: 70,
			},
		},
		GeneratedAt: now,
	}

	assert.Equal(t, 100, metrics.TotalNotes)
	assert.Equal(t, 30, metrics.OpenNotes)
	assert.Equal(t, 70, metrics.ResolvedNotes)
	assert.Equal(t, 4, len(metrics.BySeverity))
	assert.Equal(t, 5, metrics.BySeverity[models.QualitySeverityCritical])
	assert.Equal(t, 2, len(metrics.RecentTrend))
	assert.Equal(t, now, metrics.GeneratedAt)
}

// TestQualityNoteWithLinkedData は関連データを持つ QualityNote をテストします
func TestQualityNoteWithLinkedData(t *testing.T) {
	note := &models.QualityNote{
		ID:       uuid.New(),
		NoteID:   "QN-2024-002",
		Severity: models.QualitySeverityHigh,
		NoteText: "ドキュメントが古い",
		LinkedFiles: []string{
			"docs/architecture/design.md",
			"docs/api/endpoints.md",
		},
		LinkedChunks: []string{
			uuid.New().String(),
			uuid.New().String(),
			uuid.New().String(),
		},
		Reviewer:  "dev-team@example.com",
		Status:    models.QualityStatusOpen,
		CreatedAt: time.Now(),
	}

	assert.Equal(t, 2, len(note.LinkedFiles))
	assert.Equal(t, 3, len(note.LinkedChunks))
	assert.Contains(t, note.LinkedFiles[0], "docs/architecture")
	assert.Contains(t, note.LinkedFiles[1], "docs/api")
}

// TestQualityNoteWithEmptyLinkedData は関連データが空の QualityNote をテストします
func TestQualityNoteWithEmptyLinkedData(t *testing.T) {
	note := &models.QualityNote{
		ID:           uuid.New(),
		NoteID:       "QN-2024-003",
		Severity:     models.QualitySeverityLow,
		NoteText:     "軽微な問題",
		LinkedFiles:  []string{},
		LinkedChunks: []string{},
		Reviewer:     "qa@example.com",
		Status:       models.QualityStatusOpen,
		CreatedAt:    time.Now(),
	}

	assert.Empty(t, note.LinkedFiles)
	assert.Empty(t, note.LinkedChunks)
}

// TestQualityNoteResolvedScenario は解決済みシナリオをテストします
func TestQualityNoteResolvedScenario(t *testing.T) {
	now := time.Now()
	resolvedAt := now.Add(24 * time.Hour)

	note := &models.QualityNote{
		ID:         uuid.New(),
		NoteID:     "QN-2024-004",
		Severity:   models.QualitySeverityMedium,
		NoteText:   "この問題は修正されました",
		Reviewer:   "dev@example.com",
		Status:     models.QualityStatusResolved,
		CreatedAt:  now,
		ResolvedAt: &resolvedAt,
	}

	assert.True(t, note.IsResolved())
	assert.NotNil(t, note.ResolvedAt)
	assert.True(t, note.ResolvedAt.After(note.CreatedAt))
}
