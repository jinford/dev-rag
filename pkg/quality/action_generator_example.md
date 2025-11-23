# アクション自動生成の使用例

このドキュメントでは、Phase 4のタスク4「アクション自動生成プロンプトの実装」の使用方法を説明します。

## 概要

`ActionGenerator` は週次レビューデータ（品質ノート、最新コミット情報、CODEOWNERS）を分析し、LLMを使用して優先度付きの改善アクションを自動生成します。

## 基本的な使用方法

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/jinford/dev-rag/pkg/indexer/llm"
    "github.com/jinford/dev-rag/pkg/quality"
)

func main() {
    // 1. LLMクライアントの作成
    llmClient, err := llm.NewOpenAIClient()
    if err != nil {
        panic(err)
    }

    // 2. ActionGeneratorの作成
    generator := quality.NewActionGenerator(llmClient)

    // 3. 週次レビューデータの準備
    data := quality.ActionGenerationData{
        WeekRange: "2024-01-15 to 2024-01-21",
        QualityNotes: []quality.WeeklyQualityNote{
            {
                NoteID:      "QN-001",
                Severity:    "critical",
                NoteText:    "ADR-005の旧バージョンが参照されている",
                LinkedFiles: []string{"docs/adr/ADR-005.md"},
                Reviewer:    "alice",
            },
            {
                NoteID:      "QN-002",
                Severity:    "high",
                NoteText:    "IndexSourceパラメータの説明が不足",
                LinkedFiles: []string{"pkg/indexer/README.md"},
                Reviewer:    "bob",
            },
        },
        RecentChanges: []quality.ActionRecentChange{
            {
                Hash:         "9f2d3b4",
                FilesChanged: []string{"docs/adr/ADR-005.md"},
                MergedAt:     time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC),
            },
        },
        CodeownersLookup: map[string]string{
            "docs/adr/ADR-005.md":   "architecture-team",
            "pkg/indexer/README.md": "indexer-team",
        },
    }

    // 4. アクション生成
    actions, err := generator.GenerateActions(context.Background(), data)
    if err != nil {
        panic(err)
    }

    // 5. 結果の表示
    fmt.Printf("生成されたアクション数: %d\n", len(actions))
    for i, action := range actions {
        fmt.Printf("\n[アクション %d]\n", i+1)
        fmt.Printf("  ID: %s\n", action.ActionID)
        fmt.Printf("  タイトル: %s\n", action.Title)
        fmt.Printf("  優先度: %s\n", action.Priority)
        fmt.Printf("  タイプ: %s\n", action.ActionType)
        fmt.Printf("  ステータス: %s\n", action.Status)
        fmt.Printf("  担当者ヒント: %s\n", action.OwnerHint)
        fmt.Printf("  説明: %s\n", action.Description)
        fmt.Printf("  受入基準: %s\n", action.AcceptanceCriteria)
    }
}
```

## WeeklyReviewDataからの変換

既存の `WeeklyReviewService` から取得したデータを使用する場合:

```go
package main

import (
    "context"
    "time"

    "github.com/jinford/dev-rag/pkg/quality"
)

func main() {
    // WeeklyReviewServiceからデータを取得
    weeklyReviewService := quality.NewWeeklyReviewService(
        qualityRepo,
        gitParser,
        codeownersParser,
    )

    weeklyData, err := weeklyReviewService.PrepareWeeklyReview(
        context.Background(),
        "/path/to/repo",
        time.Now().AddDate(0, 0, -7), // 1週間前
        time.Now(),
    )
    if err != nil {
        panic(err)
    }

    // ActionGenerationDataに変換
    actionData := quality.FromWeeklyReviewData(weeklyData)

    // アクション生成
    generator := quality.NewActionGenerator(llmClient)
    actions, err := generator.GenerateActions(context.Background(), actionData)
    if err != nil {
        panic(err)
    }

    // アクションの処理...
}
```

## アクションのフィルタリング

生成されたアクションから、特定の条件でフィルタリング:

```go
// P1（高優先度）のアクションのみを抽出
highPriorityActions := []models.Action{}
for _, action := range actions {
    if action.IsHighPriority() {
        highPriorityActions = append(highPriorityActions, action)
    }
}

