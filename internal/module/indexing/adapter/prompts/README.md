# LLMプロンプト実装

このパッケージは、Phase 3で実装されたLLMプロンプト生成とレスポンス処理機能を提供します。

## 概要

RAGシステムのインデックス化プロセスにおいて、LLMを活用して以下の機能を実現します：

1. **ファイルサマリー生成** - ファイル全体の要約を生成し、階層的チャンキングのレベル1（親チャンク）として使用
2. **チャンク要約生成** - 個別チャンクの要約を生成し、Embedding精度を向上
3. **ドメイン自動分類** - ファイル/ディレクトリを5つのドメイン（code、architecture、ops、tests、infra）に自動分類

## ドメイン自動分類

### 概要

ファイルやディレクトリを以下の5つのドメインに自動分類します：

- **code**: アプリケーションコード、ライブラリコード
- **architecture**: 設計文書、ADR、アーキテクチャ図
- **ops**: CI/CD設定、監視設定、運用Runbook
- **tests**: テストコード、テストデータ
- **infra**: インフラ定義（Terraform、Kubernetes YAML、Helm charts）

### 実装ファイル

- `domain_classification.go` - ドメイン分類のプロンプト生成とLLM呼び出し
- `domain_classification_test.go` - 単体テスト
- `domain_classification_integration_test.go` - 実際のLLM APIを使用した統合テスト

### 使用方法

```go
package main

import (
    "context"
    "fmt"

    "github.com/jinford/dev-rag/pkg/indexer/llm"
    "github.com/jinford/dev-rag/pkg/indexer/llm/prompts"
)

func main() {
    // LLMクライアントを作成
    llmClient, err := llm.NewOpenAIClientWithModel("gpt-4o-mini")
    if err != nil {
        panic(err)
    }

    // TokenCounterを作成
    tokenCounter, err := llm.NewTokenCounter()
    if err != nil {
        panic(err)
    }

    // ドメイン分類器を作成
    classifier := prompts.NewDomainClassifier(llmClient, tokenCounter)

    // ドメイン分類リクエストを構築
    req := prompts.DomainClassificationRequest{
        NodePath:         "pkg/indexer/indexer_test.go",
        NodeType:         "file",
        DetectedLanguage: "Go",
        LinesOfCode:      200,
        SampleLines:      "L1: package indexer\nL2: import \"testing\"\n...",
        DirectoryHints: &prompts.DirectoryHint{
            Pattern:         "*_test.go",
            SuggestedDomain: "tests",
        },
    }

    // LLM分類を実行
    resp, err := classifier.Classify(context.Background(), req)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Domain: %s (confidence: %.2f)\n", resp.Domain, resp.Confidence)
    fmt.Printf("Rationale: %s\n", resp.Rationale)
}
```

### Indexerとの統合

Indexerにドメイン分類器を設定することで、インデックス化プロセスでLLM分類を使用できます：

```go
// Indexerを作成
indexer, err := indexer.NewIndexer(...)
if err != nil {
    panic(err)
}

// LLMドメイン分類器を設定
llmClient, _ := llm.NewOpenAIClientWithModel("gpt-4o-mini")
tokenCounter, _ := llm.NewTokenCounter()
domainClassifier := prompts.NewDomainClassifier(llmClient, tokenCounter)

indexer.SetDomainClassifier(domainClassifier)

// インデックス化を実行すると、LLM分類が使用される
result, err := indexer.IndexSource(ctx, models.SourceTypeGit, params)
```

### ルールベース分類との統合

LLM分類は以下のロジックでルールベース分類と統合されます：

1. **ルールベース分類を実行** - パス、ファイル名、拡張子からドメインを推定
2. **ディレクトリヒントを生成** - ルールベース結果をLLMへのヒントとして使用
3. **LLM分類を実行** - ファイル内容とヒントを元にLLMが分類
4. **信頼度チェック** - LLMの信頼度が閾値（0.5）未満の場合はルールベース結果にフォールバック

### プロンプト仕様

#### システムプロンプト

```
You are a file classification assistant.

Your task is to classify files and directories into one of these domains:
- code: Application code, library code
- architecture: Design documents, ADRs, architecture diagrams
- ops: CI/CD config, monitoring config, runbooks
- tests: Test code, test data
- infra: Infrastructure definitions (Terraform, Kubernetes YAML, Helm charts)

Guidelines:
- Always return exactly one domain
- If information is insufficient, fall back to "code"
- Provide a rationale in 2 sentences or less, citing line numbers when relevant (e.g., L12)
- Consider directory hints when available
- Return a valid JSON response
```

#### ユーザープロンプト

