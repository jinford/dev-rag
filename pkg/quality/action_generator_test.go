package quality

import (
	"context"
	"testing"
	"time"

	"github.com/jinford/dev-rag/pkg/indexer/llm"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLLMClient はテスト用のモックLLMクライアント
type mockLLMClient struct {
	response llm.CompletionResponse
	err      error
}

func (m *mockLLMClient) GenerateCompletion(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	if m.err != nil {
		return llm.CompletionResponse{}, m.err
	}
	return m.response, nil
}

func TestParseActionResponse(t *testing.T) {
	t.Run("正常系: 有効なJSON配列", func(t *testing.T) {
		jsonStr := `[
			{
				"prompt_version": "1.1",
				"priority": "P1",
				"action_type": "reindex",
				"title": "ADR-005とREADME.enの再インデックス",
				"description": "quality_notes #12 で旧バージョン参照が指摘",
				"linked_files": ["docs/adr/ADR-005.md", "README.en.md"],
				"owner_hint": "architecture-team",
				"acceptance_criteria": "VectorStoreにcommit 9f2d3b4のチャンクが存在する",
				"status": "open"
			},
			{
				"prompt_version": "1.1",
				"priority": "P2",
				"action_type": "doc_fix",
				"title": "IndexSourceパラメータ説明の追記",
				"description": "quality_notes #42 でAPI引数説明不足が判明",
				"linked_files": ["pkg/indexer/README.md"],
				"owner_hint": "unassigned",
				"acceptance_criteria": "CIでREADME lintチェックが通る",
				"status": "open"
			}
		]`

		actions, err := ParseActionResponse(jsonStr)
		require.NoError(t, err)
		require.Len(t, actions, 2)

		// 1つ目のアクション検証
		assert.Equal(t, "1.1", actions[0].PromptVersion)
		assert.Equal(t, models.ActionPriorityP1, actions[0].Priority)
		assert.Equal(t, models.ActionTypeReindex, actions[0].ActionType)
		assert.Equal(t, "ADR-005とREADME.enの再インデックス", actions[0].Title)
		assert.Equal(t, "quality_notes #12 で旧バージョン参照が指摘", actions[0].Description)
		assert.Equal(t, []string{"docs/adr/ADR-005.md", "README.en.md"}, actions[0].LinkedFiles)
		assert.Equal(t, "architecture-team", actions[0].OwnerHint)
		assert.Equal(t, "VectorStoreにcommit 9f2d3b4のチャンクが存在する", actions[0].AcceptanceCriteria)
		assert.Equal(t, models.ActionStatusOpen, actions[0].Status)

		// 2つ目のアクション検証
		assert.Equal(t, models.ActionPriorityP2, actions[1].Priority)
		assert.Equal(t, models.ActionTypeDocFix, actions[1].ActionType)
		assert.Equal(t, models.ActionStatusOpen, actions[1].Status)
	})

	t.Run("正常系: 空の配列", func(t *testing.T) {
		jsonStr := `[]`

		actions, err := ParseActionResponse(jsonStr)
		require.NoError(t, err)
		assert.Empty(t, actions)
	})

	t.Run("正常系: status=noop", func(t *testing.T) {
		jsonStr := `[
			{
				"prompt_version": "1.1",
				"priority": "P3",
				"action_type": "investigate",
				"title": "調査タスク",
				"description": "commit abc123で解消済み",
				"linked_files": [],
				"owner_hint": "unassigned",
				"acceptance_criteria": "N/A",
				"status": "noop"
			}
		]`

		actions, err := ParseActionResponse(jsonStr)
		require.NoError(t, err)
		require.Len(t, actions, 1)

		assert.Equal(t, models.ActionStatusNoop, actions[0].Status)
		assert.True(t, actions[0].IsNoop())
	})

	t.Run("正常系: すべてのaction_type", func(t *testing.T) {
		jsonStr := `[
			{
				"prompt_version": "1.1",
				"priority": "P1",
				"action_type": "reindex",
				"title": "再インデックス",
				"description": "説明",
				"linked_files": [],
				"owner_hint": "team-a",
				"acceptance_criteria": "条件",
				"status": "open"
			},
			{
				"prompt_version": "1.1",
				"priority": "P2",
				"action_type": "doc_fix",
				"title": "ドキュメント修正",
				"description": "説明",
				"linked_files": [],
				"owner_hint": "team-b",
				"acceptance_criteria": "条件",
				"status": "open"
			},
			{
				"prompt_version": "1.1",
				"priority": "P2",
				"action_type": "test_update",
				"title": "テスト更新",
				"description": "説明",
				"linked_files": [],
				"owner_hint": "team-c",
				"acceptance_criteria": "条件",
				"status": "open"
			},
			{
				"prompt_version": "1.1",
				"priority": "P3",
				"action_type": "investigate",
				"title": "調査",
				"description": "説明",
				"linked_files": [],
				"owner_hint": "team-d",
				"acceptance_criteria": "条件",
				"status": "open"
			}
		]`

		actions, err := ParseActionResponse(jsonStr)
		require.NoError(t, err)
		require.Len(t, actions, 4)

		assert.Equal(t, models.ActionTypeReindex, actions[0].ActionType)
		assert.Equal(t, models.ActionTypeDocFix, actions[1].ActionType)
		assert.Equal(t, models.ActionTypeTestUpdate, actions[2].ActionType)
		assert.Equal(t, models.ActionTypeInvestigate, actions[3].ActionType)
	})

	t.Run("異常系: 不正なJSON", func(t *testing.T) {
		jsonStr := `invalid json`

		_, err := ParseActionResponse(jsonStr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal JSON")
	})

	t.Run("異常系: 不正なaction_type", func(t *testing.T) {
		jsonStr := `[
			{
				"prompt_version": "1.1",
				"priority": "P1",
				"action_type": "invalid_type",
				"title": "テスト",
				"description": "説明",
				"linked_files": [],
				"owner_hint": "team",
				"acceptance_criteria": "条件",
				"status": "open"
			}
		]`

		_, err := ParseActionResponse(jsonStr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid action_type")
	})

	t.Run("異常系: 不正なpriority", func(t *testing.T) {
		jsonStr := `[
			{
				"prompt_version": "1.1",
				"priority": "P0",
				"action_type": "reindex",
				"title": "テスト",
				"description": "説明",
				"linked_files": [],
				"owner_hint": "team",
				"acceptance_criteria": "条件",
				"status": "open"
			}
		]`

		_, err := ParseActionResponse(jsonStr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid priority")
	})

	t.Run("異常系: 不正なstatus", func(t *testing.T) {
		jsonStr := `[
			{
				"prompt_version": "1.1",
				"priority": "P1",
				"action_type": "reindex",
				"title": "テスト",
				"description": "説明",
				"linked_files": [],
				"owner_hint": "team",
				"acceptance_criteria": "条件",
				"status": "invalid_status"
			}
		]`

		_, err := ParseActionResponse(jsonStr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
	})
}

func TestActionGenerator_GenerateActions(t *testing.T) {
	t.Run("正常系: アクション生成成功", func(t *testing.T) {
		mockClient := &mockLLMClient{
			response: llm.CompletionResponse{
				Content: `[
					{
						"prompt_version": "1.1",
						"priority": "P1",
						"action_type": "reindex",
						"title": "テストアクション",
						"description": "テスト説明",
						"linked_files": ["test.go"],
						"owner_hint": "test-team",
						"acceptance_criteria": "テスト条件",
						"status": "open"
					}
				]`,
				TokensUsed: 100,
				Model:      "gpt-4",
			},
			err: nil,
		}

		generator := NewActionGenerator(mockClient)

		data := ActionGenerationData{
			WeekRange: "2024-01-15 to 2024-01-21",
			QualityNotes: []WeeklyQualityNote{
				{
					NoteID:   "QN-001",
					Severity: "critical",
					NoteText: "問題",
					LinkedFiles: []string{"test.go"},
					Reviewer: "alice",
				},
			},
			RecentChanges:    []ActionRecentChange{},
			CodeownersLookup: map[string]string{},
		}

		actions, err := generator.GenerateActions(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, actions, 1)

		// メタデータが追加されていることを確認
		assert.NotEqual(t, "", actions[0].ID.String())
		assert.False(t, actions[0].CreatedAt.IsZero())
		assert.NotEmpty(t, actions[0].ActionID)

		// アクション内容の確認
		assert.Equal(t, "テストアクション", actions[0].Title)
		assert.Equal(t, models.ActionPriorityP1, actions[0].Priority)
		assert.Equal(t, models.ActionTypeReindex, actions[0].ActionType)
	})

	t.Run("正常系: 0件のquality_notes", func(t *testing.T) {
		mockClient := &mockLLMClient{
			response: llm.CompletionResponse{
				Content:    `[]`,
				TokensUsed: 10,
				Model:      "gpt-4",
			},
			err: nil,
		}

		generator := NewActionGenerator(mockClient)

		data := ActionGenerationData{
			WeekRange:        "2024-01-15 to 2024-01-21",
			QualityNotes:     []WeeklyQualityNote{},
			RecentChanges:    []ActionRecentChange{},
			CodeownersLookup: map[string]string{},
		}

		actions, err := generator.GenerateActions(context.Background(), data)
		require.NoError(t, err)
		assert.Empty(t, actions)
	})

	t.Run("正常系: 10件のquality_notes（キャパシティ超過）", func(t *testing.T) {
		// LLMは最大5件のアクションを返すべき
		mockClient := &mockLLMClient{
			response: llm.CompletionResponse{
				Content: `[
					{"prompt_version": "1.1", "priority": "P1", "action_type": "reindex", "title": "アクション1", "description": "説明1", "linked_files": [], "owner_hint": "team", "acceptance_criteria": "条件", "status": "open"},
					{"prompt_version": "1.1", "priority": "P1", "action_type": "reindex", "title": "アクション2", "description": "説明2", "linked_files": [], "owner_hint": "team", "acceptance_criteria": "条件", "status": "open"},
					{"prompt_version": "1.1", "priority": "P2", "action_type": "doc_fix", "title": "アクション3", "description": "説明3", "linked_files": [], "owner_hint": "team", "acceptance_criteria": "条件", "status": "open"},
					{"prompt_version": "1.1", "priority": "P2", "action_type": "doc_fix", "title": "アクション4", "description": "説明4", "linked_files": [], "owner_hint": "team", "acceptance_criteria": "条件", "status": "open"},
					{"prompt_version": "1.1", "priority": "P3", "action_type": "investigate", "title": "アクション5", "description": "説明5", "linked_files": [], "owner_hint": "team", "acceptance_criteria": "条件", "status": "open"},
					{"prompt_version": "1.1", "priority": "P3", "action_type": "investigate", "title": "アクション6", "description": "説明6（超過）", "linked_files": [], "owner_hint": "team", "acceptance_criteria": "条件", "status": "noop"}
				]`,
				TokensUsed: 200,
				Model:      "gpt-4",
			},
			err: nil,
		}

		generator := NewActionGenerator(mockClient)

		notes := make([]WeeklyQualityNote, 10)
		for i := 0; i < 10; i++ {
			notes[i] = WeeklyQualityNote{
				NoteID:   "QN-" + string(rune('A'+i)),
				Severity: "medium",
				NoteText: "問題" + string(rune('A'+i)),
				LinkedFiles: []string{"file.go"},
				Reviewer: "reviewer",
			}
		}

		data := ActionGenerationData{
			WeekRange:        "2024-01-15 to 2024-01-21",
			QualityNotes:     notes,
			RecentChanges:    []ActionRecentChange{},
			CodeownersLookup: map[string]string{},
		}

		actions, err := generator.GenerateActions(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, actions, 6)

		// 最初の5件がopen、6件目がnoopであることを確認
		for i := 0; i < 5; i++ {
			assert.Equal(t, models.ActionStatusOpen, actions[i].Status, "アクション %d はopenであるべき", i+1)
		}
		assert.Equal(t, models.ActionStatusNoop, actions[5].Status, "アクション6はnoopであるべき")
	})

	t.Run("正常系: recent_changesで解消済み", func(t *testing.T) {
		mockClient := &mockLLMClient{
			response: llm.CompletionResponse{
				Content: `[
					{
						"prompt_version": "1.1",
						"priority": "P1",
						"action_type": "reindex",
						"title": "ファイル再インデックス",
						"description": "commit abc123で解消済み",
						"linked_files": ["file.go"],
						"owner_hint": "team",
						"acceptance_criteria": "N/A",
						"status": "noop"
					}
				]`,
				TokensUsed: 100,
				Model:      "gpt-4",
			},
			err: nil,
		}

		generator := NewActionGenerator(mockClient)

		data := ActionGenerationData{
			WeekRange: "2024-01-15 to 2024-01-21",
			QualityNotes: []WeeklyQualityNote{
				{
					NoteID:      "QN-001",
					Severity:    "critical",
					NoteText:    "file.goの問題",
					LinkedFiles: []string{"file.go"},
					Reviewer:    "alice",
				},
			},
			RecentChanges: []ActionRecentChange{
				{
					Hash:         "abc123",
					FilesChanged: []string{"file.go"},
					MergedAt:     time.Now(),
				},
			},
			CodeownersLookup: map[string]string{},
		}

		actions, err := generator.GenerateActions(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, actions, 1)

		assert.Equal(t, models.ActionStatusNoop, actions[0].Status)
		assert.Contains(t, actions[0].Description, "abc123")
	})

	t.Run("異常系: LLMエラー", func(t *testing.T) {
		mockClient := &mockLLMClient{
			response: llm.CompletionResponse{},
			err:      assert.AnError,
		}

		generator := NewActionGenerator(mockClient)

		data := ActionGenerationData{
			WeekRange:        "2024-01-15 to 2024-01-21",
			QualityNotes:     []WeeklyQualityNote{},
			RecentChanges:    []ActionRecentChange{},
			CodeownersLookup: map[string]string{},
		}

		_, err := generator.GenerateActions(context.Background(), data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate completion")
	})

	t.Run("異常系: 不正なJSONレスポンス", func(t *testing.T) {
		mockClient := &mockLLMClient{
			response: llm.CompletionResponse{
				Content:    `invalid json response`,
				TokensUsed: 50,
				Model:      "gpt-4",
			},
			err: nil,
		}

		generator := NewActionGenerator(mockClient)

		data := ActionGenerationData{
			WeekRange:        "2024-01-15 to 2024-01-21",
			QualityNotes:     []WeeklyQualityNote{},
			RecentChanges:    []ActionRecentChange{},
			CodeownersLookup: map[string]string{},
		}

		_, err := generator.GenerateActions(context.Background(), data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse action response")
	})
}

func TestValidationFunctions(t *testing.T) {
	t.Run("isValidActionType", func(t *testing.T) {
		assert.True(t, isValidActionType(models.ActionTypeReindex))
		assert.True(t, isValidActionType(models.ActionTypeDocFix))
		assert.True(t, isValidActionType(models.ActionTypeTestUpdate))
		assert.True(t, isValidActionType(models.ActionTypeInvestigate))
		assert.False(t, isValidActionType(models.ActionType("invalid")))
	})

	t.Run("isValidPriority", func(t *testing.T) {
		assert.True(t, isValidPriority(models.ActionPriorityP1))
		assert.True(t, isValidPriority(models.ActionPriorityP2))
		assert.True(t, isValidPriority(models.ActionPriorityP3))
		assert.False(t, isValidPriority(models.ActionPriority("P0")))
		assert.False(t, isValidPriority(models.ActionPriority("invalid")))
	})

	t.Run("isValidStatus", func(t *testing.T) {
		assert.True(t, isValidStatus(models.ActionStatusOpen))
		assert.True(t, isValidStatus(models.ActionStatusNoop))
		assert.True(t, isValidStatus(models.ActionStatusCompleted))
		assert.False(t, isValidStatus(models.ActionStatus("invalid")))
	})
}
