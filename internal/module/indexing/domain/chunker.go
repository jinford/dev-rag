package domain

import (
	"context"
)

// Chunker はファイルをチャンクに分割する戦略インターフェース
type Chunker interface {
	// Chunk はファイルの内容をチャンクに分割します
	// path: ファイルパス、content: ファイル内容
	Chunk(ctx context.Context, path string, content string) ([]*Chunk, error)
}

// ChunkerFactory は言語に応じた適切な Chunker を取得するファクトリインターフェース
type ChunkerFactory interface {
	// GetChunker は指定された言語に対応する Chunker を取得します
	GetChunker(language string) (Chunker, error)
}

// LanguageDetector はファイルの言語を検出するインターフェース
type LanguageDetector interface {
	// DetectLanguage はファイルパスと内容から言語を検出します
	DetectLanguage(path string, content []byte) (string, error)
}

// FileFilter はファイルのインデックス対象判定を行うインターフェース
type FileFilter interface {
	// ShouldIndex はファイルをインデックス対象とすべきかを判定します
	ShouldIndex(file *File) (bool, error)
}
