package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ChunkContext はチャンクの重要度評価に必要なコンテキスト情報を表します
type ChunkContext struct {
	// ファイルメタデータ
	FilePath    string
	FileSize    int64
	ContentType string
	Language    *string
	Domain      *string

	// 依存情報
	ImportCount      int      // このチャンクがインポートされている数
	CallCount        int      // このチャンクが呼び出されている数
	DependencyDepth  int      // 依存グラフにおける深さ
	Centrality       float64  // グラフの中心性スコア
	ImportedPackages []string // インポートしているパッケージ
	CalledFunctions  []string // 呼び出している関数

	// 編集履歴
	CommitHash    *string    // Git コミットハッシュ
	Author        *string    // 最終更新者
	UpdatedAt     *time.Time // ファイル最終更新日時
	EditFrequency int        // 編集頻度（過去N日間の編集回数など）

	// コード品質指標
	LinesOfCode          *int     // コード行数
	CommentRatio         *float64 // コメント比率
	CyclomaticComplexity *int     // 循環的複雑度

	// スナップショット情報
	SnapshotID       uuid.UUID
	VersionIdentifier string
}

// ImportanceEvaluator はチャンクの重要度を評価するインターフェース
type ImportanceEvaluator interface {
	// Evaluate はチャンクとそのコンテキスト情報から重要度スコアを計算します
	Evaluate(ctx context.Context, chunk *Chunk, ctxInfo ChunkContext) (float64, error)
}

// ImportanceCalculator は後方互換性のため残すエイリアス（非推奨）
// 新規コードでは ImportanceEvaluator を使用してください
type ImportanceCalculator interface {
	// CalculateFileImportance はファイルの重要度を計算します
	CalculateFileImportance(file *File) float64

	// CalculateChunkImportance はチャンクの重要度を計算します
	CalculateChunkImportance(chunk *Chunk) float64
}
