package summary

import (
	"context"

	"github.com/google/uuid"
	"github.com/samber/mo"
)

// Repository は要約のデータアクセスインターフェース
type Repository interface {
	// Summary CRUD
	CreateSummary(ctx context.Context, s *Summary) (*Summary, error)
	GetSummaryByID(ctx context.Context, id uuid.UUID) (mo.Option[*Summary], error)
	GetFileSummary(ctx context.Context, snapshotID uuid.UUID, path string) (mo.Option[*Summary], error)
	GetDirectorySummary(ctx context.Context, snapshotID uuid.UUID, path string) (mo.Option[*Summary], error)
	GetArchitectureSummary(ctx context.Context, snapshotID uuid.UUID, archType ArchType) (mo.Option[*Summary], error)
	UpdateSummary(ctx context.Context, s *Summary) error
	DeleteSummary(ctx context.Context, id uuid.UUID) error

	// Summary一覧取得
	ListFileSummariesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*Summary, error)
	ListDirectorySummariesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*Summary, error)
	ListDirectorySummariesByDepth(ctx context.Context, snapshotID uuid.UUID, depth int) ([]*Summary, error)
	ListArchitectureSummariesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*Summary, error)

	// 差分検知用
	GetMaxDirectoryDepth(ctx context.Context, snapshotID uuid.UUID) (int, error)

	// Embedding
	CreateSummaryEmbedding(ctx context.Context, e *SummaryEmbedding) error
	UpsertSummaryEmbedding(ctx context.Context, e *SummaryEmbedding) error
	GetSummaryEmbedding(ctx context.Context, summaryID uuid.UUID) (mo.Option[*SummaryEmbedding], error)
}

// LLMClient はLLM呼び出しのインターフェース
type LLMClient interface {
	GenerateCompletion(ctx context.Context, prompt string) (string, error)
}

// Embedder はEmbedding生成のインターフェース
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	ModelName() string
}
