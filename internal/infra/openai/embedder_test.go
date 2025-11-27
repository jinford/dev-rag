package openai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEmbedderOptionsOverrideDefaults(t *testing.T) {
	embedder := NewEmbedder("dummy-key",
		WithEmbeddingModel("custom-model"),
		WithEmbeddingDimension(42),
	)

	meta := embedder.Metadata()
	assert.Equal(t, "custom-model", meta.ModelName)
	assert.Equal(t, 42, meta.Dimension)
}
