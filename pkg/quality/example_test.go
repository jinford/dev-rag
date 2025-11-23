package quality_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jinford/dev-rag/pkg/quality"
	"github.com/jinford/dev-rag/pkg/repository"
	"github.com/jinford/dev-rag/pkg/sqlc"
)

// Example_weeklyReview は週次レビューデータ準備の使用例です
func Example_weeklyReview() {
	// データベース接続を初期化 (実際の環境では適切に設定)
	// db, err := pgxpool.New(context.Background(), "postgresql://...")
	// if err != nil {
	//     log.Fatal(err)
	// }
	// queries := sqlc.New(db)
	var queries sqlc.Querier // 実際にはデータベース接続を使用

	// リポジトリとパーサーを初期化
	qualityRepo := repository.NewQualityRepositoryR(queries)
	gitParser := quality.NewGitLogParser()
	coParser := quality.NewCodeownersParser()

	// 週次レビューサービスを作成
	service := quality.NewWeeklyReviewService(qualityRepo, gitParser, coParser)

	// 週次レビューデータを準備 (例: 2024年1月15日〜22日)
	startDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 22, 0, 0, 0, 0, time.UTC)
	repoPath := "/path/to/repository"

	ctx := context.Background()
	reviewData, err := service.PrepareWeeklyReview(ctx, repoPath, startDate, endDate)
	if err != nil {
		log.Fatal(err)
	}

	// JSON形式で出力
	jsonStr, err := reviewData.ToJSON()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("週次レビューデータ:")
	fmt.Println(jsonStr)

	// レビューデータの内容を確認
	fmt.Printf("期間: %s 〜 %s\n", reviewData.WeekRange.StartDate, reviewData.WeekRange.EndDate)
	fmt.Printf("品質ノート数: %d\n", len(reviewData.QualityNotes))
	fmt.Printf("最新コミット数: %d\n", len(reviewData.RecentChanges))
	fmt.Printf("CODEOWNERSエントリ数: %d\n", len(reviewData.CodeownersLookup))
}

// Example_gitLogParser はGitログパーサーの使用例です
func Example_gitLogParser() {
	parser := quality.NewGitLogParser()

	// Gitログをパース (例: 2024年1月15日〜22日)
	startDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 22, 0, 0, 0, 0, time.UTC)
	repoPath := "/path/to/repository"

	ctx := context.Background()
	commits, err := parser.ParseGitLog(ctx, repoPath, startDate, endDate)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("取得したコミット数: %d\n", len(commits))
	for _, commit := range commits {
		fmt.Printf("コミット: %s (%s)\n", commit.Hash, commit.Author)
		fmt.Printf("  日時: %s\n", commit.MergedAt)
		fmt.Printf("  メッセージ: %s\n", commit.Message)
		fmt.Printf("  変更ファイル: %v\n", commit.FilesChanged)
	}
}

// Example_codeownersParser はCODEOWNERSパーサーの使用例です
func Example_codeownersParser() {
	parser := quality.NewCodeownersParser()

	// CODEOWNERSファイルをパース
	repoPath := "/path/to/repository"
	codeownersLookup, err := parser.ParseCodeowners(repoPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("CODEOWNERSエントリ数: %d\n", len(codeownersLookup))
	for pattern, owners := range codeownersLookup {
		fmt.Printf("パターン: %s → オーナー: %v\n", pattern, owners)
	}

	// 特定のファイルのオーナーを取得
	filePath := "pkg/quality/weekly_review.go"
	owners := parser.GetOwnersForFile(filePath, codeownersLookup)
	fmt.Printf("\n%s のオーナー: %v\n", filePath, owners)
}
