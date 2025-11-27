package search

import (
	"time"

	"github.com/google/uuid"
	"github.com/samber/mo"
)

// SearchResult はベクトル検索の結果を表す
type SearchResult struct {
	ChunkID     uuid.UUID `json:"chunkID"`
	FilePath    string    `json:"filePath"`
	StartLine   int       `json:"startLine"`
	EndLine     int       `json:"endLine"`
	Content     string    `json:"content"`
	Score       float64   `json:"score"`
	PrevContent *string   `json:"prevContent,omitempty"`
	NextContent *string   `json:"nextContent,omitempty"`
}

// SearchFilter は検索時の任意フィルタを表す
type SearchFilter struct {
	PathPrefix  *string
	ContentType *string
}

// ChunkContext はチャンクのコンテキスト情報を表す（階層検索用）
type ChunkContext struct {
	ID        uuid.UUID `json:"id"`
	FileID    uuid.UUID `json:"fileID"`
	Ordinal   int       `json:"ordinal"`
	StartLine int       `json:"startLine"`
	EndLine   int       `json:"endLine"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`

	// 構造メタデータ（階層検索で使用）
	Type       *string `json:"type,omitempty"`
	Name       *string `json:"name,omitempty"`
	ParentName *string `json:"parentName,omitempty"`

	// 階層関係
	Level int `json:"level"`
}

// SummarySearchResult は要約検索の結果を表す
type SummarySearchResult struct {
	SummaryID   uuid.UUID `json:"summaryID"`
	SummaryType string    `json:"summaryType"` // "file" | "directory" | "architecture"
	TargetPath  string    `json:"targetPath"`
	ArchType    *string   `json:"archType,omitempty"`
	Content     string    `json:"content"`
	Score       float64   `json:"score"`
}

// SummarySearchFilter は要約検索時のフィルタ
type SummarySearchFilter struct {
	SummaryTypes []string // フィルタする要約タイプ（空なら全て）
	PathPrefix   *string  // パスプレフィックスでフィルタ
}

// HybridSearchResult はハイブリッド検索の結果
type HybridSearchResult struct {
	Chunks    []*SearchResult        `json:"chunks"`
	Summaries []*SummarySearchResult `json:"summaries"`
}

// HybridSearchParams はハイブリッド検索のパラメータ
// ProductIDとSnapshotIDの使い分け:
// - ProductID が Some の場合: そのプロダクトに属する全スナップショットを横断検索
// - ProductID が None の場合: SnapshotID で指定された単一スナップショットのみを検索
type HybridSearchParams struct {
	ProductID     mo.Option[uuid.UUID] // プロダクト横断検索用
	SnapshotID    uuid.UUID            // 単一スナップショット検索用
	Query         string
	ChunkLimit    int
	SummaryLimit  int
	ChunkFilter   *SearchFilter
	SummaryFilter *SummarySearchFilter
}

// SummarySearchParams は要約検索のパラメータ
type SummarySearchParams struct {
	SnapshotID uuid.UUID
	Query      string
	Limit      int
	Filter     *SummarySearchFilter
}
