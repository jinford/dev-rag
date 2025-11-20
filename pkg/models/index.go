package models

import (
	"time"

	"github.com/google/uuid"
)

// === Index集約: File + Chunk + Embedding ===

// File はスナップショット内のファイル情報を表します
type File struct {
	ID          uuid.UUID `json:"id"`
	SnapshotID  uuid.UUID `json:"snapshotID"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType"`
	ContentHash string    `json:"contentHash"`
	Language    *string   `json:"language,omitempty"` // Phase 1追加
	Domain      *string   `json:"domain,omitempty"`   // Phase 1追加
	CreatedAt   time.Time `json:"createdAt"`
}

// Chunk はファイルを分割したチャンクを表します
type Chunk struct {
	ID          uuid.UUID `json:"id"`
	FileID      uuid.UUID `json:"fileID"`
	Ordinal     int       `json:"ordinal"`
	StartLine   int       `json:"startLine"`
	EndLine     int       `json:"endLine"`
	Content     string    `json:"content"`
	ContentHash string    `json:"contentHash"`
	TokenCount  int       `json:"tokenCount"`
	CreatedAt   time.Time `json:"createdAt"`

	// 構造メタデータ (Phase 1追加)
	Type                 *string  `json:"type,omitempty"`
	Name                 *string  `json:"name,omitempty"`
	ParentName           *string  `json:"parentName,omitempty"`
	Signature            *string  `json:"signature,omitempty"`
	DocComment           *string  `json:"docComment,omitempty"`
	Imports              []string `json:"imports,omitempty"`
	Calls                []string `json:"calls,omitempty"`
	LinesOfCode          *int     `json:"linesOfCode,omitempty"`
	CommentRatio         *float64 `json:"commentRatio,omitempty"`
	CyclomaticComplexity *int     `json:"cyclomaticComplexity,omitempty"`
	EmbeddingContext     *string  `json:"embeddingContext,omitempty"`

	// 階層関係と重要度 (Phase 2追加)
	Level           int      `json:"level"`
	ImportanceScore *float64 `json:"importanceScore,omitempty"`

	// 詳細な依存関係情報 (Phase 2タスク4追加)
	StandardImports  []string `json:"standardImports,omitempty"`  // 標準ライブラリ
	ExternalImports  []string `json:"externalImports,omitempty"`  // 外部依存
	InternalCalls    []string `json:"internalCalls,omitempty"`    // 内部関数呼び出し
	ExternalCalls    []string `json:"externalCalls,omitempty"`    // 外部関数呼び出し
	TypeDependencies []string `json:"typeDependencies,omitempty"` // 型依存

	// トレーサビリティ・バージョン管理 (Phase 1追加)
	SourceSnapshotID *uuid.UUID `json:"sourceSnapshotID,omitempty"`
	GitCommitHash    *string    `json:"gitCommitHash,omitempty"`
	Author           *string    `json:"author,omitempty"`
	UpdatedAt        *time.Time `json:"updatedAt,omitempty"` // ファイル最終更新日時
	IndexedAt        time.Time  `json:"indexedAt"`
	FileVersion      *string    `json:"fileVersion,omitempty"`
	IsLatest         bool       `json:"isLatest"`

	// 決定的な識別子 (Phase 1追加)
	ChunkKey string `json:"chunkKey"` // {product_name}/{source_name}/{file_path}#L{start}-L{end}@{commit_hash}
}

// Embedding はチャンクのEmbeddingベクトルを表します
type Embedding struct {
	ChunkID   uuid.UUID `json:"chunkID"`
	Vector    []float32 `json:"vector"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"createdAt"`
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

// SnapshotFile はスナップショット内の全ファイルリスト（インデックス対象外含む）を表します
// Phase 2タスク7: カバレッジマップ構築用
type SnapshotFile struct {
	ID         uuid.UUID `json:"id"`
	SnapshotID uuid.UUID `json:"snapshotID"`
	FilePath   string    `json:"filePath"`
	FileSize   int64     `json:"fileSize"`
	Domain     *string   `json:"domain,omitempty"`
	Indexed    bool      `json:"indexed"`
	SkipReason *string   `json:"skipReason,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// DomainCoverage はドメイン別のカバレッジ情報を表します
// Phase 2タスク7: カバレッジマップ構築用
type DomainCoverage struct {
	Domain           string  `json:"domain"`
	TotalFiles       int     `json:"totalFiles"`
	IndexedFiles     int     `json:"indexedFiles"`
	IndexedChunks    int     `json:"indexedChunks"`
	CoverageRate     float64 `json:"coverageRate"`
	AvgCommentRatio  float64 `json:"avgCommentRatio"`
	AvgComplexity    float64 `json:"avgComplexity"`
	UnindexedImportantFiles []string `json:"unindexedImportantFiles,omitempty"`
}

// CoverageMap はリポジトリ全体のカバレッジマップを表します
// Phase 2タスク7: カバレッジマップ構築用
type CoverageMap struct {
	SnapshotID       string           `json:"snapshotID"`
	SnapshotVersion  string           `json:"snapshotVersion"`
	TotalFiles       int              `json:"totalFiles"`
	TotalIndexedFiles int             `json:"totalIndexedFiles"`
	TotalChunks      int              `json:"totalChunks"`
	OverallCoverage  float64          `json:"overallCoverage"`
	DomainCoverages  []DomainCoverage `json:"domainCoverages"`
	GeneratedAt      time.Time        `json:"generatedAt"`
}

// AlertSeverity はアラートの深刻度を表します
// Phase 2タスク8: カバレッジアラート機能用
type AlertSeverity string

const (
	AlertSeverityWarning AlertSeverity = "warning"
	AlertSeverityError   AlertSeverity = "error"
)

// Alert はカバレッジに関するアラートを表します
// Phase 2タスク8: カバレッジアラート機能用
type Alert struct {
	Severity    AlertSeverity `json:"severity"`
	Message     string        `json:"message"`
	Domain      string        `json:"domain,omitempty"`
	Details     interface{}   `json:"details,omitempty"`
	GeneratedAt time.Time     `json:"generatedAt"`
}