```
Classify the following file or directory into one of these domains:
- code: Application code, library code
- architecture: Design documents, ADRs, architecture diagrams
- ops: CI/CD config, monitoring config, runbooks
- tests: Test code, test data
- infra: Infrastructure definitions (Terraform, Kubernetes YAML, Helm charts)

Path: pkg/indexer/indexer_test.go
Type: file
Language: Go
Lines of Code: 200
Last Modified: 2024-01-15

Sample Lines:
L1: package indexer
L2: import "testing"
...

Directory Hints: {"pattern":"*_test.go","suggested_domain":"tests"}

Additional classification hints:
- tests: *_test.*, *.spec.*, /tests/ directory
- architecture: docs/adr/, docs/design/, docs/decisions/
- ops: .github/workflows/, ci/, monitoring/
- infra: infra/, terraform/, k8s/, helm/

Return a JSON response with the following structure:
{
  "prompt_version": "1.1",
  "domain": "code",
  "rationale": "...",
  "confidence": 0.81
}
```

#### レスポンス形式

```json
{
  "prompt_version": "1.1",
  "domain": "tests",
  "rationale": "File name contains _test.go pattern, indicating test code (L1).",
  "confidence": 0.95
}
```

### 温度設定

ドメイン分類では完全に決定論的な結果を得るため、温度を `0.0` に設定しています。

### サンプル行抽出

`ExtractSampleLines` 関数は、ファイルの先頭25行と末尾25行を抽出します：

```go
content := "line1\nline2\n...\nline100"
sampleLines := prompts.ExtractSampleLines(content, 25)
```

抽出されたサンプル行には行番号が付与されます：

```
L1: line1
L2: line2
L3: line3
...
L25: line25

... (omitted) ...

L76: line76
...
L100: line100
```

### テスト

#### 単体テスト

```bash
go test ./pkg/indexer/llm/prompts -v
```

#### 統合テスト（実際のLLM APIを使用）

環境変数 `OPENAI_API_KEY` を設定した状態で実行：

```bash
export OPENAI_API_KEY="your-api-key"
go test ./pkg/indexer/llm/prompts -v -run TestDomainClassifier_Integration
```

### エラーハンドリング

- **LLM呼び出し失敗** - ルールベース結果にフォールバック
- **信頼度が低い（< 0.5）** - ルールベース結果にフォールバック
- **不正なドメイン** - エラーを返す
- **JSON解析失敗** - エラーを返す（OpenAIClientで最大1回リトライ）

### パフォーマンス

- **温度**: 0.0（完全に決定論的）
- **最大トークン数**: 300
- **平均トークン数**: 450トークン（プロンプト + レスポンス）
- **推奨レート制限**: 1分あたり最大10リクエスト

## ファイルサマリー生成

### 概要

ファイル全体を要約し、階層的チャンキングのレベル1（親チャンク）として使用します。

### 実装ファイル

- `file_summary.go` - ファイルサマリー生成のプロンプト生成とLLM呼び出し
- `file_summary_test.go` - 単体テスト
- `file_summary_integration_test.go` - 統合テスト

### 温度設定

ファイルサマリー生成では、安定性を重視しつつある程度の表現の幅を持たせるため、温度を `0.3` に設定しています。

## プロンプトバージョン管理

### 概要

すべてのLLMプロンプトにはバージョン番号が付与され、プロンプトの変更履歴を追跡可能にしています。これにより、プロンプトの更新時に互換性を確認し、古いバージョンの応答も一定期間サポートできます。

### バージョン管理の仕組み

1. **プロンプトバージョンフィールド** - すべてのLLM応答に `prompt_version` フィールドを含める（初期値: "1.1"）
2. **バージョン互換性チェック** - レスポンスの `prompt_version` を検証し、期待するバージョンと異なる場合はログ警告
3. **バージョン更新時のマイグレーション** - プロンプトを更新した場合はバージョンを上げる仕組み
4. **一元管理** - `PromptVersionRegistry` でバージョンを一元管理

### 使用方法

#### バージョン取得

```go
import "github.com/jinford/dev-rag/pkg/indexer/llm"

// デフォルトレジストリからバージョンを取得
version, err := llm.DefaultPromptVersionRegistry.GetVersion(llm.PromptTypeFileSummary)
if err != nil {
    log.Printf("Failed to get version: %v", err)
}
fmt.Printf("Current version: %s\n", version)
```

#### バージョン検証

```go
// LLM応答のバージョンを検証
valid := llm.DefaultPromptVersionRegistry.ValidateVersion(
    llm.PromptTypeFileSummary,
    response.PromptVersion,
)
if !valid {
    log.Printf("[WARN] Version mismatch detected")
}
```

