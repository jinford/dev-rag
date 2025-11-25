package indexing

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-enry/go-enry/v2"
)

// ContentTypeDetector はファイルの種別（MIMEタイプ）を判定する。
type ContentTypeDetector struct{}

// NewContentTypeDetector は ContentTypeDetector を生成する。
func NewContentTypeDetector() *ContentTypeDetector {
	return &ContentTypeDetector{}
}

// DetectContentType はファイルパスと内容からMIMEタイプを判定する。
func (d *ContentTypeDetector) DetectContentType(path string, content []byte) string {
	filename := filepath.Base(path)
	language := enry.GetLanguage(filename, content)

	if mime := languageToMimeType(language); mime != "" {
		return mime
	}

	if len(content) > 0 {
		detected := http.DetectContentType(content)
		if idx := strings.Index(detected, ";"); idx != -1 {
			detected = detected[:idx]
		}
		return strings.TrimSpace(detected)
	}

	return "text/plain"
}

func languageToMimeType(language string) string {
	mapping := map[string]string{
		"Go":              "text/x-go",
		"JavaScript":      "text/javascript",
		"TypeScript":      "text/x-typescript",
		"Python":          "text/x-python",
		"Java":            "text/x-java",
		"C":               "text/x-c",
		"C++":             "text/x-c++",
		"C#":              "text/x-csharp",
		"Ruby":            "text/x-ruby",
		"PHP":             "text/x-php",
		"Rust":            "text/x-rust",
		"Swift":           "text/x-swift",
		"Kotlin":          "text/x-kotlin",
		"Scala":           "text/x-scala",
		"Shell":           "text/x-shellscript",
		"Bash":            "text/x-shellscript",
		"Markdown":        "text/markdown",
		"HTML":            "text/html",
		"CSS":             "text/css",
		"SCSS":            "text/x-scss",
		"SASS":            "text/x-sass",
		"Less":            "text/x-less",
		"JSON":            "application/json",
		"YAML":            "text/x-yaml",
		"XML":             "text/xml",
		"SQL":             "text/x-sql",
		"Dockerfile":      "text/x-dockerfile",
		"Makefile":        "text/x-makefile",
		"Protocol Buffer": "text/x-protobuf",
		"Thrift":          "text/x-thrift",
		"GraphQL":         "application/graphql",
		"Terraform":       "text/x-terraform",
		"HCL":             "text/x-hcl",
	}
	if mime, ok := mapping[language]; ok {
		return mime
	}
	return ""
}
