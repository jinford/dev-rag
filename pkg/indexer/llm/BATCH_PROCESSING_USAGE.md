# バッチ処理機能の使用方法

## 概要

LLMバッチ処理機能は、複数のLLMリクエストを並列実行し、プログレス表示とエラーハンドリングを提供します。

## 主要コンポーネント

### 1. BatchProcessor

複数のLLMリクエストを並列実行するコアコンポーネント。

```go
// BatchProcessorの作成
processor := llm.NewBatchProcessor(client, llm.BatchProcessorConfig{
    MaxConcurrency: 10, // 最大同時実行数
    ProgressCallback: func(progress llm.BatchProgress) {
        log.Printf("Progress: %s", progress.String())
    },
    ErrorHandler: errorHandler,
    PromptSection: llm.PromptSectionFileSummary,
})

// リクエストの作成
requests := []llm.BatchRequest{
    {
        ID: "file1",
        Request: llm.CompletionRequest{
            Prompt: "Summarize this file...",
            Temperature: 0.3,
            MaxTokens: 400,
        },
    },
    // ... more requests
}

// バッチ処理の実行
ctx := context.Background()
results := processor.ProcessBatch(ctx, requests)

// 統計情報の計算
stats := llm.CalculateBatchStats(results)
log.Printf("Stats: %s", stats.String())
```

### 2. BatchProcessorWithProgress

プログレス表示機能を統合したバッチプロセッサー。

```go
// プログレスロガーの作成
logger := llm.NewProgressLogger(
    2*time.Second,  // ログ出力間隔
    true,           // コンソール出力を有効化
    true,           // 詳細ログを有効化
)

// プログレス表示付きプロセッサーの作成
processor := llm.NewBatchProcessorWithProgress(
    client,
    llm.BatchProcessorConfig{
        MaxConcurrency: 10,
    },
    logger,
    true, // プログレスバーを表示
)

// バッチ処理の実行
results, stats := processor.ProcessBatch(ctx, requests)
```

### 3. ProgressLogger

プログレス情報のロギングを管理。

```go
// ロガーの作成
logger := llm.NewProgressLogger(
    1*time.Second,  // ログ出力間隔
    true,           // コンソール出力
    false,          // 詳細ログは無効
)

// プログレスの記録（BatchProcessorから自動的に呼ばれる）
logger.LogProgress(progress)

// 最終結果の記録
logger.LogFinal(progress, stats)
```

### 4. ProgressTracker

手動でプログレスを追跡する場合に使用。

```go
tracker := llm.NewProgressTracker(totalRequests, logger)

// リクエスト完了時に呼ぶ
tracker.OnComplete(true)  // 成功
tracker.OnComplete(false) // 失敗

// 現在の進捗を取得
progress := tracker.GetProgress()
```

## 使用例

### 例1: ファイルサマリーの一括生成

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/jinford/dev-rag/pkg/indexer/llm"
)

