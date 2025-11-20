# LLMクライアントパッケージ

このパッケージは、LLMサービス(OpenAI API等)とのやり取りを抽象化し、統一的なインターフェースを提供します。

## 概要

- **目的**: LLMを使用した要約生成、ドメイン分類、アクション生成等の機能を実装
- **Phase**: Phase 3 (LLMによる要約生成 + ドメイン分類)
- **設計書参照**: `docs/rag-ingestion-design.md` 9.1節

## 主要コンポーネント

### LLMClient インターフェース

すべてのLLM実装が満たすべきインターフェースです。

```go
type LLMClient interface {
    GenerateCompletion(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}
```

### OpenAIClient 実装

OpenAI APIを使用したLLMクライアント実装です。

**特徴:**
- gpt-4o-mini または gpt-4o をサポート
- レート制限エラー時のExponential Backoff
- JSON形式レスポンスの妥当性検証
- 不正JSONレスポンス時の自動リトライ(最大1回)
- タイムアウト処理

## 使用方法

### 基本的な使用例

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/jinford/dev-rag/pkg/indexer/llm"
)

func main() {
    // 環境変数 OPENAI_API_KEY からAPIキーを読み込む
    client, err := llm.NewOpenAIClient()
    if err != nil {
        log.Fatal(err)
    }

    // プロンプトを送信
    req := llm.CompletionRequest{
        Prompt:         "Go言語とは何ですか？",
        Temperature:    0.3,
        MaxTokens:      500,
        ResponseFormat: "text",
    }

    resp, err := client.GenerateCompletion(context.Background(), req)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Response: %s\n", resp.Content)
    fmt.Printf("Tokens used: %d\n", resp.TokensUsed)
}
```

### JSON形式のレスポンスを取得

```go
req := llm.CompletionRequest{
    Prompt: `以下のファイルを要約してください。

File: main.go
Content:
package main

func main() {
    fmt.Println("Hello, World!")
}

以下のJSON形式で返してください:
{
  "prompt_version": "1.1",
  "summary": ["項目1", "項目2"]
}`,
    Temperature:    0.3,
    MaxTokens:      400,
    ResponseFormat: "json",
}

resp, err := client.GenerateCompletion(ctx, req)
if err != nil {
    log.Fatal(err)
}

// JSONをパース
var result map[string]interface{}
json.Unmarshal([]byte(resp.Content), &result)
```

### カスタムモデルの使用

```go
// gpt-4o を使用
client, err := llm.NewOpenAIClientWithModel("gpt-4o")
if err != nil {
    log.Fatal(err)
}
```

### タイムアウトの設定

```go
client, _ := llm.NewOpenAIClient()
client.SetTimeout(30 * time.Second)
```

## エラーハンドリング

### レート制限エラー

レート制限エラー(HTTP 429)が発生した場合、自動的にExponential Backoffでリトライします。

- 最大リトライ回数: 3回
- バックオフ時間: 2秒, 4秒, 8秒 (最大32秒)

### JSON解析エラー

`ResponseFormat: "json"` を指定した場合、レスポンスが有効なJSONでない場合は最大1回リトライします。

### タイムアウトエラー

デフォルトのタイムアウトは60秒です。`SetTimeout()`で変更可能です。

### エラーログの記録 (Phase 3 タスク8)

LLM API呼び出しの失敗情報を構造化ログとして記録する機能を提供します。

#### ErrorHandler の初期化

```go
// グローバルエラーハンドラーを初期化
logDir := "logs/llm"
err := llm.InitGlobalErrorHandler(logDir)
if err != nil {
    log.Fatal(err)
}
defer llm.CloseGlobalErrorHandler()
```

#### エラーログの自動記録

各プロンプト実装（FileSummaryGenerator、ChunkSummaryGenerator、DomainClassifier）は、エラー発生時に自動的にログを記録します。

**記録される情報:**
- タイムスタンプ
- エラータイプ（JSON解析失敗、レート制限、タイムアウト、不明）
- プロンプトセクション（9.2, 9.3, 9.4等）
- 送信したプロンプト（最大5000文字）
- LLMのレスポンス（最大5000文字）
- エラーメッセージ
- リトライ回数

**ログファイル:**
- 形式: `llm_errors_YYYY-MM-DD.jsonl`（JSON Lines形式）
- 場所: 指定したログディレクトリ
- ローテーション: 日次

**ログファイルの例:**
```json
{"timestamp":"2025-11-20T17:51:01Z","error_type":"parse_failed","prompt_section":"9.2","prompt":"Summarize the following file...","response":"{invalid json","error_message":"failed to parse JSON response","retry_count":1}
```

#### エラーレスポンスの生成

JSON解析が2回失敗した場合、以下の形式のエラーレスポンスを返します：

```json
{
  "error": "parse_failed",
  "prompt_section": "9.2"
}
```

#### フォールバック戦略

**ファイルサマリー生成失敗時:**
- ルールベース版（`pkg/indexer/chunker/file_summarizer.go`）にフォールバック
- 呼び出し側でエラーをキャッチし、ルールベース版を実行

**チャンク要約生成失敗時:**
- 要約なしで処理を継続
- 呼び出し側でエラーを無視し、元のチャンク内容を使用

**ドメイン分類失敗時:**
- ルールベース結果を使用
- 呼び出し側でエラーをキャッチし、ルールベース分類を使用

#### エラーハンドリングの無効化

ログディレクトリに空文字列を指定すると、エラーログ機能を無効化できます：

```go
// エラーログを無効化
err := llm.InitGlobalErrorHandler("")
```

## テスト

### 単体テストの実行

```bash
go test -v ./pkg/indexer/llm/
```

### モックの使用

テストでは`MockLLMClient`を使用できます。

```go
mock := &llm.MockLLMClient{
    GenerateCompletionFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
        return llm.CompletionResponse{
            Content:    `{"summary": ["test"]}`,
            TokensUsed: 10,
            Model:      "mock-model",
        }, nil
    },
}

