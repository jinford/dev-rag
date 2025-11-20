package llm

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// mockLLMClientForBatch はバッチテスト用のモッククライアント
type mockLLMClientForBatch struct {
	callCount   int32
	delay       time.Duration
	failPattern func(int) bool // n番目のリクエストを失敗させるパターン
}

func (m *mockLLMClientForBatch) GenerateCompletion(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	callIndex := int(atomic.AddInt32(&m.callCount, 1)) - 1

	// 遅延をシミュレート
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return CompletionResponse{}, ctx.Err()
		}
	}

	// 失敗パターンをチェック
	if m.failPattern != nil && m.failPattern(callIndex) {
		return CompletionResponse{}, errors.New("simulated error")
	}

	// 成功レスポンスを返す
	return CompletionResponse{
		Content:       fmt.Sprintf("Response for: %s", req.Prompt),
		TokensUsed:    100,
		PromptVersion: "1.1",
		Model:         "test-model",
	}, nil
}

func TestBatchProcessor_ProcessBatch(t *testing.T) {
	tests := []struct {
		name           string
		requestCount   int
		maxConcurrency int
		delay          time.Duration
		failPattern    func(int) bool
		wantSuccess    int
		wantFailed     int
	}{
		{
			name:           "正常: 10リクエストを5並列で処理",
			requestCount:   10,
			maxConcurrency: 5,
			delay:          10 * time.Millisecond,
			failPattern:    nil,
			wantSuccess:    10,
			wantFailed:     0,
		},
		{
			name:           "正常: 20リクエストを10並列で処理",
			requestCount:   20,
			maxConcurrency: 10,
			delay:          5 * time.Millisecond,
			failPattern:    nil,
			wantSuccess:    20,
			wantFailed:     0,
		},
		{
			name:           "エラー処理: 一部のリクエストが失敗",
			requestCount:   10,
			maxConcurrency: 5,
			delay:          5 * time.Millisecond,
			failPattern: func(i int) bool {
				// 2番目と5番目のリクエストを失敗させる
				return i == 1 || i == 4
			},
			wantSuccess: 8,
			wantFailed:  2,
		},
		{
			name:           "エラー処理: すべてのリクエストが失敗",
			requestCount:   5,
			maxConcurrency: 3,
			delay:          5 * time.Millisecond,
			failPattern: func(i int) bool {
				return true // すべて失敗
			},
			wantSuccess: 0,
			wantFailed:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// モッククライアントを作成
			mockClient := &mockLLMClientForBatch{
				delay:       tt.delay,
				failPattern: tt.failPattern,
			}

			// バッチプロセッサーを作成
			progressCallCount := 0
			processor := NewBatchProcessor(mockClient, BatchProcessorConfig{
				MaxConcurrency: tt.maxConcurrency,
				ProgressCallback: func(progress BatchProgress) {
					progressCallCount++
					t.Logf("Progress: %s", progress.String())
				},
				PromptSection: PromptSectionFileSummary,
			})

			// テストリクエストを作成
			requests := make([]BatchRequest, tt.requestCount)
			for i := 0; i < tt.requestCount; i++ {
				requests[i] = BatchRequest{
					ID: fmt.Sprintf("req-%d", i),
					Request: CompletionRequest{
						Prompt:      fmt.Sprintf("test prompt %d", i),
						Temperature: 0.0,
						MaxTokens:   100,
					},
				}
			}

			// バッチ処理を実行
			ctx := context.Background()
			startTime := time.Now()
			results := processor.ProcessBatch(ctx, requests)
			elapsed := time.Since(startTime)

			// 結果を検証
			if len(results) != tt.requestCount {
				t.Errorf("results count = %d, want %d", len(results), tt.requestCount)
			}

			successCount := 0
			failedCount := 0
			for i, result := range results {
				if result.ID != fmt.Sprintf("req-%d", i) {
					t.Errorf("result[%d].ID = %s, want req-%d", i, result.ID, i)
				}

				if result.Error != nil {
					failedCount++
				} else {
					successCount++
				}
			}

			if successCount != tt.wantSuccess {
				t.Errorf("success count = %d, want %d", successCount, tt.wantSuccess)
			}

			if failedCount != tt.wantFailed {
				t.Errorf("failed count = %d, want %d", failedCount, tt.wantFailed)
			}

			// プログレスコールバックが呼ばれたことを確認
			if progressCallCount == 0 {
				t.Error("progress callback was not called")
			}

			// 並列処理が実際に機能していることを確認
			// （完全に直列実行した場合の時間より短いはず）
			serialTime := tt.delay * time.Duration(tt.requestCount)
			if elapsed >= serialTime {
				t.Errorf("elapsed time %s >= serial time %s, parallelism may not be working", elapsed, serialTime)
			}

			t.Logf("Processed %d requests in %s (serial would take %s)",
				tt.requestCount, elapsed.Round(time.Millisecond), serialTime)
		})
	}
}