func generateFileSummaries(client llm.LLMClient, files []string) {
    // プログレスロガーの作成
    logger := llm.NewProgressLogger(2*time.Second, true, true)

    // プロセッサーの作成
    processor := llm.NewBatchProcessorWithProgress(
        client,
        llm.BatchProcessorConfig{
            MaxConcurrency: 10,
            PromptSection: llm.PromptSectionFileSummary,
        },
        logger,
        true, // プログレスバーを表示
    )

    // リクエストの準備
    requests := make([]llm.BatchRequest, len(files))
    for i, file := range files {
        requests[i] = llm.BatchRequest{
            ID: file,
            Request: llm.CompletionRequest{
                Prompt: fmt.Sprintf("Summarize this file: %s", file),
                Temperature: 0.3,
                MaxTokens: 400,
                ResponseFormat: "json",
            },
        }
    }

    // バッチ処理の実行
    ctx := context.Background()
    results, stats := processor.ProcessBatch(ctx, requests)

    // 結果の処理
    for _, result := range results {
        if result.Error != nil {
            log.Printf("Failed to process %s: %v", result.ID, result.Error)
            continue
        }

        fmt.Printf("Summary for %s: %s\n", result.ID, result.Response.Content)
    }

    // 失敗したファイルのリスト
    if len(stats.FailedRequests) > 0 {
        log.Printf("Failed files: %v", stats.FailedRequests)
    }
}
```

### 例2: チャンク要約の並列生成

```go
func generateChunkSummaries(client llm.LLMClient, chunks []Chunk) {
    // レート制限付きクライアントを作成
    throttledClient := llm.NewThrottledLLMClient(client, 10)

    // プロセッサーの作成
    processor := llm.NewBatchProcessor(throttledClient, llm.BatchProcessorConfig{
        MaxConcurrency: 10,
        ProgressCallback: func(progress llm.BatchProgress) {
            if progress.Completed%100 == 0 {
                log.Printf("Processed %d/%d chunks", progress.Completed, progress.Total)
            }
        },
        PromptSection: llm.PromptSectionChunkSummary,
    })

    // リクエストの準備
    requests := make([]llm.BatchRequest, len(chunks))
    for i, chunk := range chunks {
        requests[i] = llm.BatchRequest{
            ID: chunk.ID,
            Request: llm.CompletionRequest{
                Prompt: fmt.Sprintf("Summarize: %s", chunk.Content),
                Temperature: 0.2,
                MaxTokens: 80,
            },
        }
    }

    // コンテキストのタイムアウト設定
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()

    // バッチ処理の実行
    results := processor.ProcessBatch(ctx, requests)

    // 統計情報の出力
    stats := llm.CalculateBatchStats(results)
    log.Printf("Completed: Total=%d, Success=%d, Failed=%d, AvgDuration=%s",
        stats.TotalRequests,
        stats.SuccessCount,
        stats.FailureCount,
        stats.AverageDuration,
    )
}
```

### 例3: エラーハンドリング付きのドメイン分類

```go
func classifyDomains(client llm.LLMClient, errorHandler *llm.ErrorHandler, nodes []Node) {
    processor := llm.NewBatchProcessor(client, llm.BatchProcessorConfig{
        MaxConcurrency: 10,
        ErrorHandler: errorHandler,
        PromptSection: llm.PromptSectionDomainClassification,
    })

    requests := make([]llm.BatchRequest, len(nodes))
    for i, node := range nodes {
        requests[i] = llm.BatchRequest{
            ID: node.Path,
            Request: llm.CompletionRequest{
                Prompt: buildDomainClassificationPrompt(node),
                Temperature: 0.0,
                MaxTokens: 150,
                ResponseFormat: "json",
            },
        }
    }

    ctx := context.Background()
    results := processor.ProcessBatch(ctx, requests)

    // 結果の処理
    for _, result := range results {
        if result.Error != nil {
            // エラーは既にErrorHandlerに記録されている
            log.Printf("Using fallback for %s", result.ID)
            // ルールベースのフォールバック処理
            continue
        }

        // 成功した場合の処理
        // ...
    }
}
```

## 設定パラメータ

### BatchProcessorConfig

| フィールド | 型 | 説明 | デフォルト値 |
|-----------|------|------|------------|
| MaxConcurrency | int | 最大同時実行数 | 10 |
| ProgressCallback | func | プログレス更新コールバック | nil |
| ErrorHandler | *ErrorHandler | エラーハンドラー | nil |
| PromptSection | PromptSection | プロンプトセクション | "" |

### ProgressLogger設定

| パラメータ | 型 | 説明 |
|----------|------|------|
| interval | time.Duration | ログ出力間隔 |
| enableConsole | bool | コンソール出力を有効化 |
| enableDetailed | bool | 詳細ログを有効化 |

## パフォーマンス最適化

### 並列度の調整

```go
// CPU負荷が高い場合は並列度を下げる
config := llm.BatchProcessorConfig{
    MaxConcurrency: 5,
}