resp, err := mock.GenerateCompletion(context.Background(), req)
```

### 統合テスト

実際のOpenAI APIを呼び出す統合テストは、環境変数 `INTEGRATION_TEST=true` を設定した場合のみ実行されます。

```bash
export OPENAI_API_KEY="your-api-key"
export INTEGRATION_TEST="true"
go test -v ./pkg/indexer/llm/
```

## 環境変数

| 変数名 | 説明 | 必須 |
|--------|------|------|
| `OPENAI_API_KEY` | OpenAI APIキー | はい |

## 実装の詳細

### Exponential Backoff

レート制限エラー時は以下のロジックでリトライします:

```
試行1: 即座に実行
試行2: 2秒待機
試行3: 4秒待機
試行4: 8秒待機
```

最大待機時間は32秒です。

### JSON妥当性検証

`ResponseFormat: "json"` が指定された場合、以下の検証を行います:

1. レスポンスを`json.Unmarshal()`でパース
2. パースに失敗した場合は1回リトライ
3. 2回目も失敗した場合は`ErrInvalidResponseFormat`を返す

## トークンカウントとコスト管理 (Phase 3 タスク2)

### TokenCounter

`tiktoken`の`cl100k_base`エンコーディングを使用してトークン数を正確にカウントします。

```go
counter, err := llm.NewTokenCounter()
if err != nil {
    log.Fatal(err)
}

// テキストのトークン数をカウント
tokens := counter.CountTokens("Hello, World!")
fmt.Printf("Tokens: %d\n", tokens)

// プロンプトとレスポンスのトークン数を分けてカウント
usage := counter.CountPromptAndResponse(prompt, response)
fmt.Printf("Prompt tokens: %d\n", usage.PromptTokens)
fmt.Printf("Response tokens: %d\n", usage.ResponseTokens)
fmt.Printf("Total tokens: %d\n", usage.TotalTokens)
```

### CostManager

モデルごとの価格設定を管理し、APIコストを計算します。

```go
// 設定ファイルから価格情報を読み込む
cm, err := llm.NewCostManager("config/llm_pricing.yaml")
if err != nil {
    log.Fatal(err)
}

// トークン使用量からコストを計算
usage := llm.TokenUsage{
    PromptTokens:   1000,
    ResponseTokens: 500,
    TotalTokens:    1500,
}
cost, err := cm.CalculateCost("gpt-4o-mini", usage)
fmt.Printf("Cost: $%.4f\n", cost)

// 使用量を記録
err = cm.RecordUsage("gpt-4o-mini", usage, "summarization")
if err != nil {
    log.Fatal(err)
}

// 統計情報を取得
totalCost := cm.GetTotalCost()
costsByModel := cm.GetCostsByModel()
requestCount := cm.GetRequestCount()
```

**価格設定ファイル** (`config/llm_pricing.yaml`):

```yaml
models:
  gpt-4o-mini:
    input_price_per_1k_tokens: 0.00015
    output_price_per_1k_tokens: 0.0006
    provider: openai
    description: "GPT-4o mini - Cost-efficient small model"

default_model: gpt-4o-mini

cost_limits:
  daily_max_cost: 10.0
  warning_threshold: 5.0
  enable_alerts: true
```

### RateLimiter

トークンバケットアルゴリズムとセマフォを使用してレート制限を実装します。

```go
// 1分あたり10リクエストまでに制限
limiter := llm.NewRateLimiter(10)

// レート制限に従って待機
ctx := context.Background()
err := limiter.Wait(ctx)
if err != nil {
    log.Fatal(err)
}
defer limiter.Release()

// API呼び出しを実行
// ...
```

**ThrottledLLMClient** - レート制限付きのLLMクライアント:

```go
// 通常のLLMクライアントをレート制限付きでラップ
baseClient, _ := llm.NewOpenAIClient()
throttledClient := llm.NewThrottledLLMClient(baseClient, 10) // 10 req/min

