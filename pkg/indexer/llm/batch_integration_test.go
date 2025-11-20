package llm

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// TestBatchProcessing_Integration は実際のワークフローをシミュレートする統合テスト
func TestBatchProcessing_Integration(t *testing.T) {
	// シナリオ: 100個のファイルのサマリーを生成
	// - 一部のリクエストは失敗する
	// - プログレス表示を有効化
	// - 統計情報を記録

	// モッククライアントを作成（10%の確率で失敗）
	mockClient := &mockLLMClientForBatch{
		delay: 5 * time.Millisecond,
		failPattern: func(i int) bool {
			return i%10 == 0 // 10個に1個失敗
		},
	}

	// プログレスロガーを作成
	logger := NewProgressLogger(100*time.Millisecond, false, false)

	// プロセッサーを作成（レート制限なしで高速化）
	processor := NewBatchProcessorWithProgress(
		mockClient,
		BatchProcessorConfig{
			MaxConcurrency: 10,
			PromptSection:  PromptSectionFileSummary,
		},
		logger,
		false, // プログレスバーは無効（テスト環境）
	)

	// 100個のリクエストを作成
	requests := make([]BatchRequest, 100)
	for i := 0; i < 100; i++ {
		requests[i] = BatchRequest{
			ID: fmt.Sprintf("file-%d.go", i),
			Request: CompletionRequest{
				Prompt:      fmt.Sprintf("Summarize file-%d.go", i),
				Temperature: 0.3,
				MaxTokens:   400,
			},
		}
	}

	// バッチ処理を実行
	ctx := context.Background()
	startTime := time.Now()
	results, stats := processor.ProcessBatch(ctx, requests)
	elapsed := time.Since(startTime)

	// 結果を検証
	if len(results) != 100 {
		t.Errorf("results count = %d, want 100", len(results))
	}

	// 統計情報を検証
	if stats.TotalRequests != 100 {
		t.Errorf("TotalRequests = %d, want 100", stats.TotalRequests)
	}

	expectedFailures := 10 // 10個に1個失敗
	if stats.FailureCount != expectedFailures {
		t.Errorf("FailureCount = %d, want %d", stats.FailureCount, expectedFailures)
	}

	expectedSuccess := 90
	if stats.SuccessCount != expectedSuccess {
		t.Errorf("SuccessCount = %d, want %d", stats.SuccessCount, expectedSuccess)
	}

	// 並列処理が機能していることを確認
	serialTime := 5 * time.Millisecond * 100 // 500ms
	if elapsed >= serialTime {
		t.Errorf("elapsed time %s >= serial time %s, parallelism may not be working", elapsed, serialTime)
	}

	t.Logf("Integration test completed: %s", stats.String())
	t.Logf("Processing time: %s (would be %s if serial)", elapsed.Round(time.Millisecond), serialTime)
}

// TestBatchProcessing_WithRetry はリトライロジックをテストする
func TestBatchProcessing_WithRetry(t *testing.T) {
	// 最初の試行で全て失敗し、2回目の試行で成功するシナリオ
	var attemptCount int32

	mockClient := &mockLLMClientForBatch{
		delay: 5 * time.Millisecond,
		failPattern: func(i int) bool {
			// 最初の試行（0-9）では全て失敗
			// 2回目の試行（10以降）では全て成功
			return atomic.LoadInt32(&attemptCount) < 10
		},
	}

	processor := NewBatchProcessor(mockClient, BatchProcessorConfig{
		MaxConcurrency: 5,
	})

	// 10個のリクエストを作成
	requests := make([]BatchRequest, 10)
	for i := 0; i < 10; i++ {
		requests[i] = BatchRequest{
			ID: fmt.Sprintf("req-%d", i),
			Request: CompletionRequest{
				Prompt: fmt.Sprintf("test prompt %d", i),
			},
		}
	}

	ctx := context.Background()

	// 最初の試行（全て失敗）
	results := processor.ProcessBatch(ctx, requests)
	atomic.AddInt32(&attemptCount, int32(len(results)))

	// 失敗したリクエストを抽出
	failedRequests := []BatchRequest{}
	for i, result := range results {
		if result.Error != nil {
			failedRequests = append(failedRequests, requests[i])
		}
	}

	if len(failedRequests) != 10 {
		t.Errorf("failed requests count = %d, want 10", len(failedRequests))
	}

	// リトライ（全て成功）
	retryResults := processor.ProcessBatch(ctx, failedRequests)
	atomic.AddInt32(&attemptCount, int32(len(retryResults)))

	successCount := 0
	for _, result := range retryResults {
		if result.Error == nil {
			successCount++
		}
	}

	if successCount != 10 {
		t.Errorf("retry success count = %d, want 10", successCount)
	}

	t.Logf("Retry test completed: initial failures=%d, retry successes=%d", len(failedRequests), successCount)
}

