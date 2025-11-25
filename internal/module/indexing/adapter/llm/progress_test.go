package llm

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestProgressLogger_LogProgress(t *testing.T) {
	// ProgressLoggerの基本的な動作を確認
	logger := NewProgressLogger(100*time.Millisecond, false, false)

	progress := BatchProgress{
		Total:                  100,
		Completed:              50,
		Failed:                 5,
		ElapsedTime:            1 * time.Minute,
		EstimatedTimeRemaining: 1 * time.Minute,
	}

	// パニックしないことを確認
	logger.LogProgress(progress)

	// インターバル内の呼び出し
	logger.LogProgress(progress)

	// インターバル後の呼び出し
	time.Sleep(150 * time.Millisecond)
	logger.LogProgress(progress)

	// 完了時の呼び出し
	progress.Completed = 100
	logger.LogProgress(progress)
}

func TestProgressLogger_LogProgress_Detailed(t *testing.T) {
	// 詳細ログモードでパニックしないことを確認
	logger := NewProgressLogger(100*time.Millisecond, true, true)

	progress := BatchProgress{
		Total:                  100,
		Completed:              50,
		Failed:                 5,
		ElapsedTime:            1 * time.Minute,
		EstimatedTimeRemaining: 1 * time.Minute,
	}

	logger.LogProgress(progress)
	// ログが出力されたことを確認（標準出力に影響しない）
}

func TestProgressLogger_LogFinal(t *testing.T) {
	// 最終ログでパニックしないことを確認
	logger := NewProgressLogger(100*time.Millisecond, true, false)

	progress := BatchProgress{
		Total:                  100,
		Completed:              100,
		Failed:                 5,
		ElapsedTime:            2 * time.Minute,
		EstimatedTimeRemaining: 0,
	}

	stats := BatchStats{
		TotalRequests:   100,
		SuccessCount:    95,
		FailureCount:    5,
		AverageDuration: 100 * time.Millisecond,
		FailedRequests:  []string{"req-1", "req-2", "req-3", "req-4", "req-5"},
	}

	logger.LogFinal(progress, stats)
}

func TestProgressTracker(t *testing.T) {
	logger := NewProgressLogger(50*time.Millisecond, false, false) // コンソール出力を無効化
	tracker := NewProgressTracker(10, logger)

	// 初期状態を確認
	progress := tracker.GetProgress()
	if progress.Total != 10 {
		t.Errorf("Total = %d, want 10", progress.Total)
	}
	if progress.Completed != 0 {
		t.Errorf("Completed = %d, want 0", progress.Completed)
	}

	// 成功を記録
	tracker.OnComplete(true)
	tracker.OnComplete(true)
	tracker.OnComplete(true)

	progress = tracker.GetProgress()
	if progress.Completed != 3 {
		t.Errorf("Completed = %d, want 3", progress.Completed)
	}
	if progress.Failed != 0 {
		t.Errorf("Failed = %d, want 0", progress.Failed)
	}

	// 失敗を記録
	tracker.OnComplete(false)
	tracker.OnComplete(false)

	progress = tracker.GetProgress()
	if progress.Completed != 5 {
		t.Errorf("Completed = %d, want 5", progress.Completed)
	}
	if progress.Failed != 2 {
		t.Errorf("Failed = %d, want 2", progress.Failed)
	}

	// ETAが計算されることを確認
	if progress.EstimatedTimeRemaining <= 0 {
		t.Error("EstimatedTimeRemaining should be > 0")
	}
}

func TestProgressBar_Update(t *testing.T) {
	// 標準出力をキャプチャするのは難しいので、基本的な動作のみ確認
	bar := NewProgressBar(100, 50, "Test")

	// パニックしないことを確認
	bar.Update(0)
	bar.Update(50)
	bar.Update(100)
	bar.Update(150) // 範囲外でもパニックしない

	// 初期状態を確認
	if bar.total != 100 {
		t.Errorf("total = %d, want 100", bar.total)
	}
	if bar.width != 50 {
		t.Errorf("width = %d, want 50", bar.width)
	}
	if bar.prefix != "Test" {
		t.Errorf("prefix = %s, want Test", bar.prefix)
	}
}

func TestBatchProcessorWithProgress(t *testing.T) {
	// モッククライアントを作成
	mockClient := &mockLLMClientForBatch{
		delay: 10 * time.Millisecond,
	}

	// プログレスロガーを作成（コンソール出力を無効化）
	logger := NewProgressLogger(50*time.Millisecond, false, false)

	// プログレス表示付きプロセッサーを作成
	processor := NewBatchProcessorWithProgress(
		mockClient,
		BatchProcessorConfig{
			MaxConcurrency: 5,
		},
		logger,
		false, // プログレスバーを無効化（テスト環境では表示しない）
	)

	// テストリクエストを作成
	requests := make([]BatchRequest, 20)
	for i := 0; i < 20; i++ {
		requests[i] = BatchRequest{
			ID: fmt.Sprintf("req-%d", i),
			Request: CompletionRequest{
				Prompt: fmt.Sprintf("test prompt %d", i),
			},
		}
	}

	// バッチ処理を実行
	ctx := context.Background()
	results, stats := processor.ProcessBatch(ctx, requests)

	// 結果を確認
	if len(results) != 20 {
		t.Errorf("results count = %d, want 20", len(results))
	}

	if stats.TotalRequests != 20 {
		t.Errorf("TotalRequests = %d, want 20", stats.TotalRequests)
	}

	if stats.SuccessCount != 20 {
		t.Errorf("SuccessCount = %d, want 20", stats.SuccessCount)
	}

	if stats.FailureCount != 0 {
		t.Errorf("FailureCount = %d, want 0", stats.FailureCount)
	}
}

func TestBatchProcessorWithProgress_SetBarConfig(t *testing.T) {
	mockClient := &mockLLMClientForBatch{}
	logger := NewProgressLogger(50*time.Millisecond, false, false)

	processor := NewBatchProcessorWithProgress(
		mockClient,
		BatchProcessorConfig{},
		logger,
		false,
	)

	// バー設定を変更
	processor.SetBarConfig(80, "Custom")

	if processor.barWidth != 80 {
		t.Errorf("barWidth = %d, want 80", processor.barWidth)
	}

	if processor.barPrefix != "Custom" {
		t.Errorf("barPrefix = %s, want Custom", processor.barPrefix)
	}
}

func TestProgressLogger_LogProgress_Completion(t *testing.T) {
	logger := NewProgressLogger(100*time.Millisecond, true, false)

	// 完了時は即座にログが出力されることを確認（パニックしない）
	progress := BatchProgress{
		Total:                  100,
		Completed:              100,
		Failed:                 0,
		ElapsedTime:            2 * time.Minute,
		EstimatedTimeRemaining: 0,
	}

	logger.LogProgress(progress)
}

func TestProgressTracker_ConcurrentAccess(t *testing.T) {
	// 並行アクセスでもパニックしないことを確認
	logger := NewProgressLogger(50*time.Millisecond, false, false)
	tracker := NewProgressTracker(100, logger)

	done := make(chan bool)

	// 複数のgoroutineから同時にOnCompleteを呼ぶ
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				tracker.OnComplete(j%2 == 0)
			}
			done <- true
		}()
	}

	// すべてのgoroutineが完了するまで待機
	for i := 0; i < 10; i++ {
		<-done
	}

	// 結果を確認
	progress := tracker.GetProgress()
	if progress.Completed != 100 {
		t.Errorf("Completed = %d, want 100", progress.Completed)
	}
}
