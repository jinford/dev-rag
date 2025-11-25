# Provenance パッケージ

データ起源トレーサビリティ機能を提供するパッケージです。Phase 2タスク9で実装されました。

## 概要

このパッケージは以下の機能を提供します:

1. **Provenance Graph**: チャンクIDから起源情報（ファイルパス、コミットハッシュ、スナップショットID）へのマッピングを管理
2. **履歴管理**: 同一ファイルの異なるバージョンのチャンクを区別
3. **ランキング調整**: 検索時に最新バージョンのチャンクを優先

## 主要コンポーネント

### ProvenanceGraph

チャンクIDから起源情報へのマッピングを管理するインメモリグラフ構造です。

**主なメソッド:**
- `Add(prov *ChunkProvenance)`: Provenance情報を追加
- `Get(chunkID uuid.UUID)`: チャンクIDから起源情報を取得
- `GetByChunkKey(chunkKey string)`: ChunkKeyから全バージョンのチャンクIDを取得
- `GetLatestByChunkKey(chunkKey string)`: ChunkKeyから最新バージョンを取得
- `GetFileHistory(filePath string)`: ファイルパスから全バージョンの履歴を取得
- `GetLatestVersions()`: 全チャンクの中から最新バージョンのみを取得
- `IsLatest(chunkID uuid.UUID)`: チャンクが最新バージョンかどうかを判定
- `TraceProvenance(chunkID uuid.UUID)`: 起源情報を人間が読める形式で返す

### Ranker

検索結果のランキングを調整し、最新バージョンのチャンクを優先します。

**主なメソッド:**
- `AdjustRanking(ctx context.Context, results []*models.SearchResult)`: ランキング調整を実行
- `DeduplicateByLatest(ctx context.Context, results []*RankedResult)`: 同一範囲の重複を除外
- `FilterByLatestOnly(ctx context.Context, results []*models.SearchResult)`: 最新バージョンのみをフィルタリング
- `GetProvenanceInfo(chunkID uuid.UUID)`: Provenance情報を取得

## 使用例

### 基本的な使い方

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/jinford/dev-rag/pkg/provenance"
    "github.com/jinford/dev-rag/internal/module/indexing/domain"
)

func main() {
    // ProvenanceGraphの作成
    pg := provenance.NewProvenanceGraph()

    // Provenance情報の追加
    chunkID := uuid.New()
    prov := &provenance.ChunkProvenance{
        ChunkID:          chunkID,
        SnapshotID:       uuid.New(),
        FilePath:         "src/main.go",
        GitCommitHash:    "abc123",
        ChunkKey:         "product/source/src/main.go#L1-L10@abc123",
        IsLatest:         true,
        IndexedAt:        time.Now(),
        SourceSnapshotID: uuid.New(),
    }

    err := pg.Add(prov)
    if err != nil {
        panic(err)
    }

    // 起源情報の取得
    retrieved, err := pg.Get(chunkID)
    if err != nil {
        panic(err)
    }

    fmt.Printf("File: %s\n", retrieved.FilePath)
    fmt.Printf("Commit: %s\n", retrieved.GitCommitHash)
    fmt.Printf("Is Latest: %v\n", retrieved.IsLatest)

    // 起源情報のトレース
    trace, err := pg.TraceProvenance(chunkID)
    if err != nil {
        panic(err)
    }
    fmt.Println(trace)
}
```

### 検索結果のランキング調整

```go
package main

import (
    "context"
    "fmt"

    "github.com/jinford/dev-rag/pkg/provenance"
    "github.com/jinford/dev-rag/internal/module/indexing/domain"
)

func main() {
    // ProvenanceGraphとRankerの作成
    pg := provenance.NewProvenanceGraph()

    // Provenance情報を追加 (省略)

    // カスタム設定でRankerを作成
    config := &provenance.RankingConfig{
        LatestVersionBoost: 0.2,  // 最新バージョンに20%のブースト
        RecencyDecayFactor: 0.15, // 古いバージョンに15%の減衰
        MinScore:           0.5,  // 最小スコア閾値
    }
    ranker := provenance.NewRanker(pg, config)

    // 検索結果 (例)
    results := []*models.SearchResult{
        // ... 検索結果のリスト
    }

    // ランキング調整
    ctx := context.Background()
    rankedResults, err := ranker.AdjustRanking(ctx, results)
    if err != nil {
        panic(err)
    }

    // 結果の表示
    for i, rr := range rankedResults {
        fmt.Printf("%d. Chunk ID: %s\n", i+1, rr.ChunkID)
        fmt.Printf("   Original Score: %.3f\n", rr.OriginalScore)
        fmt.Printf("   Adjusted Score: %.3f\n", rr.AdjustedScore)
        fmt.Printf("   Is Latest: %v\n", rr.IsLatest)
        fmt.Printf("   Boost Applied: %.3f\n", rr.BoostApplied)
    }
}
```

### 重複除外

```go
package main

