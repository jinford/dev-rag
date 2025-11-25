package indexing

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// === Product ===

// Product はプロダクト(複数のソースをまとめる単位)を表す
type Product struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ProductWithStats はプロダクトと統計情報を含む構造体
type ProductWithStats struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Description     *string    `json:"description,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	SourceCount     int        `json:"sourceCount"`
	LastIndexedAt   *time.Time `json:"lastIndexedAt,omitempty"`
	WikiGeneratedAt *time.Time `json:"wikiGeneratedAt,omitempty"`
}

// === Source ===

// Source は情報ソース(Git、Confluence、PDF等)の基本情報を表す
type Source struct {
	ID         uuid.UUID      `json:"id"`
	ProductID  uuid.UUID      `json:"productID"`
	Name       string         `json:"name"`
	SourceType SourceType     `json:"sourceType"`
	Metadata   SourceMetadata `json:"metadata"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// SourceType はソースの種別を表す
type SourceType string

const (
	SourceTypeGit        SourceType = "git"
	SourceTypeConfluence SourceType = "confluence"
	SourceTypeRedmine    SourceType = "redmine"
	SourceTypeLocal      SourceType = "local"
)

// SourceMetadata はソースタイプ固有のメタデータを表す
type SourceMetadata map[string]any

func (m SourceMetadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

func (m *SourceMetadata) Scan(value any) error {
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, m)
}

// SourceSnapshot はソースの特定バージョン時点のスナップショットを表す
type SourceSnapshot struct {
	ID                uuid.UUID  `json:"id"`
	SourceID          uuid.UUID  `json:"sourceID"`
	VersionIdentifier string     `json:"versionIdentifier"`
	Indexed           bool       `json:"indexed"`
	IndexedAt         *time.Time `json:"indexedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
}

// GitRef はGit専用の参照(ブランチ、タグ)を表す
type GitRef struct {
	ID         uuid.UUID `json:"id"`
	SourceID   uuid.UUID `json:"sourceID"`
	RefName    string    `json:"refName"`
	SnapshotID uuid.UUID `json:"snapshotID"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// === Index (File + Chunk + Embedding) ===

// File はスナップショット内のファイル情報を表す
type File struct {
	ID          uuid.UUID `json:"id"`
	SnapshotID  uuid.UUID `json:"snapshotID"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType"`
	ContentHash string    `json:"contentHash"`
	Language    *string   `json:"language,omitempty"`
	Domain      *string   `json:"domain,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Chunk はファイルを分割したチャンクを表す
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

	// 構造メタデータ
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

	// 階層関係と重要度
	Level           int      `json:"level"`
	ImportanceScore *float64 `json:"importanceScore,omitempty"`

	// 詳細な依存関係情報
	StandardImports  []string `json:"standardImports,omitempty"`
	ExternalImports  []string `json:"externalImports,omitempty"`
	InternalCalls    []string `json:"internalCalls,omitempty"`
	ExternalCalls    []string `json:"externalCalls,omitempty"`
	TypeDependencies []string `json:"typeDependencies,omitempty"`

	// トレーサビリティ・バージョン管理
	SourceSnapshotID *uuid.UUID `json:"sourceSnapshotID,omitempty"`
	GitCommitHash    *string    `json:"gitCommitHash,omitempty"`
	Author           *string    `json:"author,omitempty"`
	UpdatedAt        *time.Time `json:"updatedAt,omitempty"`
	IndexedAt        time.Time  `json:"indexedAt"`
	FileVersion      *string    `json:"fileVersion,omitempty"`
	IsLatest         bool       `json:"isLatest"`

	// 決定的な識別子
	ChunkKey string `json:"chunkKey"`
}

// ChunkMetadata はチャンク作成時のメタデータを表す
type ChunkMetadata struct {
	Type                 *string
	Name                 *string
	ParentName           *string
	Signature            *string
	DocComment           *string
	Imports              []string
	Calls                []string
	LinesOfCode          *int
	CommentRatio         *float64
	CyclomaticComplexity *int
	EmbeddingContext     *string
	Level                int
	ImportanceScore      *float64
	StandardImports      []string
	ExternalImports      []string
	InternalCalls        []string
	ExternalCalls        []string
	TypeDependencies     []string
	SourceSnapshotID     *uuid.UUID
	GitCommitHash        *string
	Author               *string
	UpdatedAt            *time.Time
	FileVersion          *string
	IsLatest             bool
	ChunkKey             string
}

// Embedding はチャンクのEmbeddingベクトルを表す
type Embedding struct {
	ChunkID   uuid.UUID `json:"chunkID"`
	Vector    []float32 `json:"vector"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"createdAt"`
}

// ChunkDependency はチャンク間の依存関係を表す
type ChunkDependency struct {
	ID          uuid.UUID `json:"id"`
	FromChunkID uuid.UUID `json:"fromChunkID"`
	ToChunkID   uuid.UUID `json:"toChunkID"`
	DepType     string    `json:"depType"`
	Symbol      *string   `json:"symbol,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// === Coverage ===

// SnapshotFile はスナップショット内の全ファイルリスト(インデックス対象外含む)を表す
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

// DomainCoverage はドメイン別のカバレッジ情報を表す
type DomainCoverage struct {
	Domain                  string   `json:"domain"`
	TotalFiles              int      `json:"totalFiles"`
	IndexedFiles            int      `json:"indexedFiles"`
	IndexedChunks           int      `json:"indexedChunks"`
	CoverageRate            float64  `json:"coverageRate"`
	AvgCommentRatio         float64  `json:"avgCommentRatio"`
	AvgComplexity           float64  `json:"avgComplexity"`
	UnindexedImportantFiles []string `json:"unindexedImportantFiles,omitempty"`
}

// CoverageMap はリポジトリ全体のカバレッジマップを表す
type CoverageMap struct {
	SnapshotID        string           `json:"snapshotID"`
	SnapshotVersion   string           `json:"snapshotVersion"`
	TotalFiles        int              `json:"totalFiles"`
	TotalIndexedFiles int              `json:"totalIndexedFiles"`
	TotalChunks       int              `json:"totalChunks"`
	OverallCoverage   float64          `json:"overallCoverage"`
	DomainCoverages   []DomainCoverage `json:"domainCoverages"`
	GeneratedAt       time.Time        `json:"generatedAt"`
}

// AlertSeverity はアラートの深刻度を表す
type AlertSeverity string

const (
	AlertSeverityWarning AlertSeverity = "warning"
	AlertSeverityError   AlertSeverity = "error"
)

// Alert はカバレッジに関するアラートを表す
type Alert struct {
	Severity    AlertSeverity `json:"severity"`
	Message     string        `json:"message"`
	Domain      string        `json:"domain,omitempty"`
	Details     interface{}   `json:"details,omitempty"`
	GeneratedAt time.Time     `json:"generatedAt"`
}