// オープン状態のアクションのみを抽出
openActions := []models.Action{}
for _, action := range actions {
    if action.IsOpen() {
        openActions = append(openActions, action)
    }
}

// reindexタイプのアクションのみを抽出
reindexActions := []models.Action{}
for _, action := range actions {
    if action.ActionType == models.ActionTypeReindex {
        reindexActions = append(reindexActions, action)
    }
}
```

## プロンプトのカスタマイズ

プロンプトを直接生成して確認する場合:

```go
package main

import (
    "fmt"
    "github.com/jinford/dev-rag/pkg/quality"
)

func main() {
    prompt := quality.NewActionGenerationPrompt()

    data := quality.ActionGenerationData{
        // データを設定...
    }

    promptText, err := prompt.GeneratePrompt(data)
    if err != nil {
        panic(err)
    }

    // プロンプトの内容を確認
    fmt.Println(promptText)
}
```

## エラーハンドリング

```go
actions, err := generator.GenerateActions(context.Background(), data)
if err != nil {
    // エラータイプに応じた処理
    if strings.Contains(err.Error(), "failed to generate completion") {
        // LLM APIエラー
        fmt.Println("LLM APIへの接続に失敗しました")
    } else if strings.Contains(err.Error(), "failed to parse action response") {
        // JSONパースエラー
        fmt.Println("LLMレスポンスのパースに失敗しました")
    } else {
        // その他のエラー
        fmt.Printf("予期しないエラー: %v\n", err)
    }
    return
}
```

## テスト

単体テストでモックLLMクライアントを使用する例:

```go
package mypackage

import (
    "context"
    "testing"

    "github.com/jinford/dev-rag/pkg/indexer/llm"
    "github.com/jinford/dev-rag/pkg/quality"
    "github.com/stretchr/testify/assert"
)

type mockLLMClient struct {
    response llm.CompletionResponse
    err      error
}

func (m *mockLLMClient) GenerateCompletion(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
    return m.response, m.err
}

func TestMyFunction(t *testing.T) {
    // モックLLMクライアントの作成
    mockClient := &mockLLMClient{
        response: llm.CompletionResponse{
            Content: `[
                {
                    "prompt_version": "1.1",
                    "priority": "P1",
                    "action_type": "reindex",
                    "title": "テストアクション",
                    "description": "説明",
                    "linked_files": [],
                    "owner_hint": "team",
                    "acceptance_criteria": "条件",
                    "status": "open"
                }
            ]`,
        },
    }

    generator := quality.NewActionGenerator(mockClient)

    // テストデータを使ってアクション生成
    data := quality.ActionGenerationData{
        // テストデータ...
    }

    actions, err := generator.GenerateActions(context.Background(), data)
    assert.NoError(t, err)
    assert.Len(t, actions, 1)
}
```

## 制限事項

- 1週間あたり最大5件のアクションが生成されます（チームキャパシティ上限）
- 超過分は `status: "noop"` として返されます
- `recent_changes` で既に解消済みの問題も `status: "noop"` として返されます
- LLMのレスポンスはJSON形式である必要があります
- JSON解析に失敗した場合、最大1回リトライします

## 関連ファイル

- `/home/nakashima95engr/ghq/github.com/jinford/dev-rag/pkg/models/action.go` - Action構造体の定義
- `/home/nakashima95engr/ghq/github.com/jinford/dev-rag/pkg/quality/action_prompt.go` - プロンプトテンプレート
- `/home/nakashima95engr/ghq/github.com/jinford/dev-rag/pkg/quality/action_generator.go` - アクション生成ロジック
- `/home/nakashima95engr/ghq/github.com/jinford/dev-rag/pkg/quality/action_prompt_test.go` - プロンプトのテスト
- `/home/nakashima95engr/ghq/github.com/jinford/dev-rag/pkg/quality/action_generator_test.go` - ジェネレーターのテスト
