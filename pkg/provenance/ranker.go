package provenance

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
)

// RankingConfig はランキング調整の設定を表します
type RankingConfig struct {
	// LatestVersionBoost は最新バージョンのチャンクに与えるスコアブースト (0.0〜1.0)
	// デフォルト: 0.15 (15%のブースト)
	LatestVersionBoost float64

	// RecencyDecayFactor は古いバージョンに対する減衰係数 (0.0〜1.0)
	// 0.0 = 減衰なし、1.0 = 完全に減衰
	// デフォルト: 0.1 (10%の減衰)
	RecencyDecayFactor float64

	// MinScore は最小スコア（これ以下のスコアは除外）
	// デフォルト: 0.0 (除外しない)
	MinScore float64
}

// DefaultRankingConfig はデフォルトのランキング設定を返します
func DefaultRankingConfig() *RankingConfig {
	return &RankingConfig{
		LatestVersionBoost: 0.15,
		RecencyDecayFactor: 0.1,
		MinScore:           0.0,
	}
}

// Ranker は検索結果のランキングを調整します
type Ranker struct {
	graph  *ProvenanceGraph
	config *RankingConfig
}

// NewRanker は新しいRankerを生成します
func NewRanker(graph *ProvenanceGraph, config *RankingConfig) *Ranker {
	if config == nil {
		config = DefaultRankingConfig()
	}
	return &Ranker{
		graph:  graph,
		config: config,
	}
}

// RankedResult はランキング調整後の検索結果を表します
type RankedResult struct {
	*models.SearchResult
	OriginalScore float64 `json:"originalScore"` // 元のスコア
	AdjustedScore float64 `json:"adjustedScore"` // 調整後のスコア
	IsLatest      bool    `json:"isLatest"`      // 最新バージョンかどうか
	BoostApplied  float64 `json:"boostApplied"`  // 適用されたブースト値
}

// AdjustRanking は検索結果のランキングを調整します
// 最新バージョンのチャンクにブーストを与え、古いバージョンには減衰を適用します
func (r *Ranker) AdjustRanking(ctx context.Context, results []*models.SearchResult) ([]*RankedResult, error) {
	if len(results) == 0 {
		return []*RankedResult{}, nil
	}

	rankedResults := make([]*RankedResult, 0, len(results))

	for _, result := range results {
		// Provenance情報を取得
		prov, err := r.graph.Get(result.ChunkID)
		if err != nil {
			// Provenance情報がない場合はスコアをそのまま使用
			rankedResults = append(rankedResults, &RankedResult{
				SearchResult:  result,
				OriginalScore: result.Score,
				AdjustedScore: result.Score,
				IsLatest:      false,
				BoostApplied:  0.0,
			})
			continue
		}

		// スコア調整の計算
		originalScore := result.Score
		boost := 0.0

		if prov.IsLatest {
			// 最新バージョンにはブーストを適用
			boost = r.config.LatestVersionBoost
		} else {
			// 古いバージョンには減衰を適用（マイナスのブースト）
			boost = -r.config.RecencyDecayFactor
		}

		// 調整後のスコアを計算（スコアは0.0〜1.0の範囲と仮定）
		adjustedScore := originalScore + boost
		// スコアが1.0を超えないようにクリップ
		adjustedScore = math.Min(adjustedScore, 1.0)
		// スコアが0.0未満にならないようにクリップ
		adjustedScore = math.Max(adjustedScore, 0.0)

		rankedResults = append(rankedResults, &RankedResult{
			SearchResult:  result,
			OriginalScore: originalScore,
			AdjustedScore: adjustedScore,
			IsLatest:      prov.IsLatest,
			BoostApplied:  boost,
		})
	}

	// 調整後のスコアでソート（降順）
	sort.Slice(rankedResults, func(i, j int) bool {
		return rankedResults[i].AdjustedScore > rankedResults[j].AdjustedScore
	})

	// 最小スコア以下の結果を除外
	if r.config.MinScore > 0.0 {
		filtered := make([]*RankedResult, 0)
		for _, rr := range rankedResults {
			if rr.AdjustedScore >= r.config.MinScore {
				filtered = append(filtered, rr)
			}
		}
		rankedResults = filtered
	}

	return rankedResults, nil
}

// DeduplicateByLatest は同一ファイルの同一範囲に対して複数バージョンが含まれる場合、
// 最新バージョンのみを残して重複を除外します
func (r *Ranker) DeduplicateByLatest(ctx context.Context, results []*RankedResult) ([]*RankedResult, error) {
	if len(results) == 0 {
		return []*RankedResult{}, nil
	}

	// ChunkKeyベースで重複を検出
	chunkKeyToResults := make(map[string][]*RankedResult)

	for _, result := range results {
		prov, err := r.graph.Get(result.ChunkID)
		if err != nil {
			// Provenance情報がない場合はそのまま追加
			continue
		}

		// ChunkKeyから@コミットハッシュを除外したベースキーを生成
		// 例: "product/source/file.go#L10-L20@abc123" -> "product/source/file.go#L10-L20"
		baseKey := extractBaseChunkKey(prov.ChunkKey)
		chunkKeyToResults[baseKey] = append(chunkKeyToResults[baseKey], result)
	}

	deduplicated := make([]*RankedResult, 0)

	for _, groupResults := range chunkKeyToResults {
		if len(groupResults) == 1 {
			// 重複がない場合はそのまま追加
			deduplicated = append(deduplicated, groupResults[0])
			continue
		}

		// 重複がある場合は最新バージョンのみを残す
		var latest *RankedResult
		for _, result := range groupResults {
			if result.IsLatest {
				latest = result
				break
			}
		}

		if latest == nil {
			// IsLatestフラグがない場合は、AdjustedScoreが最も高いものを選択
			latest = groupResults[0]
			for _, result := range groupResults[1:] {
				if result.AdjustedScore > latest.AdjustedScore {
					latest = result
				}
			}
		}

		deduplicated = append(deduplicated, latest)
	}

	// 調整後のスコアで再ソート
	sort.Slice(deduplicated, func(i, j int) bool {
		return deduplicated[i].AdjustedScore > deduplicated[j].AdjustedScore
	})

	return deduplicated, nil
}

// extractBaseChunkKey はChunkKeyから@コミットハッシュを除外したベースキーを返します
// 例: "product/source/file.go#L10-L20@abc123" -> "product/source/file.go#L10-L20"
func extractBaseChunkKey(chunkKey string) string {
	for i := len(chunkKey) - 1; i >= 0; i-- {
		if chunkKey[i] == '@' {
			return chunkKey[:i]
		}
	}
	return chunkKey
}

// FilterByLatestOnly は最新バージョンのチャンクのみをフィルタリングします
func (r *Ranker) FilterByLatestOnly(ctx context.Context, results []*models.SearchResult) ([]*models.SearchResult, error) {
	if len(results) == 0 {
		return []*models.SearchResult{}, nil
	}

	filtered := make([]*models.SearchResult, 0)

	for _, result := range results {
		isLatest, err := r.graph.IsLatest(result.ChunkID)
		if err != nil {
			// Provenance情報がない場合は含める
			filtered = append(filtered, result)
			continue
		}

		if isLatest {
			filtered = append(filtered, result)
		}
	}

	return filtered, nil
}

// GetProvenanceInfo は検索結果のProvenance情報を取得します
func (r *Ranker) GetProvenanceInfo(chunkID uuid.UUID) (*ChunkProvenance, error) {
	prov, err := r.graph.Get(chunkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provenance info for chunk %s: %w", chunkID, err)
	}
	return prov, nil
}
