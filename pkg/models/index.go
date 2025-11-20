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
