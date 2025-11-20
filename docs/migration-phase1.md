# Phase 1 マイグレーションガイド

このドキュメントは、Phase 1（基本チャンキング拡張 + 構造メタデータ抽出 + Embedding強化）の機能を既存のdev-rag環境に適用するためのマイグレーション手順を説明します。

## 概要

Phase 1では、以下の拡張が行われました：

1. **スキーマ拡張**
   - `files`テーブルに`language`, `domain`カラムを追加
   - `chunks`テーブルに構造メタデータ、トレーサビリティ情報、`chunk_key`カラムを追加

2. **機能拡張**
   - Go言語のAST解析によるチャンク化
   - 構造メタデータの抽出（関数名、型、シグネチャ等）
   - 循環的複雑度、コメント比率などのコード品質メトリクス
   - Embeddingコンテキストの構築
   - トレーサビリティ情報の記録（Git commit hash, author, updated_at）
   - 決定的な識別子（chunk_key）の生成

3. **チャンク品質検証**
   - コメント比率95%以上のチャンクを除外
   - トークンサイズ制約の強化（100〜1600トークン）

## マイグレーション戦略

### 基本方針

- **ダウンタイムゼロ**: 既存データとの共存を保証
- **段階的移行**: 新規インデックスから新機能を適用
- **ロールバック可能**: 各ステップは独立して実行可能

### マイグレーションのフェーズ

1. **フェーズ1**: スキーマ変更（カラム追加）
2. **フェーズ2**: 既存データのバックフィル（オプショナル）
3. **フェーズ3**: 新規インデックスの実行
4. **フェーズ4**: 検証とモニタリング

## 前提条件

- PostgreSQL 12以上、pgvector拡張がインストール済み
- dev-ragのバージョン: v0.2.0以上
- データベースのバックアップが取得済み
- 管理者権限でのDB接続が可能

## マイグレーション手順

### ステップ1: データベースバックアップ

```bash
# PostgreSQLのバックアップを作成
pg_dump -h localhost -U devrag -d devrag > devrag_backup_$(date +%Y%m%d_%H%M%S).sql

# バックアップの確認
ls -lh devrag_backup_*.sql
```

### ステップ2: スキーマ変更の適用

#### 2.1 マイグレーションSQLの実行

```bash
# マイグレーションSQLを適用
psql -h localhost -U devrag -d devrag -f schema/migrations/002_add_chunk_metadata.up.sql
```

マイグレーションファイルの内容:

```sql
-- ファイルテーブルの拡張
ALTER TABLE files ADD COLUMN IF NOT EXISTS language VARCHAR(50);
ALTER TABLE files ADD COLUMN IF NOT EXISTS domain VARCHAR(50);

-- チャンクテーブルの拡張: 構造メタデータ
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS chunk_type VARCHAR(50);
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS chunk_name VARCHAR(255);
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS parent_name VARCHAR(255);
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS signature TEXT;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS doc_comment TEXT;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS imports JSONB;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS calls JSONB;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS lines_of_code INTEGER;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS comment_ratio NUMERIC(3,2);
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS cyclomatic_complexity INTEGER;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS embedding_context TEXT;

-- チャンクテーブルの拡張: トレーサビリティ情報
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS source_snapshot_id UUID REFERENCES source_snapshots(id) ON DELETE CASCADE;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS git_commit_hash VARCHAR(40);
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS author VARCHAR(255);
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS indexed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS file_version VARCHAR(100);
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS is_latest BOOLEAN NOT NULL DEFAULT true;

-- チャンクテーブルの拡張: 決定的な識別子
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS chunk_key VARCHAR(512);

-- インデックスの追加
CREATE INDEX IF NOT EXISTS idx_chunks_source_snapshot ON chunks(source_snapshot_id);
CREATE INDEX IF NOT EXISTS idx_chunks_git_commit_hash ON chunks(git_commit_hash);
CREATE INDEX IF NOT EXISTS idx_chunks_is_latest ON chunks(is_latest);
CREATE INDEX IF NOT EXISTS idx_chunks_indexed_at ON chunks(indexed_at);
CREATE INDEX IF NOT EXISTS idx_chunks_updated_at ON chunks(updated_at);
```

#### 2.2 スキーマ変更の確認

```bash
# テーブル定義の確認
psql -h localhost -U devrag -d devrag -c "\d+ files"
psql -h localhost -U devrag -d devrag -c "\d+ chunks"

# カラムが追加されたことを確認
psql -h localhost -U devrag -d devrag -c "
SELECT column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_name = 'files' AND column_name IN ('language', 'domain');
"

psql -h localhost -U devrag -d devrag -c "
SELECT column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_name = 'chunks'
AND column_name IN ('chunk_type', 'chunk_key', 'source_snapshot_id', 'is_latest');
"
```

