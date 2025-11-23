package quality

import (
	"context"
	"fmt"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
)

// WeeklyReview は週次レビューを実行するサービスです
// 既存のWeeklyReviewServiceとGitParser/CodeownersParserを組み合わせて使用します
type WeeklyReview struct {
	qualityRepo      QualityNoteRepository
	codeownersParser SimpleCodeownersParser
	gitParser        SimpleGitParser
	repoPath         string
}

// SimpleCodeownersParser は簡易的な CODEOWNERS パーサーのインターフェースです
type SimpleCodeownersParser interface {
	GetOwner(filePath string) string
}

// SimpleGitParser は簡易的な Git パーサーのインターフェースです
type SimpleGitParser interface {
	GetRecentChanges(since time.Time) ([]GitChange, error)
}

// GitChange はGitの変更情報を表します
type GitChange struct {
	CommitHash string
	Date       time.Time
	Author     string
	Message    string
	Files      []string
}

// NewWeeklyReview は新しいWeeklyReviewを作成します
func NewWeeklyReview(
	qualityRepo QualityNoteRepository,
	codeownersParser SimpleCodeownersParser,
	gitParser SimpleGitParser,
	repoPath string,
) *WeeklyReview {
	return &WeeklyReview{
		qualityRepo:      qualityRepo,
		codeownersParser: codeownersParser,
		gitParser:        gitParser,
		repoPath:         repoPath,
	}
}

// PrepareWeeklyReviewData は週次レビューデータを準備します
func (wr *WeeklyReview) PrepareWeeklyReviewData(ctx context.Context, startDate, endDate time.Time) (ActionGenerationData, error) {
	// 品質ノートを取得
	notes, err := wr.qualityRepo.ListQualityNotesByDateRange(ctx, startDate, endDate)
	if err != nil {
		return ActionGenerationData{}, fmt.Errorf("failed to get quality notes: %w", err)
	}

	// 品質ノートをActionGenerationData形式に変換
	weeklyNotes := make([]WeeklyQualityNote, 0, len(notes))
	for _, note := range notes {
		weeklyNotes = append(weeklyNotes, WeeklyQualityNote{
			NoteID:      note.NoteID,
			Severity:    string(note.Severity),
			NoteText:    note.NoteText,
			LinkedFiles: note.LinkedFiles,
			Reviewer:    note.Reviewer,
		})
	}

	// 最近の変更を取得
	changes, err := wr.gitParser.GetRecentChanges(startDate)
	if err != nil {
		return ActionGenerationData{}, fmt.Errorf("failed to get recent changes: %w", err)
	}

	// 変更をActionGenerationData形式に変換
	recentChanges := make([]ActionRecentChange, 0, len(changes))
	for _, change := range changes {
		recentChanges = append(recentChanges, ActionRecentChange{
			Hash:         change.CommitHash,
			FilesChanged: change.Files,
			MergedAt:     change.Date,
		})
	}

	// CODEOWNERS情報を構築
	codeownersLookup := make(map[string]string)
	// 品質ノートの関連ファイルからオーナー情報を取得
	for _, note := range notes {
		for _, file := range note.LinkedFiles {
			if _, exists := codeownersLookup[file]; !exists {
				owner := wr.codeownersParser.GetOwner(file)
				codeownersLookup[file] = owner
			}
		}
	}

	return ActionGenerationData{
		WeekRange:        fmt.Sprintf("%s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")),
		QualityNotes:     weeklyNotes,
		RecentChanges:    recentChanges,
		CodeownersLookup: codeownersLookup,
	}, nil
}

// SimpleCodeownersParserImpl は CODEOWNERS ファイルをパースするパーサーの簡易実装です
type SimpleCodeownersParserImpl struct {
	parser   *CodeownersParser
	repoPath string
	cache    map[string][]string
}

// NewSimpleCodeownersParser は新しい SimpleCodeownersParserImpl を作成します
func NewSimpleCodeownersParser(repoPath string) *SimpleCodeownersParserImpl {
	return &SimpleCodeownersParserImpl{
		parser:   NewCodeownersParser(),
		repoPath: repoPath,
		cache:    nil,
	}
}

// GetOwner は指定されたファイルのオーナーを返します
func (scp *SimpleCodeownersParserImpl) GetOwner(filePath string) string {
	// キャッシュが未初期化の場合はパースする
	if scp.cache == nil {
		owners, err := scp.parser.ParseCodeowners(scp.repoPath)
		if err != nil {
			return "unassigned"
		}
		scp.cache = owners
	}

	// ファイルパスからオーナーを検索
	if owners, ok := scp.cache[filePath]; ok && len(owners) > 0 {
		return owners[0] // 最初のオーナーを返す
	}

	return "unassigned"
}

// SimpleGitParserImpl は Git リポジトリから最近の変更を取得するパーサーの簡易実装です
type SimpleGitParserImpl struct {
	repoPath string
}

// NewSimpleGitParser は新しい SimpleGitParserImpl を作成します
func NewSimpleGitParser(repoPath string) *SimpleGitParserImpl {
	return &SimpleGitParserImpl{
		repoPath: repoPath,
	}
}

// GetRecentChanges は指定された日時以降の変更を取得します
func (sgp *SimpleGitParserImpl) GetRecentChanges(since time.Time) ([]GitChange, error) {
	// 簡易実装: 実際には git log を実行して変更を取得する
	// ここでは空のスライスを返す
	return []GitChange{}, nil
}

// PrepareActionGenerationData は models.QualityNote のリストを ActionGenerationData に変換します
func PrepareActionGenerationData(notes []*models.QualityNote, changes []GitChange, codeowners map[string]string, startDate, endDate time.Time) ActionGenerationData {
	weeklyNotes := make([]WeeklyQualityNote, 0, len(notes))
	for _, note := range notes {
		weeklyNotes = append(weeklyNotes, WeeklyQualityNote{
			NoteID:      note.NoteID,
			Severity:    string(note.Severity),
			NoteText:    note.NoteText,
			LinkedFiles: note.LinkedFiles,
			Reviewer:    note.Reviewer,
		})
	}

	recentChanges := make([]ActionRecentChange, 0, len(changes))
	for _, change := range changes {
		recentChanges = append(recentChanges, ActionRecentChange{
			Hash:         change.CommitHash,
			FilesChanged: change.Files,
			MergedAt:     change.Date,
		})
	}

	return ActionGenerationData{
		WeekRange:        fmt.Sprintf("%s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")),
		QualityNotes:     weeklyNotes,
		RecentChanges:    recentChanges,
		CodeownersLookup: codeowners,
	}
}
