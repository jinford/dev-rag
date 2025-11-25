# 依存関係解析パッケージ

Phase 2 タスク4の実装: 詳細な依存関係抽出機能を提供します。

## 概要

このパッケージは、Goソースコードから以下の詳細な依存関係情報を抽出します:

1. **インポート情報の詳細化**
   - 標準ライブラリと外部依存の区別
   - パッケージバージョン情報（go.modから取得）
   - インポートの使用状況追跡

2. **関数呼び出し解析**
   - 内部/外部関数呼び出しの区別
   - メソッド呼び出しのレシーバー型記録
   - ビルトイン関数の識別

3. **型依存の抽出**
   - 構造体フィールドの型情報
   - 関数引数・戻り値の型情報
   - 型の使用箇所追跡

4. **依存グラフの構築**
   - チャンク間の依存関係をグラフ構造で保持
   - 循環依存の検出
   - 中心性スコアの計算
   - トポロジカルソート

## 使用方法

### 基本的な使用例

```go
package main

import (
    "fmt"
    "github.com/jinford/dev-rag/pkg/indexer/dependency"
)

func main() {
    // go.modからバージョン情報を取得
    parser := dependency.NewGoModParser()
    versions, err := parser.ParseFile("go.mod")
    if err != nil {
        panic(err)
    }

    // ソースコードを解析
    analyzer := dependency.NewAnalyzer()
    source := `package main

import (
    "fmt"
    "github.com/google/uuid"
)

func main() {
    id := uuid.New()
    fmt.Println(id.String())
}
`

    info, err := analyzer.Analyze(source, versions)
    if err != nil {
        panic(err)
    }

    // インポート情報を表示
    for path, importInfo := range info.Imports {
        fmt.Printf("Import: %s\n", path)
        fmt.Printf("  Type: %v\n", importInfo.Type)
        fmt.Printf("  Version: %s\n", importInfo.Version)
        fmt.Printf("  Used: %v\n", importInfo.IsUsed)
    }

    // 関数呼び出しを表示
    for _, call := range info.FunctionCalls {
        fmt.Printf("Call: %s\n", call.Name)
        fmt.Printf("  Type: %v\n", call.Type)
        if call.Package != "" {
            fmt.Printf("  Package: %s\n", call.Package)
        }
    }

    // 型依存を表示
    for typeName, typeDep := range info.TypeDeps {
        fmt.Printf("Type: %s\n", typeName)
        fmt.Printf("  Kind: %v\n", typeDep.Kind)
        if len(typeDep.FieldTypes) > 0 {
            fmt.Printf("  Fields: %v\n", typeDep.FieldTypes)
        }
    }
}
```

### 依存グラフの構築と解析

```go
package main

import (
    "fmt"
    "github.com/google/uuid"
    "github.com/jinford/dev-rag/pkg/indexer/dependency"
)

func main() {
    // グラフを作成
    graph := dependency.NewGraph()

    // ノードを追加
    node1 := &dependency.Node{
        ChunkID:  uuid.New(),
        Name:     "funcA",
        Type:     "function",
        FilePath: "a.go",
    }
    node2 := &dependency.Node{
        ChunkID:  uuid.New(),
        Name:     "funcB",
        Type:     "function",
        FilePath: "b.go",
    }

    graph.AddNode(node1)
    graph.AddNode(node2)

    // エッジを追加（funcA -> funcB）
    edge := &dependency.Edge{
        From:         node1.ChunkID,
        To:           node2.ChunkID,
        RelationType: dependency.RelationTypeCalls,
        Weight:       1,
    }
    graph.AddEdge(edge)

    // 依存関係を取得
    deps := graph.GetDependencies(node1.ChunkID)
    fmt.Printf("funcA depends on: %d nodes\n", len(deps))

    // 被参照回数を取得
    refCount := graph.GetReferenceCount(node2.ChunkID)
    fmt.Printf("funcB is referenced by: %d nodes\n", refCount)

    // 循環依存を検出
    cycles := graph.DetectCycles()
    fmt.Printf("Cycles detected: %d\n", len(cycles))

    // 中心性スコアを計算
    centrality := graph.CalculateCentrality(node1.ChunkID)
    fmt.Printf("funcA centrality: %.2f\n", centrality)

    // グラフ統計を取得
    stats := graph.GetStats()
    fmt.Printf("Graph stats:\n")
    fmt.Printf("  Nodes: %d\n", stats.NodeCount)
    fmt.Printf("  Edges: %d\n", stats.EdgeCount)
    fmt.Printf("  Avg In-Degree: %.2f\n", stats.AvgInDegree)
    fmt.Printf("  Avg Out-Degree: %.2f\n", stats.AvgOutDegree)
    fmt.Printf("  Cycles: %d\n", stats.CycleCount)
}
```

## API リファレンス

### Analyzer

`Analyzer`はGoソースコードから依存関係情報を抽出します。

#### メソッド

- `NewAnalyzer() *Analyzer`: 新しいAnalyzerを作成
- `Analyze(content string, goModData map[string]string) (*DependencyInfo, error)`: ソースコードを解析

### DependencyInfo

