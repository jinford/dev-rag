# データベーススキーマ設計書

**システム名：社内リポジトリ向け RAG 基盤および Wiki 自動生成システム**

---

## 1. 概要

本ドキュメントでは、PostgreSQL + pgvector を用いたデータベーススキーマの詳細を定義する。

### 1.1 前提条件

- PostgreSQL バージョン: 14 以上
- pgvector 拡張: 0.5.0 以上
- UUID 拡張: uuid-ossp または gen_random_uuid() が使用可能

### 1.2 ER 図

```mermaid
erDiagram
    repositories ||--o{ snapshots : "has"
    snapshots ||--o{ files : "contains"
    snapshots ||--o{ wiki_metadata : "has"
    files ||--o{ chunks : "split into"
    chunks ||--|| embeddings : "has"

    repositories {
        uuid id PK
        varchar name UK
        text url
        varchar default_branch
        timestamp created_at
        timestamp updated_at
    }

    snapshots {
        uuid id PK
        uuid repository_id FK
        varchar commit_hash
        varchar ref_name
        boolean indexed
        timestamp indexed_at
        timestamp created_at
    }

    files {
        uuid id PK
        uuid snapshot_id FK
        text path
        bigint size
        varchar language
        varchar content_hash
        varchar source_type
        timestamp created_at
    }

    chunks {
        uuid id PK
        uuid file_id FK
        integer ordinal
        integer start_line
        integer end_line
        text content
        varchar content_hash
        integer token_count
        timestamp created_at
    }

    embeddings {
        uuid chunk_id PK_FK
        vector vector
        varchar model
        timestamp created_at
    }

    wiki_metadata {
        uuid id PK
        uuid repository_id FK
        uuid snapshot_id FK
        text output_path
        integer file_count
        timestamp generated_at
        timestamp created_at
    }
```

---

## 2. テーブル定義

### 2.1 repositories テーブル

リポジトリの基本情報を管理する。

```sql
CREATE TABLE repositories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    url TEXT NOT NULL,
    default_branch VARCHAR(100) NOT NULL DEFAULT 'main',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- インデックス
CREATE INDEX idx_repositories_name ON repositories(name);

-- コメント
COMMENT ON TABLE repositories IS 'Gitリポジトリの基本情報';
COMMENT ON COLUMN repositories.id IS 'リポジトリの一意識別子';
COMMENT ON COLUMN repositories.name IS 'リポジトリ名（一意）';
COMMENT ON COLUMN repositories.url IS 'GitリポジトリのURL（SSH/HTTPS）';
COMMENT ON COLUMN repositories.default_branch IS 'デフォルトブランチ名';
```

### 2.2 snapshots テーブル

リポジトリの特定コミット時点のスナップショットを管理する。

```sql
CREATE TABLE snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    commit_hash VARCHAR(40) NOT NULL,
    ref_name VARCHAR(255),
    indexed BOOLEAN NOT NULL DEFAULT FALSE,
    indexed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_snapshots_repo_commit UNIQUE (repository_id, commit_hash)
);

-- インデックス
CREATE INDEX idx_snapshots_repository_id ON snapshots(repository_id);
CREATE INDEX idx_snapshots_commit_hash ON snapshots(commit_hash);
CREATE INDEX idx_snapshots_ref_name ON snapshots(ref_name);
CREATE INDEX idx_snapshots_indexed ON snapshots(indexed) WHERE indexed = TRUE;

-- コメント
COMMENT ON TABLE snapshots IS 'リポジトリの特定コミット時点のスナップショット';
COMMENT ON COLUMN snapshots.id IS 'スナップショットの一意識別子';
COMMENT ON COLUMN snapshots.repository_id IS '対象リポジトリのID';
COMMENT ON COLUMN snapshots.commit_hash IS 'Gitコミットハッシュ（40文字のSHA-1）';
COMMENT ON COLUMN snapshots.ref_name IS '参照名（ブランチ名またはタグ名）';
COMMENT ON COLUMN snapshots.indexed IS 'インデックス完了フラグ';
COMMENT ON COLUMN snapshots.indexed_at IS 'インデックス完了日時';
```