import (
    "context"
    "fmt"

    "github.com/jinford/dev-rag/pkg/provenance"
)

func main() {
    pg := provenance.NewProvenanceGraph()
    ranker := provenance.NewRanker(pg, nil) // デフォルト設定を使用

    // Provenance情報と検索結果を追加 (省略)

    ctx := context.Background()
    rankedResults, _ := ranker.AdjustRanking(ctx, results)

    // 同一ファイル・同一範囲の重複を除外 (最新バージョンのみを残す)
    deduplicated, err := ranker.DeduplicateByLatest(ctx, rankedResults)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Original results: %d\n", len(rankedResults))
    fmt.Printf("Deduplicated results: %d\n", len(deduplicated))
}
```

### 最新バージョンのみをフィルタリング

```go
package main

import (
    "context"
    "fmt"

    "github.com/jinford/dev-rag/pkg/provenance"
    "github.com/jinford/dev-rag/internal/module/indexing/domain"
)

func main() {
    pg := provenance.NewProvenanceGraph()
    ranker := provenance.NewRanker(pg, nil)

    // Provenance情報を追加 (省略)

    results := []*models.SearchResult{
        // ... 検索結果のリスト
    }

    ctx := context.Background()

    // 最新バージョンのみをフィルタリング
    latestOnly, err := ranker.FilterByLatestOnly(ctx, results)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Filtered to %d latest versions\n", len(latestOnly))
}
```

## データ構造

### ChunkProvenance

```go
type ChunkProvenance struct {
    ChunkID          uuid.UUID  // チャンクID
    SnapshotID       uuid.UUID  // スナップショットID
    FilePath         string     // ファイルパス
    GitCommitHash    string     // Gitコミットハッシュ
    ChunkKey         string     // 決定的な識別子
    IsLatest         bool       // 最新バージョンフラグ
    Author           *string    // 最終更新者
    UpdatedAt        *time.Time // ファイル最終更新日時
    IndexedAt        time.Time  // インデックス作成日時
    FileVersion      *string    // ファイルバージョン識別子
    SourceSnapshotID uuid.UUID  // ソーススナップショットID
}
```

### RankingConfig

```go
type RankingConfig struct {
    LatestVersionBoost float64 // 最新バージョンのブースト (0.0〜1.0)
    RecencyDecayFactor float64 // 古いバージョンの減衰係数 (0.0〜1.0)
    MinScore           float64 // 最小スコア閾値
}
```

デフォルト設定:
- `LatestVersionBoost`: 0.15 (15%のブースト)
- `RecencyDecayFactor`: 0.1 (10%の減衰)
- `MinScore`: 0.0 (除外しない)

### RankedResult

```go
type RankedResult struct {
    *models.SearchResult
    OriginalScore float64 // 元のスコア
    AdjustedScore float64 // 調整後のスコア
    IsLatest      bool    // 最新バージョンかどうか
    BoostApplied  float64 // 適用されたブースト値
}
```

## 設計の特徴

### 軽量なインメモリ実装

Phase 2では軽量なインメモリ実装として設計されています。大規模システムでの永続化が必要な場合は、Phase 3以降でデータベースバックエンドへの拡張を検討します。

### スレッドセーフ

ProvenanceGraphは内部でsync.RWMutexを使用しており、並行アクセスに対して安全です。

### 柔軟なランキング調整

RankingConfigを通じて、ブーストや減衰の係数をカスタマイズ可能です。ユースケースに応じて調整してください。

## テスト

すべての主要機能は単体テストでカバーされています:

```bash
# 全テストの実行
go test -v ./pkg/provenance/...

# Provenance Graphのテストのみ
go test -v ./pkg/provenance/... -run TestProvenanceGraph

# Rankerのテストのみ
go test -v ./pkg/provenance/... -run TestRanker
```

## 今後の拡張

Phase 3以降で以下の拡張を検討しています:

1. **データベース永続化**: PostgreSQLへのProvenance情報の永続化
2. **GraphQL API**: Provenance情報を取得するためのAPI
3. **可視化ツール**: Provenance Graphを可視化するツール
4. **監査ログ**: Provenance情報の変更履歴を記録

## 参照

- [RAGインジェスト設計書](../../docs/rag-ingestion-design.md) 7節「データ起源トレーサビリティ」
- [Phase 2タスク一覧](../../tasks/phase2-hierarchical-chunking.md) タスク9
