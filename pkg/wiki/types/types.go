package types

import "github.com/google/uuid"

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