### 2.3 files テーブル

スナップショット内のファイル情報を管理する。

```sql
CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id UUID NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    size BIGINT NOT NULL,
    language VARCHAR(50),
    content_hash VARCHAR(64) NOT NULL,
    source_type VARCHAR(20) NOT NULL CHECK (source_type IN ('code', 'doc', 'wiki')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_files_snapshot_path UNIQUE (snapshot_id, path)
);

-- インデックス
CREATE INDEX idx_files_snapshot_id ON files(snapshot_id);
CREATE INDEX idx_files_source_type ON files(snapshot_id, source_type);
CREATE INDEX idx_files_path ON files(path);
CREATE INDEX idx_files_content_hash ON files(content_hash);

-- コメント
COMMENT ON TABLE files IS 'スナップショット内のファイル情報';
COMMENT ON COLUMN files.id IS 'ファイルの一意識別子';
COMMENT ON COLUMN files.snapshot_id IS '所属するスナップショットのID';
COMMENT ON COLUMN files.path IS 'リポジトリルートからの相対パス';
COMMENT ON COLUMN files.size IS 'ファイルサイズ（バイト）';
COMMENT ON COLUMN files.language IS 'プログラミング言語種別';
COMMENT ON COLUMN files.content_hash IS 'ファイル内容のSHA-256ハッシュ';
COMMENT ON COLUMN files.source_type IS 'ソース種別（code/doc/wiki）';
```

### 2.4 chunks テーブル

ファイルを分割したチャンク情報を管理する。

```sql
CREATE TABLE chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,
    start_line INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    content TEXT NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    token_count INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_chunks_file_ordinal UNIQUE (file_id, ordinal),
    CONSTRAINT chk_chunks_lines CHECK (end_line >= start_line)
);

-- インデックス
CREATE INDEX idx_chunks_file_id ON chunks(file_id);
CREATE INDEX idx_chunks_file_ordinal ON chunks(file_id, ordinal);
CREATE INDEX idx_chunks_content_hash ON chunks(content_hash);

-- コメント
COMMENT ON TABLE chunks IS 'ファイルを分割したチャンク';
COMMENT ON COLUMN chunks.id IS 'チャンクの一意識別子';
COMMENT ON COLUMN chunks.file_id IS '所属するファイルのID';
COMMENT ON COLUMN chunks.ordinal IS 'ファイル内でのチャンク序数（0始まり）';
COMMENT ON COLUMN chunks.start_line IS 'チャンクの開始行番号';
COMMENT ON COLUMN chunks.end_line IS 'チャンクの終了行番号';
COMMENT ON COLUMN chunks.content IS 'チャンクのテキスト内容';
COMMENT ON COLUMN chunks.content_hash IS 'チャンク内容のSHA-256ハッシュ';
COMMENT ON COLUMN chunks.token_count IS '推定トークン数';
```

### 2.5 embeddings テーブル

チャンクのEmbeddingベクトルを管理する。

```sql
-- pgvector拡張が必要
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE embeddings (
    chunk_id UUID PRIMARY KEY REFERENCES chunks(id) ON DELETE CASCADE,
    vector VECTOR(1536) NOT NULL,
    model VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ベクトル検索用インデックス（IVFFlat）
-- lists パラメータは総チャンク数に応じて調整（目安: sqrt(総行数)）
CREATE INDEX idx_embeddings_vector_cosine ON embeddings
USING ivfflat (vector vector_cosine_ops)
WITH (lists = 100);

-- 必要に応じて他の距離関数用のインデックスも作成可能
-- CREATE INDEX idx_embeddings_vector_l2 ON embeddings
-- USING ivfflat (vector vector_l2_ops)
-- WITH (lists = 100);

-- コメント
COMMENT ON TABLE embeddings IS 'チャンクのEmbeddingベクトル';
COMMENT ON COLUMN embeddings.chunk_id IS 'チャンクID（主キー兼外部キー）';
COMMENT ON COLUMN embeddings.vector IS 'Embeddingベクトル（1536次元）';
COMMENT ON COLUMN embeddings.model IS '使用したEmbeddingモデル名';
```

### 2.6 wiki_metadata テーブル