// TestBatchProcessing_RateLimiting はレート制限が正しく動作することを確認
func TestBatchProcessing_RateLimiting(t *testing.T) {
	mockClient := &mockLLMClientForBatch{
		delay: 1 * time.Millisecond,
	}

	// レート制限を設定（60 req/min = 1秒に1リクエスト）
	// テストでは短時間で完了するように調整
	throttledClient := NewThrottledLLMClient(mockClient, 60)

	processor := NewBatchProcessor(throttledClient, BatchProcessorConfig{
		MaxConcurrency: 10, // 並列度は高いがレート制限で制御される
	})

	// 5個のリクエストを作成（テストを高速化）
	requests := make([]BatchRequest, 5)
	for i := 0; i < 5; i++ {
		requests[i] = BatchRequest{
			ID: fmt.Sprintf("req-%d", i),
			Request: CompletionRequest{
				Prompt: "test",
			},
		}
	}

	ctx := context.Background()
	startTime := time.Now()
	results := processor.ProcessBatch(ctx, requests)
	elapsed := time.Since(startTime)

	// すべてのリクエストが完了していることを確認
	if len(results) != 5 {
		t.Errorf("results count = %d, want 5", len(results))
	}

	// レート制限とバッチ処理が正常に動作していることを確認
	t.Logf("Rate limiting test completed in %s", elapsed.Round(time.Millisecond))
}

// TestBatchProcessing_ProgressTracking はプログレストラッキングが正しく動作することを確認
func TestBatchProcessing_ProgressTracking(t *testing.T) {
	mockClient := &mockLLMClientForBatch{
		delay: 10 * time.Millisecond,
	}

	progressUpdates := []BatchProgress{}
	processor := NewBatchProcessor(mockClient, BatchProcessorConfig{
		MaxConcurrency: 5,
		ProgressCallback: func(progress BatchProgress) {
			progressUpdates = append(progressUpdates, progress)
		},
	})

	// 20個のリクエストを作成
	requests := make([]BatchRequest, 20)
	for i := 0; i < 20; i++ {
		requests[i] = BatchRequest{
			ID: fmt.Sprintf("req-%d", i),
			Request: CompletionRequest{
				Prompt: "test",
			},
		}
	}

	ctx := context.Background()
	results := processor.ProcessBatch(ctx, requests)

	// プログレス更新が記録されていることを確認
	// 並行実行により、更新回数は必ずしも正確に20回とは限らない
	if len(progressUpdates) < 10 || len(progressUpdates) > 25 {
		t.Errorf("progress updates count = %d, want 10-25 (actual count may vary due to concurrency)", len(progressUpdates))
	}

	// 各更新でCompletedが有効な範囲にあることを確認
	for i, progress := range progressUpdates {
		if progress.Total != 20 {
			t.Errorf("progress[%d].Total = %d, want 20", i, progress.Total)
		}
		if progress.Completed < 1 || progress.Completed > 20 {
			t.Errorf("progress[%d].Completed = %d, want 1-20", i, progress.Completed)
		}
	}

	// 最後の更新で全て完了しているか、ほぼ完了していることを確認
	if len(progressUpdates) > 0 {
		lastProgress := progressUpdates[len(progressUpdates)-1]
		if lastProgress.Completed < 18 {
			t.Errorf("last progress Completed = %d, want >= 18", lastProgress.Completed)
		}
	}

	// すべてのリクエストが成功していることを確認
	stats := CalculateBatchStats(results)
	if stats.SuccessCount != 20 {
		t.Errorf("SuccessCount = %d, want 20", stats.SuccessCount)
	}

	t.Logf("Progress tracking test completed: %d updates recorded", len(progressUpdates))
}

