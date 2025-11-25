package detector

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-enry/go-enry/v2"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// contentTypeDetector はファイルの種別（MIMEタイプ）を判定します
type contentTypeDetector struct{}

// NewContentTypeDetector は新しいContentTypeDetectorを作成します
func NewContentTypeDetector() domain.Detector {
	return &contentTypeDetector{}
}

// DetectContentType はファイルパスと内容からMIMEタイプを判定します
func (d *contentTypeDetector) DetectContentType(path string, content []byte) string {
	// ファイル名を取得
	filename := filepath.Base(path)

	// go-enryで言語を判定（ファイル名と内容の両方を使用）
	language := enry.GetLanguage(filename, content)

	// 言語からMIMEタイプへのマッピング
	mimeType := languageToMimeType(language)
	if mimeType != "" {
		return mimeType
	}

	// http.DetectContentTypeでファイル内容から判定
	// 最初の512バイトを使用
	if len(content) > 0 {
		detectedType := http.DetectContentType(content)
		// パラメータ部分（; charset=utf-8など）を除去
		if idx := strings.Index(detectedType, ";"); idx != -1 {
			detectedType = detectedType[:idx]
		}
		return strings.TrimSpace(detectedType)
	}

	// 内容が空の場合はプレーンテキスト
	return "text/plain"
}

// languageToMimeType は言語名をMIMEタイプに変換します
func languageToMimeType(language string) string {
	// go-enryが返す言語名とMIMEタイプのマッピング
	mapping := map[string]string{
		"Go":         "text/x-go",
		"JavaScript": "text/javascript",
		"TypeScript": "text/x-typescript",
		"Python":     "text/x-python",
		"Java":       "text/x-java",
		"C":          "text/x-c",
		"C++":        "text/x-c++",
		"C#":         "text/x-csharp",
		"Ruby":       "text/x-ruby",
		"PHP":        "text/x-php",
		"Rust":       "text/x-rust",
		"Swift":      "text/x-swift",
		"Kotlin":     "text/x-kotlin",
		"Scala":      "text/x-scala",
		"Shell":      "text/x-shellscript",
		"Bash":       "text/x-shellscript",
		"Markdown":   "text/markdown",
		"HTML":       "text/html",
		"CSS":        "text/css",
		"SCSS":       "text/x-scss",
		"SASS":       "text/x-sass",
		"Less":       "text/x-less",
		"JSON":       "application/json",
		"YAML":       "text/x-yaml",
		"XML":        "text/xml",
		"SQL":        "text/x-sql",
		"Dockerfile": "text/x-dockerfile",
		"Makefile":   "text/x-makefile",
		"Protocol Buffer": "text/x-protobuf",
		"Thrift":     "text/x-thrift",
		"GraphQL":    "application/graphql",
		"Terraform":  "text/x-terraform",
		"HCL":        "text/x-hcl",
	}

	if mime, ok := mapping[language]; ok {
		return mime
	}

	return ""
}