// 通常通り使用するだけでレート制限が適用される
resp, err := throttledClient.GenerateCompletion(ctx, req)

// レート制限の状態を確認
status := throttledClient.GetRateLimiterStatus()
fmt.Printf("Available tokens: %d\n", status.AvailableTokens)
```

### LLMMetrics

LLM APIの使用状況メトリクスを記録・集計します。

```go
metrics := llm.NewLLMMetrics()

// リクエストのメトリクスを記録
metric := llm.RequestMetric{
    Model:       "gpt-4o-mini",
    RequestType: "summarization",
    Usage:       usage,
    Cost:        0.00045,
    Latency:     2 * time.Second,
    Success:     true,
}
metrics.RecordRequest(metric)

// メトリクスのスナップショットを取得
snapshot := metrics.GetSnapshot()
fmt.Printf("Total API requests: %d\n", snapshot.TotalAPIRequests)
fmt.Printf("Success rate: %.2f%%\n", snapshot.SuccessRate)
fmt.Printf("Total cost: $%.4f\n", snapshot.TotalCost)

// JSON形式でエクスポート
data, err := metrics.ExportJSON()
ioutil.WriteFile("metrics.json", data, 0644)

// サマリーを表示
metrics.PrintSummary()
```

**メトリクス内容:**
- APIコール回数 (成功/失敗/リトライ)
- トークン使用量 (プロンプト/レスポンス/合計)
- コスト (総コスト、モデル別、リクエストタイプ別)
- レイテンシ (平均、P50、P95、P99)
- エラー統計

## プロンプトバージョン管理 (Phase 3 タスク6)

### 概要

すべてのLLMプロンプトにバージョン番号を付与し、プロンプトの変更履歴を追跡可能にします。バージョン管理により、プロンプトの更新時に互換性を確認し、古いバージョンの応答も一定期間サポートできます。

### PromptVersionRegistry

プロンプトバージョンを一元管理するレジストリです。

```go
// デフォルトレジストリを使用
version, err := llm.DefaultPromptVersionRegistry.GetVersion(llm.PromptTypeFileSummary)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Current version: %s\n", version)

// バージョン検証
valid := llm.DefaultPromptVersionRegistry.ValidateVersion(
    llm.PromptTypeFileSummary,
    response.PromptVersion,
)
if !valid {
    // バージョン不一致が検出された場合、警告ログが出力される
    log.Printf("[WARN] Version mismatch detected")
}

// バージョン互換性チェック
compatible := llm.DefaultPromptVersionRegistry.IsCompatible(
    llm.PromptTypeFileSummary,
    "1.0", // 古いバージョン
)
fmt.Printf("Compatible: %v\n", compatible) // メジャーバージョンが同じなら true
```

### プロンプトタイプ

以下のプロンプトタイプが定義されています：

```go
const (
    PromptTypeFileSummary          PromptType = "file_summary"
    PromptTypeChunkSummary         PromptType = "chunk_summary"
    PromptTypeDomainClassification PromptType = "domain_classification"
    PromptTypeActionGeneration     PromptType = "action_generation"
)
```

### バージョン互換性ルール

- **メジャーバージョン**（最初の数字）が同じなら互換性あり
- 例: 1.0, 1.1, 1.2 は互換性あり、2.0 は互換性なし

### バージョン更新手順

プロンプトを変更した場合は、以下の手順でバージョンを更新します：

1. **プロンプト定数のバージョンを更新**

```go
const (
    FileSummaryPromptVersion = "1.2"  // 1.1 から 1.2 に更新
)
```

2. **レジストリのデフォルトバージョンを更新**

```go
func NewPromptVersionRegistry() *PromptVersionRegistry {
    return &PromptVersionRegistry{
        versions: map[PromptType]string{
            PromptTypeFileSummary: "1.2",  // 更新
            // ...
        },
    }
}
```

3. **プロンプトテンプレート内のバージョン番号を更新**

```go
Return a JSON response with the following structure:
{
  "prompt_version": "1.2",  // 更新
  "summary": ["item1", "item2", ...],
  ...
}
```

### ログ出力

バージョン不一致が検出されると、以下のようなログが出力されます：

```
[WARN] Prompt version mismatch for file_summary: expected=1.2, received=1.1
```

バージョンを更新すると、以下のログが出力されます：

```
[INFO] Updated prompt version for file_summary: 1.1 -> 1.2
```

### テスト

```bash
go test ./pkg/indexer/llm -v -run TestVersion
```

詳細は `pkg/indexer/llm/prompts/README.md` を参照してください。

## 今後の拡張

Phase 3の他のタスクで以下の機能を追加予定:

- バッチ処理管理 (タスク9)

## 参考資料

- [設計書 9.1節: 運用ガイドライン](../../docs/rag-ingestion-design.md#91-運用ガイドライン)
- [Phase 3タスク一覧](../../tasks/phase3-llm-summarization.md)
- [OpenAI API Documentation](https://platform.openai.com/docs/api-reference/chat)
