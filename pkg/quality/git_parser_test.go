package quality

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitLogOutput(t *testing.T) {
	parser := NewGitLogParser()

	tests := []struct {
		name          string
		output        string
		expectedCount int
		expectedFirst *GitCommit
		expectedError bool
	}{
		{
			name: "正常なログ出力",
			output: `abc123|||John Doe|||2024-01-15T10:30:00Z|||Initial commit
README.md
main.go

def456|||Jane Smith|||2024-01-16T14:20:00Z|||Add feature
src/feature.go
src/feature_test.go
`,
			expectedCount: 2,
			expectedFirst: &GitCommit{
				Hash:         "abc123",
				Author:       "John Doe",
				MergedAt:     time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Message:      "Initial commit",
				FilesChanged: []string{"README.md", "main.go"},
			},
			expectedError: false,
		},
		{
			name:          "空の出力",
			output:        "",
			expectedCount: 0,
			expectedError: false,
		},
		{
			name: "ファイル変更なしのコミット",
			output: `abc123|||John Doe|||2024-01-15T10:30:00Z|||Empty commit

`,
			expectedCount: 1,
			expectedFirst: &GitCommit{
				Hash:         "abc123",
				Author:       "John Doe",
				MergedAt:     time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Message:      "Empty commit",
				FilesChanged: []string{},
			},
			expectedError: false,
		},
		{
			name: "複数ファイル変更",
			output: `abc123|||John Doe|||2024-01-15T10:30:00Z|||Large commit
file1.go
file2.go
file3.go
file4.go
file5.go
`,
			expectedCount: 1,
			expectedFirst: &GitCommit{
				Hash:         "abc123",
				Author:       "John Doe",
				MergedAt:     time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Message:      "Large commit",
				FilesChanged: []string{"file1.go", "file2.go", "file3.go", "file4.go", "file5.go"},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commits, err := parser.parseGitLogOutput(tt.output)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, commits, tt.expectedCount)

			if tt.expectedFirst != nil && len(commits) > 0 {
				assert.Equal(t, tt.expectedFirst.Hash, commits[0].Hash)
				assert.Equal(t, tt.expectedFirst.Author, commits[0].Author)
				assert.Equal(t, tt.expectedFirst.MergedAt.Unix(), commits[0].MergedAt.Unix())
				assert.Equal(t, tt.expectedFirst.Message, commits[0].Message)
				assert.Equal(t, tt.expectedFirst.FilesChanged, commits[0].FilesChanged)
			}
		})
	}
}

func TestParseGitLogOutput_InvalidFormat(t *testing.T) {
	parser := NewGitLogParser()

	// 不正なフォーマット(区切り文字が3つのみで4つ未満)
	output := `abc123|||John Doe|||2024-01-15T10:30:00Z
README.md
`

	_, err := parser.parseGitLogOutput(output)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid git log format")
}

func TestParseGitLogOutput_InvalidDate(t *testing.T) {
	parser := NewGitLogParser()

	// 不正な日付フォーマット
	output := `abc123|||John Doe|||invalid-date|||Initial commit
README.md
`

	_, err := parser.parseGitLogOutput(output)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse date")
}