解析結果を保持する構造体です。

#### フィールド

- `Imports map[string]*ImportInfo`: インポート情報（key: インポートパス）
- `FunctionCalls []*FunctionCall`: 関数呼び出しリスト
- `TypeDeps map[string]*TypeDependency`: 型依存情報（key: 型名）

### ImportInfo

インポート情報の詳細を表します。

#### フィールド

- `Path string`: インポートパス
- `Alias string`: エイリアス
- `Type ImportType`: 標準ライブラリ/外部依存/内部パッケージ
- `Version string`: バージョン情報
- `IsUsed bool`: 実際に使用されているか
- `UsageCount int`: 使用回数
- `UsedBy []string`: 使用している関数・型のリスト

### FunctionCall

関数呼び出し情報を表します。

#### フィールド

- `Name string`: 関数名
- `Package string`: パッケージ名
- `Type CallType`: 呼び出しタイプ（内部/外部/メソッド/ビルトイン）
- `ReceiverType string`: レシーバー型（メソッドの場合）
- `Arguments []string`: 引数の型情報

### TypeDependency

型依存情報を表します。

#### フィールド

- `TypeName string`: 型名
- `Package string`: パッケージ名
- `Kind TypeKind`: 型の種類（基本型/構造体/インターフェース等）
- `UsedBy []string`: 使用している関数・型のリスト
- `FieldTypes []string`: 構造体フィールドの型
- `ParameterOf []string`: パラメータとして使用されている関数のリスト
- `ReturnTypeOf []string`: 戻り値として使用されている関数のリスト

### Graph

依存関係グラフを表します。

#### メソッド

- `NewGraph() *Graph`: 新しいグラフを作成
- `AddNode(node *Node)`: ノードを追加
- `AddEdge(edge *Edge) error`: エッジを追加
- `GetDependencies(chunkID uuid.UUID) []*Node`: 依存しているノードを取得
- `GetDependents(chunkID uuid.UUID) []*Node`: 依存されているノードを取得
- `GetReferenceCount(chunkID uuid.UUID) int`: 被参照回数を取得
- `DetectCycles() [][]*Node`: 循環依存を検出
- `CalculateCentrality(chunkID uuid.UUID) float64`: 中心性スコアを計算
- `GetTopologicalOrder() ([]*Node, error)`: トポロジカルソートを実行
- `GetStats() *GraphStats`: グラフ統計を取得

### GoModParser

go.modファイルを解析してバージョン情報を取得します。

#### メソッド

- `NewGoModParser() *GoModParser`: 新しいパーサーを作成
- `ParseFile(filePath string) (map[string]string, error)`: go.modファイルを解析
- `ParseContent(content string) (map[string]string, error)`: 文字列からgo.modを解析
- `GetModulePath(content string) string`: モジュールパスを取得

#### ヘルパー関数

- `IsStandardLibrary(pkg string) bool`: 標準ライブラリかどうか判定
- `GetVersion(versions map[string]string, pkg string) string`: パッケージのバージョンを取得

## 設計原則

本実装は以下の設計原則に従っています:

1. **段階的な解析**: インポート → 関数呼び出し → 型依存の順に段階的に解析
2. **詳細な分類**: 依存関係を複数のタイプに分類（標準/外部/内部、関数/メソッド等）
3. **使用状況追跡**: インポートや型が実際に使用されているかを追跡
4. **グラフ構造**: 依存関係をグラフとして表現し、循環依存検出や中心性計算が可能
5. **拡張性**: 新しい解析機能の追加が容易な設計

## Phase 2 タスク4との対応

以下の要件をすべて実装しています:

- ✅ インポート情報の詳細化（標準ライブラリと外部依存を区別）
- ✅ パッケージバージョン情報の抽出（go.mod参照）
- ✅ 関数呼び出し解析（AST解析で関数呼び出しを抽出）
- ✅ 内部関数呼び出しと外部関数呼び出しを区別
- ✅ メソッド呼び出しのレシーバー型を記録
- ✅ 型依存の抽出（構造体フィールドの型情報）
- ✅ 関数引数・戻り値の型情報
- ✅ 依存グラフの構築（チャンク間の依存関係をグラフ構造で保持）
- ✅ 循環依存の検出

## テスト

```bash
# すべてのテストを実行
go test -v ./pkg/indexer/dependency/...

# カバレッジレポートを生成
go test -coverprofile=coverage.out ./pkg/indexer/dependency/...
go tool cover -html=coverage.out
```

## 今後の拡張

以下の機能拡張が考えられます:

1. **他言語のサポート**: TypeScript/JavaScript、Python等
2. **より高度なグラフ解析**: PageRank、Betweenness Centrality等
3. **依存関係の可視化**: GraphvizやD3.jsでの可視化
4. **変更影響分析**: コード変更の影響範囲を分析
5. **パッケージ間依存**: ファイル/パッケージレベルの依存関係分析

## 参考資料

- [RAGインジェスト設計書 4.3節「依存関係」](../../../docs/rag-ingestion-design.md)
- [Phase 2タスク定義](../../../tasks/phase2-hierarchical-chunking.md)
