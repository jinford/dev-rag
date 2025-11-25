package wiki

import (
	"context"

	"github.com/google/uuid"
)

// Repository はWiki生成に必要なデータアクセスインターフェース
type Repository interface {
	// WikiMetadata の操作
	GetWikiMetadata(ctx context.Context, productID uuid.UUID) (*WikiMetadata, error)
	CreateWikiMetadata(ctx context.Context, productID uuid.UUID, outputPath string, fileCount int) (*WikiMetadata, error)
	UpdateWikiMetadata(ctx context.Context, id uuid.UUID, fileCount int) error

	// FileSummary の操作
	GetFileSummary(ctx context.Context, fileID uuid.UUID) (*FileSummary, error)
	ListFileSummaries(ctx context.Context, productID uuid.UUID) ([]*FileSummary, error)
	CreateFileSummary(ctx context.Context, fileID uuid.UUID, summary string, embedding []float32, metadata []byte) (*FileSummary, error)
	UpdateFileSummary(ctx context.Context, id uuid.UUID, summary string, embedding []float32, metadata []byte) error
	DeleteFileSummary(ctx context.Context, id uuid.UUID) error

	// リポジトリ構造の取得
	GetRepoStructure(ctx context.Context, sourceID uuid.UUID, snapshotID uuid.UUID) (*RepoStructure, error)

	// ソースとスナップショット情報の取得
	GetSourceInfo(ctx context.Context, sourceID uuid.UUID) (*SourceInfo, error)
	GetSnapshotInfo(ctx context.Context, snapshotID uuid.UUID) (*SnapshotInfo, error)
}
