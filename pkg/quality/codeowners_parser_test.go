package quality

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCodeowners(t *testing.T) {
	parser := NewCodeownersParser()

	// テスト用の一時ディレクトリを作成
	tmpDir := t.TempDir()

	// CODEOWNERSファイルを作成
	codeownersPath := filepath.Join(tmpDir, "CODEOWNERS")
	content := `# This is a comment
# Default owners
* @default-owner

# Go files
*.go @go-team @backend-team

# Frontend files
/frontend/ @frontend-team

# Documentation
/docs/ @doc-team
README.md @all-team

# Specific file
/config/database.yml @dba-team @backend-team
`
	err := os.WriteFile(codeownersPath, []byte(content), 0644)
	require.NoError(t, err)

	// パース実行
	result, err := parser.ParseCodeowners(tmpDir)
	require.NoError(t, err)

	// 検証
	assert.Len(t, result, 6)
	assert.Equal(t, []string{"@default-owner"}, result["*"])
	assert.Equal(t, []string{"@go-team", "@backend-team"}, result["*.go"])
	assert.Equal(t, []string{"@frontend-team"}, result["/frontend/"])
	assert.Equal(t, []string{"@doc-team"}, result["/docs/"])
	assert.Equal(t, []string{"@all-team"}, result["README.md"])
	assert.Equal(t, []string{"@dba-team", "@backend-team"}, result["/config/database.yml"])
}

func TestParseCodeowners_NoFile(t *testing.T) {
	parser := NewCodeownersParser()

	// CODEOWNERSファイルが存在しないディレクトリ
	tmpDir := t.TempDir()

	// パース実行
	result, err := parser.ParseCodeowners(tmpDir)
	require.NoError(t, err)

	// 空のマップが返される
	assert.Empty(t, result)
}

func TestParseCodeowners_GitHubLocation(t *testing.T) {
	parser := NewCodeownersParser()

	// テスト用の一時ディレクトリを作成
	tmpDir := t.TempDir()
	githubDir := filepath.Join(tmpDir, ".github")
	err := os.Mkdir(githubDir, 0755)
	require.NoError(t, err)

	// .github/CODEOWNERSファイルを作成
	codeownersPath := filepath.Join(githubDir, "CODEOWNERS")
	content := `* @github-team`
	err = os.WriteFile(codeownersPath, []byte(content), 0644)
	require.NoError(t, err)

	// パース実行
	result, err := parser.ParseCodeowners(tmpDir)
	require.NoError(t, err)

	// 検証
	assert.Len(t, result, 1)
	assert.Equal(t, []string{"@github-team"}, result["*"])
}

func TestGetOwnersForFile(t *testing.T) {
	parser := NewCodeownersParser()

	codeownersLookup := map[string][]string{
		"*":                    {"@default-owner"},
		"*.go":                 {"@go-team"},
		"/frontend/":           {"@frontend-team"},
		"/docs/":               {"@doc-team"},
		"README.md":            {"@all-team"},
		"/config/database.yml": {"@dba-team"},
	}

	tests := []struct {
		name           string
		filePath       string
		expectedOwners []string
	}{
		{
			name:           "完全一致: README.md",
			filePath:       "README.md",
			expectedOwners: []string{"@all-team"},
		},
		{
			name:           "完全一致: /config/database.yml",
			filePath:       "/config/database.yml",
			expectedOwners: []string{"@dba-team"},
		},
		{
			name:           "パターンマッチ: *.go",
			filePath:       "main.go",
			expectedOwners: []string{"@go-team"},
		},
		{
			name:           "パターンマッチ: *.go (ディレクトリ内)",
			filePath:       "pkg/models/user.go",
			expectedOwners: []string{"@go-team"},
		},
		{
			name:           "ディレクトリマッチ: /frontend/",
			filePath:       "frontend/app.js",
			expectedOwners: []string{"@frontend-team"},
		},
		{
			name:           "ディレクトリマッチ: /docs/",
			filePath:       "docs/guide.md",
			expectedOwners: []string{"@doc-team"},
		},
		{
			name:           "デフォルトオーナー",
			filePath:       "unknown/file.txt",
			expectedOwners: []string{"@default-owner"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owners := parser.GetOwnersForFile(tt.filePath, codeownersLookup)
			assert.Equal(t, tt.expectedOwners, owners)
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		filePath string
		expected bool
	}{
		{
			name:     "ワイルドカード: すべてのファイル",
			pattern:  "*",
			filePath: "any/file.txt",
			expected: true,
		},
		{
			name:     "ワイルドカード: 拡張子マッチ",
			pattern:  "*.go",
			filePath: "main.go",
			expected: true,
		},
		{
			name:     "ワイルドカード: 拡張子マッチ (ディレクトリ内)",
			pattern:  "*.go",
			filePath: "pkg/models/user.go",
			expected: true,
		},
		{
			name:     "ワイルドカード: 拡張子アンマッチ",
			pattern:  "*.go",
			filePath: "main.js",
			expected: false,
		},
		{
			name:     "ディレクトリマッチ: 完全一致",
			pattern:  "/frontend/",
			filePath: "frontend",
			expected: true,
		},
		{
			name:     "ディレクトリマッチ: サブファイル",
			pattern:  "/frontend/",
			filePath: "frontend/app.js",
			expected: true,
		},
		{
			name:     "ディレクトリマッチ: アンマッチ",
			pattern:  "/frontend/",
			filePath: "backend/app.js",
			expected: false,
		},
		{
			name:     "絶対パスマッチ",
			pattern:  "/config/database.yml",
			filePath: "config/database.yml",
			expected: true,
		},
		{
			name:     "絶対パスアンマッチ",
			pattern:  "/config/database.yml",
			filePath: "other/database.yml",
			expected: false,
		},
		{
			name:     "ファイル名のみ: 完全一致",
			pattern:  "README.md",
			filePath: "README.md",
			expected: true,
		},
		{
			name:     "ファイル名のみ: ディレクトリ内",
			pattern:  "README.md",
			filePath: "docs/README.md",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchPattern(tt.pattern, tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		str      string
		expected bool
	}{
		{
			name:     "完全一致",
			pattern:  "test.go",
			str:      "test.go",
			expected: true,
		},
		{
			name:     "完全アンマッチ",
			pattern:  "test.go",
			str:      "test.js",
			expected: false,
		},
		{
			name:     "単一ワイルドカード",
			pattern:  "*.go",
			str:      "main.go",
			expected: true,
		},
		{
			name:     "単一ワイルドカード: アンマッチ",
			pattern:  "*.go",
			str:      "main.js",
			expected: false,
		},
		{
			name:     "複数ワイルドカード",
			pattern:  "*_test.go",
			str:      "user_test.go",
			expected: true,
		},
		{
			name:     "前方ワイルドカード",
			pattern:  "*test",
			str:      "unittest",
			expected: true,
		},
		{
			name:     "後方ワイルドカード",
			pattern:  "test*",
			str:      "test123",
			expected: true,
		},
		{
			name:     "全ワイルドカード",
			pattern:  "*",
			str:      "anything",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchWildcard(tt.pattern, tt.str)
			assert.Equal(t, tt.expected, result)
		})
	}
}
