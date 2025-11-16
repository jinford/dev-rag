package models

import (
	"time"

	"github.com/google/uuid"
)

// Embedding はチャンクのEmbeddingベクトルを表します
type Embedding struct {
	ChunkID   uuid.UUID `json:"chunkID"`
	Vector    []float32 `json:"vector"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"createdAt"`
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
