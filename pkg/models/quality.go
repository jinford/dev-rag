package models

import (
	"time"

	"github.com/google/uuid"
)

// QualitySeverity は品質ノートの深刻度を表します
type QualitySeverity string

const (
	QualitySeverityCritical QualitySeverity = "critical"
	QualitySeverityHigh     QualitySeverity = "high"
	QualitySeverityMedium   QualitySeverity = "medium"
	QualitySeverityLow      QualitySeverity = "low"
)

// QualityStatus は品質ノートのステータスを表します
type QualityStatus string

const (
	QualityStatusOpen     QualityStatus = "open"
	QualityStatusResolved QualityStatus = "resolved"
)

// QualityNote はRAG回答の品質フィードバックを表します
// 品質フィードバックのデータモデル定義
type QualityNote struct {
	ID           uuid.UUID       `json:"id"`
	NoteID       string          `json:"noteID" validate:"required,max=100"`    // ビジネス識別子（例: QN-2024-001）
	Severity     QualitySeverity `json:"severity" validate:"required,oneof=critical high medium low"`
	NoteText     string          `json:"noteText" validate:"required"`          // 問題の内容
	LinkedFiles  []string        `json:"linkedFiles,omitempty"`                 // 関連ファイルパスのリスト
	LinkedChunks []string        `json:"linkedChunks,omitempty"`                // 関連チャンクIDのリスト
	Reviewer     string          `json:"reviewer" validate:"required,max=255"`  // レビュー者
	Status       QualityStatus   `json:"status" validate:"required,oneof=open resolved"`
	CreatedAt    time.Time       `json:"createdAt"`
	ResolvedAt   *time.Time      `json:"resolvedAt,omitempty"`
}

// IsResolved は品質ノートが解決済みかどうかを返します
func (qn *QualityNote) IsResolved() bool {
	return qn.Status == QualityStatusResolved
}

// IsCritical は品質ノートが致命的かどうかを返します
func (qn *QualityNote) IsCritical() bool {
	return qn.Severity == QualitySeverityCritical
}

// QualityNoteFilter は品質ノートのフィルタ条件を表します
type QualityNoteFilter struct {
	Severity  *QualitySeverity `json:"severity,omitempty"`
	Status    *QualityStatus   `json:"status,omitempty"`
	StartDate *time.Time       `json:"startDate,omitempty"`
	EndDate   *time.Time       `json:"endDate,omitempty"`
	Limit     *int             `json:"limit,omitempty"`
}

// QualityMetrics は品質メトリクスを表します
// 品質メトリクスの定量評価用
type QualityMetrics struct {
	TotalNotes           int                     `json:"totalNotes"`
	OpenNotes            int                     `json:"openNotes"`
	ResolvedNotes        int                     `json:"resolvedNotes"`
	BySeverity           map[QualitySeverity]int `json:"bySeverity"`
	RecentTrend          []QualityTrendPoint     `json:"recentTrend,omitempty"`
	// インデックス鮮度情報
	AverageFreshnessDays float64 `json:"averageFreshnessDays"` // 平均鮮度（日数）
	StaleChunkCount      int     `json:"staleChunkCount"`      // 古いチャンク数
	FreshnessThreshold   int     `json:"freshnessThreshold"`   // 鮮度閾値（日数）
	GeneratedAt          time.Time `json:"generatedAt"`
}

// QualityTrendPoint は品質メトリクスの時系列データポイントを表します
type QualityTrendPoint struct {
	Date          time.Time `json:"date"`
	OpenCount     int       `json:"openCount"`
	ResolvedCount int       `json:"resolvedCount"`
}

// ChunkFreshness はチャンクの鮮度情報を表します
// インデックス鮮度の監視用
type ChunkFreshness struct {
	ChunkID         uuid.UUID `json:"chunkID"`
	FilePath        string    `json:"filePath"`
	ChunkKey        string    `json:"chunkKey"`
	GitCommitHash   string    `json:"gitCommitHash"`
	LatestCommit    string    `json:"latestCommit"`
	FreshnessDays   int       `json:"freshnessDays"`   // 最新コミットとの差分日数
	IsStale         bool      `json:"isStale"`         // 閾値を超えているか
	LastUpdated     time.Time `json:"lastUpdated"`
}

// FreshnessReport はインデックス鮮度レポートを表します
type FreshnessReport struct {
	TotalChunks          int                `json:"totalChunks"`
	StaleChunks          int                `json:"staleChunks"`
	AverageFreshnessDays float64            `json:"averageFreshnessDays"`
	FreshnessThreshold   int                `json:"freshnessThreshold"`
	StaleChunkDetails    []ChunkFreshness   `json:"staleChunkDetails"`
	GeneratedAt          time.Time          `json:"generatedAt"`
}
