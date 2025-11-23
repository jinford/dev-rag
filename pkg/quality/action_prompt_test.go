package quality

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionGenerationPrompt_GeneratePrompt(t *testing.T) {
	prompt := NewActionGenerationPrompt()

	t.Run("正常系: 標準的な週次レビューデータ", func(t *testing.T) {
		data := ActionGenerationData{
			WeekRange: "2024-01-15 to 2024-01-21",
			QualityNotes: []WeeklyQualityNote{
				{
					NoteID:      "QN-001",
					Severity:    "critical",
					NoteText:    "ADR-005の旧バージョンが参照されている",
					LinkedFiles: []string{"docs/adr/ADR-005.md"},
					Reviewer:    "alice",
				},
				{
					NoteID:      "QN-002",
					Severity:    "high",
					NoteText:    "IndexSourceパラメータの説明が不足",
					LinkedFiles: []string{"pkg/indexer/README.md"},
					Reviewer:    "bob",
				},
			},
			RecentChanges: []ActionRecentChange{
				{
					Hash:         "9f2d3b4",
					FilesChanged: []string{"docs/adr/ADR-005.md"},
					MergedAt:     time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC),
				},
			},
			CodeownersLookup: map[string]string{
				"docs/adr/ADR-005.md":       "architecture-team",
				"pkg/indexer/README.md":     "indexer-team",
			},
		}

		result, err := prompt.GeneratePrompt(data)
		require.NoError(t, err)

		// プロンプトの基本構造を確認
		assert.Contains(t, result, "You are a quality management assistant")
		assert.Contains(t, result, "Week Range: 2024-01-15 to 2024-01-21")
		assert.Contains(t, result, "Quality Notes:")
		assert.Contains(t, result, "Recent Changes:")
		assert.Contains(t, result, "CODEOWNERS Lookup:")

		// ルールが含まれていることを確認
		assert.Contains(t, result, "action_type: reindex, doc_fix, test_update, investigate")
		assert.Contains(t, result, "priority: critical→P1, high→P2, medium/low→P3")
		assert.Contains(t, result, "Maximum 5 actions per week")

		// データが含まれていることを確認
		assert.Contains(t, result, "QN-001")
		assert.Contains(t, result, "QN-002")
		assert.Contains(t, result, "9f2d3b4")
		assert.Contains(t, result, "architecture-team")
	})

	t.Run("正常系: 0件のquality_notes", func(t *testing.T) {
		data := ActionGenerationData{
			WeekRange:        "2024-01-15 to 2024-01-21",
			QualityNotes:     []WeeklyQualityNote{},
			RecentChanges:    []ActionRecentChange{},
			CodeownersLookup: map[string]string{},
		}

		result, err := prompt.GeneratePrompt(data)
		require.NoError(t, err)

		// プロンプトが生成されることを確認
		assert.Contains(t, result, "Quality Notes:")
		assert.Contains(t, result, "[]") // 空配列
	})

	t.Run("正常系: 10件のquality_notes（キャパシティ超過）", func(t *testing.T) {
		notes := make([]WeeklyQualityNote, 10)
		for i := 0; i < 10; i++ {
			notes[i] = WeeklyQualityNote{
				NoteID:      "QN-" + string(rune('A'+i)),
				Severity:    "medium",
				NoteText:    "問題" + string(rune('A'+i)),
				LinkedFiles: []string{"file" + string(rune('A'+i)) + ".go"},
				Reviewer:    "reviewer",
			}
		}

		data := ActionGenerationData{
			WeekRange:        "2024-01-15 to 2024-01-21",
			QualityNotes:     notes,
			RecentChanges:    []ActionRecentChange{},
			CodeownersLookup: map[string]string{},
		}

		result, err := prompt.GeneratePrompt(data)
		require.NoError(t, err)

		// すべてのnoteが含まれていることを確認
		for i := 0; i < 10; i++ {
			assert.Contains(t, result, "QN-"+string(rune('A'+i)))
		}

		// キャパシティ制限のルールが含まれていることを確認
		assert.Contains(t, result, "Maximum 5 actions per week")
	})

	t.Run("正常系: recent_changesで解消済みのnotes", func(t *testing.T) {
		data := ActionGenerationData{
			WeekRange: "2024-01-15 to 2024-01-21",
			QualityNotes: []WeeklyQualityNote{
				{
					NoteID:      "QN-001",
					Severity:    "critical",
					NoteText:    "docs/adr/ADR-005.mdの問題",
					LinkedFiles: []string{"docs/adr/ADR-005.md"},
					Reviewer:    "alice",
				},
			},
			RecentChanges: []ActionRecentChange{
				{
					Hash:         "abc123",
					FilesChanged: []string{"docs/adr/ADR-005.md"},
					MergedAt:     time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC),
				},
			},
			CodeownersLookup: map[string]string{},
		}

		result, err := prompt.GeneratePrompt(data)
		require.NoError(t, err)

		// recent_changesのルールが含まれていることを確認
		assert.Contains(t, result, "If recent_changes already resolved the issue")
		assert.Contains(t, result, "abc123")
	})
}