#### バージョン更新

プロンプトを変更した場合は、以下の手順でバージョンを更新します：

1. プロンプト定数のバージョンを更新

```go
const (
    // FileSummaryPromptVersion はファイルサマリープロンプトのバージョン
    FileSummaryPromptVersion = "1.2"  // 1.1 から 1.2 に更新
)
```

2. `version.go` のデフォルトバージョンを更新

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

3. プロンプトテンプレート内のバージョン番号を更新

```go
Return a JSON response with the following structure:
{
  "prompt_version": "1.2",  // 更新
  "summary": ["item1", "item2", ...],
  ...
}
```

#### バージョン互換性

バージョン互換性のルール：

- **メジャーバージョン**（最初の数字）が同じなら互換性あり
- 例: 1.0, 1.1, 1.2 は互換性あり、2.0 は互換性なし

```go
// 古いバージョン1.0の応答が現在のバージョン1.1と互換性があるか確認
compatible := llm.DefaultPromptVersionRegistry.IsCompatible(
    llm.PromptTypeFileSummary,
    "1.0",
)
fmt.Printf("Compatible: %v\n", compatible)  // true
```

### プロンプトタイプ

以下のプロンプトタイプが定義されています：

- `PromptTypeFileSummary` - ファイルサマリー生成プロンプト
- `PromptTypeChunkSummary` - チャンク要約生成プロンプト
- `PromptTypeDomainClassification` - ドメイン分類プロンプト
- `PromptTypeActionGeneration` - アクション生成プロンプト（Phase 4で使用）

### すべてのバージョンを取得

```go
allVersions := llm.DefaultPromptVersionRegistry.GetAllVersions()
for promptType, version := range allVersions {
    fmt.Printf("%s: %s\n", promptType, version)
}
```

### ログ出力

バージョン不一致が検出されると、以下のようなログが出力されます：

```
[WARN] Prompt version mismatch for file_summary: expected=1.2, received=1.1
```

### テスト

バージョン管理のテストは `version_test.go` で実装されています：

```bash
go test ./pkg/indexer/llm -v -run TestVersion
```

## チャンク要約生成（Phase 3タスク4対応）

### 概要

個別チャンクの要約を80トークン以内で生成し、Embedding用テキストの冒頭に追加します。これにより、Embedding精度を向上させます。

### 実装ファイル

- `chunk_summary.go` - チャンク要約生成のプロンプト生成とLLM呼び出し
- `chunk_summary_test.go` - 単体テスト

### 温度設定

チャンク要約生成では、より決定論的な結果を得るため、温度を `0.2` に設定しています。

### 使用方法

```go
import (
    "context"
    "github.com/jinford/dev-rag/pkg/indexer/llm"
    "github.com/jinford/dev-rag/pkg/indexer/llm/prompts"
)

// LLMクライアントとトークンカウンターを作成
llmClient, _ := llm.NewOpenAIClientWithModel("gpt-4o-mini")
tokenCounter, _ := llm.NewTokenCounter()

// チャンク要約ジェネレーターを作成
generator := prompts.NewChunkSummaryGenerator(llmClient, tokenCounter)

// チャンク要約リクエストを構築
req := prompts.ChunkSummaryRequest{
    ChunkID:       "src/indexer/indexer.go#L10-L68",
    ParentContext: "HTTPサーバ起動処理とインデクサ初期化の流れを記述",
    ChunkContent:  "func IndexSource(...) { ... }",
}

// チャンク要約を生成
resp, err := generator.Generate(context.Background(), req)
if err != nil {
    panic(err)
}

fmt.Printf("Summary: %s\n", resp.SummarySentence)
fmt.Printf("Focus Entities: %v\n", resp.FocusEntities)
fmt.Printf("Confidence: %.2f\n", resp.Confidence)

// Embedding用コンテキストを構築
embeddingText := prompts.BuildEmbeddingContext(resp, req.ChunkContent)
```

### 信頼度の目安

- **0.75**: 明確な入出力や副作用が特定できる
- **0.55**: 記述が抽象的/情報不足
- **0.35以下**: 親文脈と矛盾する可能性

### 対象チャンクの選定

コスト削減のため、重要度スコアが閾値（例: 0.6）を超えるチャンクのみを対象とすることを推奨します。

## 参考資料

- [Phase 3タスク一覧](../../../../tasks/phase3-llm-summarization.md)
- [RAGインジェスト設計書 9.4節](../../../../docs/rag-ingestion-design.md#94-ドメイン自動分類プロンプト)
- [LLM運用ガイドライン](../../../../docs/rag-ingestion-design.md#91-運用ガイドライン)