### ステップ3: sqlcコードの再生成

```bash
# sqlcを使ってクエリコードを再生成
cd /path/to/dev-rag
sqlc generate

# 生成されたコードの確認
ls -l pkg/sqlc/*.sql.go
```

### ステップ4: 既存データのバックフィル（オプショナル）

既存チャンクに`source_snapshot_id`をバックフィルします。これにより、既存チャンクもトレーサビリティ情報を持つようになります。

```sql
-- 既存チャンクにsource_snapshot_idをバックフィル
UPDATE chunks c
SET source_snapshot_id = f.snapshot_id
FROM files f
WHERE c.file_id = f.id
AND c.source_snapshot_id IS NULL;

-- バックフィルの確認
SELECT
    COUNT(*) AS total_chunks,
    COUNT(source_snapshot_id) AS chunks_with_snapshot,
    COUNT(*) - COUNT(source_snapshot_id) AS chunks_without_snapshot
FROM chunks;
```

**注意**: `git_commit_hash`, `author`, `updated_at`のバックフィルは、既存スナップショットから取得できる場合のみ実行してください。取得できない場合は、NULLのままで問題ありません。

### ステップ5: アプリケーションのデプロイ

#### 5.1 バイナリのビルド

```bash
# 最新コードのビルド
cd /path/to/dev-rag
go build -o dev-rag cmd/dev-rag/main.go

# バージョン確認
./dev-rag version
```

#### 5.2 設定ファイルの更新

環境変数や設定ファイルに変更はありません。既存の設定をそのまま使用できます。

#### 5.3 アプリケーションの再起動

```bash
# systemdの場合
sudo systemctl restart dev-rag

# Dockerの場合
docker-compose restart dev-rag

# 起動確認
./dev-rag product list
```

### ステップ6: 新規インデックスの実行

Phase 1の機能を有効にするには、ソースを再インデックスする必要があります。

#### 6.1 テストインデックスの実行

まず、テスト用のプロダクト/ソースで新機能を検証します。

```bash
# テスト用プロダクトのインデックス実行
./dev-rag index git \
  --url git@github.com:your-org/test-repo.git \
  --product test-product \
  --ref main \
  --force-init

# インデックス結果の確認
./dev-rag source show --product test-product --source test-repo
```

#### 6.2 メタデータの検証

```sql
-- メタデータが設定されているチャンクを確認
SELECT
    c.id,
    c.chunk_type,
    c.chunk_name,
    c.parent_name,
    c.signature,
    c.chunk_key,
    c.source_snapshot_id,
    c.git_commit_hash,
    c.is_latest,
    f.path,
    f.language,
    f.domain
FROM chunks c
JOIN files f ON c.file_id = f.id
WHERE c.chunk_type IS NOT NULL
LIMIT 10;

-- 言語別のファイル数を確認
SELECT
    language,
    COUNT(*) AS file_count
FROM files
WHERE language IS NOT NULL
GROUP BY language
ORDER BY file_count DESC;

-- ドメイン別のファイル数を確認
SELECT
    domain,
    COUNT(*) AS file_count
FROM files
WHERE domain IS NOT NULL
GROUP BY domain
ORDER BY file_count DESC;

-- chunk_keyの一意性を確認
SELECT
    chunk_key,
    COUNT(*) AS count
FROM chunks
WHERE chunk_key IS NOT NULL
GROUP BY chunk_key
HAVING COUNT(*) > 1;
-- 結果が0件であることを確認（重複なし）
```

#### 6.3 本番プロダクトの再インデックス

テストが成功したら、本番プロダクトを順次再インデックスします。

```bash
# プロダクト一覧を取得
./dev-rag product list

# 各プロダクトを再インデックス（--force-initなしで差分更新）
for product in $(./dev-rag product list --format json | jq -r '.[].name'); do
  echo "Re-indexing product: $product"
  ./dev-rag source list --product "$product" --format json | \
    jq -r '.[].name' | \
    while read source; do
      echo "  Re-indexing source: $source"
      ./dev-rag index git \
        --url "$(get_source_url $product $source)" \
        --product "$product" \
        --ref main
    done
done
```

**注意**: 大量のデータを再インデックスする場合は、並列実行数を制限し、API Rate Limitに注意してください。

### ステップ7: モニタリングと検証

#### 7.1 インデックス統計の確認

