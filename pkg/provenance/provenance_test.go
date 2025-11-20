package provenance

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvenanceGraph_Add(t *testing.T) {
	pg := NewProvenanceGraph()

	chunkID := uuid.New()
	snapshotID := uuid.New()
	sourceSnapshotID := uuid.New()

	prov := &ChunkProvenance{
		ChunkID:          chunkID,
		SnapshotID:       snapshotID,
		FilePath:         "src/main.go",
		GitCommitHash:    "abc123",
		ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: sourceSnapshotID,
	}

	err := pg.Add(prov)
	require.NoError(t, err)

	assert.Equal(t, 1, pg.Count())

	retrieved, err := pg.Get(chunkID)
	require.NoError(t, err)
	assert.Equal(t, prov.ChunkID, retrieved.ChunkID)
	assert.Equal(t, prov.FilePath, retrieved.FilePath)
	assert.Equal(t, prov.GitCommitHash, retrieved.GitCommitHash)
}

func TestProvenanceGraph_Add_Validation(t *testing.T) {
	pg := NewProvenanceGraph()

	t.Run("nil provenance", func(t *testing.T) {
		err := pg.Add(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("nil chunk ID", func(t *testing.T) {
		prov := &ChunkProvenance{
			ChunkID:          uuid.Nil,
			SnapshotID:       uuid.New(),
			FilePath:         "src/main.go",
			GitCommitHash:    "abc123",
			SourceSnapshotID: uuid.New(),
		}
		err := pg.Add(prov)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "chunk ID")
	})

	t.Run("nil snapshot ID", func(t *testing.T) {
		prov := &ChunkProvenance{
			ChunkID:          uuid.New(),
			SnapshotID:       uuid.Nil,
			FilePath:         "src/main.go",
			GitCommitHash:    "abc123",
			SourceSnapshotID: uuid.New(),
		}
		err := pg.Add(prov)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "snapshot ID")
	})

	t.Run("empty file path", func(t *testing.T) {
		prov := &ChunkProvenance{
			ChunkID:          uuid.New(),
			SnapshotID:       uuid.New(),
			FilePath:         "",
			GitCommitHash:    "abc123",
			SourceSnapshotID: uuid.New(),
		}
		err := pg.Add(prov)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file path")
	})
}

func TestProvenanceGraph_GetByChunkKey(t *testing.T) {
	pg := NewProvenanceGraph()

	// 同じファイル・同じ範囲の異なるバージョンを追加
	chunkKey1 := "product/source/src/main.go#L1-L10@abc123"
	chunkKey2 := "product/source/src/main.go#L1-L10@def456"

	prov1 := &ChunkProvenance{
		ChunkID:          uuid.New(),
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "abc123",
		ChunkKey:         chunkKey1,
		IsLatest:         false,
		IndexedAt:        time.Now().Add(-1 * time.Hour),
		SourceSnapshotID: uuid.New(),
	}

	prov2 := &ChunkProvenance{
		ChunkID:          uuid.New(),
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "def456",
		ChunkKey:         chunkKey2,
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	require.NoError(t, pg.Add(prov1))
	require.NoError(t, pg.Add(prov2))

	// chunkKey1で検索
	chunks, err := pg.GetByChunkKey(chunkKey1)
	require.NoError(t, err)
	assert.Equal(t, 1, len(chunks))
	assert.Equal(t, prov1.ChunkID, chunks[0])

	// chunkKey2で検索
	chunks, err = pg.GetByChunkKey(chunkKey2)
	require.NoError(t, err)
	assert.Equal(t, 1, len(chunks))
	assert.Equal(t, prov2.ChunkID, chunks[0])
}

func TestProvenanceGraph_GetLatestByChunkKey(t *testing.T) {
	pg := NewProvenanceGraph()

	baseKey := "product/source/src/main.go#L1-L10"

	// 古いバージョン
	oldProv := &ChunkProvenance{
		ChunkID:          uuid.New(),
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "abc123",
		ChunkKey:         baseKey + "@abc123",
		IsLatest:         false,
		IndexedAt:        time.Now().Add(-2 * time.Hour),
		SourceSnapshotID: uuid.New(),
	}

	// 最新バージョン
	latestProv := &ChunkProvenance{
		ChunkID:          uuid.New(),
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "def456",
		ChunkKey:         baseKey + "@def456",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	require.NoError(t, pg.Add(oldProv))
	require.NoError(t, pg.Add(latestProv))

	// 最新バージョンを取得
	latest, err := pg.GetLatestByChunkKey(baseKey + "@def456")
	require.NoError(t, err)
	assert.Equal(t, latestProv.ChunkID, latest.ChunkID)
	assert.True(t, latest.IsLatest)
}

func TestProvenanceGraph_GetFileHistory(t *testing.T) {
	pg := NewProvenanceGraph()

	filePath := "src/main.go"

	// 同じファイルの異なるチャンク（異なる行範囲）を追加
	prov1 := &ChunkProvenance{
		ChunkID:          uuid.New(),
		SnapshotID:       uuid.New(),
		FilePath:         filePath,
		GitCommitHash:    "abc123",
		ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	prov2 := &ChunkProvenance{
		ChunkID:          uuid.New(),
		SnapshotID:       uuid.New(),
		FilePath:         filePath,
		GitCommitHash:    "abc123",
		ChunkKey:         "product/source/src/main.go#L11-L20@abc123",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	require.NoError(t, pg.Add(prov1))
	require.NoError(t, pg.Add(prov2))

	history, err := pg.GetFileHistory(filePath)
	require.NoError(t, err)
	assert.Equal(t, filePath, history.FilePath)
	assert.Equal(t, 2, len(history.Versions))
}

func TestProvenanceGraph_GetLatestVersions(t *testing.T) {
	pg := NewProvenanceGraph()

	// 最新バージョン
	latest1 := &ChunkProvenance{
		ChunkID:          uuid.New(),
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "abc123",
		ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	// 古いバージョン
	old1 := &ChunkProvenance{
		ChunkID:          uuid.New(),
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "old123",
		ChunkKey:         "product/source/src/main.go#L1-L10@old123",
		IsLatest:         false,
		IndexedAt:        time.Now().Add(-1 * time.Hour),
		SourceSnapshotID: uuid.New(),
	}

	// 別の最新バージョン
	latest2 := &ChunkProvenance{
		ChunkID:          uuid.New(),
		SnapshotID:       uuid.New(),
		FilePath:         "src/utils.go",
		GitCommitHash:    "def456",
		ChunkKey:         "product/source/src/utils.go#L1-L10@def456",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	require.NoError(t, pg.Add(latest1))
	require.NoError(t, pg.Add(old1))
	require.NoError(t, pg.Add(latest2))

	latestVersions := pg.GetLatestVersions()
	assert.Equal(t, 2, len(latestVersions))

	// 最新バージョンのみが含まれていることを確認
	for _, prov := range latestVersions {
		assert.True(t, prov.IsLatest)
	}
}

func TestProvenanceGraph_IsLatest(t *testing.T) {
	pg := NewProvenanceGraph()

	latestID := uuid.New()
	oldID := uuid.New()

	latest := &ChunkProvenance{
		ChunkID:          latestID,
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "abc123",
		ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	old := &ChunkProvenance{
		ChunkID:          oldID,
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "old123",
		ChunkKey:         "product/source/src/main.go#L1-L10@old123",
		IsLatest:         false,
		IndexedAt:        time.Now().Add(-1 * time.Hour),
		SourceSnapshotID: uuid.New(),
	}

	require.NoError(t, pg.Add(latest))
	require.NoError(t, pg.Add(old))

	isLatest, err := pg.IsLatest(latestID)
	require.NoError(t, err)
	assert.True(t, isLatest)

	isLatest, err = pg.IsLatest(oldID)
	require.NoError(t, err)
	assert.False(t, isLatest)

	// 存在しないIDの場合
	_, err = pg.IsLatest(uuid.New())
	assert.Error(t, err)
}

func TestProvenanceGraph_TraceProvenance(t *testing.T) {
	pg := NewProvenanceGraph()

	chunkID := uuid.New()
	author := "John Doe"
	updatedAt := time.Now().Add(-1 * time.Hour)

	prov := &ChunkProvenance{
		ChunkID:          chunkID,
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "abc123",
		ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
		IsLatest:         true,
		Author:           &author,
		UpdatedAt:        &updatedAt,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	require.NoError(t, pg.Add(prov))

	trace, err := pg.TraceProvenance(chunkID)
	require.NoError(t, err)
	assert.Contains(t, trace, "Chunk ID:")
	assert.Contains(t, trace, "File: src/main.go")
	assert.Contains(t, trace, "Git Commit: abc123")
	assert.Contains(t, trace, "Is Latest: true")
	assert.Contains(t, trace, "Author: John Doe")
}

func TestProvenanceGraph_Clear(t *testing.T) {
	pg := NewProvenanceGraph()

	prov := &ChunkProvenance{
		ChunkID:          uuid.New(),
		SnapshotID:       uuid.New(),
		FilePath:         "src/main.go",
		GitCommitHash:    "abc123",
		ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
		IsLatest:         true,
		IndexedAt:        time.Now(),
		SourceSnapshotID: uuid.New(),
	}

	require.NoError(t, pg.Add(prov))
	assert.Equal(t, 1, pg.Count())

	pg.Clear()
	assert.Equal(t, 0, pg.Count())
}

func TestProvenanceGraph_Concurrent(t *testing.T) {
	pg := NewProvenanceGraph()

	// 並行アクセスのテスト
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			prov := &ChunkProvenance{
				ChunkID:          uuid.New(),
				SnapshotID:       uuid.New(),
				FilePath:         "src/main.go",
				GitCommitHash:    "abc123",
				ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
				IsLatest:         true,
				IndexedAt:        time.Now(),
				SourceSnapshotID: uuid.New(),
			}
			err := pg.Add(prov)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, pg.Count())
}
