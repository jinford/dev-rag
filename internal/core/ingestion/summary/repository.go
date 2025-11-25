package summary

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrNotFound は要約が見つからない場合のエラー
var ErrNotFound = errors.New("summary not found")

// Repository は要約のデータアクセスインターフェース
type Repository interface {
	// Summary CRUD
	CreateSummary(ctx context.Context, s *Summary) (*Summary, error)
	GetSummaryByID(ctx context.Context, id uuid.UUID) (*Summary, error)
	GetFileSummary(ctx context.Context, snapshotID uuid.UUID, path string) (*Summary, error)
	GetDirectorySummary(ctx context.Context, snapshotID uuid.UUID, path string) (*Summary, error)
	GetArchitectureSummary(ctx context.Context, snapshotID uuid.UUID, archType ArchType) (*Summary, error)
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
	GetSummaryEmbedding(ctx context.Context, summaryID uuid.UUID) (*SummaryEmbedding, error)
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
