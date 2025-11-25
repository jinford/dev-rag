package llm

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BatchRequest は単一のバッチリクエスト項目
type BatchRequest struct {
	// ID は項目の識別子（ログ・トラッキング用）
	ID string
	// Request はLLMへのリクエスト
	Request CompletionRequest
}

// BatchResult は単一のバッチ処理結果
type BatchResult struct {
	// ID はリクエストの識別子
	ID string
	// Response はLLMからのレスポンス（成功時）
	Response CompletionResponse
	// Error はエラー情報（失敗時）
	Error error
	// Duration は処理時間
	Duration time.Duration
}

// BatchProcessorConfig はバッチ処理の設定
type BatchProcessorConfig struct {
	// MaxConcurrency は同時実行数の上限
	MaxConcurrency int
	// ProgressCallback はプログレス更新時に呼ばれるコールバック
	ProgressCallback func(progress BatchProgress)
	// ErrorHandler はエラーハンドラー（オプション）
	ErrorHandler *ErrorHandler
	// PromptSection はエラーログ用のプロンプトセクション
	PromptSection PromptSection
}

// BatchProgress はバッチ処理の進捗状況
type BatchProgress struct {
	// Total は総リクエスト数
	Total int
	// Completed は完了したリクエスト数
	Completed int
	// Failed は失敗したリクエスト数
	Failed int
	// ElapsedTime は経過時間
	ElapsedTime time.Duration
	// EstimatedTimeRemaining は推定残り時間
	EstimatedTimeRemaining time.Duration
}

// String はプログレスを文字列表現で返す
func (p BatchProgress) String() string {
	percentage := 0.0
	if p.Total > 0 {
		percentage = float64(p.Completed) / float64(p.Total) * 100
	}

	eta := "N/A"
	if p.EstimatedTimeRemaining > 0 {
		eta = p.EstimatedTimeRemaining.Round(time.Second).String()
	}

	return fmt.Sprintf(
		"Progress: %d/%d (%.1f%%) | Failed: %d | Elapsed: %s | ETA: %s",
		p.Completed,
		p.Total,
		percentage,
		p.Failed,
		p.ElapsedTime.Round(time.Second),
		eta,
	)
}

// BatchProcessor は複数のLLMリクエストを並列実行する
type BatchProcessor struct {
	client LLMClient
	config BatchProcessorConfig
}

// NewBatchProcessor は新しいBatchProcessorを作成する
func NewBatchProcessor(client LLMClient, config BatchProcessorConfig) *BatchProcessor {
	// デフォルト値の設定
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 10
	}

	return &BatchProcessor{
		client: client,
		config: config,
	}
}

// ProcessBatch は複数のリクエストを並列実行する
// contextがキャンセルされた場合、処理中のリクエストは中断される
// 一部のリクエストが失敗しても、残りのリクエストは継続される
func (bp *BatchProcessor) ProcessBatch(ctx context.Context, requests []BatchRequest) []BatchResult {
	total := len(requests)
	if total == 0 {
		return []BatchResult{}
	}

	// 進捗管理
	var mu sync.Mutex
	completed := 0
	failed := 0
	startTime := time.Now()

	// 結果を格納するスライス（順序を保持）
	results := make([]BatchResult, total)

	// ワーカープールで並列実行
	semaphore := make(chan struct{}, bp.config.MaxConcurrency)
	var wg sync.WaitGroup

	for i, req := range requests {
		wg.Add(1)

		go func(index int, batchReq BatchRequest) {
			defer wg.Done()

			// セマフォを取得（並列度を制限）
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				// コンテキストがキャンセルされた場合
				results[index] = BatchResult{
					ID:    batchReq.ID,
					Error: ctx.Err(),
				}
				mu.Lock()
				failed++
				completed++
				mu.Unlock()
				bp.notifyProgress(total, completed, failed, startTime)
				return
			}

			// リクエストを処理
			reqStartTime := time.Now()
			resp, err := bp.client.GenerateCompletion(ctx, batchReq.Request)
			duration := time.Since(reqStartTime)

			// 結果を記録
			results[index] = BatchResult{
				ID:       batchReq.ID,
				Response: resp,
				Error:    err,
				Duration: duration,
			}

			// 統計情報を更新
			mu.Lock()
			completed++
			if err != nil {
				failed++

				// エラーをログに記録
				if bp.config.ErrorHandler != nil {
					errorType := ErrorTypeUnknown
					if ctx.Err() != nil {
						errorType = ErrorTypeTimeout
					}

					_ = bp.config.ErrorHandler.LogError(ErrorRecord{
						Timestamp:     time.Now(),
						ErrorType:     errorType,
						PromptSection: bp.config.PromptSection,
						Prompt:        TruncateString(batchReq.Request.Prompt, 500),
						Response:      "",
						ErrorMessage:  err.Error(),
						RetryCount:    0,
					})
				}
			}
			mu.Unlock()

			// プログレス通知
			bp.notifyProgress(total, completed, failed, startTime)
		}(i, req)
	}

	// すべてのワーカーが完了するまで待機
	wg.Wait()

	return results
}

