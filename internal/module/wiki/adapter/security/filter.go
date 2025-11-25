package security

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Filter は秘匿情報のフィルタリングを実装する
type Filter struct {
	// 秘匿情報検出用の正規表現パターン
	sensitivePatterns []*regexp.Regexp

	// 除外するファイル名パターン
	sensitiveFilePatterns []string
}

// NewFilter は新しいSecurityFilterを作成する
func NewFilter() *Filter {
	return &Filter{
		sensitivePatterns: compileSensitivePatterns(),
		sensitiveFilePatterns: []string{
			// 環境変数ファイル
			".env",
			".env.*",
			"*.env",
			// 認証情報ファイル
			"*credentials*",
			"*secrets*",
			"*password*",
			"*token*",
			"*apikey*",
			"*private*",
			"*secret*",
			// 証明書・キー
			"*.pem",
			"*.key",
			"*.p12",
			"*.pfx",
			"*.jks",
			// SSH関連
			"id_rsa",
			"id_dsa",
			"id_ecdsa",
			"id_ed25519",
			"*.pub",
			// クラウドプロバイダー関連
			".aws/credentials",
			".gcp/credentials",
			".azure/credentials",
			// その他
			"*.keystore",
			"*.truststore",
		},
	}
}

// compileSensitivePatterns は秘匿情報を検出するための正規表現パターンをコンパイルする
func compileSensitivePatterns() []*regexp.Regexp {
	patterns := []string{
		// APIキー (汎用)
		`(?i)api[_-]?key\s*[:=]\s*["\']?([a-zA-Z0-9_\-]{20,})["\']?`,
		// AWS関連
		`(?i)aws[_-]?access[_-]?key[_-]?id\s*[:=]\s*["\']?([A-Z0-9]{20})["\']?`,
		`(?i)aws[_-]?secret[_-]?access[_-]?key\s*[:=]\s*["\']?([A-Za-z0-9/+=]{40})["\']?`,
		// GCP関連
		`(?i)google[_-]?api[_-]?key\s*[:=]\s*["\']?([a-zA-Z0-9_\-]{39})["\']?`,
		// GitHub関連
		`(?i)github[_-]?token\s*[:=]\s*["\']?(ghp_[a-zA-Z0-9]{20,})["\']?`,
		// OpenAI関連
		`(?i)openai[_-]?api[_-]?key\s*[:=]\s*["\']?(sk-[a-zA-Z0-9]{48})["\']?`,
		// Slack関連
		`(?i)slack[_-]?token\s*[:=]\s*["\']?(xox[baprs]-[a-zA-Z0-9\-]{10,72})["\']?`,
		// プライベートキー
		`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`,
		`-----BEGIN\s+ENCRYPTED\s+PRIVATE\s+KEY-----`,
		`-----BEGIN\s+OPENSSH\s+PRIVATE\s+KEY-----`,
		// パスワード（一般的なパターン）
		`(?i)password\s*[:=]\s*["\']([^"\']{8,})["\']`,
		`(?i)passwd\s*[:=]\s*["\']([^"\']{8,})["\']`,
		// データベース接続文字列
		`(?i)(postgres|mysql|mongodb|redis)://[^:]+:([^@]+)@`,
		// JWT トークン
		`eyJ[a-zA-Z0-9_\-]+\.eyJ[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+`,
		// Bearer トークン
		`(?i)bearer\s+[a-zA-Z0-9_\-\.]{20,}`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			// コンパイルエラーは無視（開発時に検出されるべき）
			continue
		}
		compiled = append(compiled, re)
	}
	return compiled
}

// ContainsSensitiveInfo はコンテンツに秘匿情報が含まれているかチェックする
func (f *Filter) ContainsSensitiveInfo(content string) bool {
	for _, pattern := range f.sensitivePatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// MaskSensitiveInfo はコンテンツ内の秘匿情報をマスクする
func (f *Filter) MaskSensitiveInfo(content string) string {
	masked := content
	for _, pattern := range f.sensitivePatterns {
		masked = pattern.ReplaceAllString(masked, "***MASKED***")
	}
	return masked
}

// FilterFiles はファイルリストから秘匿情報を含む可能性のあるファイルをフィルタリングする
// 秘匿情報を含む可能性のあるファイルを除外したリストを返す
func (f *Filter) FilterFiles(files []string) []string {
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		if !f.isSensitiveFile(file) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

// isSensitiveFile はファイルが秘匿情報を含む可能性があるかチェックする
func (f *Filter) isSensitiveFile(filePath string) bool {
	fileName := filepath.Base(filePath)
	lowerFileName := strings.ToLower(fileName)

	for _, pattern := range f.sensitiveFilePatterns {
		// ワイルドカード展開
		if strings.Contains(pattern, "*") {
			matched, err := filepath.Match(strings.ToLower(pattern), lowerFileName)
			if err == nil && matched {
				return true
			}
		} else {
			// 完全一致チェック
			if strings.ToLower(pattern) == lowerFileName {
				return true
			}
		}
	}

	// ディレクトリパスに秘匿情報を示すキーワードが含まれているかチェック
	lowerPath := strings.ToLower(filePath)
	sensitiveKeywords := []string{
		"secrets",
		"credentials",
		"private",
		".ssh",
		".gnupg",
		".password",
	}

	for _, keyword := range sensitiveKeywords {
		if strings.Contains(lowerPath, keyword) {
			return true
		}
	}

	return false
}

// ShouldExclude はファイルパスが除外対象かどうかを判定する
func (f *Filter) ShouldExclude(path string) bool {
	return f.isSensitiveFile(path)
}
