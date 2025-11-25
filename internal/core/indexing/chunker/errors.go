package chunker

import (
	"errors"
	"fmt"
)

var (
	// ErrUnsupportedLanguage はサポートされていない言語の場合に返されます
	ErrUnsupportedLanguage = errors.New("unsupported language")

	// ErrInvalidConfig は設定が不正な場合に返されます
	ErrInvalidConfig = errors.New("invalid chunker config")

	// ErrEmptyContent はコンテンツが空の場合に返されます
	ErrEmptyContent = errors.New("empty content")

	// ErrParseFailed はパース処理が失敗した場合に返されます
	ErrParseFailed = errors.New("parse failed")

	// ErrTokenCountExceeded はトークン数が上限を超えた場合に返されます
	ErrTokenCountExceeded = errors.New("token count exceeded")
)

// ChunkerError はChunker固有のエラーを表します
type ChunkerError struct {
	Op       string // 操作名
	Language Language
	Path     string
	Err      error
}

func (e *ChunkerError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("chunker: %s: %s (language=%s, path=%s)", e.Op, e.Err, e.Language, e.Path)
	}
	return fmt.Sprintf("chunker: %s: %s (language=%s)", e.Op, e.Err, e.Language)
}

func (e *ChunkerError) Unwrap() error {
	return e.Err
}

// NewChunkerError は新しいChunkerErrorを作成します
func NewChunkerError(op string, language Language, path string, err error) *ChunkerError {
	return &ChunkerError{
		Op:       op,
		Language: language,
		Path:     path,
		Err:      err,
	}
}