Wiki生成の実行履歴とメタデータを管理する。

```sql
CREATE TABLE wiki_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    snapshot_id UUID NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    output_path TEXT NOT NULL,
    file_count INTEGER NOT NULL DEFAULT 0,
    generated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_wiki_metadata_snapshot UNIQUE (snapshot_id)
);

-- インデックス
CREATE INDEX idx_wiki_metadata_repository_id ON wiki_metadata(repository_id);
CREATE INDEX idx_wiki_metadata_snapshot_id ON wiki_metadata(snapshot_id);
CREATE INDEX idx_wiki_metadata_generated_at ON wiki_metadata(generated_at DESC);

-- コメント
COMMENT ON TABLE wiki_metadata IS 'Wiki生成の実行履歴とメタデータ';
COMMENT ON COLUMN wiki_metadata.id IS 'Wiki生成レコードの一意識別子';
COMMENT ON COLUMN wiki_metadata.repository_id IS '対象リポジトリのID';
COMMENT ON COLUMN wiki_metadata.snapshot_id IS '対象スナップショットのID';
COMMENT ON COLUMN wiki_metadata.output_path IS 'Wikiファイルの出力先パス（例: /var/lib/dev-rag/wikis/myapp/）';
COMMENT ON COLUMN wiki_metadata.file_count IS '生成されたWikiファイル数';
COMMENT ON COLUMN wiki_metadata.generated_at IS 'Wiki生成完了日時';
```

---

## 3. マイグレーション戦略

### 3.1 マイグレーションツール

- **ツール**: golang-migrate/migrate または goose
- **ファイル命名**: `<version>_<description>.up.sql` / `<version>_<description>.down.sql`

### 3.2 マイグレーションファイル例

#### 001_init_schema.up.sql

```sql
-- pgvector拡張のインストール
CREATE EXTENSION IF NOT EXISTS vector;

-- repositoriesテーブル
CREATE TABLE repositories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    url TEXT NOT NULL,
    default_branch VARCHAR(100) NOT NULL DEFAULT 'main',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_repositories_name ON repositories(name);

-- snapshotsテーブル
CREATE TABLE snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    commit_hash VARCHAR(40) NOT NULL,
    ref_name VARCHAR(255),
    indexed BOOLEAN NOT NULL DEFAULT FALSE,
    indexed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_snapshots_repo_commit UNIQUE (repository_id, commit_hash)
);

CREATE INDEX idx_snapshots_repository_id ON snapshots(repository_id);
CREATE INDEX idx_snapshots_commit_hash ON snapshots(commit_hash);
CREATE INDEX idx_snapshots_ref_name ON snapshots(ref_name);
CREATE INDEX idx_snapshots_indexed ON snapshots(indexed) WHERE indexed = TRUE;

-- filesテーブル
CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id UUID NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    size BIGINT NOT NULL,
    language VARCHAR(50),
    content_hash VARCHAR(64) NOT NULL,
    source_type VARCHAR(20) NOT NULL CHECK (source_type IN ('code', 'doc', 'wiki')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_files_snapshot_path UNIQUE (snapshot_id, path)
);

CREATE INDEX idx_files_snapshot_id ON files(snapshot_id);
CREATE INDEX idx_files_source_type ON files(snapshot_id, source_type);
CREATE INDEX idx_files_path ON files(path);
CREATE INDEX idx_files_content_hash ON files(content_hash);

-- chunksテーブル
CREATE TABLE chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,
    start_line INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    content TEXT NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    token_count INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_chunks_file_ordinal UNIQUE (file_id, ordinal),
    CONSTRAINT chk_chunks_lines CHECK (end_line >= start_line)
);

CREATE INDEX idx_chunks_file_id ON chunks(file_id);
CREATE INDEX idx_chunks_file_ordinal ON chunks(file_id, ordinal);
CREATE INDEX idx_chunks_content_hash ON chunks(content_hash);

-- embeddingsテーブル
CREATE TABLE embeddings (
    chunk_id UUID PRIMARY KEY REFERENCES chunks(id) ON DELETE CASCADE,
    vector VECTOR(1536) NOT NULL,
    model VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_embeddings_vector_cosine ON embeddings
USING ivfflat (vector vector_cosine_ops)
WITH (lists = 100);

-- wiki_metadataテーブル
CREATE TABLE wiki_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    snapshot_id UUID NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    output_path TEXT NOT NULL,
    file_count INTEGER NOT NULL DEFAULT 0,
    generated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_wiki_metadata_snapshot UNIQUE (snapshot_id)
);

CREATE INDEX idx_wiki_metadata_repository_id ON wiki_metadata(repository_id);
CREATE INDEX idx_wiki_metadata_snapshot_id ON wiki_metadata(snapshot_id);
CREATE INDEX idx_wiki_metadata_generated_at ON wiki_metadata(generated_at DESC);
```

