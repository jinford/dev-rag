package models

import (
	"time"

	"github.com/google/uuid"
)

// Product はプロダクト（複数のソースをまとめる単位）を表します
type Product struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
