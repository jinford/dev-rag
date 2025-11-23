package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/config"
	"github.com/jinford/dev-rag/pkg/db"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/repository"
	"github.com/jinford/dev-rag/pkg/sqlc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB はテスト用のデータベースを初期化します
func setupTestDB(t *testing.T) (*db.DB, func()) {
	t.Helper()

	ctx := context.Background()

	// テスト用の設定を作成
	testCfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "testuser",
			Password: "testpass",
			DBName:   "dev_rag_test",
			SSLMode:  "disable",
		},
	}

	// データベース接続
	testDB, err := db.New(ctx, db.ConnectionParams{
		Host:     testCfg.Database.Host,
		Port:     testCfg.Database.Port,
		User:     testCfg.Database.User,
		Password: testCfg.Database.Password,
		DBName:   testCfg.Database.DBName,
		SSLMode:  testCfg.Database.SSLMode,
	})

	if err != nil {
		t.Skip("テストデータベースに接続できません。統合テストをスキップします:", err)
		return nil, func() {}
	}

	// クリーンアップ関数
	cleanup := func() {
		// テストデータをクリーンアップ
		_, _ = testDB.Pool.Exec(ctx, "DELETE FROM quality_notes WHERE note_id LIKE 'TEST-%'")
		testDB.Close()
	}

	return testDB, cleanup
}

// TestCreateNoteFromFlags はフラグから品質ノートを作成する関数をテストします
func TestCreateNoteFromFlags(t *testing.T) {
	tests := []struct {
		name        string
		flags       map[string]string
		expectError bool
	}{
		{
			name: "有効なフラグ",
			flags: map[string]string{
				"note-id":  "TEST-001",
				"severity": "high",
				"text":     "Test issue",
				"reviewer": "John Doe",
			},
			expectError: false,
		},
		{
			name: "必須フラグが欠けている",
			flags: map[string]string{
				"note-id": "TEST-002",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// モックコマンドを作成 (簡易版)
			// 実際のテストでは、urfave/cli/v3のテストユーティリティを使用する必要があります
			t.Skip("urfave/cli/v3のモックが必要です")
		})
	}
}

