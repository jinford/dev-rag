package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestChunkMetadataWithHierarchy は ChunkMetadata構造体に階層関係フィールドが追加されたことを確認します
func TestChunkMetadataWithHierarchy(t *testing.T) {
	metadata := &ChunkMetadata{
		Level:           2,
		ImportanceScore: floatPtr(0.8523),
		IsLatest:        true,
		ChunkKey:        "test/key",
	}

	assert.Equal(t, 2, metadata.Level)
	assert.NotNil(t, metadata.ImportanceScore)
	assert.InDelta(t, 0.8523, *metadata.ImportanceScore, 0.0001)
}

func floatPtr(f float64) *float64 {
	return &f
}
