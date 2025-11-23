package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
)

// QualityNoteRepository は品質ノートリポジトリのインターフェースです
type QualityNoteRepository interface {
	ListQualityNotesByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*models.QualityNote, error)
}

// GitLogParserInterface はGitログパーサーのインターフェースです
type GitLogParserInterface interface {
	ParseGitLog(ctx context.Context, repoPath string, startDate, endDate time.Time) ([]GitCommit, error)
}

// CodeownersParserInterface はCODEOWNERSパーサーのインターフェースです
type CodeownersParserInterface interface {
	ParseCodeowners(repoPath string) (map[string][]string, error)
}

// WeekRange は週の期間を表します
type WeekRange struct {
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}

// RecentChange は最近のコミット情報を表します
type RecentChange struct {
	Hash         string    `json:"hash"`
	FilesChanged []string  `json:"filesChanged"`
	MergedAt     time.Time `json:"mergedAt"`
	Author       string    `json:"author"`
	Message      string    `json:"message"`
}

// WeeklyReviewData は週次レビューのデータを表します
type WeeklyReviewData struct {
	WeekRange        WeekRange             `json:"weekRange"`
	QualityNotes     []*models.QualityNote `json:"qualityNotes"`
	RecentChanges    []RecentChange        `json:"recentChanges"`
	CodeownersLookup map[string][]string   `json:"codeownersLookup"`
}

// WeeklyReviewService は週次レビューデータの準備サービスです
type WeeklyReviewService struct {
	qualityRepo QualityNoteRepository
	gitParser   GitLogParserInterface
	coParser    CodeownersParserInterface
}

// NewWeeklyReviewService は新しい週次レビューサービスを作成します
func NewWeeklyReviewService(
	qualityRepo QualityNoteRepository,
	gitParser GitLogParserInterface,
	coParser CodeownersParserInterface,
) *WeeklyReviewService {
	return &WeeklyReviewService{
		qualityRepo: qualityRepo,
		gitParser:   gitParser,
		coParser:    coParser,
	}
}

// PrepareWeeklyReview は指定期間の週次レビューデータを準備します
func (s *WeeklyReviewService) PrepareWeeklyReview(ctx context.Context, repoPath string, startDate, endDate time.Time) (*WeeklyReviewData, error) {
	// 1. 品質ノートを取得
	qualityNotes, err := s.qualityRepo.ListQualityNotesByDateRange(ctx, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get quality notes: %w", err)
	}

	// 2. Gitログから最新コミット情報を取得
	gitCommits, err := s.gitParser.ParseGitLog(ctx, repoPath, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse git log: %w", err)
	}

	// GitCommit を RecentChange に変換
	recentChanges := make([]RecentChange, 0, len(gitCommits))
	for _, commit := range gitCommits {
		recentChanges = append(recentChanges, RecentChange{
			Hash:         commit.Hash,
			FilesChanged: commit.FilesChanged,
			MergedAt:     commit.MergedAt,
			Author:       commit.Author,
			Message:      commit.Message,
		})
	}

	// 3. CODEOWNERSファイルからオーナー情報を取得
	codeownersLookup, err := s.coParser.ParseCodeowners(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CODEOWNERS: %w", err)
	}

	return &WeeklyReviewData{
		WeekRange: WeekRange{
			StartDate: startDate,
			EndDate:   endDate,
		},
		QualityNotes:     qualityNotes,
		RecentChanges:    recentChanges,
		CodeownersLookup: codeownersLookup,
	}, nil
}

// ToJSON は WeeklyReviewData を JSON 文字列に変換します
func (w *WeeklyReviewData) ToJSON() (string, error) {
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal weekly review data: %w", err)
	}
	return string(data), nil
}