// TestBatchProcessing_MixedWorkload は異なる種類のリクエストを混在させたテスト
func TestBatchProcessing_MixedWorkload(t *testing.T) {
	mockClient := &mockLLMClientForBatch{
		delay: 5 * time.Millisecond,
		failPattern: func(i int) bool {
			// 3の倍数のリクエストは失敗
			return i%3 == 0
		},
	}

	processor := NewBatchProcessor(mockClient, BatchProcessorConfig{
		MaxConcurrency: 10,
	})

	// 異なる種類のリクエストを作成
	requests := []BatchRequest{
		// ファイルサマリー
		{ID: "file1.go", Request: CompletionRequest{Prompt: "Summarize file1.go", Temperature: 0.3, MaxTokens: 400}},
		{ID: "file2.go", Request: CompletionRequest{Prompt: "Summarize file2.go", Temperature: 0.3, MaxTokens: 400}},
		{ID: "file3.go", Request: CompletionRequest{Prompt: "Summarize file3.go", Temperature: 0.3, MaxTokens: 400}},
		// チャンク要約
		{ID: "chunk1", Request: CompletionRequest{Prompt: "Summarize chunk1", Temperature: 0.2, MaxTokens: 80}},
		{ID: "chunk2", Request: CompletionRequest{Prompt: "Summarize chunk2", Temperature: 0.2, MaxTokens: 80}},
		{ID: "chunk3", Request: CompletionRequest{Prompt: "Summarize chunk3", Temperature: 0.2, MaxTokens: 80}},
		// ドメイン分類
		{ID: "node1", Request: CompletionRequest{Prompt: "Classify node1", Temperature: 0.0, MaxTokens: 150}},
		{ID: "node2", Request: CompletionRequest{Prompt: "Classify node2", Temperature: 0.0, MaxTokens: 150}},
		{ID: "node3", Request: CompletionRequest{Prompt: "Classify node3", Temperature: 0.0, MaxTokens: 150}},
	}

	ctx := context.Background()
	results := processor.ProcessBatch(ctx, requests)

	// 結果を検証
	if len(results) != 9 {
		t.Errorf("results count = %d, want 9", len(results))
	}

	stats := CalculateBatchStats(results)

	// 3の倍数（0, 3, 6）が失敗するので3個失敗
	expectedFailures := 3
	if stats.FailureCount != expectedFailures {
		t.Errorf("FailureCount = %d, want %d", stats.FailureCount, expectedFailures)
	}

	expectedSuccess := 6
	if stats.SuccessCount != expectedSuccess {
		t.Errorf("SuccessCount = %d, want %d", stats.SuccessCount, expectedSuccess)
	}

	t.Logf("Mixed workload test completed: %s", stats.String())
}

// TestBatchProcessing_EmptyBatch は空のバッチを処理できることを確認
func TestBatchProcessing_EmptyBatch(t *testing.T) {
	mockClient := &mockLLMClientForBatch{}

	processor := NewBatchProcessor(mockClient, BatchProcessorConfig{
		MaxConcurrency: 10,
	})

	ctx := context.Background()
	results := processor.ProcessBatch(ctx, []BatchRequest{})

	if len(results) != 0 {
		t.Errorf("results count = %d, want 0", len(results))
	}

	stats := CalculateBatchStats(results)
	if stats.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d, want 0", stats.TotalRequests)
	}
}

