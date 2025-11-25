package summary

import (
	"time"

	"github.com/google/uuid"
)

// SummaryType は要約の種類
type SummaryType string

const (
	SummaryTypeFile         SummaryType = "file"
	SummaryTypeDirectory    SummaryType = "directory"
	SummaryTypeArchitecture SummaryType = "architecture"
)

// ArchType はアーキテクチャ要約の種類
type ArchType string

const (
	ArchTypeOverview   ArchType = "overview"
	ArchTypeTechStack  ArchType = "tech_stack"
	ArchTypeDataFlow   ArchType = "data_flow"
	ArchTypeComponents ArchType = "components"
)

// Summary は要約のドメインモデル
type Summary struct {
	ID          uuid.UUID
	SnapshotID  uuid.UUID
	SummaryType SummaryType
	TargetPath  string
	Depth       *int
	ParentPath  *string
	ArchType    *ArchType
	Content     string
	ContentHash string
	SourceHash  string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SummaryEmbedding は要約のEmbedding
type SummaryEmbedding struct {
	SummaryID uuid.UUID
	Vector    []float32
	Model     string
	CreatedAt time.Time
}

// FileInfo はファイル情報（要約生成用）
type FileInfo struct {
	ID          uuid.UUID
	SnapshotID  uuid.UUID
	Path        string
	ContentHash string
	Language    *string
}

// DirectoryInfo はディレクトリ情報（要約生成用）
type DirectoryInfo struct {
	Path           string
	Depth          int
	ParentPath     *string
	Files          []*FileInfo
	Subdirectories []string
}