#### 001_init_schema.down.sql

```sql
DROP TABLE IF EXISTS wiki_metadata;
DROP TABLE IF EXISTS embeddings;
DROP TABLE IF EXISTS chunks;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS snapshots;
DROP TABLE IF EXISTS repositories;
DROP EXTENSION IF EXISTS vector;
```

---

## 4. クエリ例

### 4.1 基本的なCRUD操作

#### リポジトリ登録

```sql
INSERT INTO repositories (name, url, default_branch)
VALUES ('my-service', 'git@github.com:example/my-service.git', 'main')
RETURNING id, name, created_at;
```

#### スナップショット作成

```sql
INSERT INTO snapshots (repository_id, commit_hash, ref_name)
VALUES ('550e8400-e29b-41d4-a716-446655440000', 'abc123def456...', 'main')
ON CONFLICT (repository_id, commit_hash) DO UPDATE
SET ref_name = EXCLUDED.ref_name
RETURNING id;
```

#### ファイル登録

```sql
INSERT INTO files (snapshot_id, path, size, language, content_hash, source_type)
VALUES
    ('660e8400-e29b-41d4-a716-446655440000', 'src/main.go', 1024, 'go', 'hash123', 'code'),
    ('660e8400-e29b-41d4-a716-446655440000', 'README.md', 512, 'markdown', 'hash456', 'doc')
ON CONFLICT (snapshot_id, path) DO NOTHING;
```

#### チャンクとEmbedding登録

```sql
-- チャンク登録
INSERT INTO chunks (file_id, ordinal, start_line, end_line, content, content_hash, token_count)
VALUES ('770e8400-e29b-41d4-a716-446655440000', 0, 1, 50, 'package main...', 'chunkhash1', 200)
RETURNING id;

-- Embedding登録
INSERT INTO embeddings (chunk_id, vector, model)
VALUES ('880e8400-e29b-41d4-a716-446655440000', '[0.1, 0.2, ..., 0.9]', 'text-embedding-3-small');
```

### 4.2 ベクトル検索クエリ

#### 基本的なベクトル検索

```sql
WITH target_snapshot AS (
    SELECT s.id
    FROM snapshots s
    JOIN repositories r ON r.id = s.repository_id
    WHERE r.name = 'my-service'
      AND s.ref_name = 'main'
      AND s.indexed = TRUE
    ORDER BY s.created_at DESC
    LIMIT 1
)
SELECT
    f.path,
    c.start_line,
    c.end_line,
    c.content,
    -- スコア計算: 1 - cosine_distance で正規化（0〜1、1が最も類似）
    1 - (e.vector <=> $1::vector) AS score
FROM embeddings e
JOIN chunks c ON c.id = e.chunk_id
JOIN files f ON f.id = c.file_id
WHERE f.snapshot_id = (SELECT id FROM target_snapshot)
ORDER BY e.vector <=> $1::vector
LIMIT 10;
```

**パラメータ:**
- `$1`: クエリのEmbeddingベクトル（VECTOR型）

#### フィルタ付きベクトル検索

