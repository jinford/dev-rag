package wiki

import (
	"time"

	"github.com/google/uuid"
)

// WikiMetadata はWiki生成の実行履歴とメタデータを表す
type WikiMetadata struct {
	ID          uuid.UUID `json:"id"`
	ProductID   uuid.UUID `json:"productID"`
	OutputPath  string    `json:"outputPath"`
	FileCount   int       `json:"fileCount"`
	GeneratedAt time.Time `json:"generatedAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

// FileSummary はファイルの要約情報を表す
type FileSummary struct {
	ID        uuid.UUID `json:"id"`
	FileID    uuid.UUID `json:"fileID"`
	Summary   string    `json:"summary"`
	Embedding []float32 `json:"embedding"`
	Metadata  []byte    `json:"metadata"` // JSONB
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RepoStructure はリポジトリの構造情報を保持する
type RepoStructure struct {
	SourceID    uuid.UUID        // ソースID
	SnapshotID  uuid.UUID        // スナップショットID
	RootPath    string           // リポジトリのルートパス
	Directories []*DirectoryInfo // すべてのディレクトリ
	Files       []*FileInfo      // すべてのファイル
}

// DirectoryInfo はディレクトリの情報を保持する
type DirectoryInfo struct {
	Path           string         // ディレクトリパス
	ParentPath     string         // 親ディレクトリ（空文字列=ルート）
	Depth          int            // 深さ（0=ルート）
	Files          []string       // このディレクトリ直下のファイル
	Subdirectories []string       // このディレクトリ直下のサブディレクトリ
	TotalFiles     int            // 配下すべてのファイル数
	Languages      map[string]int // 使用言語とファイル数
}

// FileInfo はファイルの情報を保持する
type FileInfo struct {
	FileID   uuid.UUID // ファイルID
	Path     string    // ファイルパス
	Size     int64     // ファイルサイズ
	Language string    // プログラミング言語
	Domain   string    // ドメイン分類
	Hash     string    // コンテンツハッシュ
}

// SourceInfo はソース情報を表す
type SourceInfo struct {
	ID   uuid.UUID
	Name string
}

// SnapshotInfo はスナップショット情報を表す
type SnapshotInfo struct {
	ID                uuid.UUID
	SourceID          uuid.UUID
	VersionIdentifier string
	Indexed           bool
	IndexedAt         *time.Time
	CreatedAt         time.Time
}