```sql
-- 全体統計
SELECT
    COUNT(DISTINCT f.snapshot_id) AS snapshot_count,
    COUNT(DISTINCT f.id) AS file_count,
    COUNT(DISTINCT c.id) AS chunk_count,
    COUNT(DISTINCT e.id) AS embedding_count
FROM files f
LEFT JOIN chunks c ON f.id = c.file_id
LEFT JOIN embeddings e ON c.id = e.chunk_id;

-- メタデータ付きチャンクの割合
SELECT
    COUNT(*) AS total_chunks,
    COUNT(CASE WHEN chunk_type IS NOT NULL THEN 1 END) AS chunks_with_metadata,
    ROUND(100.0 * COUNT(CASE WHEN chunk_type IS NOT NULL THEN 1 END) / COUNT(*), 2) AS metadata_percentage
FROM chunks;

-- トレーサビリティ情報の充足率
SELECT
    COUNT(*) AS total_chunks,
    COUNT(source_snapshot_id) AS chunks_with_snapshot,
    COUNT(git_commit_hash) AS chunks_with_commit,
    COUNT(chunk_key) AS chunks_with_key,
    ROUND(100.0 * COUNT(source_snapshot_id) / COUNT(*), 2) AS snapshot_percentage,
    ROUND(100.0 * COUNT(git_commit_hash) / COUNT(*), 2) AS commit_percentage,
    ROUND(100.0 * COUNT(chunk_key) / COUNT(*), 2) AS key_percentage
FROM chunks;
```

#### 7.2 検索機能の検証

新しいメタデータが検索結果に反映されていることを確認します。

```bash
# 検索API（実装後に実行）
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "function definition",
    "productID": "your-product-id",
    "limit": 5
  }'
```

#### 7.3 ログの確認

```bash
# アプリケーションログを確認
tail -f /var/log/dev-rag/dev-rag.log | grep -E "(metadata|chunk_key|ast)"

# エラーログを確認
grep -i error /var/log/dev-rag/dev-rag.log | tail -20
```

## トラブルシューティング

### 問題1: マイグレーション失敗

**症状**: スキーマ変更が失敗する

**原因**: カラム追加時の制約違反、または既存データとの競合

**対処法**:
```bash
# マイグレーションをロールバック
psql -h localhost -U devrag -d devrag -f schema/migrations/002_add_chunk_metadata.down.sql

# バックアップから復元
psql -h localhost -U devrag -d devrag < devrag_backup_YYYYMMDD_HHMMSS.sql

# マイグレーションSQLを確認して再実行
```

### 問題2: chunk_keyの重複エラー

**症状**: `ERROR: duplicate key value violates unique constraint "uq_chunks_chunk_key"`

**原因**: 同じchunk_keyが複数回生成された

**対処法**:
```sql
-- 重複しているchunk_keyを確認
SELECT chunk_key, COUNT(*)
FROM chunks
WHERE chunk_key IS NOT NULL
GROUP BY chunk_key
HAVING COUNT(*) > 1;

-- 古いchunkを削除またはis_latestをfalseに設定
UPDATE chunks
SET is_latest = false
WHERE id IN (
    SELECT id
    FROM (
        SELECT id, ROW_NUMBER() OVER (PARTITION BY chunk_key ORDER BY indexed_at DESC) AS rn
        FROM chunks
        WHERE chunk_key IS NOT NULL
    ) t
    WHERE rn > 1
);
```

### 問題3: メタデータが抽出されない

**症状**: 再インデックス後もchunk_typeがNULL

**原因**: ファイルがGo言語として認識されていない、またはAST解析が失敗している

**対処法**:
```sql
-- ファイルの言語を確認
SELECT path, language, content_type
FROM files
WHERE language IS NULL OR language != 'Go';

-- ログを確認
grep -i "ast parsing failed" /var/log/dev-rag/dev-rag.log
```

解決策:
- ファイルの拡張子が`.go`であることを確認
- content_typeが`text/x-go`として検出されているか確認
- AST解析のエラーログを確認し、構文エラーがあれば修正

### 問題4: Embedding生成でAPI Rate Limitエラー

**症状**: `429 Too Many Requests` エラー

**原因**: OpenAI APIのRate Limitに達した

**対処法**:
```bash
# インデックス実行時にバッチサイズを調整（実装次第）
# または、インデックス実行を時間をおいて分割する

# 一時的に待機
sleep 60

# 再実行
./dev-rag index git --url ... --product ... --ref main
```

## ダウンタイムを最小化する方法

### 戦略1: Blue-Green Deployment

1. 新環境（Green）を構築し、Phase 1のコードとスキーマを適用
2. 新環境でインデックスを実行
3. 検証が完了したら、トラフィックを新環境に切り替え
4. 旧環境（Blue）を削除

### 戦略2: Rolling Update

1. スキーマ変更を先に適用（既存データとの共存を保証）
2. アプリケーションを段階的にアップデート
3. 新規インデックスから新機能を適用
4. 既存インデックスは再インデックス時に更新

**推奨**: 本番環境では戦略2（Rolling Update）を推奨します。Phase 1のスキーマ変更は後方互換性があり、既存機能に影響を与えません。

## ロールバック手順

Phase 1の適用後に問題が発生した場合は、以下の手順でロールバックできます。

