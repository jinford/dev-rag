package quality

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestNewFreshnessMonitor(t *testing.T) {
	repoPath := "/test/repo"

	tests := []struct {
		name              string
		threshold         int
		expectedThreshold int
	}{
		{
			name:              "デフォルト閾値",
			threshold:         0,
			expectedThreshold: 30,
		},
		{
			name:              "カスタム閾値",
			threshold:         60,
			expectedThreshold: 60,
		},
		{
			name:              "負の閾値（デフォルトにフォールバック）",
			threshold:         -10,
			expectedThreshold: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := NewFreshnessMonitor(nil, repoPath, tt.threshold)
			assert.NotNil(t, monitor)
			assert.Equal(t, tt.expectedThreshold, monitor.defaultThreshold)
			assert.Equal(t, repoPath, monitor.repoPath)
		})
	}
}

func TestGenerateReindexActions(t *testing.T) {
	ctx := context.Background()
	monitor := NewFreshnessMonitor(nil, "/test/repo", 30)

	chunkID1 := uuid.New()
	chunkID2 := uuid.New()
	chunkID3 := uuid.New()

	staleChunks := []models.ChunkFreshness{
		{
			ChunkID:       chunkID1,
			FilePath:      "/path/to/file1.go",
			ChunkKey:      "file1.go#L1-L10",
			FreshnessDays: 45,
			IsStale:       true,
		},
		{
			ChunkID:       chunkID2,
			FilePath:      "/path/to/file1.go",
			ChunkKey:      "file1.go#L11-L20",
			FreshnessDays: 50,
			IsStale:       true,
		},
		{
			ChunkID:       chunkID3,
			FilePath:      "/path/to/file2.go",
			ChunkKey:      "file2.go#L1-L10",
			FreshnessDays: 40,
			IsStale:       true,
		},
	}

	actions, err := monitor.GenerateReindexActions(ctx, staleChunks)

	assert.NoError(t, err)
	assert.Len(t, actions, 2, "2つのファイルに対してアクションが生成されるべき")

	// ファイルパス別にアクションをマッピング
	actionsByFile := make(map[string]ReindexAction)
	for _, action := range actions {
		actionsByFile[action.FilePath] = action
	}

	// file1.go のアクションを確認
	action1, ok := actionsByFile["/path/to/file1.go"]
	assert.True(t, ok)
	assert.Len(t, action1.ChunkIDs, 2)
	assert.Equal(t, "stale_chunks_detected", action1.Reason)

	// file2.go のアクションを確認
	action2, ok := actionsByFile["/path/to/file2.go"]
	assert.True(t, ok)
	assert.Len(t, action2.ChunkIDs, 1)
	assert.Equal(t, "stale_chunks_detected", action2.Reason)
}

func TestGenerateReindexActions_EmptyInput(t *testing.T) {
	ctx := context.Background()
	monitor := NewFreshnessMonitor(nil, "/test/repo", 30)

	actions, err := monitor.GenerateReindexActions(ctx, []models.ChunkFreshness{})

	assert.NoError(t, err)
	assert.Empty(t, actions, "空の入力に対してはアクションが生成されないべき")
}

func TestCalculateChunkFreshness_NoGitHash(t *testing.T) {
	// このテストは実際のリポジトリが必要なため、スキップ
	t.Skip("実際のリポジトリが必要なため、統合テストでカバー")
}

func TestGenerateFreshnessReport_EmptyStaleChunks(t *testing.T) {
	// このテストは実際のリポジトリが必要なため、スキップ
	t.Skip("実際のリポジトリが必要なため、統合テストでカバー")
}
