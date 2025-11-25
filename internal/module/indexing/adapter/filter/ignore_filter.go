package filter

import (
	"fmt"
	"os"
	"path/filepath"

	gitignore "github.com/sabhiram/go-gitignore"
)

// IgnoreFilter は .gitignore と .devragignore のパターンマッチングを提供します
type IgnoreFilter struct {
	patterns *gitignore.GitIgnore
}

// NewIgnoreFilter は新しいIgnoreFilterを作成します
// repoPath 配下の .gitignore と .devragignore を読み込みます
func NewIgnoreFilter(repoPath string) (*IgnoreFilter, error) {
	var patterns []string

	// .gitignore を読み込み
	gitignorePath := filepath.Join(repoPath, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		gitignorePatterns, err := readIgnoreFile(gitignorePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read .gitignore: %w", err)
		}
		patterns = append(patterns, gitignorePatterns...)
	}

	// .devragignore を読み込み
	devragignorePath := filepath.Join(repoPath, ".devragignore")
	if _, err := os.Stat(devragignorePath); err == nil {
		devragignorePatterns, err := readIgnoreFile(devragignorePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read .devragignore: %w", err)
		}
		patterns = append(patterns, devragignorePatterns...)
	}

	// デフォルトの除外パターンを追加
	patterns = append(patterns, getDefaultIgnorePatterns()...)

	// GitIgnoreオブジェクトを作成
	var matcher *gitignore.GitIgnore
	if len(patterns) > 0 {
		matcher = gitignore.CompileIgnoreLines(patterns...)
	}

	return &IgnoreFilter{
		patterns: matcher,
	}, nil
}

// ShouldIgnore はパスが除外対象かどうかを判定します
func (f *IgnoreFilter) ShouldIgnore(path string) bool {
	if f.patterns == nil {
		return false
	}
	return f.patterns.MatchesPath(path)
}

// readIgnoreFile は ignore ファイルを読み込んでパターンのスライスを返します
func readIgnoreFile(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var patterns []string
	lines := splitLines(string(content))
	for _, line := range lines {
		// 空行とコメント行をスキップ
		if line == "" || line[0] == '#' {
			continue
		}
		patterns = append(patterns, line)
	}

	return patterns, nil
}

// splitLines は文字列を行に分割します
func splitLines(s string) []string {
	var lines []string
	var line string
	for _, c := range s {
		if c == '\n' || c == '\r' {
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
		} else {
			line += string(c)
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// getDefaultIgnorePatterns はデフォルトの除外パターンを返します
func getDefaultIgnorePatterns() []string {
	return []string{
		// Git関連
		".git",
		".gitignore",
		".gitattributes",
		".gitmodules",

		// 依存関係・ビルド成果物
		"node_modules",
		"vendor",
		"dist",
		"build",
		"target",
		"out",
		"bin",
		"obj",
		".next",
		".nuxt",
		".vuepress/dist",

		// IDE/エディタ関連
		".vscode",
		".idea",
		".DS_Store",
		"*.swp",
		"*.swo",
		"*~",

		// ログファイル
		"*.log",
		"logs",

		// 一時ファイル
		"*.tmp",
		"*.temp",
		"tmp",
		"temp",

		// 環境変数・機密情報
		".env",
		".env.local",
		".env.*.local",
		"*.pem",
		"*.key",
		"*.crt",
		"*.p12",

		// バイナリファイル
		"*.exe",
		"*.dll",
		"*.so",
		"*.dylib",
		"*.a",
		"*.o",
		"*.jar",
		"*.war",
		"*.ear",
		"*.zip",
		"*.tar",
		"*.gz",
		"*.bz2",
		"*.7z",
		"*.rar",

		// 画像・メディアファイル（大きいファイルを除外）
		"*.png",
		"*.jpg",
		"*.jpeg",
		"*.gif",
		"*.bmp",
		"*.ico",
		"*.svg",
		"*.webp",
		"*.mp4",
		"*.avi",
		"*.mov",
		"*.wmv",
		"*.flv",
		"*.mp3",
		"*.wav",
		"*.ogg",
		"*.flac",

		// フォント
		"*.ttf",
		"*.otf",
		"*.woff",
		"*.woff2",
		"*.eot",

		// データベースファイル
		"*.db",
		"*.sqlite",
		"*.sqlite3",

		// テストカバレッジ
		"coverage",
		".coverage",
		"*.cover",
		"*.lcov",

		// キャッシュ
		".cache",
		"*.cache",
		"__pycache__",
		"*.pyc",
		".pytest_cache",

		// ドキュメント生成
		".docusaurus",
		"docs/.vuepress/.cache",
		"docs/.vuepress/dist",
	}
}