func TestFromQualityNotes(t *testing.T) {
	t.Run("正常系: QualityNoteの変換", func(t *testing.T) {
		notes := []models.QualityNote{
			{
				NoteID:      "QN-001",
				Severity:    models.QualitySeverityCritical,
				NoteText:    "重大な問題",
				LinkedFiles: []string{"file1.go", "file2.go"},
				Reviewer:    "alice",
			},
			{
				NoteID:      "QN-002",
				Severity:    models.QualitySeverityHigh,
				NoteText:    "重要な問題",
				LinkedFiles: []string{"file3.go"},
				Reviewer:    "bob",
			},
		}

		result := FromQualityNotes(notes)

		require.Len(t, result, 2)

		assert.Equal(t, "QN-001", result[0].NoteID)
		assert.Equal(t, "critical", result[0].Severity)
		assert.Equal(t, "重大な問題", result[0].NoteText)
		assert.Equal(t, []string{"file1.go", "file2.go"}, result[0].LinkedFiles)
		assert.Equal(t, "alice", result[0].Reviewer)

		assert.Equal(t, "QN-002", result[1].NoteID)
		assert.Equal(t, "high", result[1].Severity)
	})

	t.Run("正常系: 空のスライス", func(t *testing.T) {
		notes := []models.QualityNote{}
		result := FromQualityNotes(notes)

		assert.Empty(t, result)
	})
}

func TestActionGenerationPrompt_GeneratePrompt_JSONValidity(t *testing.T) {
	prompt := NewActionGenerationPrompt()

	data := ActionGenerationData{
		WeekRange: "2024-01-15 to 2024-01-21",
		QualityNotes: []WeeklyQualityNote{
			{
				NoteID:      "QN-001",
				Severity:    "critical",
				NoteText:    "テスト問題",
				LinkedFiles: []string{"file.go"},
				Reviewer:    "tester",
			},
		},
		RecentChanges: []ActionRecentChange{
			{
				Hash:         "abc123",
				FilesChanged: []string{"file.go"},
				MergedAt:     time.Now(),
			},
		},
		CodeownersLookup: map[string]string{
			"file.go": "team-a",
		},
	}

	result, err := prompt.GeneratePrompt(data)
	require.NoError(t, err)

	// Quality Notes のJSONが有効であることを確認
	notesStart := strings.Index(result, "Quality Notes:\n") + len("Quality Notes:\n")
	notesEnd := strings.Index(result[notesStart:], "\n\n")
	notesJSON := result[notesStart : notesStart+notesEnd]

	var notes []WeeklyQualityNote
	err = json.Unmarshal([]byte(notesJSON), &notes)
	assert.NoError(t, err, "Quality Notes JSON should be valid")

	// Recent Changes のJSONが有効であることを確認
	changesStart := strings.Index(result, "Recent Changes:\n") + len("Recent Changes:\n")
	changesEnd := strings.Index(result[changesStart:], "\n\n")
	changesJSON := result[changesStart : changesStart+changesEnd]

	var changes []ActionRecentChange
	err = json.Unmarshal([]byte(changesJSON), &changes)
	assert.NoError(t, err, "Recent Changes JSON should be valid")

	// CODEOWNERS Lookup のJSONが有効であることを確認
	ownersStart := strings.Index(result, "CODEOWNERS Lookup:\n") + len("CODEOWNERS Lookup:\n")
	ownersEnd := strings.Index(result[ownersStart:], "\n\n")
	ownersJSON := result[ownersStart : ownersStart+ownersEnd]

	var owners map[string]string
	err = json.Unmarshal([]byte(ownersJSON), &owners)
	assert.NoError(t, err, "CODEOWNERS Lookup JSON should be valid")
}