func TestBatchProcessor_ProcessBatch_EmptyRequests(t *testing.T) {
	mockClient := &mockLLMClientForBatch{}
	processor := NewBatchProcessor(mockClient, BatchProcessorConfig{
		MaxConcurrency: 10,
	})

	ctx := context.Background()
	results := processor.ProcessBatch(ctx, []BatchRequest{})

	if len(results) != 0 {
		t.Errorf("results count = %d, want 0", len(results))
	}
}

func TestBatchProcessor_ProcessBatch_ContextCancellation(t *testing.T) {
	// 長い遅延を持つモッククライアント
	mockClient := &mockLLMClientForBatch{
		delay: 1 * time.Second,
	}

	processor := NewBatchProcessor(mockClient, BatchProcessorConfig{
		MaxConcurrency: 5,
	})

	// テストリクエストを作成
	requests := make([]BatchRequest, 10)
	for i := 0; i < 10; i++ {
		requests[i] = BatchRequest{
			ID: fmt.Sprintf("req-%d", i),
			Request: CompletionRequest{
				Prompt: fmt.Sprintf("test prompt %d", i),
			},
		}
	}

	// コンテキストを100ms後にキャンセル
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	startTime := time.Now()
	results := processor.ProcessBatch(ctx, requests)
	elapsed := time.Since(startTime)

	// すべてのリクエストが処理されたことを確認（失敗も含む）
	if len(results) != 10 {
		t.Errorf("results count = %d, want 10", len(results))
	}

	// 少なくともいくつかのリクエストがタイムアウトで失敗していることを確認
	timeoutCount := 0
	for _, result := range results {
		if result.Error != nil && errors.Is(result.Error, context.DeadlineExceeded) {
			timeoutCount++
		}
	}

	if timeoutCount == 0 {
		t.Error("expected some requests to timeout, but none did")
	}

	// タイムアウト時間より長くかかっていないことを確認
	if elapsed > 500*time.Millisecond {
		t.Errorf("elapsed time %s > 500ms, context cancellation may not be working", elapsed)
	}

	t.Logf("Context cancelled after %s, %d/%d requests timed out",
		elapsed.Round(time.Millisecond), timeoutCount, 10)
}

