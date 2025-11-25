package domain

import (
	"context"

	"github.com/google/uuid"
)

// FileSummarizer はファイルの要約を生成するインターフェース
type FileSummarizer interface {
	// SummarizeFile はファイルの要約を生成します
	SummarizeFile(ctx context.Context, fileID uuid.UUID, content string) (*FileSummaryResult, error)
}

// DirectorySummarizer はディレクトリの要約を生成するインターフェース
type DirectorySummarizer interface {
	// SummarizeDirectory はディレクトリの要約を生成します
	SummarizeDirectory(ctx context.Context, structure *RepoStructure, directory *DirectoryInfo) (*DirectorySummaryResult, error)
}

// ArchitectureSummarizer はアーキテクチャの要約を生成するインターフェース
type ArchitectureSummarizer interface {
	// SummarizeArchitecture はアーキテクチャの要約を生成します
	SummarizeArchitecture(ctx context.Context, structure *RepoStructure, summaryType string) (*ArchitectureSummaryResult, error)
}

// FileSummaryResult はファイル要約の結果
type FileSummaryResult struct {
	FileID    uuid.UUID
	Summary   string
	Embedding []float32
	Metadata  map[string]interface{}
}

// DirectorySummaryResult はディレクトリ要約の結果
type DirectorySummaryResult struct {
	Path       string
	ParentPath string
	Depth      int
	Summary    string
	Embedding  []float32
	Metadata   map[string]interface{}
}

// ArchitectureSummaryResult はアーキテクチャ要約の結果
type ArchitectureSummaryResult struct {
	SnapshotID  uuid.UUID
	SummaryType string
	Summary     string
	Embedding   []float32
	Metadata    map[string]interface{}
}
