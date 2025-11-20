package importance

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/indexer/dependency"
)

// ScoreWeights は重要度スコア計算の重みを表します
type ScoreWeights struct {
	ReferenceCount float64 // 参照回数の重み（デフォルト: 0.4）
	Centrality     float64 // 中心性スコアの重み（デフォルト: 0.3）
	EditFrequency  float64 // 編集頻度の重み（デフォルト: 0.3）
}

// DefaultWeights はデフォルトの重み設定を返します
func DefaultWeights() ScoreWeights {
	return ScoreWeights{
		ReferenceCount: 0.4,
		Centrality:     0.3,
		EditFrequency:  0.3,
	}
}

// Validate は重みの合計が1.0であることを検証します
func (w ScoreWeights) Validate() error {
	sum := w.ReferenceCount + w.Centrality + w.EditFrequency
	if math.Abs(sum-1.0) > 0.001 {
		return fmt.Errorf("weights sum must be 1.0, got %.3f", sum)
	}
	return nil
}

// FileEditHistory はファイルの編集履歴情報を表します
type FileEditHistory struct {
	FilePath   string
	EditCount  int       // 編集回数
	LastEdited time.Time // 最終編集日時
}

// ChunkScore は個別チャンクのスコア詳細を表します
type ChunkScore struct {
	ChunkID uuid.UUID

	// 個別スコア（正規化前）
	RawReferenceCount int     // 被参照回数（生値）
	RawCentrality     float64 // 中心性スコア（生値）
	RawEditFrequency  int     // 編集回数（生値）

	// 正規化後のスコア（0.0〜1.0）
	NormalizedReferenceCount float64
	NormalizedCentrality     float64
	NormalizedEditFrequency  float64

	// 総合スコア（重み付け平均、0.0〜1.0）
	FinalScore float64
}

// Calculator は重要度スコアを計算します
type Calculator struct {
	weights     ScoreWeights
	graph       *dependency.Graph
	editHistory map[string]*FileEditHistory // key: file path
}

// NewCalculator は新しいCalculatorを作成します
func NewCalculator(graph *dependency.Graph, editHistory map[string]*FileEditHistory, weights *ScoreWeights) *Calculator {
	w := DefaultWeights()
	if weights != nil {
		w = *weights
	}

	return &Calculator{
		weights:     w,
		graph:       graph,
		editHistory: editHistory,
	}
}

// CalculateAll はすべてのチャンクの重要度スコアを計算します
func (c *Calculator) CalculateAll(ctx context.Context) (map[uuid.UUID]*ChunkScore, error) {
	if err := c.weights.Validate(); err != nil {
		return nil, fmt.Errorf("invalid weights: %w", err)
	}

	// 全チャンクのIDを取得
	chunkIDs := make([]uuid.UUID, 0, len(c.graph.Nodes))
	for chunkID := range c.graph.Nodes {
		chunkIDs = append(chunkIDs, chunkID)
	}

	if len(chunkIDs) == 0 {
		return make(map[uuid.UUID]*ChunkScore), nil
	}

	// 参照回数、中心性、編集頻度の最大値を計算（正規化用）
	maxRefCount := 0
	maxCentrality := 0.0
	maxEditFreq := 0

	rawScores := make(map[uuid.UUID]*ChunkScore)

	for _, chunkID := range chunkIDs {
		node := c.graph.Nodes[chunkID]
		if node == nil {
			continue
		}

		// 参照回数を取得
		refCount := c.graph.GetReferenceCount(chunkID)

		// 中心性スコアを計算
		centrality := c.graph.CalculateCentrality(chunkID)

		// 編集頻度を取得
		editFreq := 0
		if history, ok := c.editHistory[node.FilePath]; ok {
			editFreq = history.EditCount
		}

		// 最大値を更新
		if refCount > maxRefCount {
			maxRefCount = refCount
		}
		if centrality > maxCentrality {
			maxCentrality = centrality
		}
		if editFreq > maxEditFreq {
			maxEditFreq = editFreq
		}

		rawScores[chunkID] = &ChunkScore{
			ChunkID:           chunkID,
			RawReferenceCount: refCount,
			RawCentrality:     centrality,
			RawEditFrequency:  editFreq,
		}
	}

	// 正規化と総合スコア計算
	for chunkID, score := range rawScores {
		// 正規化（0.0〜1.0）
		score.NormalizedReferenceCount = c.normalize(float64(score.RawReferenceCount), float64(maxRefCount))
		score.NormalizedCentrality = c.normalize(score.RawCentrality, maxCentrality)
		score.NormalizedEditFrequency = c.normalize(float64(score.RawEditFrequency), float64(maxEditFreq))

		// 重み付け平均で総合スコアを算出
		score.FinalScore = c.weights.ReferenceCount*score.NormalizedReferenceCount +
			c.weights.Centrality*score.NormalizedCentrality +
			c.weights.EditFrequency*score.NormalizedEditFrequency

		rawScores[chunkID] = score
	}

	return rawScores, nil
}

// Calculate は単一チャンクの重要度スコアを計算します（個別計算用）
func (c *Calculator) Calculate(ctx context.Context, chunkID uuid.UUID) (*ChunkScore, error) {
	if err := c.weights.Validate(); err != nil {
		return nil, fmt.Errorf("invalid weights: %w", err)
	}

	node := c.graph.Nodes[chunkID]
	if node == nil {
		return nil, fmt.Errorf("chunk not found in graph: %s", chunkID)
	}

	// 参照回数を取得
	refCount := c.graph.GetReferenceCount(chunkID)

	// 中心性スコアを計算
	centrality := c.graph.CalculateCentrality(chunkID)

	// 編集頻度を取得
	editFreq := 0
	if history, ok := c.editHistory[node.FilePath]; ok {
		editFreq = history.EditCount
	}

	// グラフ全体の最大値を計算（正規化用）
	maxRefCount := 0
	maxCentrality := 0.0
	maxEditFreq := 0

	for id := range c.graph.Nodes {
		rc := c.graph.GetReferenceCount(id)
		cent := c.graph.CalculateCentrality(id)

		if rc > maxRefCount {
			maxRefCount = rc
		}
		if cent > maxCentrality {
			maxCentrality = cent
		}

		n := c.graph.Nodes[id]
		if n != nil {
			if h, ok := c.editHistory[n.FilePath]; ok {
				if h.EditCount > maxEditFreq {
					maxEditFreq = h.EditCount
				}
			}
		}
	}

	score := &ChunkScore{
		ChunkID:           chunkID,
		RawReferenceCount: refCount,
		RawCentrality:     centrality,
		RawEditFrequency:  editFreq,
	}

	// 正規化（0.0〜1.0）
	score.NormalizedReferenceCount = c.normalize(float64(refCount), float64(maxRefCount))
	score.NormalizedCentrality = c.normalize(centrality, maxCentrality)
	score.NormalizedEditFrequency = c.normalize(float64(editFreq), float64(maxEditFreq))

	// 重み付け平均で総合スコアを算出
	score.FinalScore = c.weights.ReferenceCount*score.NormalizedReferenceCount +
		c.weights.Centrality*score.NormalizedCentrality +
		c.weights.EditFrequency*score.NormalizedEditFrequency

	return score, nil
}

// normalize は値を0.0〜1.0の範囲に正規化します
func (c *Calculator) normalize(value, max float64) float64 {
	if max == 0.0 {
		return 0.0
	}
	normalized := value / max
	if normalized > 1.0 {
		return 1.0
	}
	if normalized < 0.0 {
		return 0.0
	}
	return normalized
}