func TestCalculateBatchStats(t *testing.T) {
	tests := []struct {
		name    string
		results []BatchResult
		want    BatchStats
	}{
		{
			name: "正常: すべて成功",
			results: []BatchResult{
				{ID: "1", Duration: 100 * time.Millisecond, Error: nil},
				{ID: "2", Duration: 200 * time.Millisecond, Error: nil},
				{ID: "3", Duration: 150 * time.Millisecond, Error: nil},
			},
			want: BatchStats{
				TotalRequests:   3,
				SuccessCount:    3,
				FailureCount:    0,
				AverageDuration: 150 * time.Millisecond,
				MinDuration:     100 * time.Millisecond,
				MaxDuration:     200 * time.Millisecond,
				FailedRequests:  nil,
			},
		},
		{
			name: "正常: 一部失敗",
			results: []BatchResult{
				{ID: "1", Duration: 100 * time.Millisecond, Error: nil},
				{ID: "2", Duration: 0, Error: errors.New("error")},
				{ID: "3", Duration: 200 * time.Millisecond, Error: nil},
			},
			want: BatchStats{
				TotalRequests:   3,
				SuccessCount:    2,
				FailureCount:    1,
				AverageDuration: 150 * time.Millisecond,
				MinDuration:     100 * time.Millisecond,
				MaxDuration:     200 * time.Millisecond,
				FailedRequests:  []string{"2"},
			},
		},
		{
			name: "正常: すべて失敗",
			results: []BatchResult{
				{ID: "1", Duration: 0, Error: errors.New("error1")},
				{ID: "2", Duration: 0, Error: errors.New("error2")},
			},
			want: BatchStats{
				TotalRequests:   2,
				SuccessCount:    0,
				FailureCount:    2,
				AverageDuration: 0,
				MinDuration:     0,
				MaxDuration:     0,
				FailedRequests:  []string{"1", "2"},
			},
		},
		{
			name:    "正常: 空の結果",
			results: []BatchResult{},
			want: BatchStats{
				TotalRequests:  0,
				SuccessCount:   0,
				FailureCount:   0,
				MinDuration:    0,
				FailedRequests: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateBatchStats(tt.results)

			if got.TotalRequests != tt.want.TotalRequests {
				t.Errorf("TotalRequests = %d, want %d", got.TotalRequests, tt.want.TotalRequests)
			}
			if got.SuccessCount != tt.want.SuccessCount {
				t.Errorf("SuccessCount = %d, want %d", got.SuccessCount, tt.want.SuccessCount)
			}
			if got.FailureCount != tt.want.FailureCount {
				t.Errorf("FailureCount = %d, want %d", got.FailureCount, tt.want.FailureCount)
			}
			if got.AverageDuration != tt.want.AverageDuration {
				t.Errorf("AverageDuration = %s, want %s", got.AverageDuration, tt.want.AverageDuration)
			}
			if got.MinDuration != tt.want.MinDuration {
				t.Errorf("MinDuration = %s, want %s", got.MinDuration, tt.want.MinDuration)
			}
			if got.MaxDuration != tt.want.MaxDuration {
				t.Errorf("MaxDuration = %s, want %s", got.MaxDuration, tt.want.MaxDuration)
			}

			if len(got.FailedRequests) != len(tt.want.FailedRequests) {
				t.Errorf("FailedRequests count = %d, want %d", len(got.FailedRequests), len(tt.want.FailedRequests))
			}
		})
	}
}

func TestBatchProgress_String(t *testing.T) {
	progress := BatchProgress{
		Total:                  100,
		Completed:              45,
		Failed:                 5,
		ElapsedTime:            2 * time.Minute,
		EstimatedTimeRemaining: 3 * time.Minute,
	}

	result := progress.String()
	expected := "Progress: 45/100 (45.0%) | Failed: 5 | Elapsed: 2m0s | ETA: 3m0s"

	if result != expected {
		t.Errorf("String() = %q, want %q", result, expected)
	}
}

func TestBatchStats_String(t *testing.T) {
	stats := BatchStats{
		TotalRequests:   10,
		SuccessCount:    8,
		FailureCount:    2,
		AverageDuration: 150 * time.Millisecond,
		MinDuration:     100 * time.Millisecond,
		MaxDuration:     200 * time.Millisecond,
	}

	result := stats.String()
	t.Log(result)

	// 主要な情報が含まれていることを確認
	if result == "" {
		t.Error("String() returned empty string")
	}
}

func TestNewBatchProcessor_DefaultConfig(t *testing.T) {
	mockClient := &mockLLMClientForBatch{}

	// MaxConcurrencyを指定しない場合
	processor := NewBatchProcessor(mockClient, BatchProcessorConfig{})

	if processor.config.MaxConcurrency != 10 {
		t.Errorf("MaxConcurrency = %d, want 10 (default)", processor.config.MaxConcurrency)
	}
}

func TestBatchProcessor_OrderPreservation(t *testing.T) {
	// 順序が保持されることを確認
	mockClient := &mockLLMClientForBatch{
		delay: 10 * time.Millisecond,
	}

	processor := NewBatchProcessor(mockClient, BatchProcessorConfig{
		MaxConcurrency: 5,
	})

	requests := make([]BatchRequest, 20)
	for i := 0; i < 20; i++ {
		requests[i] = BatchRequest{
			ID: fmt.Sprintf("req-%d", i),
			Request: CompletionRequest{
				Prompt: fmt.Sprintf("prompt %d", i),
			},
		}
	}

	ctx := context.Background()
	results := processor.ProcessBatch(ctx, requests)

	// 結果の順序がリクエストの順序と一致することを確認
	for i, result := range results {
		expectedID := fmt.Sprintf("req-%d", i)
		if result.ID != expectedID {
			t.Errorf("results[%d].ID = %s, want %s (order not preserved)", i, result.ID, expectedID)
		}
	}
}