```sql
WITH target_snapshot AS (
    SELECT s.id
    FROM snapshots s
    JOIN repositories r ON r.id = s.repository_id
    WHERE r.name = $1  -- リポジトリ名
      AND s.ref_name = $2  -- ブランチ名
      AND s.indexed = TRUE
    ORDER BY s.created_at DESC
    LIMIT 1
)
SELECT
    f.path,
    c.start_line,
    c.end_line,
    c.content,
    -- スコア計算: 1 - cosine_distance で正規化（0〜1、1が最も類似）
    1 - (e.vector <=> $3::vector) AS score
FROM embeddings e
JOIN chunks c ON c.id = e.chunk_id
JOIN files f ON f.id = c.file_id
WHERE f.snapshot_id = (SELECT id FROM target_snapshot)
  AND ($4::text IS NULL OR f.path LIKE $4 || '%')  -- パスプレフィックス
  AND ($5::text IS NULL OR f.source_type = $5)     -- ソース種別
ORDER BY e.vector <=> $3::vector
LIMIT $6;  -- 取得件数
```

**パラメータ:**
- `$1`: リポジトリ名
- `$2`: 参照名（ブランチ/タグ）
- `$3`: クエリベクトル
- `$4`: パスプレフィックス（オプション）
- `$5`: ソース種別（オプション）
- `$6`: 取得件数

#### 前後コンテキスト付き検索

```sql
WITH target_snapshot AS (
    SELECT s.id
    FROM snapshots s
    JOIN repositories r ON r.id = s.repository_id
    WHERE r.name = $1 AND s.ref_name = $2 AND s.indexed = TRUE
    ORDER BY s.created_at DESC
    LIMIT 1
),
top_chunks AS (
    SELECT
        c.id,
        c.file_id,
        c.ordinal,
        f.path,
        c.start_line,
        c.end_line,
        c.content,
        -- スコア計算: 1 - cosine_distance で正規化（0〜1、1が最も類似）
        1 - (e.vector <=> $3::vector) AS score
    FROM embeddings e
    JOIN chunks c ON c.id = e.chunk_id
    JOIN files f ON f.id = c.file_id
    WHERE f.snapshot_id = (SELECT id FROM target_snapshot)
    ORDER BY e.vector <=> $3::vector
    LIMIT $4
)
SELECT
    tc.path,
    tc.start_line,
    tc.end_line,
    tc.content,
    tc.score,
    -- 前のチャンク（オプション）
    prev.content AS prev_content,
    prev.start_line AS prev_start_line,
    prev.end_line AS prev_end_line,
    -- 後のチャンク（オプション）
    next.content AS next_content,
    next.start_line AS next_start_line,
    next.end_line AS next_end_line
FROM top_chunks tc
LEFT JOIN chunks prev ON prev.file_id = tc.file_id AND prev.ordinal = tc.ordinal - 1
LEFT JOIN chunks next ON next.file_id = tc.file_id AND next.ordinal = tc.ordinal + 1
ORDER BY tc.score DESC;
```

### 4.3 差分インデックス用クエリ

#### 既存ファイルハッシュ取得

```sql
SELECT path, content_hash
FROM files
WHERE snapshot_id = $1;
```

#### 変更されたファイルの検出と処理

```sql
-- アプリケーション側で新旧のファイルリストを比較
-- 新規: 旧スナップショットに存在しないパス → インデックス作成
-- 削除: 新スナップショットに存在しないパス → DELETE実行
-- 変更: content_hash が異なるファイル → chunks/embeddings削除後、再インデックス
-- リネーム: 削除+追加として扱う（ハッシュ一致でも再インデックス）
```

#### 削除されたファイルの除去

```sql
-- 削除されたファイルをcascade削除（chunks/embeddingsも自動削除）
DELETE FROM files
WHERE snapshot_id = $1  -- 新スナップショットID
  AND path = ANY($2::text[]);  -- 削除対象パス配列

-- または、新スナップショットに存在しないファイルを一括削除
DELETE FROM files
WHERE snapshot_id = $1
  AND path NOT IN (SELECT unnest($2::text[]));  -- 新スナップショットのファイルリスト
```

#### 変更されたファイルのチャンク・Embedding削除

```sql
-- ファイル更新時（古いチャンクを削除して新規作成）
DELETE FROM chunks WHERE file_id = $1;

-- Embeddingはcascade削除で自動的に削除される
```

### 4.4 Wiki生成関連クエリ

#### Wiki生成メタデータ登録