// レート制限に余裕がある場合は並列度を上げる
config := llm.BatchProcessorConfig{
    MaxConcurrency: 20,
}
```

### レート制限との組み合わせ

```go
// ThrottledLLMClientを使用してレート制限を適用
throttledClient := llm.NewThrottledLLMClient(client, 10) // 10 req/min

// BatchProcessorで並列度を制御
processor := llm.NewBatchProcessor(throttledClient, llm.BatchProcessorConfig{
    MaxConcurrency: 10,
})
```

### コンテキストタイムアウト

```go
// 長時間実行される処理にはタイムアウトを設定
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
defer cancel()

results := processor.ProcessBatch(ctx, requests)
```

## エラーハンドリング

### 部分的な失敗の処理

```go
results := processor.ProcessBatch(ctx, requests)

// 成功した結果のみを処理
for _, result := range results {
    if result.Error == nil {
        // 成功した結果の処理
        processResult(result)
    }
}

// 失敗した結果のリトライ
failedRequests := []llm.BatchRequest{}
for _, result := range results {
    if result.Error != nil {
        // 元のリクエストを再度実行
        failedRequests = append(failedRequests, findOriginalRequest(result.ID))
    }
}

if len(failedRequests) > 0 {
    log.Printf("Retrying %d failed requests", len(failedRequests))
    retryResults := processor.ProcessBatch(ctx, failedRequests)
    // ...
}
```

### タイムアウトの検出

```go
import "errors"

for _, result := range results {
    if errors.Is(result.Error, context.DeadlineExceeded) {
        log.Printf("Request %s timed out", result.ID)
        // タイムアウト時の特別な処理
    }
}
```

## モニタリング

### 進捗状況の監視

```go
config := llm.BatchProcessorConfig{
    ProgressCallback: func(progress llm.BatchProgress) {
        // メトリクスサーバーに送信
        metrics.RecordBatchProgress(
            progress.Completed,
            progress.Failed,
            progress.ElapsedTime,
        )

        // アラートの発行
        if float64(progress.Failed)/float64(progress.Completed) > 0.1 {
            alerts.Send("High failure rate in batch processing")
        }
    },
}
```

### 統計情報の記録

```go
stats := llm.CalculateBatchStats(results)

// メトリクスの記録
metrics.RecordBatchStats(map[string]interface{}{
    "total_requests":   stats.TotalRequests,
    "success_count":    stats.SuccessCount,
    "failure_count":    stats.FailureCount,
    "avg_duration_ms":  stats.AverageDuration.Milliseconds(),
    "min_duration_ms":  stats.MinDuration.Milliseconds(),
    "max_duration_ms":  stats.MaxDuration.Milliseconds(),
})
```

## ベストプラクティス

1. **適切な並列度の設定**: API providerのレート制限に応じて調整
2. **タイムアウトの設定**: 長時間実行される処理には必ずタイムアウトを設定
3. **エラーハンドリング**: 部分的な失敗を許容し、失敗したリクエストを記録
4. **プログレス表示**: 長時間実行される処理には進捗表示を有効化
5. **リトライロジック**: 一時的なエラーに対してはリトライを実装
6. **メトリクスの記録**: 統計情報を記録して後で分析できるようにする

## トラブルシューティング

### 問題: リクエストが遅い

- MaxConcurrencyを増やす
- ネットワーク接続を確認
- LLM APIのレスポンス時間を確認

### 問題: 頻繁にタイムアウトする

- タイムアウト時間を延長
- MaxConcurrencyを減らす
- MaxTokensを減らしてプロンプトを最適化

### 問題: 高いエラー率

- ErrorHandlerのログを確認
- API認証情報を確認
- レート制限を確認
- プロンプトの形式を確認
