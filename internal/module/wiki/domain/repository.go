package domain

import (
	"context"

	"github.com/google/uuid"
)

// === Wiki Metadata Repository Port ===

// WikiMetadataRepository はWikiメタデータの永続化ポートです
type WikiMetadataRepository interface {
	WikiMetadataReader
	WikiMetadataWriter
}

// WikiMetadataReader はWikiメタデータの読み取り操作を定義します
type WikiMetadataReader interface {
	GetByProductID(ctx context.Context, productID uuid.UUID) (*WikiMetadata, error)
}

// WikiMetadataWriter はWikiメタデータの書き込み操作を定義します
type WikiMetadataWriter interface {
	Upsert(ctx context.Context, productID uuid.UUID, outputPath string, fileCount int) (*WikiMetadata, error)
	Delete(ctx context.Context, productID uuid.UUID) error
}

// === File Summary Repository Port ===

// FileSummaryRepository はファイルサマリーの永続化ポートです
type FileSummaryRepository interface {
	FileSummaryReader
	FileSummaryWriter
}

// FileSummaryReader はファイルサマリーの読み取り操作を定義します
type FileSummaryReader interface {
	// 将来的な検索機能のための予約
}

// FileSummaryWriter はファイルサマリーの書き込み操作を定義します
type FileSummaryWriter interface {
	Upsert(ctx context.Context, fileID uuid.UUID, summary string, embedding []float32, metadataJSON []byte) (*FileSummary, error)
}

// === Directory Summary Repository Port ===

// DirectorySummaryRepository はディレクトリサマリーの永続化ポートです
// 将来の実装のためのプレースホルダー
type DirectorySummaryRepository interface {
	DirectorySummaryReader
	DirectorySummaryWriter
}

// DirectorySummaryReader はディレクトリサマリーの読み取り操作を定義します
type DirectorySummaryReader interface {
	// 将来実装予定
}

// DirectorySummaryWriter はディレクトリサマリーの書き込み操作を定義します
type DirectorySummaryWriter interface {
	// 将来実装予定
}

// === Architecture Summary Repository Port ===

// ArchitectureSummaryRepository はアーキテクチャサマリーの永続化ポートです
// 将来の実装のためのプレースホルダー
type ArchitectureSummaryRepository interface {
	ArchitectureSummaryReader
	ArchitectureSummaryWriter
}

// ArchitectureSummaryReader はアーキテクチャサマリーの読み取り操作を定義します
type ArchitectureSummaryReader interface {
	// 将来実装予定
}

// ArchitectureSummaryWriter はアーキテクチャサマリーの書き込み操作を定義します
type ArchitectureSummaryWriter interface {
	// 将来実装予定
}

// === Indexing Data Reader Port ===

// IndexingDataReader は indexing モジュールのデータを読み取るポートです
// Wiki 生成で必要な indexing データへのアクセスを提供します
type IndexingDataReader interface {
	// ListSourcesByProduct はプロダクトに属するソース一覧を取得します
	ListSourcesByProduct(ctx context.Context, productID uuid.UUID) ([]*SourceInfo, error)

	// GetLatestIndexedSnapshot は最新のインデックス済みスナップショットを取得します
	GetLatestIndexedSnapshot(ctx context.Context, sourceID uuid.UUID) (*SnapshotInfo, error)

	// ListFilesBySnapshot はスナップショット配下のファイル一覧を取得します
	ListFilesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*FileInfo, error)
}

// Searcher はベクトル検索を実行するポートです
// search モジュールの Searcher と同じインターフェースを持つ必要があります
type Searcher interface {
	Search(ctx context.Context, params SearchParams) (*SearchResponse, error)
}

// SearchParams は検索パラメータ
type SearchParams struct {
	Query         string
	Limit         int
	ProductID     *uuid.UUID
	SourceID      *uuid.UUID
	PathPrefix    string
	ContentType   string
	ContextBefore int
	ContextAfter  int
}

// SearchResponse は検索レスポンス
type SearchResponse struct {
	Results []*SearchResult
}

// SearchResult はベクトル検索の結果を表します
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
