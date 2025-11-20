package provenance

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRanker_AdjustRanking(t *testing.T) {
	pg := NewProvenanceGraph()
	config := DefaultRankingConfig()
	ranker := NewRanker(pg, config)

	// チャンクの準備
	latestChunkID := uuid.New()
	oldChunkID := uuid.New()
	unknownChunkID := uuid.New()

	// 最新バージョンのProvenance
	latestProv := &ChunkProvenance{
		ChunkID:          latestChunkID,
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "abc123",
		ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	// 古いバージョンのProvenance
	oldProv := &ChunkProvenance{
		ChunkID:          oldChunkID,
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "old123",
		ChunkKey:         "product/source/src/main.go#L1-L10@old123",
		IsLatest:         false,
		IndexedAt:        time.Now().Add(-1 * time.Hour),
		SourceSnapshotID: uuid.New(),
	}

	require.NoError(t, pg.Add(latestProv))
	require.NoError(t, pg.Add(oldProv))

	// 検索結果の準備
	results := []*models.SearchResult{
		{
			ChunkID:   latestChunkID,
			FilePath:  "src/main.go",
			StartLine: 1,
			EndLine:   10,
			Content:   "latest content",
			Score:     0.8,
		},
		{
			ChunkID:   oldChunkID,
			FilePath:  "src/main.go",
			StartLine: 1,
			EndLine:   10,
			Content:   "old content",
			Score:     0.85, // 元のスコアは高い
		},
		{
			ChunkID:   unknownChunkID,
			FilePath:  "src/utils.go",
			StartLine: 1,
			EndLine:   10,
			Content:   "unknown content",
			Score:     0.75,
		},
	}

	ctx := context.Background()
	rankedResults, err := ranker.AdjustRanking(ctx, results)
	require.NoError(t, err)
	assert.Equal(t, 3, len(rankedResults))

	// 最新バージョンはブーストされているはず
	for _, rr := range rankedResults {
		if rr.ChunkID == latestChunkID {
			assert.True(t, rr.IsLatest)
			assert.Greater(t, rr.AdjustedScore, rr.OriginalScore)
			assert.Equal(t, config.LatestVersionBoost, rr.BoostApplied)
		}
		if rr.ChunkID == oldChunkID {
			assert.False(t, rr.IsLatest)
			assert.Less(t, rr.AdjustedScore, rr.OriginalScore)
			assert.Equal(t, -config.RecencyDecayFactor, rr.BoostApplied)
		}
		if rr.ChunkID == unknownChunkID {
			// Provenance情報がない場合はスコアがそのまま
			assert.Equal(t, rr.OriginalScore, rr.AdjustedScore)
			assert.Equal(t, 0.0, rr.BoostApplied)
		}
	}

	// スコアの順序を確認（調整後のスコアでソートされているはず）
	assert.True(t, rankedResults[0].AdjustedScore >= rankedResults[1].AdjustedScore)
	assert.True(t, rankedResults[1].AdjustedScore >= rankedResults[2].AdjustedScore)
}

func TestRanker_AdjustRanking_EmptyResults(t *testing.T) {
	pg := NewProvenanceGraph()
	ranker := NewRanker(pg, DefaultRankingConfig())

	ctx := context.Background()
	rankedResults, err := ranker.AdjustRanking(ctx, []*models.SearchResult{})
	require.NoError(t, err)
	assert.Equal(t, 0, len(rankedResults))
}

func TestRanker_AdjustRanking_MinScore(t *testing.T) {
	pg := NewProvenanceGraph()
	config := &RankingConfig{
		LatestVersionBoost: 0.15,
		RecencyDecayFactor: 0.1,
		MinScore:           0.7, // 最小スコアを設定
	}
	ranker := NewRanker(pg, config)

	chunkID1 := uuid.New()
	chunkID2 := uuid.New()

	prov1 := &ChunkProvenance{
		ChunkID:          chunkID1,
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "abc123",
		ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	prov2 := &ChunkProvenance{
		ChunkID:          chunkID2,
		SnapshotID:       uuid.New(),
		FilePath:         "src/utils.go",
		GitCommitHash:    "def456",
		ChunkKey:         "product/source/src/utils.go#L1-L10@def456",
		IsLatest:         false,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	require.NoError(t, pg.Add(prov1))
	require.NoError(t, pg.Add(prov2))

	results := []*models.SearchResult{
		{
			ChunkID:   chunkID1,
			FilePath:  "src/main.go",
			StartLine: 1,
			EndLine:   10,
			Content:   "high score content",
			Score:     0.8, // ブースト後: 0.95
		},
		{
			ChunkID:   chunkID2,
			FilePath:  "src/utils.go",
			StartLine: 1,
			EndLine:   10,
			Content:   "low score content",
			Score:     0.65, // 減衰後: 0.55 (MinScore以下)
		},
	}

	ctx := context.Background()
	rankedResults, err := ranker.AdjustRanking(ctx, results)
	require.NoError(t, err)

	// MinScore以下のものは除外されるはず
	assert.Equal(t, 1, len(rankedResults))
	assert.Equal(t, chunkID1, rankedResults[0].ChunkID)
}

func TestRanker_DeduplicateByLatest(t *testing.T) {
	pg := NewProvenanceGraph()
	ranker := NewRanker(pg, DefaultRankingConfig())

	// 同一ファイル・同一範囲の異なるバージョン
	latestChunkID := uuid.New()
	oldChunkID := uuid.New()

	latestProv := &ChunkProvenance{
		ChunkID:          latestChunkID,
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "abc123",
		ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	oldProv := &ChunkProvenance{
		ChunkID:          oldChunkID,
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "old123",
		ChunkKey:         "product/source/src/main.go#L1-L10@old123",
		IsLatest:         false,
		IndexedAt:        time.Now().Add(-1 * time.Hour),
		SourceSnapshotID: uuid.New(),
	}

	require.NoError(t, pg.Add(latestProv))
	require.NoError(t, pg.Add(oldProv))

	rankedResults := []*RankedResult{
		{
			SearchResult: &models.SearchResult{
				ChunkID:   latestChunkID,
				FilePath:  "src/main.go",
				StartLine: 1,
				EndLine:   10,
				Content:   "latest content",
				Score:     0.8,
			},
			OriginalScore: 0.8,
			AdjustedScore: 0.95,
			IsLatest:      true,
			BoostApplied:  0.15,
		},
		{
			SearchResult: &models.SearchResult{
				ChunkID:   oldChunkID,
				FilePath:  "src/main.go",
				StartLine: 1,
				EndLine:   10,
				Content:   "old content",
				Score:     0.85,
			},
			OriginalScore: 0.85,
			AdjustedScore: 0.75,
			IsLatest:      false,
			BoostApplied:  -0.1,
		},
	}

	ctx := context.Background()
	deduplicated, err := ranker.DeduplicateByLatest(ctx, rankedResults)
	require.NoError(t, err)

	// 最新バージョンのみが残るはず
	assert.Equal(t, 1, len(deduplicated))
	assert.Equal(t, latestChunkID, deduplicated[0].ChunkID)
	assert.True(t, deduplicated[0].IsLatest)
}

func TestRanker_FilterByLatestOnly(t *testing.T) {
	pg := NewProvenanceGraph()
	ranker := NewRanker(pg, DefaultRankingConfig())

	latestChunkID := uuid.New()
	oldChunkID := uuid.New()
	unknownChunkID := uuid.New()

	latestProv := &ChunkProvenance{
		ChunkID:          latestChunkID,
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "abc123",
		ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	oldProv := &ChunkProvenance{
		ChunkID:          oldChunkID,
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "old123",
		ChunkKey:         "product/source/src/main.go#L1-L10@old123",
		IsLatest:         false,
		IndexedAt:        time.Now().Add(-1 * time.Hour),
		SourceSnapshotID: uuid.New(),
	}

	require.NoError(t, pg.Add(latestProv))
	require.NoError(t, pg.Add(oldProv))

	results := []*models.SearchResult{
		{ChunkID: latestChunkID, FilePath: "src/main.go", Score: 0.8},
		{ChunkID: oldChunkID, FilePath: "src/main.go", Score: 0.85},
		{ChunkID: unknownChunkID, FilePath: "src/utils.go", Score: 0.75},
	}

	ctx := context.Background()
	filtered, err := ranker.FilterByLatestOnly(ctx, results)
	require.NoError(t, err)

	// 最新バージョン + Provenance情報なしのみが残るはず
	assert.Equal(t, 2, len(filtered))

	foundLatest := false
	foundUnknown := false
	for _, result := range filtered {
		if result.ChunkID == latestChunkID {
			foundLatest = true
		}
		if result.ChunkID == unknownChunkID {
			foundUnknown = true
		}
		// oldChunkIDは含まれないはず
		assert.NotEqual(t, oldChunkID, result.ChunkID)
	}

	assert.True(t, foundLatest)
	assert.True(t, foundUnknown)
}

func TestRanker_GetProvenanceInfo(t *testing.T) {
	pg := NewProvenanceGraph()
	ranker := NewRanker(pg, DefaultRankingConfig())

	chunkID := uuid.New()
	prov := &ChunkProvenance{
		ChunkID:          chunkID,
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "abc123",
		ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	require.NoError(t, pg.Add(prov))

	retrieved, err := ranker.GetProvenanceInfo(chunkID)
	require.NoError(t, err)
	assert.Equal(t, chunkID, retrieved.ChunkID)
	assert.Equal(t, "src/main.go", retrieved.FilePath)
	assert.True(t, retrieved.IsLatest)

	// 存在しないIDの場合
	_, err = ranker.GetProvenanceInfo(uuid.New())
	assert.Error(t, err)
}

func TestExtractBaseChunkKey(t *testing.T) {
	tests := []struct {
		name     string
		chunkKey string
		expected string
	}{
		{
			name:     "with commit hash",
			chunkKey: "product/source/src/main.go#L1-L10@abc123",
			expected: "product/source/src/main.go#L1-L10",
		},
		{
			name:     "without commit hash",
			chunkKey: "product/source/src/main.go#L1-L10",
			expected: "product/source/src/main.go#L1-L10",
		},
		{
			name:     "multiple @ symbols",
			chunkKey: "product/source/src/main@v2.go#L1-L10@abc123",
			expected: "product/source/src/main@v2.go#L1-L10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBaseChunkKey(tt.chunkKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultRankingConfig(t *testing.T) {
	config := DefaultRankingConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 0.15, config.LatestVersionBoost)
	assert.Equal(t, 0.1, config.RecencyDecayFactor)
	assert.Equal(t, 0.0, config.MinScore)
}

func TestRanker_NilConfig(t *testing.T) {
	pg := NewProvenanceGraph()
	ranker := NewRanker(pg, nil)
	assert.NotNil(t, ranker.config)
	assert.Equal(t, DefaultRankingConfig().LatestVersionBoost, ranker.config.LatestVersionBoost)
}
