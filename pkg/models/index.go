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
	ContentType *string   `json:"contentType,omitempty"`
	ContentHash string    `json:"contentHash"`
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
	TokenCount  *int      `json:"tokenCount,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
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
	FilePath    string  `json:"filePath"`
	StartLine   int     `json:"startLine"`
	EndLine     int     `json:"endLine"`
	Content     string  `json:"content"`
	Score       float64 `json:"score"`
	PrevContent *string `json:"prevContent,omitempty"`
	NextContent *string `json:"nextContent,omitempty"`
}
