package quality

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GitCommit はコミット情報を表します
type GitCommit struct {
	Hash         string    `json:"hash"`
	FilesChanged []string  `json:"filesChanged"`
	MergedAt     time.Time `json:"mergedAt"`
	Author       string    `json:"author"`
	Message      string    `json:"message"`
}

// GitLogParser はGitログをパースするサービスです
type GitLogParser struct{}

// NewGitLogParser は新しいGitLogParserを作成します
func NewGitLogParser() *GitLogParser {
	return &GitLogParser{}
}

// ParseGitLog は指定期間のGitログをパースします
// git log --since=<startDate> --until=<endDate> --name-only --pretty=format:...
func (p *GitLogParser) ParseGitLog(ctx context.Context, repoPath string, startDate, endDate time.Time) ([]GitCommit, error) {
	// RFC3339形式で日付をフォーマット
	since := startDate.Format(time.RFC3339)
	until := endDate.Format(time.RFC3339)

	// git log コマンドを実行
	// フォーマット: <commit_hash>|||<author>|||<date>|||<commit_message>
	cmd := exec.CommandContext(ctx, "git", "log",
		"--since="+since,
		"--until="+until,
		"--name-only",
		"--pretty=format:%H|||%an|||%aI|||%s",
		"--no-merges",
	)
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute git log: %w", err)
	}

	return p.parseGitLogOutput(string(output))
}

// parseGitLogOutput はgit logの出力をパースします
func (p *GitLogParser) parseGitLogOutput(output string) ([]GitCommit, error) {
	var commits []GitCommit
	var currentCommit *GitCommit

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			// 空行はコミットの区切り
			if currentCommit != nil {
				commits = append(commits, *currentCommit)
				currentCommit = nil
			}
			continue
		}

		// コミット情報行かファイル名行かを判定
		if strings.Contains(line, "|||") {
			// コミット情報行
			parts := strings.Split(line, "|||")
			if len(parts) != 4 {
				return nil, fmt.Errorf("invalid git log format: %s", line)
			}

			hash := parts[0]
			author := parts[1]
			dateStr := parts[2]
			message := parts[3]

			// 日付をパース
			mergedAt, err := time.Parse(time.RFC3339, dateStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse date %s: %w", dateStr, err)
			}

			currentCommit = &GitCommit{
				Hash:         hash,
				Author:       author,
				MergedAt:     mergedAt,
				Message:      message,
				FilesChanged: []string{},
			}
		} else if currentCommit != nil {
			// ファイル名行
			currentCommit.FilesChanged = append(currentCommit.FilesChanged, line)
		}
	}

	// 最後のコミットを追加
	if currentCommit != nil {
		commits = append(commits, *currentCommit)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan git log output: %w", err)
	}

	return commits, nil
}
