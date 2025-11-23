package quality

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/indexer/llm"
	"github.com/jinford/dev-rag/pkg/models"
)

// TestHelper は統合テスト用のヘルパー構造体です
type TestHelper struct {
	t              *testing.T
	tempDir        string
	gitRepoPath    string
	qualityRepo    *MockQualityRepository
	actionRepo     *MockActionBacklogRepository
	llmClient      *MockLLMClient
	gitParser      *MockGitLogParser
	coParser       *MockCodeownersParserAdapter
	metricsCalc    *MockMetricsCalculator
	freshnessCalc  *MockFreshnessCalculator
}

// NewTestHelper は新しいテストヘルパーを作成します
func NewTestHelper(t *testing.T) *TestHelper {
	t.Helper()

	tempDir := t.TempDir()

	return &TestHelper{
		t:              t,
		tempDir:        tempDir,
		qualityRepo:    &MockQualityRepository{notes: []*models.QualityNote{}},
		actionRepo:     &MockActionBacklogRepository{actions: []*models.Action{}},
		llmClient:      &MockLLMClient{response: "[]"},
		gitParser:      &MockGitLogParser{commits: []GitCommit{}},
		coParser:       &MockCodeownersParserAdapter{owners: map[string][]string{}},
		metricsCalc:    &MockMetricsCalculator{},
		freshnessCalc:  &MockFreshnessCalculator{},
	}
}

// CreateGitRepo は一時的なGitリポジトリを作成します
func (h *TestHelper) CreateGitRepo() error {
	h.t.Helper()

	// リポジトリディレクトリを作成
	repoPath := filepath.Join(h.tempDir, "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return fmt.Errorf("failed to create repo dir: %w", err)
	}
	h.gitRepoPath = repoPath

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run git init: %w", err)
	}

	// git config
	configCmds := [][]string{
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "user.email", "test@example.com"},
	}
	for _, args := range configCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoPath
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run %v: %w", args, err)
		}
	}

	return nil
}

// CreateFileAndCommit はファイルを作成してコミットします
func (h *TestHelper) CreateFileAndCommit(relativePath, content, commitMsg string) (string, error) {
	h.t.Helper()

	if h.gitRepoPath == "" {
		return "", fmt.Errorf("git repo not initialized, call CreateGitRepo first")
	}

	// ファイルを作成
	filePath := filepath.Join(h.gitRepoPath, relativePath)
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create dir: %w", err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// git add
	cmd := exec.Command("git", "add", relativePath)
	cmd.Dir = h.gitRepoPath
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run git add: %w", err)
	}

	// git commit
	cmd = exec.Command("git", "commit", "-m", commitMsg)
	cmd.Dir = h.gitRepoPath
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run git commit: %w", err)
	}

	// コミットハッシュを取得
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = h.gitRepoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}

	commitHash := string(output)
	if len(commitHash) > 0 && commitHash[len(commitHash)-1] == '\n' {
		commitHash = commitHash[:len(commitHash)-1]
	}

	return commitHash, nil
}

// CreateOldCommit は古い日付のコミットを作成します（テスト用）
func (h *TestHelper) CreateOldCommit(relativePath, content string, daysAgo int) (string, error) {
	h.t.Helper()

	if h.gitRepoPath == "" {
		return "", fmt.Errorf("git repo not initialized, call CreateGitRepo first")
	}

	// ファイルを作成
	filePath := filepath.Join(h.gitRepoPath, relativePath)
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create dir: %w", err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// git add
	cmd := exec.Command("git", "add", relativePath)
	cmd.Dir = h.gitRepoPath
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run git add: %w", err)
	}

	// 古い日付でコミット
	oldDate := time.Now().AddDate(0, 0, -daysAgo)
	dateStr := oldDate.Format("2006-01-02 15:04:05")

	cmd = exec.Command("git", "commit", "-m", fmt.Sprintf("Old commit %d days ago", daysAgo),
		"--date", dateStr)
	cmd.Dir = h.gitRepoPath
	cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_COMMITTER_DATE=%s", dateStr))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run git commit: %w", err)
	}

	// コミットハッシュを取得
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = h.gitRepoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}

	commitHash := string(output)
	if len(commitHash) > 0 && commitHash[len(commitHash)-1] == '\n' {
		commitHash = commitHash[:len(commitHash)-1]
	}

	return commitHash, nil
}

