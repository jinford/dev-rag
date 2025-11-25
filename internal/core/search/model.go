package search

import (
	"time"

	"github.com/google/uuid"
)

// SearchResult はベクトル検索の結果を表す
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

// SearchFilter は検索時の任意フィルタを表す
type SearchFilter struct {
	PathPrefix  *string
	ContentType *string
}

// ChunkContext はチャンクのコンテキスト情報を表す（階層検索用）
type ChunkContext struct {
	ID        uuid.UUID `json:"id"`
	FileID    uuid.UUID `json:"fileID"`
	Ordinal   int       `json:"ordinal"`
	StartLine int       `json:"startLine"`
	EndLine   int       `json:"endLine"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`

	// 構造メタデータ（階層検索で使用）
	Type       *string `json:"type,omitempty"`
	Name       *string `json:"name,omitempty"`
	ParentName *string `json:"parentName,omitempty"`

	// 階層関係
	Level int `json:"level"`
}
