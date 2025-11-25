package indexing

import (
	"context"

	"github.com/google/uuid"
)

// Repository はインデックス関連の全データアクセスを統合するインターフェース
// テスト時のモック用に消費者側で定義
type Repository interface {
	// Product
	GetProductByID(ctx context.Context, id uuid.UUID) (*Product, error)
	GetProductByName(ctx context.Context, name string) (*Product, error)
	ListProducts(ctx context.Context) ([]*Product, error)
	ListProductsWithStats(ctx context.Context) ([]*ProductWithStats, error)
	CreateProductIfNotExists(ctx context.Context, name string, description *string) (*Product, error)
	UpdateProduct(ctx context.Context, id uuid.UUID, name string, description *string) (*Product, error)
	DeleteProduct(ctx context.Context, id uuid.UUID) error

	// Source
	GetSourceByID(ctx context.Context, id uuid.UUID) (*Source, error)
	GetSourceByName(ctx context.Context, name string) (*Source, error)
	ListSourcesByProductID(ctx context.Context, productID uuid.UUID) ([]*Source, error)
	CreateSourceIfNotExists(ctx context.Context, name string, sourceType SourceType, productID uuid.UUID, metadata SourceMetadata) (*Source, error)

	// SourceSnapshot
	GetSnapshotByVersion(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*SourceSnapshot, error)
	GetLatestIndexedSnapshot(ctx context.Context, sourceID uuid.UUID) (*SourceSnapshot, error)
	ListSnapshotsBySource(ctx context.Context, sourceID uuid.UUID) ([]*SourceSnapshot, error)
	CreateSnapshot(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*SourceSnapshot, error)
	MarkSnapshotIndexed(ctx context.Context, snapshotID uuid.UUID) error

	// GitRef
	GetGitRefByName(ctx context.Context, sourceID uuid.UUID, refName string) (*GitRef, error)
	ListGitRefsBySource(ctx context.Context, sourceID uuid.UUID) ([]*GitRef, error)
	UpsertGitRef(ctx context.Context, sourceID uuid.UUID, refName string, snapshotID uuid.UUID) (*GitRef, error)

	// File
	GetFileByID(ctx context.Context, id uuid.UUID) (*File, error)
	ListFilesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*File, error)
	GetFileHashesBySnapshot(ctx context.Context, snapshotID uuid.UUID) (map[string]string, error)
	GetFilesByDomain(ctx context.Context, snapshotID uuid.UUID, domain string) ([]*File, error)
	CreateFile(ctx context.Context, snapshotID uuid.UUID, path string, size int64, contentType string, contentHash string, language *string, domain *string) (*File, error)
	DeleteFileByID(ctx context.Context, id uuid.UUID) error
	DeleteFilesByPaths(ctx context.Context, snapshotID uuid.UUID, paths []string) error

	// Chunk
	GetChunkByID(ctx context.Context, id uuid.UUID) (*Chunk, error)
	ListChunksByFile(ctx context.Context, fileID uuid.UUID) ([]*Chunk, error)
	GetChunkContext(ctx context.Context, chunkID uuid.UUID, beforeCount int, afterCount int) ([]*Chunk, error)
	GetChunkChildren(ctx context.Context, parentID uuid.UUID) ([]*Chunk, error)
	GetChunkParent(ctx context.Context, chunkID uuid.UUID) (*Chunk, error)
	GetChunkTree(ctx context.Context, rootID uuid.UUID, maxDepth int) ([]*Chunk, error)
	CreateChunk(ctx context.Context, fileID uuid.UUID, ordinal int, startLine int, endLine int, content string, contentHash string, tokenCount int, metadata *ChunkMetadata) (*Chunk, error)
	BatchCreateChunks(ctx context.Context, chunks []*Chunk) error
	DeleteChunksByFileID(ctx context.Context, fileID uuid.UUID) error
	AddChunkRelation(ctx context.Context, parentID, childID uuid.UUID, ordinal int) error
	UpdateChunkImportanceScore(ctx context.Context, chunkID uuid.UUID, score float64) error
	BatchUpdateChunkImportanceScores(ctx context.Context, scores map[uuid.UUID]float64) error

	// Embedding
	CreateEmbedding(ctx context.Context, chunkID uuid.UUID, vector []float32, model string) error
	BatchCreateEmbeddings(ctx context.Context, embeddings []*Embedding) error

	// ChunkDependency
	GetDependenciesByChunk(ctx context.Context, chunkID uuid.UUID) ([]*ChunkDependency, error)
	GetIncomingDependenciesByChunk(ctx context.Context, chunkID uuid.UUID) ([]*ChunkDependency, error)
	CreateDependency(ctx context.Context, fromChunkID, toChunkID uuid.UUID, depType, symbol string) error
	DeleteDependenciesByChunk(ctx context.Context, chunkID uuid.UUID) error

	// SnapshotFile
	GetSnapshotFiles(ctx context.Context, snapshotID uuid.UUID) ([]*SnapshotFile, error)
	GetDomainCoverageStats(ctx context.Context, snapshotID uuid.UUID) ([]*DomainCoverage, error)
	CreateSnapshotFile(ctx context.Context, snapshotID uuid.UUID, filePath string, fileSize int64, domain *string, indexed bool, skipReason *string) (*SnapshotFile, error)
	UpdateSnapshotFileIndexed(ctx context.Context, snapshotID uuid.UUID, filePath string, indexed bool) error
}
