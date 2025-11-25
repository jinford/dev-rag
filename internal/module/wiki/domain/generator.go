package domain

import (
	"context"

	"github.com/google/uuid"
)

// WikiGenerator はWikiページを生成するインターフェース
type WikiGenerator interface {
	// GenerateArchitectureWiki はアーキテクチャページのMarkdownを生成します
	GenerateArchitectureWiki(ctx context.Context, input *ArchitectureWikiInput) (*WikiPage, error)

	// GenerateDirectoryWiki はディレクトリページのMarkdownを生成します
	GenerateDirectoryWiki(ctx context.Context, input *DirectoryWikiInput) (*WikiPage, error)
}

// ArchitectureWikiInput はアーキテクチャWiki生成の入力
type ArchitectureWikiInput struct {
	ProductID             uuid.UUID
	SnapshotIDs           []uuid.UUID
	ArchitectureSummaries []string        // アーキテクチャ要約（優先コンテキスト）
	SearchResults         []*SearchResult // RAG検索結果（補足コンテキスト）
}

// DirectoryWikiInput はディレクトリWiki生成の入力
type DirectoryWikiInput struct {
	SourceID          uuid.UUID
	SnapshotID        uuid.UUID
	DirectoryPath     string
	DirectorySummary  string          // ディレクトリ要約（優先コンテキスト）
	SearchResults     []*SearchResult // RAG検索結果（補足コンテキスト）
}

// WikiPage は生成されたWikiページ
type WikiPage struct {
	Title    string
	Content  string // Markdown形式
	Metadata map[string]interface{}
}
