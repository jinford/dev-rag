package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// === Product Repository Port ===

// ProductRepository はプロダクト集約の永続化ポートです
type ProductRepository interface {
	ProductReader
	ProductWriter
}

// ProductReader はプロダクトの読み取り操作を定義します
type ProductReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Product, error)
	GetByName(ctx context.Context, name string) (*Product, error)
	List(ctx context.Context) ([]*Product, error)
	GetListWithStats(ctx context.Context) ([]*ProductWithStats, error)
}

// ProductWriter はプロダクトの書き込み操作を定義します
type ProductWriter interface {
	CreateIfNotExists(ctx context.Context, name string, description *string) (*Product, error)
	Update(ctx context.Context, id uuid.UUID, name string, description *string) (*Product, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// === Source Repository Port ===

// SourceRepository はソース集約の永続化ポートです
type SourceRepository interface {
	SourceReader
	SourceWriter
}

// SourceReader はソースの読み取り操作を定義します
type SourceReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Source, error)
	GetByName(ctx context.Context, name string) (*Source, error)
	ListByProductID(ctx context.Context, productID uuid.UUID) ([]*Source, error)
	GetSnapshotByVersion(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*SourceSnapshot, error)
	GetLatestIndexedSnapshot(ctx context.Context, sourceID uuid.UUID) (*SourceSnapshot, error)
	ListSnapshotsBySource(ctx context.Context, sourceID uuid.UUID) ([]*SourceSnapshot, error)
}

// SourceWriter はソースの書き込み操作を定義します
type SourceWriter interface {
	CreateIfNotExists(ctx context.Context, name string, sourceType SourceType, productID uuid.UUID, metadata SourceMetadata) (*Source, error)
	CreateSnapshot(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*SourceSnapshot, error)
	MarkSnapshotIndexed(ctx context.Context, snapshotID uuid.UUID) error
}

// === GitRef Repository Port ===

// GitRefRepository はGit参照の永続化ポートです
type GitRefRepository interface {
	GitRefReader
	GitRefWriter
}

// GitRefReader はGit参照の読み取り操作を定義します
type GitRefReader interface {
	GetByName(ctx context.Context, sourceID uuid.UUID, refName string) (*GitRef, error)
	ListBySource(ctx context.Context, sourceID uuid.UUID) ([]*GitRef, error)
}

// GitRefWriter はGit参照の書き込み操作を定義します
type GitRefWriter interface {
	Upsert(ctx context.Context, sourceID uuid.UUID, refName string, snapshotID uuid.UUID) (*GitRef, error)
}

// === File Repository Port ===

// FileRepository はファイルの永続化ポートです
type FileRepository interface {
	FileReader
	FileWriter
}

// FileReader はファイルの読み取り操作を定義します
type FileReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*File, error)
	ListBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*File, error)
	GetHashesBySnapshot(ctx context.Context, snapshotID uuid.UUID) (map[string]string, error)
	GetByDomain(ctx context.Context, snapshotID uuid.UUID, domain string) ([]*File, error)
}

// FileWriter はファイルの書き込み操作を定義します
type FileWriter interface {
	Create(ctx context.Context, snapshotID uuid.UUID, path string, size int64, contentType string, contentHash string, language *string, domain *string) (*File, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
	DeleteByPaths(ctx context.Context, snapshotID uuid.UUID, paths []string) error
}

// === Chunk Repository Port ===

// ChunkMetadata はチャンクのメタデータを表します
type ChunkMetadata struct {
	// 構造メタデータ
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

	// 階層関係と重要度
	Level           int
	ImportanceScore *float64

	// 詳細な依存関係情報
	StandardImports  []string
	ExternalImports  []string
	InternalCalls    []string
	ExternalCalls    []string
	TypeDependencies []string

	// トレーサビリティ・バージョン管理
	SourceSnapshotID *uuid.UUID
	GitCommitHash    *string
	Author           *string
	UpdatedAt        *time.Time
	FileVersion      *string
	IsLatest         bool

	// 決定的な識別子
	ChunkKey string
}

// ChunkRepository はチャンクの永続化ポートです
type ChunkRepository interface {
	ChunkReader
	ChunkWriter
}

// ChunkReader はチャンクの読み取り操作を定義します
type ChunkReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Chunk, error)
	ListByFile(ctx context.Context, fileID uuid.UUID) ([]*Chunk, error)
	GetContext(ctx context.Context, chunkID uuid.UUID, beforeCount int, afterCount int) ([]*Chunk, error)
	GetChildIDs(ctx context.Context, parentID uuid.UUID) ([]uuid.UUID, error)
	GetParentID(ctx context.Context, chunkID uuid.UUID) (*uuid.UUID, error)
	GetChildren(ctx context.Context, parentID uuid.UUID) ([]*Chunk, error)
	GetParent(ctx context.Context, chunkID uuid.UUID) (*Chunk, error)
	GetTree(ctx context.Context, rootID uuid.UUID, maxDepth int) ([]*Chunk, error)
}

