# Phase 2 タスク6: ドメイン分類（ルールベース）実装完了報告

## 実装日
2025-11-20

## 実装内容

### 1. 既存実装の確認結果

Phase 1ですでに以下が実装されていました：

- **classifyDomain関数**: `pkg/indexer/indexer.go`（459-522行）
  - パス・ファイル名パターンでのドメイン分類ロジック
  - 5つのドメイン（code, tests, architecture, ops, infra）への分類
- **filesテーブルのdomainカラム**: マイグレーション済み（`003_add_file_metadata.up.sql`）
- **既存テストコード**: `pkg/indexer/language_domain_test.go`（22パターンのテストケース）

### 2. 新規実装内容

Phase 2で不足していた以下の機能を追加実装しました：

#### 2.1 ドメイン別カバレッジ計算用SQLクエリ

**ファイル**: `/home/nakashima95engr/ghq/github.com/jinford/dev-rag/queries/files.sql`

```sql
-- name: GetDomainCoverageBySnapshot :many
-- ドメイン別のファイル数とチャンク数を集計
SELECT
    COALESCE(f.domain, 'unknown') AS domain,
    COUNT(DISTINCT f.id) AS file_count,
    COALESCE(SUM(chunk_counts.chunk_count), 0) AS chunk_count
FROM files f
LEFT JOIN (
    SELECT file_id, COUNT(*) AS chunk_count
    FROM chunks
    GROUP BY file_id
) chunk_counts ON f.id = chunk_counts.file_id
WHERE f.snapshot_id = $1
GROUP BY f.domain
ORDER BY file_count DESC;

-- name: GetFilesByDomain :many
-- 指定したドメインのファイル一覧を取得
SELECT * FROM files
WHERE snapshot_id = $1 AND domain = $2
ORDER BY path;
```

#### 2.2 Repository層のドメイン統計取得メソッド

**ファイル**: `/home/nakashima95engr/ghq/github.com/jinford/dev-rag/pkg/repository/index_repository.go`

追加した構造体とメソッド：

```go
// DomainCoverage はドメイン別のカバレッジ統計を表します
type DomainCoverage struct {
    Domain     string
    FileCount  int64
    ChunkCount int64
}

// GetDomainCoverageBySnapshot はスナップショット配下のドメイン別統計を取得します
func (r *IndexRepositoryR) GetDomainCoverageBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*DomainCoverage, error)

// GetFilesByDomain は指定したドメインのファイル一覧を取得します
func (r *IndexRepositoryR) GetFilesByDomain(ctx context.Context, snapshotID uuid.UUID, domain string) ([]*models.File, error)
```

#### 2.3 型変換ヘルパー関数の追加

**ファイル**: `/home/nakashima95engr/ghq/github.com/jinford/dev-rag/pkg/repository/converter.go`

```go
// StringToNullableText converts string to pgtype.Text (nullable)
func StringToNullableText(s string) pgtype.Text {
    if s == "" {
        return pgtype.Text{}
    }
    return pgtype.Text{String: s, Valid: true}
}
```

#### 2.4 テストコードの追加

**ファイル**: `/home/nakashima95engr/ghq/github.com/jinford/dev-rag/pkg/repository/domain_coverage_test.go`

- `TestDomainCoverageStruct`: DomainCoverage構造体の基本検証
- `TestDomainCoverageFields`: 5つのドメインすべてのフィールド設定検証

### 3. 実行したテストの結果

#### 3.1 既存のドメイン分類テスト

```bash
go test -v ./pkg/indexer -run TestClassifyDomain
```

**結果**: ✅ PASS（22個のテストケースすべて成功）

テストカバレッジ：
- Goテストファイル、JavaScriptテスト、Rubyテスト → tests
- README、docsディレクトリ、reStructuredText → architecture
- Dockerfile、docker-compose、Kubernetes manifest → infra
- シェルスクリプト、opsディレクトリ → ops
- その他のソースコード → code

#### 3.2 新規カバレッジ計算テスト

```bash
go test -v ./pkg/repository -run TestDomainCoverage
```

**結果**: ✅ PASS（構造体とフィールド検証が成功）

#### 3.3 全体のリグレッションテスト

```bash
go test ./pkg/indexer ./pkg/repository
```

**結果**: ✅ PASS（既存機能に影響なし）

### 4. 発生した問題と解決方法

#### 問題1: sqlcの型推論の制約

**問題**: SQLクエリの `COALESCE(SUM(...), 0)` の結果型が `interface{}` として生成された。

**原因**: COALESCEの結果型をsqlcが正確に推論できない。

**解決方法**: Repository層で型アサーションを実施：

```go
var chunkCount int64
if row.ChunkCount != nil {
    if val, ok := row.ChunkCount.(int64); ok {
        chunkCount = val
    }
}
```

## 完了条件のチェックリスト

- ✅ ファイルがルールベースでドメイン分類される
  - 既存のclassifyDomain関数が5つのドメインに正しく分類
  - 22パターンのテストケースで検証済み

- ✅ 各ドメインのカバレッジが計算される
  - GetDomainCoverageBySnapshotメソッドでドメイン別のファイル数・チャンク数を集計
  - SQLクエリでファイル数とチャンク数を効率的に集計

- ✅ 単体テストが通る
  - 既存のテストコード（TestClassifyDomain）: 22個すべてPASS
  - 新規のテストコード（TestDomainCoverage）: 6個すべてPASS
  - リグレッションテスト: 既存機能に影響なし

## 実装の特徴

### 1. ルールベース分類ロジック

既存実装は以下の優先順位でドメインを判定：

1. **tests**: テストファイルのパターン（最優先）
   - `*_test.go`, `*_test.*`, `/test/`, `/tests/`, `/__tests__/`, `/spec/`

2. **ops**: 運用スクリプト（ドキュメントより優先）
   - `/scripts/`, `/ops/`, `*.sh`, `*.bash`

3. **architecture**: ドキュメント
   - `/docs/`, `/doc/`, `*.md`, `*.rst`, `*.adoc`

4. **infra**: インフラ定義
   - `Dockerfile`, `docker-compose`, `*.yml`, `*.yaml`, `/terraform/`, `/k8s/`, `*.tf`

5. **code**: デフォルト

### 2. カバレッジ計算の効率性

- 1回のSQLクエリでドメイン別のファイル数とチャンク数を集計
- LEFT JOINによりチャンクが存在しないファイルも正しくカウント
- file_count降順でソートし、重要度順に結果を返却

### 3. 拡張性

- Phase 3でLLM自動分類を実装する際の基盤として機能
- GetFilesByDomainメソッドでドメイン別のファイルリストを取得可能
- DomainCoverage構造体に将来的なメトリクス（平均品質スコア等）を追加可能

## 次のステップ

Phase 2の残りタスク：
- タスク7: カバレッジマップの構築（snapshot_filesテーブル追加）
- タスク8: カバレッジアラート機能

Phase 3（将来実装）：
- LLMによるドメイン自動分類（設計書9.4節）
- ファイル内容サンプルを使った高精度な分類
