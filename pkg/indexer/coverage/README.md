# Coverage パッケージ

Phase 2タスク7・8で実装されたカバレッジマップ構築とアラート機能を提供します。

## 概要

このパッケージは以下の機能を提供します:

1. **カバレッジマップ構築** (タスク7)
   - ドメイン別のインデックスカバレッジ統計を算出
   - 未インデックスの重要ファイルを検出
   - JSON形式でカバレッジマップをエクスポート

2. **カバレッジアラート** (タスク8)
   - 重要READMEの未インデックス検出
   - ADRドキュメントのカバレッジ不足を警告
   - テストコードのカバレッジ率が低い場合に警告

## 使用方法

### カバレッジマップの構築

```go
import (
    "github.com/jinford/dev-rag/pkg/indexer/coverage"
    "github.com/jinford/dev-rag/pkg/repository"
)

// リポジトリの初期化
indexRepo := repository.NewIndexRepositoryR(db)

// CoverageBuilderの作成
builder := coverage.NewCoverageBuilder(indexRepo)

// カバレッジマップの構築
coverageMap, err := builder.BuildCoverageMap(ctx, snapshotID, snapshotVersion)
if err != nil {
    log.Fatal(err)
}

// JSON形式でエクスポート
jsonData, err := builder.ExportToJSON(coverageMap)
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(jsonData))
```

### アラートの生成と表示

```go
import (
    "github.com/jinford/dev-rag/pkg/indexer/coverage"
)

// デフォルト設定でAlertGeneratorを作成
alertGen := coverage.NewAlertGenerator(indexRepo, nil)

// または、カスタム設定で作成
config := &coverage.AlertConfig{
    EnableMissingReadmeAlert: true,
    ADRTotalThreshold:        15,
    ADRIndexedThreshold:      8,
    TestCoverageThreshold:    25.0,
}
alertGen := coverage.NewAlertGenerator(indexRepo, config)

// アラートを生成
alerts, err := alertGen.GenerateAlerts(ctx, snapshotID, coverageMap)
if err != nil {
    log.Fatal(err)
}

// アラートを標準出力に表示
alertGen.PrintAlerts(alerts)
```

## アラート設定

### AlertConfig

アラート生成の閾値を設定します:

```go
type AlertConfig struct {
    // 重要READMEの未インデックスアラートを有効化
    EnableMissingReadmeAlert bool

    // ADRドキュメント数の閾値
    ADRTotalThreshold   int     // ADRドキュメントがこの数以上ある場合にチェック
    ADRIndexedThreshold int     // インデックス済みADRドキュメントがこの数未満の場合にアラート

    // テストコードのカバレッジ率の閾値（%）
    TestCoverageThreshold float64 // テストコードのカバレッジ率がこの値未満の場合にアラート
}
```

### デフォルト設定

```go
DefaultAlertConfig() = &AlertConfig{
    EnableMissingReadmeAlert: true,
    ADRTotalThreshold:        10,  // ADRが10件以上ある場合にチェック
    ADRIndexedThreshold:      5,   // インデックス済みADRが5件未満なら警告
    TestCoverageThreshold:    20.0, // テストカバレッジが20%未満なら警告
}
```

## アラートの種類

### 1. 重要READMEの未インデックス

**Severity**: `error` (ルートREADME) / `warning` (その他)

リポジトリルートや主要ディレクトリのREADMEがインデックスされていない場合に発行されます。

例:
```
✗ [error] リポジトリルートの重要なREADMEファイルがインデックスされていません: README.md
  ドメイン: architecture
  詳細: map[file_count:1 missing_files:[README.md]]
```

### 2. ADRドキュメントのカバレッジ不足

**Severity**: `warning`

ADRドキュメントが一定数以上存在するのに、インデックス済み数が閾値未満の場合に発行されます。

例:
```
⚠ [warning] ADRドキュメントが12件ありますが、4件しかインデックス化されていません（閾値: 5件以上）
  ドメイン: architecture
  詳細: map[indexed_adrs:4 threshold:5 total_adrs:12]
```

### 3. テストコードのカバレッジ率不足

**Severity**: `warning`

テストコードのカバレッジ率が閾値未満の場合に発行されます。

例:
```
⚠ [warning] テストコードのカバレッジ率が低すぎます: 15.00% (閾値: 20.00%以上)
  ドメイン: tests
  詳細: map[coverage_rate:15 indexed_files:15 threshold:20 total_files:100 unindexed_files:85]
```

## データモデル

### Alert

```go
type Alert struct {
    Severity    AlertSeverity  `json:"severity"`    // "warning" または "error"
    Message     string         `json:"message"`     // アラートメッセージ
    Domain      string         `json:"domain,omitempty"` // 関連ドメイン
    Details     interface{}    `json:"details,omitempty"` // 詳細情報
    GeneratedAt time.Time      `json:"generatedAt"` // 生成日時
}
```

### CoverageMap

```go
type CoverageMap struct {
    SnapshotID       string           `json:"snapshotID"`
    SnapshotVersion  string           `json:"snapshotVersion"`
    TotalFiles       int              `json:"totalFiles"`
    TotalIndexedFiles int             `json:"totalIndexedFiles"`
    TotalChunks      int              `json:"totalChunks"`
    OverallCoverage  float64          `json:"overallCoverage"`
    DomainCoverages  []DomainCoverage `json:"domainCoverages"`
    GeneratedAt      time.Time        `json:"generatedAt"`
}
```

## テスト

```bash
# 全テストを実行
go test ./pkg/indexer/coverage/... -v

# カバレッジ付きで実行
go test ./pkg/indexer/coverage/... -cover

# 特定のテストのみ実行
go test ./pkg/indexer/coverage/... -v -run TestAlert
```

## 設計資料

- [Phase 2: 階層的チャンキング + 依存関係抽出 + カバレッジマップ](/home/nakashima95engr/ghq/github.com/jinford/dev-rag/tasks/phase2-hierarchical-chunking.md)
  - タスク7: カバレッジマップの構築
  - タスク8: カバレッジアラート機能
- [RAGインジェストシステム設計書](/home/nakashima95engr/ghq/github.com/jinford/dev-rag/docs/rag-ingestion-design.md)
  - 6.2節: カバレッジの可視化
  - 6.3節: アラート機能

## 実装履歴

- **Phase 2 タスク7** (2025-11-20): カバレッジマップ構築機能を実装
  - CoverageBuilder の実装
  - snapshot_files テーブルを活用したドメイン別統計
  - 未インデックス重要ファイルの検出

- **Phase 2 タスク8** (2025-11-20): カバレッジアラート機能を実装
  - AlertGenerator の実装
  - 3種類のアラート条件（README、ADR、テストカバレッジ）
  - 設定可能な閾値
  - 標準出力へのアラート表示