// notifyProgress はプログレス状況を通知する
func (bp *BatchProcessor) notifyProgress(total, completed, failed int, startTime time.Time) {
	if bp.config.ProgressCallback == nil {
		return
	}

	elapsed := time.Since(startTime)

	// 推定残り時間を計算
	var eta time.Duration
	if completed > 0 {
		avgTimePerRequest := elapsed / time.Duration(completed)
		remaining := total - completed
		eta = avgTimePerRequest * time.Duration(remaining)
	}

	progress := BatchProgress{
		Total:                  total,
		Completed:              completed,
		Failed:                 failed,
		ElapsedTime:            elapsed,
		EstimatedTimeRemaining: eta,
	}

	bp.config.ProgressCallback(progress)
}

// BatchStats はバッチ処理の統計情報
type BatchStats struct {
	// TotalRequests は総リクエスト数
	TotalRequests int
	// SuccessCount は成功したリクエスト数
	SuccessCount int
	// FailureCount は失敗したリクエスト数
	FailureCount int
	// TotalDuration は総処理時間
	TotalDuration time.Duration
	// AverageDuration は平均処理時間
	AverageDuration time.Duration
	// MinDuration は最小処理時間
	MinDuration time.Duration
	// MaxDuration は最大処理時間
	MaxDuration time.Duration
	// FailedRequests は失敗したリクエストのIDリスト
	FailedRequests []string
}

// CalculateBatchStats はバッチ結果から統計情報を計算する
func CalculateBatchStats(results []BatchResult) BatchStats {
	stats := BatchStats{
		TotalRequests: len(results),
		MinDuration:   time.Duration(1<<63 - 1), // 最大値で初期化
	}

	var totalDuration time.Duration

	for _, result := range results {
		if result.Error != nil {
			stats.FailureCount++
			stats.FailedRequests = append(stats.FailedRequests, result.ID)
		} else {
			stats.SuccessCount++
			totalDuration += result.Duration

			if result.Duration < stats.MinDuration {
				stats.MinDuration = result.Duration
			}
			if result.Duration > stats.MaxDuration {
				stats.MaxDuration = result.Duration
			}
		}
	}

	if stats.SuccessCount > 0 {
		stats.AverageDuration = totalDuration / time.Duration(stats.SuccessCount)
	}

	if stats.MinDuration == time.Duration(1<<63-1) {
		stats.MinDuration = 0
	}

	stats.TotalDuration = totalDuration

	return stats
}

// String は統計情報を文字列表現で返す
func (s BatchStats) String() string {
	return fmt.Sprintf(
		"Batch Stats: Total=%d, Success=%d, Failed=%d, AvgDuration=%s, MinDuration=%s, MaxDuration=%s",
		s.TotalRequests,
		s.SuccessCount,
		s.FailureCount,
		s.AverageDuration.Round(time.Millisecond),
		s.MinDuration.Round(time.Millisecond),
		s.MaxDuration.Round(time.Millisecond),
	)
}
