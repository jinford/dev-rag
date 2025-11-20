package llm

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ProgressLogger はプログレス情報をログに出力する
type ProgressLogger struct {
	mu             sync.Mutex
	interval       time.Duration
	lastLogTime    time.Time
	enableConsole  bool
	enableDetailed bool
}

// NewProgressLogger は新しいProgressLoggerを作成する
func NewProgressLogger(interval time.Duration, enableConsole, enableDetailed bool) *ProgressLogger {
	return &ProgressLogger{
		interval:       interval,
		lastLogTime:    time.Now(),
		enableConsole:  enableConsole,
		enableDetailed: enableDetailed,
	}
}

// LogProgress はプログレス情報をログに出力する
// intervalで指定した間隔でのみ出力する（頻繁なログを防ぐ）
func (pl *ProgressLogger) LogProgress(progress BatchProgress) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	now := time.Now()
	if now.Sub(pl.lastLogTime) < pl.interval && progress.Completed != progress.Total {
		// インターバル内かつ未完了の場合はスキップ
		return
	}

	pl.lastLogTime = now

	if pl.enableConsole {
		log.Printf("[Batch Progress] %s", progress.String())
	}

	if pl.enableDetailed {
		pl.logDetailedProgress(progress)
	}
}

// logDetailedProgress は詳細なプログレス情報をログに出力する
func (pl *ProgressLogger) logDetailedProgress(progress BatchProgress) {
	successCount := progress.Completed - progress.Failed
	successRate := 0.0
	if progress.Completed > 0 {
		successRate = float64(successCount) / float64(progress.Completed) * 100
	}

	log.Printf("[Batch Details] Success: %d, Failed: %d, Success Rate: %.1f%%",
		successCount, progress.Failed, successRate)
}

// LogFinal は最終的な処理結果をログに出力する
func (pl *ProgressLogger) LogFinal(progress BatchProgress, stats BatchStats) {
	if !pl.enableConsole {
		return
	}

	log.Printf("[Batch Complete] %s", progress.String())
	log.Printf("[Batch Stats] %s", stats.String())

	if len(stats.FailedRequests) > 0 {
		log.Printf("[Batch Failures] Failed request IDs: %v", stats.FailedRequests)
	}
}

// ProgressTracker はバッチ処理の進捗を追跡する
type ProgressTracker struct {
	mu        sync.Mutex
	startTime time.Time
	logger    *ProgressLogger

	// 統計情報
	totalRequests     int
	completedRequests int
	failedRequests    int
}

// NewProgressTracker は新しいProgressTrackerを作成する
func NewProgressTracker(totalRequests int, logger *ProgressLogger) *ProgressTracker {
	return &ProgressTracker{
		startTime:     time.Now(),
		logger:        logger,
		totalRequests: totalRequests,
	}
}

// OnComplete はリクエスト完了時に呼ばれるコールバック
func (pt *ProgressTracker) OnComplete(success bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.completedRequests++
	if !success {
		pt.failedRequests++
	}

	// プログレス情報をログに出力
	if pt.logger != nil {
		progress := pt.getProgressLocked()
		pt.logger.LogProgress(progress)
	}
}

// GetProgress は現在の進捗状況を返す
func (pt *ProgressTracker) GetProgress() BatchProgress {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.getProgressLocked()
}

// getProgressLocked はロック取得済みの状態で進捗情報を返す
func (pt *ProgressTracker) getProgressLocked() BatchProgress {
	elapsed := time.Since(pt.startTime)
	var eta time.Duration

	if pt.completedRequests > 0 {
		avgTime := elapsed / time.Duration(pt.completedRequests)
		remaining := pt.totalRequests - pt.completedRequests
		eta = avgTime * time.Duration(remaining)
	}

	return BatchProgress{
		Total:                  pt.totalRequests,
		Completed:              pt.completedRequests,
		Failed:                 pt.failedRequests,
		ElapsedTime:            elapsed,
		EstimatedTimeRemaining: eta,
	}
}

// ProgressBar はシンプルなプログレスバーを表示する
type ProgressBar struct {
	total   int
	width   int
	prefix  string
	mu      sync.Mutex
	lastBar string
}

// NewProgressBar は新しいProgressBarを作成する
func NewProgressBar(total int, width int, prefix string) *ProgressBar {
	return &ProgressBar{
		total:  total,
		width:  width,
		prefix: prefix,
	}
}

// Update はプログレスバーを更新する
func (pb *ProgressBar) Update(completed int) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	percentage := 0.0
	if pb.total > 0 {
		percentage = float64(completed) / float64(pb.total)
	}

	filledWidth := int(float64(pb.width) * percentage)
	if filledWidth > pb.width {
		filledWidth = pb.width
	}

	bar := pb.prefix + " ["
	for i := 0; i < pb.width; i++ {
		if i < filledWidth {
			bar += "="
		} else if i == filledWidth {
			bar += ">"
		} else {
			bar += " "
		}
	}
	bar += fmt.Sprintf("] %d/%d (%.1f%%)", completed, pb.total, percentage*100)

	// 同じバーを繰り返し表示しない
	if bar != pb.lastBar {
		fmt.Printf("\r%s", bar)
		pb.lastBar = bar
	}

	// 完了時に改行
	if completed >= pb.total {
		fmt.Println()
	}
}

// BatchProcessorWithProgress はプログレス表示付きのバッチプロセッサー
type BatchProcessorWithProgress struct {
	processor   *BatchProcessor
	logger      *ProgressLogger
	showBar     bool
	barWidth    int
	barPrefix   string
}

// NewBatchProcessorWithProgress はプログレス表示付きのバッチプロセッサーを作成する
func NewBatchProcessorWithProgress(
	client LLMClient,
	config BatchProcessorConfig,
	logger *ProgressLogger,
	showBar bool,
) *BatchProcessorWithProgress {
	return &BatchProcessorWithProgress{
		processor:  NewBatchProcessor(client, config),
		logger:     logger,
		showBar:    showBar,
		barWidth:   50,
		barPrefix:  "Processing",
	}
}

// ProcessBatch はプログレス表示付きでバッチ処理を実行する
func (bp *BatchProcessorWithProgress) ProcessBatch(ctx context.Context, requests []BatchRequest) ([]BatchResult, BatchStats) {
	var bar *ProgressBar
	if bp.showBar {
		bar = NewProgressBar(len(requests), bp.barWidth, bp.barPrefix)
	}

	// プログレスコールバックを設定
	config := bp.processor.config
	originalCallback := config.ProgressCallback

	config.ProgressCallback = func(progress BatchProgress) {
		// オリジナルのコールバックを呼ぶ
		if originalCallback != nil {
			originalCallback(progress)
		}

		// ロガーに記録
		if bp.logger != nil {
			bp.logger.LogProgress(progress)
		}

		// プログレスバーを更新
		if bar != nil {
			bar.Update(progress.Completed)
		}
	}

	bp.processor.config = config

	// バッチ処理を実行
	results := bp.processor.ProcessBatch(ctx, requests)

	// 統計情報を計算
	stats := CalculateBatchStats(results)

	// 最終結果をログに出力
	if bp.logger != nil {
		progress := BatchProgress{
			Total:       len(requests),
			Completed:   len(results),
			Failed:      stats.FailureCount,
			ElapsedTime: stats.TotalDuration,
		}
		bp.logger.LogFinal(progress, stats)
	}

	return results, stats
}

// SetBarConfig はプログレスバーの設定を変更する
func (bp *BatchProcessorWithProgress) SetBarConfig(width int, prefix string) {
	bp.barWidth = width
	bp.barPrefix = prefix
}
