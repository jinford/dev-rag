package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// === Source集約: Source（ルート）+ SourceSnapshot + GitRef ===

// Source は情報ソース（Git、Confluence、PDF等）の基本情報を表します
type Source struct {
	ID         uuid.UUID      `json:"id"`
	ProductID  *uuid.UUID     `json:"productID,omitempty"`
	Name       string         `json:"name"`
	SourceType SourceType     `json:"sourceType"`
	Metadata   SourceMetadata `json:"metadata"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// SourceType はソースの種別を表します
type SourceType string

const (
	SourceTypeGit        SourceType = "git"
	SourceTypeConfluence SourceType = "confluence"
	SourceTypeRedmine    SourceType = "redmine"
	SourceTypeLocal      SourceType = "local"
)

// SourceMetadata はソースタイプ固有のメタデータを表します
type SourceMetadata map[string]any

// Value はdatabase/sql/driver.Valuerインターフェースの実装
func (m SourceMetadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// Scan はdatabase/sql.Scannerインターフェースの実装
func (m *SourceMetadata) Scan(value any) error {
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, m)
}

// SourceSnapshot はソースの特定バージョン時点のスナップショットを表します
type SourceSnapshot struct {
	ID                uuid.UUID  `json:"id"`
	SourceID          uuid.UUID  `json:"sourceID"`
	VersionIdentifier string     `json:"versionIdentifier"`
	Indexed           bool       `json:"indexed"`
	IndexedAt         *time.Time `json:"indexedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
}

// GitRef はGit専用の参照（ブランチ、タグ）を表します
type GitRef struct {
	ID         uuid.UUID `json:"id"`
	SourceID   uuid.UUID `json:"sourceID"`
	RefName    string    `json:"refName"`
	SnapshotID uuid.UUID `json:"snapshotID"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
