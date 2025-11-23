package quality

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
)

const (
	// PromptVersion はアクション生成プロンプトのバージョン
	PromptVersion = "1.1"

	// MaxActionsPerWeek は1週間あたりの最大アクション数（チームキャパシティ上限）
	MaxActionsPerWeek = 5
)

// ActionGenerationData はアクション生成に必要なデータを表します
type ActionGenerationData struct {
	WeekRange        string                       `json:"week_range"`
	QualityNotes     []WeeklyQualityNote          `json:"quality_notes"`
	RecentChanges    []ActionRecentChange         `json:"recent_changes"`
	CodeownersLookup map[string]string            `json:"codeowners_lookup"`
}

// WeeklyQualityNote はプロンプト用の品質ノート
type WeeklyQualityNote struct {
	NoteID       string   `json:"note_id"`
	Severity     string   `json:"severity"`
	NoteText     string   `json:"note_text"`
	LinkedFiles  []string `json:"linked_files"`
	Reviewer     string   `json:"reviewer"`
}

// ActionRecentChange はアクション生成用の最新コミット情報
type ActionRecentChange struct {
	Hash         string    `json:"hash"`
	FilesChanged []string  `json:"files_changed"`
	MergedAt     time.Time `json:"merged_at"`
}

// ActionGenerationPrompt はアクション生成プロンプトを構築します
type ActionGenerationPrompt struct {
	version string
}

// NewActionGenerationPrompt は新しいActionGenerationPromptを作成します
func NewActionGenerationPrompt() *ActionGenerationPrompt {
	return &ActionGenerationPrompt{
		version: PromptVersion,
	}
}

// GeneratePrompt は週次レビューデータからプロンプトを生成します
func (p *ActionGenerationPrompt) GeneratePrompt(data ActionGenerationData) (string, error) {
	// データのJSON化
	notesJSON, err := json.MarshalIndent(data.QualityNotes, "  ", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal quality notes: %w", err)
	}

	changesJSON, err := json.MarshalIndent(data.RecentChanges, "  ", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal recent changes: %w", err)
	}

	codeownersJSON, err := json.MarshalIndent(data.CodeownersLookup, "  ", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal codeowners lookup: %w", err)
	}

	// プロンプトテンプレートの構築
	var prompt strings.Builder

	prompt.WriteString("You are a quality management assistant.\n\n")
	prompt.WriteString("Analyze the following quality notes and generate prioritized action items.\n\n")

	prompt.WriteString(fmt.Sprintf("Week Range: %s\n\n", data.WeekRange))

	prompt.WriteString("Quality Notes:\n")
	prompt.WriteString(string(notesJSON))
	prompt.WriteString("\n\n")

	prompt.WriteString("Recent Changes:\n")
	prompt.WriteString(string(changesJSON))
	prompt.WriteString("\n\n")

	prompt.WriteString("CODEOWNERS Lookup:\n")
	prompt.WriteString(string(codeownersJSON))
	prompt.WriteString("\n\n")

	prompt.WriteString("Rules:\n")
	prompt.WriteString("- Each action must have an action_type: reindex, doc_fix, test_update, investigate\n")
	prompt.WriteString("- owner_hint: use codeowners_lookup first, then reviewer, otherwise \"unassigned\"\n")
	prompt.WriteString("- priority: critical→P1, high→P2, medium/low→P3\n")
	prompt.WriteString("- acceptance_criteria must be machine-verifiable\n")
	prompt.WriteString(fmt.Sprintf("- Maximum %d actions per week (team capacity limit)\n", MaxActionsPerWeek))
	prompt.WriteString("- Sort by severity descending, excess items get status:\"noop\"\n")
	prompt.WriteString("- If recent_changes already resolved the issue, set status:\"noop\" and note which commit\n")
	prompt.WriteString("- Return ONLY a JSON array with NO additional text before or after\n\n")

	prompt.WriteString("Return a JSON array with the following structure:\n")
	prompt.WriteString("[\n")
	prompt.WriteString("  {\n")
	prompt.WriteString(fmt.Sprintf("    \"prompt_version\": \"%s\",\n", PromptVersion))
	prompt.WriteString("    \"priority\": \"P1\",\n")
	prompt.WriteString("    \"action_type\": \"reindex\",\n")
	prompt.WriteString("    \"title\": \"...\",\n")
	prompt.WriteString("    \"description\": \"...\",\n")
	prompt.WriteString("    \"linked_files\": [\"...\"],\n")
	prompt.WriteString("    \"owner_hint\": \"...\",\n")
	prompt.WriteString("    \"acceptance_criteria\": \"...\",\n")
	prompt.WriteString("    \"status\": \"open\"\n")
	prompt.WriteString("  },\n")
	prompt.WriteString("  ...\n")
	prompt.WriteString("]\n")

	return prompt.String(), nil
}

// FromQualityNotes はQualityNoteをWeeklyQualityNoteに変換します
func FromQualityNotes(notes []models.QualityNote) []WeeklyQualityNote {
	result := make([]WeeklyQualityNote, len(notes))
	for i, note := range notes {
		result[i] = WeeklyQualityNote{
			NoteID:      note.NoteID,
			Severity:    string(note.Severity),
			NoteText:    note.NoteText,
			LinkedFiles: note.LinkedFiles,
			Reviewer:    note.Reviewer,
		}
	}
	return result
}

// FromWeeklyReviewData はWeeklyReviewDataをActionGenerationDataに変換します
func FromWeeklyReviewData(data *WeeklyReviewData) ActionGenerationData {
	// WeeklyQualityNoteに変換
	notes := make([]WeeklyQualityNote, 0, len(data.QualityNotes))
	for _, note := range data.QualityNotes {
		notes = append(notes, WeeklyQualityNote{
			NoteID:      note.NoteID,
			Severity:    string(note.Severity),
			NoteText:    note.NoteText,
			LinkedFiles: note.LinkedFiles,
			Reviewer:    note.Reviewer,
		})
	}

	// ActionRecentChangeに変換
	changes := make([]ActionRecentChange, 0, len(data.RecentChanges))
	for _, change := range data.RecentChanges {
		changes = append(changes, ActionRecentChange{
			Hash:         change.Hash,
			FilesChanged: change.FilesChanged,
			MergedAt:     change.MergedAt,
		})
	}

	// CodeownersLookupを変換 (map[string][]string → map[string]string)
	// 複数のオーナーがいる場合は最初のものを使用
	ownersLookup := make(map[string]string)
	for path, owners := range data.CodeownersLookup {
		if len(owners) > 0 {
			ownersLookup[path] = owners[0]
		} else {
			ownersLookup[path] = "unassigned"
		}
	}

	// WeekRangeを文字列に変換
	weekRange := fmt.Sprintf("%s to %s",
		data.WeekRange.StartDate.Format("2006-01-02"),
		data.WeekRange.EndDate.Format("2006-01-02"))

	return ActionGenerationData{
		WeekRange:        weekRange,
		QualityNotes:     notes,
		RecentChanges:    changes,
		CodeownersLookup: ownersLookup,
	}
}