// CreateQualityNote はテスト用の品質ノートを作成します
func (h *TestHelper) CreateQualityNote(severity models.QualitySeverity, noteText string, linkedFiles []string) *models.QualityNote {
	h.t.Helper()

	note := &models.QualityNote{
		ID:           uuid.New(),
		NoteID:       fmt.Sprintf("QN-TEST-%s", uuid.New().String()[:8]),
		Severity:     severity,
		NoteText:     noteText,
		LinkedFiles:  linkedFiles,
		LinkedChunks: []string{},
		Reviewer:     "test-reviewer",
		Status:       models.QualityStatusOpen,
		CreatedAt:    time.Now(),
	}

	h.qualityRepo.notes = append(h.qualityRepo.notes, note)
	return note
}

// CreateMultipleQualityNotes は複数の品質ノートを作成します
func (h *TestHelper) CreateMultipleQualityNotes(count int, severity models.QualitySeverity) []*models.QualityNote {
	h.t.Helper()

	notes := make([]*models.QualityNote, 0, count)
	for i := 0; i < count; i++ {
		note := h.CreateQualityNote(
			severity,
			fmt.Sprintf("Test note %d with %s severity", i+1, severity),
			[]string{fmt.Sprintf("file%d.go", i+1)},
		)
		notes = append(notes, note)
	}
	return notes
}

// SetLLMResponse はモックLLMクライアントのレスポンスを設定します
func (h *TestHelper) SetLLMResponse(response string) {
	h.t.Helper()
	h.llmClient.response = response
}

// SetCodeowners はCODEOWNERSマッピングを設定します
func (h *TestHelper) SetCodeowners(filePath string, owners []string) {
	h.t.Helper()
	h.coParser.owners[filePath] = owners
}

// GetGitRepoPath はGitリポジトリのパスを返します
func (h *TestHelper) GetGitRepoPath() string {
	return h.gitRepoPath
}

// GetQualityRepo は品質ノートリポジトリを返します
func (h *TestHelper) GetQualityRepo() *MockQualityRepository {
	return h.qualityRepo
}

// GetActionRepo はアクションリポジトリを返します
func (h *TestHelper) GetActionRepo() *MockActionBacklogRepository {
	return h.actionRepo
}

// GetLLMClient はLLMクライアントを返します
func (h *TestHelper) GetLLMClient() llm.LLMClient {
	return h.llmClient
}

// GetGitParser はGitパーサーを返します
func (h *TestHelper) GetGitParser() GitLogParserInterface {
	return h.gitParser
}

// GetCodeownersParser はCODEOWNERSパーサーを返します
func (h *TestHelper) GetCodeownersParser() CodeownersParserInterface {
	return h.coParser
}

// GetMetricsCalculator はメトリクス計算機を返します
func (h *TestHelper) GetMetricsCalculator() MetricsCalculatorInterface {
	return h.metricsCalc
}

// GetFreshnessCalculator は鮮度計算機を返します
func (h *TestHelper) GetFreshnessCalculator() FreshnessCalculatorInterface {
	return h.freshnessCalc
}

// SetMetrics はメトリクスを設定します
func (h *TestHelper) SetMetrics(metrics *models.QualityMetrics) {
	h.t.Helper()
	h.metricsCalc.metrics = metrics
}

// SetFreshnessReport は鮮度レポートを設定します
func (h *TestHelper) SetFreshnessReport(report *models.FreshnessReport) {
	h.t.Helper()
	h.freshnessCalc.report = report
}

// Cleanup はテストのクリーンアップを実行します
func (h *TestHelper) Cleanup() {
	h.t.Helper()
	// tempDirは t.TempDir() で管理されているため、自動的にクリーンアップされる
}

// MockGitLogParser は GitLogParserInterface のモック実装です
type MockGitLogParser struct {
	commits []GitCommit
}

// ParseGitLog は設定されたコミットを返します
func (m *MockGitLogParser) ParseGitLog(ctx context.Context, repoPath string, startDate, endDate time.Time) ([]GitCommit, error) {
	var result []GitCommit
	for _, commit := range m.commits {
		if (commit.MergedAt.After(startDate) || commit.MergedAt.Equal(startDate)) &&
			(commit.MergedAt.Before(endDate) || commit.MergedAt.Equal(endDate)) {
			result = append(result, commit)
		}
	}
	return result, nil
}

// MockCodeownersParserAdapter は CodeownersParserInterface のモック実装です
type MockCodeownersParserAdapter struct {
	owners map[string][]string
}

// ParseCodeowners は設定されたオーナーマッピングを返します
func (m *MockCodeownersParserAdapter) ParseCodeowners(repoPath string) (map[string][]string, error) {
	if m.owners == nil {
		return map[string][]string{}, nil
	}
	return m.owners, nil
}
