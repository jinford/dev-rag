package llm

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10)
	require.NotNil(t, rl)
	assert.Equal(t, 10, rl.maxRequestsPerMinute)
	assert.Equal(t, 10, rl.tokens)
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(10)
	ctx := context.Background()

	// 最初の呼び出しは即座に成功する
	err := rl.Wait(ctx)
	require.NoError(t, err)
	defer rl.Release()

	status := rl.GetStatus()
	assert.Equal(t, 9, status.AvailableTokens)
	assert.Equal(t, 1, status.ActiveRequests)
}

func TestRateLimiter_MultipleWaits(t *testing.T) {
	rl := NewRateLimiter(5)
	ctx := context.Background()

	// 5回連続で呼び出す
	for i := 0; i < 5; i++ {
		err := rl.Wait(ctx)
		require.NoError(t, err)
		defer rl.Release()
	}

	status := rl.GetStatus()
	assert.Equal(t, 0, status.AvailableTokens)
	assert.Equal(t, 5, status.ActiveRequests)
}

func TestRateLimiter_RateLimitExceeded(t *testing.T) {
	rl := NewRateLimiter(2)
	ctx := context.Background()

	// 最初の2回は即座に成功
	err := rl.Wait(ctx)
	require.NoError(t, err)
	defer rl.Release()

	err = rl.Wait(ctx)
	require.NoError(t, err)
	defer rl.Release()

	// 3回目は待機が必要
	// タイムアウト付きコンテキストでテスト
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = rl.Wait(ctx)
	elapsed := time.Since(start)

	// タイムアウトするはず
	assert.Error(t, err)
	assert.True(t, elapsed >= 100*time.Millisecond, "should wait for at least 100ms")
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	rl := NewRateLimiter(1)

	// 先にトークンを使い切る
	ctx := context.Background()
	err := rl.Wait(ctx)
	require.NoError(t, err)
	defer rl.Release()

	// キャンセル可能なコンテキストで待機
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 即座にキャンセル

	err = rl.Wait(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	rl := NewRateLimiter(10)

	// トークンを消費
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		err := rl.Wait(ctx)
		require.NoError(t, err)
		rl.Release()
	}

	// トークンがなくなったことを確認
	status := rl.GetStatus()
	assert.Equal(t, 0, status.AvailableTokens)

	// 時刻を進める（テスト用に内部状態を操作）
	rl.mu.Lock()
	rl.lastRefill = time.Now().Add(-61 * time.Second)
	rl.mu.Unlock()

	// トークンが補充されることを確認
	status = rl.GetStatus()
	assert.Equal(t, 10, status.AvailableTokens)
}

func TestRateLimiter_ConcurrentRequests(t *testing.T) {
	rl := NewRateLimiter(10)
	ctx := context.Background()

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// 10個の並列リクエスト
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := rl.Wait(ctx)
			if err == nil {
				defer rl.Release()
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// すべて成功するはず
	assert.Equal(t, 10, successCount)
}

func TestRateLimiter_GetStatus(t *testing.T) {
	rl := NewRateLimiter(10)
	ctx := context.Background()

	// 初期状態
	status := rl.GetStatus()
	assert.Equal(t, 10, status.MaxRequestsPerMinute)
	assert.Equal(t, 10, status.AvailableTokens)
	assert.Equal(t, 0, status.WaitingRequests)
	assert.Equal(t, 0, status.ActiveRequests)

	// リクエストを実行
	err := rl.Wait(ctx)
	require.NoError(t, err)
	defer rl.Release()

	status = rl.GetStatus()
	assert.Equal(t, 9, status.AvailableTokens)
	assert.Equal(t, 1, status.ActiveRequests)
}

func TestRateLimiterStatus_String(t *testing.T) {
	status := RateLimiterStatus{
		MaxRequestsPerMinute: 10,
		AvailableTokens:      5,
		WaitingRequests:      2,
		ActiveRequests:       3,
	}

	str := status.String()
	assert.Contains(t, str, "max=10/min")
	assert.Contains(t, str, "available=5")
	assert.Contains(t, str, "waiting=2")
	assert.Contains(t, str, "active=3")
}

func TestThrottledLLMClient_GenerateCompletion(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	mock := &MockLLMClient{
		GenerateCompletionFunc: func(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			return CompletionResponse{
				Content:    "test response",
				TokensUsed: 10,
				Model:      "mock-model",
			}, nil
		},
	}
	throttled := NewThrottledLLMClient(mock, 10)

	ctx := context.Background()
	req := CompletionRequest{
		Prompt:      "test prompt",
		Temperature: 0.3,
		MaxTokens:   100,
	}

	resp, err := throttled.GenerateCompletion(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "test response", resp.Content)
	mu.Lock()
	assert.Equal(t, 1, callCount)
	mu.Unlock()
}

func TestThrottledLLMClient_RateLimit(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	mock := &MockLLMClient{
		GenerateCompletionFunc: func(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			return CompletionResponse{Content: "test"}, nil
		},
	}
	throttled := NewThrottledLLMClient(mock, 5)

	ctx := context.Background()
	req := CompletionRequest{Prompt: "test"}

	// 5回連続で呼び出す
	for i := 0; i < 5; i++ {
		_, err := throttled.GenerateCompletion(ctx, req)
		require.NoError(t, err)
	}

	mu.Lock()
	assert.Equal(t, 5, callCount)
	mu.Unlock()

	status := throttled.GetRateLimiterStatus()
	assert.Equal(t, 0, status.AvailableTokens)
}

func TestThrottledLLMClient_ContextCancellation(t *testing.T) {
	mock := &MockLLMClient{
		GenerateCompletionFunc: func(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
			return CompletionResponse{Content: "test"}, nil
		},
	}
	throttled := NewThrottledLLMClient(mock, 1)

	// 先にトークンを使い切る
	ctx := context.Background()
	_, err := throttled.GenerateCompletion(ctx, CompletionRequest{Prompt: "test"})
	require.NoError(t, err)

	// キャンセル済みのコンテキストで呼び出す
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = throttled.GenerateCompletion(ctx, CompletionRequest{Prompt: "test2"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limiter wait failed")
}

func BenchmarkRateLimiter_Wait(b *testing.B) {
	rl := NewRateLimiter(1000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rl.Wait(ctx)
		rl.Release()
	}
}