// TestValidateQualityNote は品質ノートのバリデーションをテストします
func TestValidateQualityNote(t *testing.T) {
	tests := []struct {
		name        string
		note        *models.QualityNote
		expectError bool
	}{
		{
			name: "有効な品質ノート",
			note: &models.QualityNote{
				ID:       uuid.New(),
				NoteID:   "TEST-001",
				Severity: models.QualitySeverityHigh,
				NoteText: "Test issue",
				Reviewer: "John Doe",
				Status:   models.QualityStatusOpen,
			},
			expectError: false,
		},
		{
			name: "NoteIDが空",
			note: &models.QualityNote{
				ID:       uuid.New(),
				NoteID:   "",
				Severity: models.QualitySeverityHigh,
				NoteText: "Test issue",
				Reviewer: "John Doe",
				Status:   models.QualityStatusOpen,
			},
			expectError: true,
		},
		{
			name: "無効なSeverity",
			note: &models.QualityNote{
				ID:       uuid.New(),
				NoteID:   "TEST-002",
				Severity: models.QualitySeverity("invalid"),
				NoteText: "Test issue",
				Reviewer: "John Doe",
				Status:   models.QualityStatusOpen,
			},
			expectError: true,
		},
		{
			name: "NoteTextが空",
			note: &models.QualityNote{
				ID:       uuid.New(),
				NoteID:   "TEST-003",
				Severity: models.QualitySeverityHigh,
				NoteText: "",
				Reviewer: "John Doe",
				Status:   models.QualityStatusOpen,
			},
			expectError: true,
		},
		{
			name: "Reviewerが空",
			note: &models.QualityNote{
				ID:       uuid.New(),
				NoteID:   "TEST-004",
				Severity: models.QualitySeverityHigh,
				NoteText: "Test issue",
				Reviewer: "",
				Status:   models.QualityStatusOpen,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateQualityNote(tt.note)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSplitAndTrim はsplitAndTrim関数をテストします
func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "単一の要素",
			input:    "file1.go",
			expected: []string{"file1.go"},
		},
		{
			name:     "複数の要素",
			input:    "file1.go, file2.go, file3.go",
			expected: []string{"file1.go", "file2.go", "file3.go"},
		},
		{
			name:     "スペースを含む要素",
			input:    "  file1.go  ,  file2.go  ",
			expected: []string{"file1.go", "file2.go"},
		},
		{
			name:     "空の文字列",
			input:    "",
			expected: []string{},
		},
		{
			name:     "カンマのみ",
			input:    ",,,",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndTrim(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFeedbackIntegration は品質フィードバックの統合テストです
func TestFeedbackIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("統合テストをスキップします")
	}

	testDB, cleanup := setupTestDB(t)
	if testDB == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	repo := repository.NewQualityRepositoryRW(sqlc.New(testDB.Pool))

	t.Run("フィードバックの作成と取得", func(t *testing.T) {
		// フィードバックを作成
		note := &models.QualityNote{
			ID:           uuid.New(),
			NoteID:       "TEST-INTEGRATION-001",
			Severity:     models.QualitySeverityHigh,
			NoteText:     "Integration test feedback",
			Reviewer:     "Test Reviewer",
			LinkedFiles:  []string{"test/file1.go", "test/file2.go"},
			LinkedChunks: []string{"chunk-1", "chunk-2"},
			Status:       models.QualityStatusOpen,
			CreatedAt:    time.Now(),
		}

		createdNote, err := repo.CreateQualityNote(ctx, note)
		require.NoError(t, err)
		assert.Equal(t, note.NoteID, createdNote.NoteID)
		assert.Equal(t, note.Severity, createdNote.Severity)
		assert.Equal(t, note.NoteText, createdNote.NoteText)

		// 作成したフィードバックを取得
		retrievedNote, err := repo.GetQualityNoteByNoteID(ctx, note.NoteID)
		require.NoError(t, err)
		assert.Equal(t, createdNote.ID, retrievedNote.ID)
		assert.Equal(t, createdNote.NoteID, retrievedNote.NoteID)
		assert.Equal(t, len(note.LinkedFiles), len(retrievedNote.LinkedFiles))
		assert.Equal(t, len(note.LinkedChunks), len(retrievedNote.LinkedChunks))
	})

	t.Run("フィードバックのリスト取得", func(t *testing.T) {
		// 複数のフィードバックを作成
		for i := 1; i <= 3; i++ {
			note := &models.QualityNote{
				ID:        uuid.New(),
				NoteID:    fmt.Sprintf("TEST-INTEGRATION-LIST-%03d", i),
				Severity:  models.QualitySeverityMedium,
				NoteText:  fmt.Sprintf("Test feedback %d", i),
				Reviewer:  "Test Reviewer",
				Status:    models.QualityStatusOpen,
				CreatedAt: time.Now(),
			}
			_, err := repo.CreateQualityNote(ctx, note)
			require.NoError(t, err)
		}

		// リストを取得
		filter := &models.QualityNoteFilter{
			Status: func() *models.QualityStatus { s := models.QualityStatusOpen; return &s }(),
		}
		notes, err := repo.ListQualityNotes(ctx, filter)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(notes), 3)
	})

	t.Run("フィードバックの解決", func(t *testing.T) {
		// フィードバックを作成
		note := &models.QualityNote{
			ID:        uuid.New(),
			NoteID:    "TEST-INTEGRATION-RESOLVE-001",
			Severity:  models.QualitySeverityCritical,
			NoteText:  "Critical issue",
			Reviewer:  "Test Reviewer",
			Status:    models.QualityStatusOpen,
			CreatedAt: time.Now(),
		}

		createdNote, err := repo.CreateQualityNote(ctx, note)
		require.NoError(t, err)
		assert.False(t, createdNote.IsResolved())

		// 解決済みに更新
		updatedNote, err := repo.UpdateQualityNoteStatus(ctx, createdNote.ID, models.QualityStatusResolved)
		require.NoError(t, err)
		assert.True(t, updatedNote.IsResolved())
		assert.NotNil(t, updatedNote.ResolvedAt)
	})
}

// TestFeedbackImportJSON はJSON一括インポートのテストです
func TestFeedbackImportJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("統合テストをスキップします")
	}

	testDB, cleanup := setupTestDB(t)
	if testDB == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	repo := repository.NewQualityRepositoryRW(sqlc.New(testDB.Pool))

	// テスト用のJSONファイルを作成
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "test_import.json")

	importData := []struct {
		NoteID       string   `json:"note_id"`
		Severity     string   `json:"severity"`
		NoteText     string   `json:"note_text"`
		Reviewer     string   `json:"reviewer"`
		LinkedFiles  []string `json:"linked_files"`
		LinkedChunks []string `json:"linked_chunks"`
	}{
		{
			NoteID:       "TEST-IMPORT-001",
			Severity:     "high",
			NoteText:     "Import test 1",
			Reviewer:     "Test Reviewer",
			LinkedFiles:  []string{"file1.go"},
			LinkedChunks: []string{"chunk-1"},
		},
		{
			NoteID:   "TEST-IMPORT-002",
			Severity: "medium",
			NoteText: "Import test 2",
			Reviewer: "Test Reviewer",
		},
	}

	jsonData, err := json.Marshal(importData)
	require.NoError(t, err)

	err = os.WriteFile(jsonFile, jsonData, 0644)
	require.NoError(t, err)

	// JSONファイルを読み込んでインポート
	data, err := os.ReadFile(jsonFile)
	require.NoError(t, err)

	type ImportNote struct {
		NoteID       string   `json:"note_id"`
		Severity     string   `json:"severity"`
		NoteText     string   `json:"note_text"`
		Reviewer     string   `json:"reviewer"`
		LinkedFiles  []string `json:"linked_files"`
		LinkedChunks []string `json:"linked_chunks"`
	}

	var importNotes []ImportNote
	err = json.Unmarshal(data, &importNotes)
	require.NoError(t, err)

	// インポート処理
	for _, in := range importNotes {
		note := &models.QualityNote{
			ID:           uuid.New(),
			NoteID:       in.NoteID,
			Severity:     models.QualitySeverity(in.Severity),
			NoteText:     in.NoteText,
			Reviewer:     in.Reviewer,
			LinkedFiles:  in.LinkedFiles,
			LinkedChunks: in.LinkedChunks,
			Status:       models.QualityStatusOpen,
			CreatedAt:    time.Now(),
		}

		_, err := repo.CreateQualityNote(ctx, note)
		require.NoError(t, err)
	}

	// インポートされたデータを確認
	for _, in := range importNotes {
		retrievedNote, err := repo.GetQualityNoteByNoteID(ctx, in.NoteID)
		require.NoError(t, err)
		assert.Equal(t, in.NoteID, retrievedNote.NoteID)
		assert.Equal(t, in.Severity, string(retrievedNote.Severity))
	}
}