### ステップ1: アプリケーションのロールバック

```bash
# 旧バージョンのバイナリをデプロイ
cp /path/to/backup/dev-rag /usr/local/bin/dev-rag

# 再起動
sudo systemctl restart dev-rag
```

### ステップ2: スキーマのロールバック（オプショナル）

**注意**: スキーマのロールバックは慎重に行ってください。新規カラムを削除すると、データが失われる可能性があります。

```bash
# マイグレーションのロールバック
psql -h localhost -U devrag -d devrag -f schema/migrations/002_add_chunk_metadata.down.sql
```

ロールバックSQL (`002_add_chunk_metadata.down.sql`):

```sql
-- インデックスの削除
DROP INDEX IF EXISTS idx_chunks_updated_at;
DROP INDEX IF EXISTS idx_chunks_indexed_at;
DROP INDEX IF EXISTS idx_chunks_is_latest;
DROP INDEX IF EXISTS idx_chunks_git_commit_hash;
DROP INDEX IF EXISTS idx_chunks_source_snapshot;

-- チャンクテーブルのカラム削除
ALTER TABLE chunks DROP COLUMN IF EXISTS chunk_key;
ALTER TABLE chunks DROP COLUMN IF EXISTS is_latest;
ALTER TABLE chunks DROP COLUMN IF EXISTS file_version;
ALTER TABLE chunks DROP COLUMN IF EXISTS indexed_at;
ALTER TABLE chunks DROP COLUMN IF EXISTS updated_at;
ALTER TABLE chunks DROP COLUMN IF EXISTS author;
ALTER TABLE chunks DROP COLUMN IF EXISTS git_commit_hash;
ALTER TABLE chunks DROP COLUMN IF EXISTS source_snapshot_id;
ALTER TABLE chunks DROP COLUMN IF EXISTS embedding_context;
ALTER TABLE chunks DROP COLUMN IF EXISTS cyclomatic_complexity;
ALTER TABLE chunks DROP COLUMN IF EXISTS comment_ratio;
ALTER TABLE chunks DROP COLUMN IF EXISTS lines_of_code;
ALTER TABLE chunks DROP COLUMN IF EXISTS calls;
ALTER TABLE chunks DROP COLUMN IF EXISTS imports;
ALTER TABLE chunks DROP COLUMN IF EXISTS doc_comment;
ALTER TABLE chunks DROP COLUMN IF EXISTS signature;
ALTER TABLE chunks DROP COLUMN IF EXISTS parent_name;
ALTER TABLE chunks DROP COLUMN IF EXISTS chunk_name;
ALTER TABLE chunks DROP COLUMN IF EXISTS chunk_type;

-- ファイルテーブルのカラム削除
ALTER TABLE files DROP COLUMN IF EXISTS domain;
ALTER TABLE files DROP COLUMN IF EXISTS language;
```

### ステップ3: データベースの復元（最終手段）

```bash
# バックアップから完全復元
psql -h localhost -U devrag -d devrag < devrag_backup_YYYYMMDD_HHMMSS.sql
```

## パフォーマンスへの影響

### インデックス作成時間

Phase 1では以下の追加処理が行われるため、インデックス作成時間が増加します：

- **AST解析**: Goファイルごとに約10〜50ms（ファイルサイズに依存）
- **メタデータ抽出**: チャンクごとに約1〜5ms
- **Embeddingコンテキスト構築**: チャンクごとに約1〜2ms

**見積**: 1000ファイルのリポジトリで、インデックス作成時間が約10〜20%増加

### データベースサイズ

- **chunksテーブル**: 1チャンクあたり約500バイト〜2KB増加（メタデータによる）
- **全体**: 1万チャンクで約5〜20MB増加

### 検索パフォーマンス

- **影響なし**: ベクトル検索の性能は変わりません
- **拡張機能**: chunk_keyによる検索、is_latestフィルタなどが利用可能に

## 次のステップ

Phase 1のマイグレーションが完了したら、以下のステップに進むことができます：

1. **Phase 2の準備**: 階層的チャンキング、依存関係抽出、カバレッジマップ
2. **検索機能の拡張**: メタデータを活用したフィルタリング、ランキング調整
3. **Wiki生成の強化**: コード構造を反映したドキュメント生成
4. **モニタリング**: メタデータ充足率、品質メトリクスのダッシュボード構築

## まとめ

このマイグレーションガイドでは、Phase 1の機能を既存環境に安全に適用する手順を説明しました。

**重要なポイント**:
- スキーマ変更は既存データとの共存を保証
- 新規インデックスから新機能を段階的に適用
- ロールバック可能な設計
- ダウンタイムゼロで移行可能

**質問やサポートが必要な場合**:
- GitHubのIssueを作成: https://github.com/your-org/dev-rag/issues
- ドキュメント: https://docs.dev-rag.example.com