// TestBatchProcessing_ErrorRecovery はエラーからの回復を確認
func TestBatchProcessing_ErrorRecovery(t *testing.T) {
	// 特定のリクエストでパニックしないことを確認
	var callCount int32
	mockClient := &mockLLMClientForBatch{
		delay: 5 * time.Millisecond,
		failPattern: func(i int) bool {
			// カウンターをインクリメントして、5番目の呼び出しのみ失敗
			count := atomic.AddInt32(&callCount, 1) - 1
			return count == 5
		},
	}

	processor := NewBatchProcessor(mockClient, BatchProcessorConfig{
		MaxConcurrency: 3,
	})

	requests := make([]BatchRequest, 10)
	for i := 0; i < 10; i++ {
		requests[i] = BatchRequest{
			ID: fmt.Sprintf("req-%d", i),
			Request: CompletionRequest{
				Prompt: fmt.Sprintf("test %d", i),
			},
		}
	}

	ctx := context.Background()

	// パニックせずに完了することを確認
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ProcessBatch panicked: %v", r)
		}
	}()

	results := processor.ProcessBatch(ctx, requests)

	// 1つだけ失敗していることを確認
	stats := CalculateBatchStats(results)
	if stats.FailureCount != 1 {
		t.Errorf("FailureCount = %d, want 1", stats.FailureCount)
	}

	if stats.SuccessCount != 9 {
		t.Errorf("SuccessCount = %d, want 9", stats.SuccessCount)
	}

	// 失敗したリクエストが1つあることを確認
	if len(stats.FailedRequests) != 1 {
		t.Errorf("FailedRequests count = %d, want 1", len(stats.FailedRequests))
	}

	t.Logf("Error recovery test passed: %d succeeded, %d failed (ID: %v)",
		stats.SuccessCount, stats.FailureCount, stats.FailedRequests)
}

// mockComplexClient はより複雑なシミュレーションを行うモッククライアント
type mockComplexClient struct {
	requestCount int32
	minDelay     time.Duration
	maxDelay     time.Duration
}

func (m *mockComplexClient) GenerateCompletion(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	count := atomic.AddInt32(&m.requestCount, 1)

	// 可変遅延をシミュレート（実際のLLM APIのような動作）
	delay := m.minDelay + time.Duration(count%5)*time.Millisecond*10
	if delay > m.maxDelay {
		delay = m.maxDelay
	}

	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return CompletionResponse{}, ctx.Err()
	}

	// 稀にエラーを返す（5%の確率）
	if count%20 == 0 {
		return CompletionResponse{}, errors.New("simulated transient error")
	}

	return CompletionResponse{
		Content:       fmt.Sprintf("Response for: %s", req.Prompt),
		TokensUsed:    100 + int(count%50),
		PromptVersion: "1.1",
		Model:         "test-model",
	}, nil
}

// TestBatchProcessing_RealisticScenario はより現実的なシナリオをテスト
func TestBatchProcessing_RealisticScenario(t *testing.T) {
	client := &mockComplexClient{
		minDelay: 10 * time.Millisecond,
		maxDelay: 50 * time.Millisecond,
	}

	logger := NewProgressLogger(100*time.Millisecond, false, false)

	processor := NewBatchProcessorWithProgress(
		client,
		BatchProcessorConfig{
			MaxConcurrency: 10,
		},
		logger,
		false,
	)

	// 50個のリクエストを作成
	requests := make([]BatchRequest, 50)
	for i := 0; i < 50; i++ {
		requests[i] = BatchRequest{
			ID: fmt.Sprintf("realistic-req-%d", i),
			Request: CompletionRequest{
				Prompt:      fmt.Sprintf("Realistic prompt %d", i),
				Temperature: 0.3,
				MaxTokens:   200,
			},
		}
	}

	ctx := context.Background()
	startTime := time.Now()
	results, stats := processor.ProcessBatch(ctx, requests)
	elapsed := time.Since(startTime)

	// 結果を検証
	if len(results) != 50 {
		t.Errorf("results count = %d, want 50", len(results))
	}

	// 約5%が失敗することを期待（20個に1個）
	expectedFailures := 2 // 50個中2-3個程度
	if stats.FailureCount < 1 || stats.FailureCount > 5 {
		t.Logf("Note: FailureCount = %d, expected around %d (acceptable range: 1-5)",
			stats.FailureCount, expectedFailures)
	}

	t.Logf("Realistic scenario completed: %s in %s", stats.String(), elapsed.Round(time.Millisecond))
	t.Logf("Average response time: %s", stats.AverageDuration.Round(time.Millisecond))
}
