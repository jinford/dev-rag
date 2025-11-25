# pkg/query パッケージ

## 概要

`pkg/query`パッケージは、階層検索のサポートとLLMコンテキスト構築機能を提供します。

Phase 2のタスク10「階層検索のサポート」で実装されました。

## 主要な機能

### 1. 階層情報を含む検索結果の拡張

`Querier`は、既存の検索結果に親チャンクや子チャンクの階層情報を追加します。

### 2. LLMコンテキストの構築

`ContextBuilder`は、検索結果をLLMに送信するための最適な形式に整形します。

## 使用例

### 基本的な使用例

```go
package main

import (
    "context"
    "fmt"

    "github.com/jinford/dev-rag/pkg/query"
    "github.com/jinford/dev-rag/pkg/search"
)

func main() {
    ctx := context.Background()

    // 1. 通常の検索を実行
    searcher := search.NewSearcher(db, embedder)
    searchParams := search.SearchParams{
        ProductID: &productID,
        Query:     "user authentication",
        Limit:     5,
    }

    baseResults, err := searcher.SearchByProduct(ctx, searchParams)
    if err != nil {
        panic(err)
    }

    // 2. 階層情報を追加
    querier := query.NewQuerier(indexRepo)
    options := &query.SearchOptions{
        IncludeParent:   true,  // 親チャンクを含める
        IncludeChildren: true,  // 子チャンクを含める
        MaxDepth:        1,     // 階層の最大深さ
    }

    enrichedResults, err := querier.EnrichSearchResultsWithHierarchy(
        ctx,
        baseResults.Chunks,
        options,
    )
    if err != nil {
        panic(err)
    }

    // 3. LLMコンテキストを構築
    contextBuilder := query.NewContextBuilder(8000) // 8000トークン
    llmContext := contextBuilder.BuildContextWithHierarchy(enrichedResults)

    // 4. トークン制限内に収める
    llmContext = contextBuilder.TruncateToTokenLimit(llmContext)

    // 5. LLMに送信
    response := sendToLLM(llmContext, "ユーザー認証はどのように実装されていますか？")
    fmt.Println(response)
}
```

### 既存のHierarchicalSearcherとの連携

既存の`pkg/search/HierarchicalSearcher`を使用している場合の統合例:

```go
package main

import (
    "context"

    "github.com/jinford/dev-rag/pkg/query"
    "github.com/jinford/dev-rag/pkg/search"
)

func main() {
    ctx := context.Background()

    // 既存のHierarchicalSearcherを使用した検索
    searcher := search.NewSearcher(db, embedder)
    searchParams := search.SearchParams{
        ProductID: &productID,
        Query:     "user authentication",
        Limit:     5,
    }

    hierarchicalOptions := search.HierarchicalSearchOptions{
        IncludeParent:   true,
        IncludeChildren: true,
    }

    hierarchicalResults, err := searcher.SearchByProductWithHierarchy(
        ctx,
        searchParams,
        hierarchicalOptions,
    )
    if err != nil {
        panic(err)
    }

    // HierarchicalSearcherの結果をContextBuilderで整形
    // (HierarchicalSearchResultをEnhancedSearchResultに変換)
    enrichedResults := convertToEnhancedResults(hierarchicalResults.Chunks)

    // LLMコンテキストを構築
    contextBuilder := query.NewContextBuilder(8000)
    llmContext := contextBuilder.BuildContextWithHierarchy(enrichedResults)
    llmContext = contextBuilder.TruncateToTokenLimit(llmContext)

    // LLMに送信
    response := sendToLLM(llmContext, "質問内容")
    fmt.Println(response)
}

func convertToEnhancedResults(
    hierarchicalResults []*search.HierarchicalSearchResult,
) []*query.EnhancedSearchResult {
    enriched := make([]*query.EnhancedSearchResult, len(hierarchicalResults))
    for i, hr := range hierarchicalResults {
        enriched[i] = &query.EnhancedSearchResult{
            SearchResult: hr.SearchResult,
            ParentChunk:  hr.Parent,
            ChildChunks:  hr.Children,
        }
    }
    return enriched
}
```

### 複数のコンテキスト構築方法

`ContextBuilder`は複数のコンテキスト構築方法を提供します:

```go
contextBuilder := query.NewContextBuilder(8000)

// 1. 階層情報を含む詳細なコンテキスト
detailedContext := contextBuilder.BuildContextWithHierarchy(enrichedResults)

// 2. シンプルなコンテキスト（階層情報なし）
simpleContext := contextBuilder.BuildSimpleContext(enrichedResults)

// 3. メタデータを含むコンテキスト（スコア、ファイルパスなど）
metadataContext := contextBuilder.BuildContextWithMetadata(enrichedResults)

// 4. コンパクトなコンテキスト（トークン制限が厳しい場合）
compactContext := contextBuilder.BuildCompactContext(enrichedResults)
```

