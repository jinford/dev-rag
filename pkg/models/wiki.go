package models

import (
	"time"

	"github.com/google/uuid"
)

// WikiMetadata はWiki生成の実行履歴とメタデータを表します
type WikiMetadata struct {
	ID          uuid.UUID `json:"id"`
	ProductID   uuid.UUID `json:"productID"`
	OutputPath  string    `json:"outputPath"`
	FileCount   int       `json:"fileCount"`
	GeneratedAt time.Time `json:"generatedAt"`
	CreatedAt   time.Time `json:"createdAt"`
}
