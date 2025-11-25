package domain

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/domain/dependency"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultWeights(t *testing.T) {
	weights := DefaultWeights()
	assert.Equal(t, 0.4, weights.ReferenceCount)
	assert.Equal(t, 0.3, weights.Centrality)
	assert.Equal(t, 0.3, weights.EditFrequency)
}

func TestScoreWeights_Validate(t *testing.T) {
	tests := []struct {
		name    string
		weights ScoreWeights
		wantErr bool
	}{
		{
			name:    "デフォルト設定（合計1.0）",
			weights: DefaultWeights(),
			wantErr: false,
		},
		{
			name: "カスタム設定（合計1.0）",
			weights: ScoreWeights{
				ReferenceCount: 0.5,
				Centrality:     0.3,
				EditFrequency:  0.2,
			},
			wantErr: false,
		},
		{
			name: "合計が1.0でない（エラー）",
			weights: ScoreWeights{
				ReferenceCount: 0.5,
				Centrality:     0.5,
				EditFrequency:  0.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.weights.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestImportanceCalculator_CalculateAll(t *testing.T) {
	ctx := context.Background()

	// 依存グラフの作成
	graph := dependency.NewGraph()

	chunk1 := uuid.New()
	chunk2 := uuid.New()
	chunk3 := uuid.New()

	graph.AddNode(&dependency.Node{ChunkID: chunk1, FilePath: "file1.go", Name: "Chunk1"})
	graph.AddNode(&dependency.Node{ChunkID: chunk2, FilePath: "file2.go", Name: "Chunk2"})
	graph.AddNode(&dependency.Node{ChunkID: chunk3, FilePath: "file3.go", Name: "Chunk3"})

	// chunk2 -> chunk1 の依存
	graph.AddEdge(&dependency.Edge{From: chunk2, To: chunk1})
	// chunk3 -> chunk1 の依存
	graph.AddEdge(&dependency.Edge{From: chunk3, To: chunk1})

	// 編集履歴の作成
	editHistory := map[string]*FileEditHistory{
		"file1.go": {
			FilePath:   "file1.go",
			EditCount:  10,
			LastEdited: time.Now(),
		},
		"file2.go": {
			FilePath:   "file2.go",
			EditCount:  5,
			LastEdited: time.Now(),
		},
		"file3.go": {
			FilePath:   "file3.go",
			EditCount:  2,
			LastEdited: time.Now(),
		},
	}

	calculator := NewImportanceCalculator(graph, editHistory, nil)

	scores, err := calculator.CalculateAll(ctx)
	require.NoError(t, err)
	require.Len(t, scores, 3)

	// chunk1は最も参照されているので、スコアが高いはず
	chunk1Score := scores[chunk1]
	require.NotNil(t, chunk1Score)
	assert.Equal(t, 2, chunk1Score.RawReferenceCount) // 2つの他のチャンクから参照されている
	assert.Equal(t, 10, chunk1Score.RawEditFrequency)
	assert.True(t, chunk1Score.FinalScore > 0.0)
	assert.True(t, chunk1Score.FinalScore <= 1.0)

	// chunk2とchunk3は参照されていない
	chunk2Score := scores[chunk2]
	require.NotNil(t, chunk2Score)
	assert.Equal(t, 0, chunk2Score.RawReferenceCount)

	chunk3Score := scores[chunk3]
	require.NotNil(t, chunk3Score)
	assert.Equal(t, 0, chunk3Score.RawReferenceCount)
}

func TestImportanceCalculator_Calculate(t *testing.T) {
	ctx := context.Background()

	graph := dependency.NewGraph()
	chunk1 := uuid.New()
	chunk2 := uuid.New()

	graph.AddNode(&dependency.Node{ChunkID: chunk1, FilePath: "file1.go", Name: "Chunk1"})
	graph.AddNode(&dependency.Node{ChunkID: chunk2, FilePath: "file2.go", Name: "Chunk2"})
	graph.AddEdge(&dependency.Edge{From: chunk2, To: chunk1})

	editHistory := map[string]*FileEditHistory{
		"file1.go": {
			FilePath:   "file1.go",
			EditCount:  5,
			LastEdited: time.Now(),
		},
	}

	calculator := NewImportanceCalculator(graph, editHistory, nil)

	score, err := calculator.Calculate(ctx, chunk1)
	require.NoError(t, err)
	require.NotNil(t, score)

	assert.Equal(t, chunk1, score.ChunkID)
	assert.Equal(t, 1, score.RawReferenceCount)
	assert.Equal(t, 5, score.RawEditFrequency)
	assert.True(t, score.FinalScore >= 0.0)
	assert.True(t, score.FinalScore <= 1.0)
}

func TestImportanceCalculator_CustomWeights(t *testing.T) {
	ctx := context.Background()

	graph := dependency.NewGraph()
	chunk1 := uuid.New()
	graph.AddNode(&dependency.Node{ChunkID: chunk1, FilePath: "file1.go", Name: "Chunk1"})

	editHistory := map[string]*FileEditHistory{}

	// カスタム重み（参照回数を重視）
	customWeights := ScoreWeights{
		ReferenceCount: 0.8,
		Centrality:     0.1,
		EditFrequency:  0.1,
	}

	calculator := NewImportanceCalculator(graph, editHistory, &customWeights)

	scores, err := calculator.CalculateAll(ctx)
	require.NoError(t, err)
	require.Len(t, scores, 1)
}

func TestImportanceCalculator_EmptyGraph(t *testing.T) {
	ctx := context.Background()

	graph := dependency.NewGraph()
	editHistory := map[string]*FileEditHistory{}

	calculator := NewImportanceCalculator(graph, editHistory, nil)

	scores, err := calculator.CalculateAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, scores)
}

func TestImportanceCalculator_Normalize(t *testing.T) {
	calculator := &ScoreCalculator{}

	tests := []struct {
		name     string
		value    float64
		max      float64
		expected float64
	}{
		{
			name:     "通常の正規化",
			value:    50.0,
			max:      100.0,
			expected: 0.5,
		},
		{
			name:     "最大値と同じ",
			value:    100.0,
			max:      100.0,
			expected: 1.0,
		},
		{
			name:     "ゼロ",
			value:    0.0,
			max:      100.0,
			expected: 0.0,
		},
		{
			name:     "最大値がゼロ",
			value:    50.0,
			max:      0.0,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculator.normalize(tt.value, tt.max)
			assert.Equal(t, tt.expected, result)
		})
	}
}