### トークン数の管理

```go
contextBuilder := query.NewContextBuilder(8000)

// コンテキストを構築
llmContext := contextBuilder.BuildContextWithHierarchy(enrichedResults)

// トークン数を概算
estimatedTokens := contextBuilder.EstimateTokenCount(llmContext)
fmt.Printf("推定トークン数: %d\n", estimatedTokens)

// トークン制限内に収める
if estimatedTokens > 8000 {
    llmContext = contextBuilder.TruncateToTokenLimit(llmContext)
}
```

## API リファレンス

### Querier

#### NewQuerier

```go
func NewQuerier(indexRepo *repository.IndexRepositoryR) *Querier
```

新しい`Querier`を作成します。

#### EnrichSearchResultsWithHierarchy

```go
func (q *Querier) EnrichSearchResultsWithHierarchy(
    ctx context.Context,
    baseResults []*models.SearchResult,
    options *SearchOptions,
) ([]*EnhancedSearchResult, error)
```

既存の検索結果に階層情報を追加します。

**パラメータ:**
- `ctx`: コンテキスト
- `baseResults`: 基本的な検索結果のスライス
- `options`: 階層検索のオプション

**戻り値:**
- 階層情報を含む検索結果のスライス

**エラー:**
- エラーが発生しても処理を続行し、その結果のみエラー情報を除外します

#### GetParentChunk

```go
func (q *Querier) GetParentChunk(ctx context.Context, chunkID interface{}) (*models.Chunk, error)
```

指定されたチャンクの親チャンクを取得します。

#### GetChildChunks

```go
func (q *Querier) GetChildChunks(ctx context.Context, chunkID interface{}) ([]*models.Chunk, error)
```

指定されたチャンクの子チャンクを取得します。

### ContextBuilder

#### NewContextBuilder

```go
func NewContextBuilder(maxTokens int) *ContextBuilder
```

新しい`ContextBuilder`を作成します。

**パラメータ:**
- `maxTokens`: LLMに送信する最大トークン数

#### BuildContextWithHierarchy

```go
func (cb *ContextBuilder) BuildContextWithHierarchy(results []*EnhancedSearchResult) string
```

階層情報を含むコンテキストを構築します。

#### BuildSimpleContext

```go
func (cb *ContextBuilder) BuildSimpleContext(results []*EnhancedSearchResult) string
```

シンプルなコンテキストを構築します（階層情報なし）。

#### BuildContextWithMetadata

```go
func (cb *ContextBuilder) BuildContextWithMetadata(results []*EnhancedSearchResult) string
```

メタデータを含むコンテキストを構築します。

#### BuildCompactContext

```go
func (cb *ContextBuilder) BuildCompactContext(results []*EnhancedSearchResult) string
```

コンパクトなコンテキストを構築します。

#### TruncateToTokenLimit

```go
func (cb *ContextBuilder) TruncateToTokenLimit(context string) string
```

トークン制限内に収まるようにコンテキストを切り詰めます。

#### EstimateTokenCount

```go
func (cb *ContextBuilder) EstimateTokenCount(context string) int
```

コンテキストのトークン数を概算します。

### SearchOptions

```go
type SearchOptions struct {
    IncludeParent   bool // 親チャンクを含める
    IncludeChildren bool // 子チャンクを含める
    MaxDepth        int  // 階層の最大深さ（デフォルト: 1）
}
```

### EnhancedSearchResult

```go
type EnhancedSearchResult struct {
    *models.SearchResult

    ParentChunk *models.Chunk   // 親チャンク
    ChildChunks []*models.Chunk // 子チャンク
}
```

## 注意点

### トークン数の概算について

現在の実装では、トークン数を文字数ベースで概算しています（1トークン ≈ 4文字）。

より正確なトークン数計算が必要な場合は、`tiktoken`などのライブラリを使用してください。

### エラーハンドリング

`EnrichSearchResultsWithHierarchy`メソッドは、個々のチャンクの階層情報取得でエラーが発生しても、
処理を続行します。エラーが発生したチャンクの階層情報は`nil`または空スライスになります。

### 既存のpkg/searchパッケージとの関係

`pkg/query`パッケージは、既存の`pkg/search`パッケージと連携して動作します。

- `pkg/search`: ベクトル検索の実行と階層情報の取得
- `pkg/query`: 階層情報を含む検索結果の整形とLLMコンテキスト構築

両者を組み合わせることで、LLMへの最適なコンテキスト構築が可能です。

## Phase 3での統合

Phase 3では、このパッケージを使用してRAGパイプラインを構築する予定です:

1. ユーザークエリを受け取る
2. `pkg/search`でベクトル検索を実行
3. `pkg/query`で階層情報を追加してコンテキストを構築
4. LLMに送信して回答を生成
5. 回答をユーザーに返す