```sql
INSERT INTO wiki_metadata (repository_id, snapshot_id, output_path, file_count, generated_at)
VALUES (
    $1,  -- repository_id
    $2,  -- snapshot_id
    $3,  -- output_path (例: /var/lib/dev-rag/wikis/myapp/)
    $4,  -- file_count
    CURRENT_TIMESTAMP
)
ON CONFLICT (snapshot_id) DO UPDATE
SET output_path = EXCLUDED.output_path,
    file_count = EXCLUDED.file_count,
    generated_at = CURRENT_TIMESTAMP
RETURNING id, generated_at;
```

#### 最新Wiki情報の取得

```sql
SELECT
    r.name AS repository_name,
    s.commit_hash,
    s.ref_name,
    wm.output_path,
    wm.file_count,
    wm.generated_at
FROM wiki_metadata wm
JOIN snapshots s ON s.id = wm.snapshot_id
JOIN repositories r ON r.id = wm.repository_id
WHERE r.name = $1  -- リポジトリ名
ORDER BY wm.generated_at DESC
LIMIT 1;
```

#### リポジトリごとのWiki生成履歴

```sql
SELECT
    r.name,
    s.commit_hash,
    s.ref_name,
    wm.file_count,
    wm.generated_at
FROM wiki_metadata wm
JOIN snapshots s ON s.id = wm.snapshot_id
JOIN repositories r ON r.id = wm.repository_id
WHERE r.name = $1  -- リポジトリ名
ORDER BY wm.generated_at DESC
LIMIT 10;
```

### 4.5 統計・メタデータクエリ

#### リポジトリごとのチャンク数

```sql
SELECT
    r.name,
    COUNT(c.id) AS chunk_count,
    SUM(c.token_count) AS total_tokens
FROM repositories r
JOIN snapshots s ON s.repository_id = r.id
JOIN files f ON f.snapshot_id = s.id
JOIN chunks c ON c.file_id = f.id
WHERE s.indexed = TRUE
GROUP BY r.id, r.name
ORDER BY chunk_count DESC;
```

#### 最新インデックス状況

```sql
SELECT
    r.name,
    s.commit_hash,
    s.ref_name,
    s.indexed,
    s.indexed_at,
    COUNT(DISTINCT f.id) AS file_count,
    COUNT(c.id) AS chunk_count
FROM repositories r
JOIN snapshots s ON s.repository_id = r.id
LEFT JOIN files f ON f.snapshot_id = s.id
LEFT JOIN chunks c ON c.file_id = f.id
WHERE s.id IN (
    SELECT DISTINCT ON (repository_id) id
    FROM snapshots
    WHERE indexed = TRUE
    ORDER BY repository_id, created_at DESC
)
GROUP BY r.id, r.name, s.commit_hash, s.ref_name, s.indexed, s.indexed_at
ORDER BY r.name;
```

#### インデックス・Wiki統合情報

```sql
SELECT
    r.name,
    s.commit_hash,
    s.ref_name,
    s.indexed,
    s.indexed_at,
    COUNT(DISTINCT f.id) AS file_count,
    COUNT(c.id) AS chunk_count,
    wm.output_path AS wiki_path,
    wm.file_count AS wiki_file_count,
    wm.generated_at AS wiki_generated_at
FROM repositories r
JOIN snapshots s ON s.repository_id = r.id
LEFT JOIN files f ON f.snapshot_id = s.id
LEFT JOIN chunks c ON c.file_id = f.id
LEFT JOIN wiki_metadata wm ON wm.snapshot_id = s.id
WHERE r.name = $1  -- リポジトリ名
  AND s.indexed = TRUE
ORDER BY s.created_at DESC
LIMIT 1;
```

---

## 5. パフォーマンスチューニング

### 5.1 pgvector インデックスの最適化

#### lists パラメータの調整

```sql
-- チャンク数が10万件の場合
-- lists = sqrt(100000) ≈ 316
DROP INDEX IF EXISTS idx_embeddings_vector_cosine;
CREATE INDEX idx_embeddings_vector_cosine ON embeddings
USING ivfflat (vector vector_cosine_ops)
WITH (lists = 316);
```