// ChunkWriter はチャンクの書き込み操作を定義します
type ChunkWriter interface {
	Create(ctx context.Context, fileID uuid.UUID, ordinal int, startLine int, endLine int, content string, contentHash string, tokenCount int, metadata *ChunkMetadata) (*Chunk, error)
	BatchCreate(ctx context.Context, chunks []*Chunk) error
	DeleteByFileID(ctx context.Context, fileID uuid.UUID) error
	AddRelation(ctx context.Context, parentID, childID uuid.UUID, ordinal int) error
	RemoveRelation(ctx context.Context, parentID, childID uuid.UUID) error
	UpdateImportanceScore(ctx context.Context, chunkID uuid.UUID, score float64) error
	BatchUpdateImportanceScores(ctx context.Context, scores map[uuid.UUID]float64) error
}

// === Embedding Repository Port ===

// EmbeddingRepository はEmbeddingの永続化ポートです
type EmbeddingRepository interface {
	EmbeddingWriter
}

// EmbeddingWriter はEmbeddingの書き込み操作を定義します
type EmbeddingWriter interface {
	Create(ctx context.Context, chunkID uuid.UUID, vector []float32, model string) error
	BatchCreate(ctx context.Context, embeddings []*Embedding) error
}

// === Dependency Repository Port ===

// DependencyRepository は依存関係の永続化ポートです
type DependencyRepository interface {
	DependencyReader
	DependencyWriter
}

// DependencyReader は依存関係の読み取り操作を定義します
type DependencyReader interface {
	GetByChunk(ctx context.Context, chunkID uuid.UUID) ([]*ChunkDependency, error)
	GetByChunkAndType(ctx context.Context, chunkID uuid.UUID, depType string) ([]*ChunkDependency, error)
	GetIncomingByChunk(ctx context.Context, chunkID uuid.UUID) ([]*ChunkDependency, error)
}

// DependencyWriter は依存関係の書き込み操作を定義します
type DependencyWriter interface {
	Create(ctx context.Context, fromChunkID, toChunkID uuid.UUID, depType, symbol string) error
	DeleteByChunk(ctx context.Context, chunkID uuid.UUID) error
}

// === Snapshot Repository Port ===

// SnapshotFileRepository はスナップショットファイルの永続化ポートです
type SnapshotFileRepository interface {
	SnapshotFileReader
	SnapshotFileWriter
}

// SnapshotFileReader はスナップショットファイルの読み取り操作を定義します
type SnapshotFileReader interface {
	GetBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*SnapshotFile, error)
	GetDomainCoverageStats(ctx context.Context, snapshotID uuid.UUID) ([]*DomainCoverage, error)
	GetUnindexedImportantFiles(ctx context.Context, snapshotID uuid.UUID) ([]string, error)
}

// SnapshotFileWriter はスナップショットファイルの書き込み操作を定義します
type SnapshotFileWriter interface {
	Create(ctx context.Context, snapshotID uuid.UUID, filePath string, fileSize int64, domain *string, indexed bool, skipReason *string) (*SnapshotFile, error)
	UpdateIndexed(ctx context.Context, snapshotID uuid.UUID, filePath string, indexed bool) error
}

// === Domain Coverage Repository Port ===

// DomainCoverageRepository はドメインカバレッジの永続化ポートです
type DomainCoverageRepository interface {
	GetBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*DomainCoverage, error)
}
