package llm

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RateLimiter はAPI呼び出しのレート制限を管理する
type RateLimiter struct {
	mu sync.Mutex

	// maxRequestsPerMinute は1分あたりの最大リクエスト数
	maxRequestsPerMinute int

	// tokens はトークンバケット
	tokens int

	// lastRefill は最後にトークンを補充した時刻
	lastRefill time.Time

	// waitQueue は待機中のリクエスト数
	waitQueue int

	// semaphore は並列実行を制御するセマフォ
	semaphore chan struct{}
}

// NewRateLimiter は新しいRateLimiterを作成する
func NewRateLimiter(maxRequestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		maxRequestsPerMinute: maxRequestsPerMinute,
		tokens:               maxRequestsPerMinute,
		lastRefill:           time.Now(),
		semaphore:            make(chan struct{}, maxRequestsPerMinute),
	}
}

// Wait はレート制限に従って待機し、実行権限を取得する
// contextがキャンセルされた場合はエラーを返す
func (rl *RateLimiter) Wait(ctx context.Context) error {
	// セマフォで並列度を制御
	select {
	case rl.semaphore <- struct{}{}:
		// セマフォを取得できた
	case <-ctx.Done():
		return ctx.Err()
	}

	// トークンバケットアルゴリズムでレート制限
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for {
		// トークンを補充
		rl.refillTokens()

		// トークンがある場合は実行を許可
		if rl.tokens > 0 {
			rl.tokens--
			return nil
		}

		// トークンがない場合は待機
		rl.waitQueue++
		rl.mu.Unlock()

		// 次の補充まで待機
		waitDuration := time.Second
		select {
		case <-time.After(waitDuration):
			// タイムアウト後に再試行
		case <-ctx.Done():
			rl.mu.Lock()
			rl.waitQueue--
			<-rl.semaphore // セマフォを解放
			return ctx.Err()
		}

		rl.mu.Lock()
		rl.waitQueue--
	}
}

// Release は実行権限を解放する
// Wait()の後に必ずRelease()を呼ぶこと（通常はdefer文で）
func (rl *RateLimiter) Release() {
	<-rl.semaphore
}

// refillTokens はトークンを補充する（内部用）
// 呼び出し側でロックを取得していることを前提とする
func (rl *RateLimiter) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)

	if elapsed < time.Minute {
		return
	}

	// 1分以上経過している場合はトークンを補充
	minutes := int(elapsed.Minutes())
	tokensToAdd := minutes * rl.maxRequestsPerMinute

	rl.tokens = min(rl.tokens+tokensToAdd, rl.maxRequestsPerMinute)
	rl.lastRefill = rl.lastRefill.Add(time.Duration(minutes) * time.Minute)
}

// GetStatus は現在の状態を返す（デバッグ・監視用）
func (rl *RateLimiter) GetStatus() RateLimiterStatus {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refillTokens()

	return RateLimiterStatus{
		MaxRequestsPerMinute: rl.maxRequestsPerMinute,
		AvailableTokens:      rl.tokens,
		WaitingRequests:      rl.waitQueue,
		ActiveRequests:       len(rl.semaphore),
	}
}

// RateLimiterStatus はレート制限の状態
type RateLimiterStatus struct {
	MaxRequestsPerMinute int
	AvailableTokens      int
	WaitingRequests      int
	ActiveRequests       int
}

// String はステータスを文字列表現で返す
func (s RateLimiterStatus) String() string {
	return fmt.Sprintf(
		"RateLimiter: max=%d/min, available=%d, waiting=%d, active=%d",
		s.MaxRequestsPerMinute,
		s.AvailableTokens,
		s.WaitingRequests,
		s.ActiveRequests,
	)
}

// min はintの最小値を返す
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ThrottledLLMClient はレート制限付きのLLMクライアント
type ThrottledLLMClient struct {
	client      LLMClient
	rateLimiter *RateLimiter
}

// NewThrottledLLMClient はレート制限付きのLLMクライアントを作成する
func NewThrottledLLMClient(client LLMClient, maxRequestsPerMinute int) *ThrottledLLMClient {
	return &ThrottledLLMClient{
		client:      client,
		rateLimiter: NewRateLimiter(maxRequestsPerMinute),
	}
}

// GenerateCompletion はレート制限に従ってLLM APIを呼び出す
func (tc *ThrottledLLMClient) GenerateCompletion(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	// レート制限に従って待機
	if err := tc.rateLimiter.Wait(ctx); err != nil {
		return CompletionResponse{}, fmt.Errorf("rate limiter wait failed: %w", err)
	}
	defer tc.rateLimiter.Release()

	// 実際のLLM APIを呼び出す
	return tc.client.GenerateCompletion(ctx, req)
}

// GetRateLimiterStatus はレート制限の状態を返す
func (tc *ThrottledLLMClient) GetRateLimiterStatus() RateLimiterStatus {
	return tc.rateLimiter.GetStatus()
}