#### VACUUM と ANALYZE

```sql
-- 定期的に実行してインデックスを最適化
VACUUM ANALYZE embeddings;
VACUUM ANALYZE chunks;
VACUUM ANALYZE files;
```

### 5.2 クエリ最適化

#### EXPLAIN ANALYZE の活用

```sql
EXPLAIN ANALYZE
SELECT ...
FROM embeddings e
JOIN chunks c ON c.id = e.chunk_id
WHERE ...
ORDER BY e.vector <=> $1::vector
LIMIT 10;
```

#### 効率的なJOIN順序

- ベクトル検索は `embeddings` テーブルから開始
- フィルタ条件（snapshot_id, source_type等）を先に適用
- 必要な情報のみをJOINで取得

### 5.3 接続プーリング

```go
// Go側の設定例
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

---

## 6. バックアップ・リストア

### 6.1 論理バックアップ（pg_dump）

```bash
# データベース全体をバックアップ
pg_dump -h localhost -U devrag -d devrag -F c -f devrag_backup.dump

# リストア
pg_restore -h localhost -U devrag -d devrag_new -c devrag_backup.dump
```

### 6.2 物理バックアップ（pg_basebackup）

```bash
# ベースバックアップ
pg_basebackup -h localhost -U replication -D /backup/pg_base -Fp -Xs -P

# WALアーカイブと組み合わせてPITR（Point-In-Time Recovery）も可能
```

### 6.3 Embeddingの再生成

万が一、Embeddingデータが破損した場合は、チャンクから再生成可能。

```sql
-- 破損したEmbeddingを削除
TRUNCATE embeddings;

-- アプリケーション側で全チャンクを読み取り、再度Embedding生成
```

---

## 7. セキュリティ設定

### 7.1 ロールと権限

```sql
-- アプリケーション用ロール作成
CREATE ROLE devrag_app WITH LOGIN PASSWORD 'secure_password';

-- 必要最小限の権限を付与
GRANT CONNECT ON DATABASE devrag TO devrag_app;
GRANT USAGE ON SCHEMA public TO devrag_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO devrag_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO devrag_app;

-- 将来作成されるテーブルにも権限を付与
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO devrag_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO devrag_app;
```

### 7.2 SSL接続

```sql
-- postgresql.conf
ssl = on
ssl_cert_file = 'server.crt'
ssl_key_file = 'server.key'

-- pg_hba.conf
hostssl all all 0.0.0.0/0 md5
```

### 7.3 行レベルセキュリティ（RLS）

将来的にマルチテナント対応する場合は、RLSを検討。

```sql
-- 例: リポジトリごとのアクセス制御
ALTER TABLE files ENABLE ROW LEVEL SECURITY;

CREATE POLICY files_access_policy ON files
    FOR ALL
    TO devrag_app
    USING (snapshot_id IN (
        SELECT s.id FROM snapshots s
        JOIN repositories r ON r.id = s.repository_id
        WHERE r.name = current_setting('app.current_repo')
    ));
```

---

## 8. モニタリング

### 8.1 重要なメトリクス

- **テーブルサイズ**: `pg_total_relation_size('embeddings')`
- **インデックスサイズ**: `pg_relation_size('idx_embeddings_vector_cosine')`
- **クエリ実行時間**: `pg_stat_statements` 拡張
- **接続数**: `pg_stat_activity`

### 8.2 スロークエリログ

```sql
-- postgresql.conf
log_min_duration_statement = 1000  -- 1秒以上のクエリをログ出力
log_line_prefix = '%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h '
```

---

## 9. まとめ

本設計書では、以下を定義した：

1. 全テーブルのスキーマ定義（repositories, snapshots, files, chunks, embeddings）
2. ER図とテーブル間の関係
3. マイグレーション戦略とSQLファイル例
4. ベクトル検索を含む各種クエリ例
5. パフォーマンスチューニング方針
6. バックアップ・リストア手順
7. セキュリティ設定とモニタリング

次のステップとして、以下を推奨する：

- マイグレーションツールのセットアップ
- 開発環境でのDB構築
- ベンチマークテストによるインデックスパラメータの調整
