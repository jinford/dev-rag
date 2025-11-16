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
    products ||--o{ sources : "has"
    sources ||--o{ source_snapshots : "has"
    sources ||--o{ git_refs : "has"
    git_refs }o--|| source_snapshots : "points to"
    source_snapshots ||--o{ files : "contains"
    products ||--o{ wiki_metadata : "generates"
    files ||--o{ chunks : "split into"
    chunks ||--|| embeddings : "has"

    products {
        uuid id PK
        varchar name UK
        text description
        timestamp created_at
        timestamp updated_at
    }

    sources {
        uuid id PK
        uuid product_id FK
        varchar name UK
        varchar source_type
        jsonb metadata
        timestamp created_at
        timestamp updated_at
    }

    source_snapshots {
        uuid id PK
        uuid source_id FK
        text version_identifier
        boolean indexed
        timestamp indexed_at
        timestamp created_at
    }

    git_refs {
        uuid id PK
        uuid source_id FK
        varchar ref_name
        uuid snapshot_id FK
        timestamp created_at
        timestamp updated_at
    }

    files {
        uuid id PK
        uuid snapshot_id FK
        text path
        bigint size
        varchar content_type
        varchar content_hash
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
        uuid product_id FK
        text output_path
        integer file_count
        timestamp generated_at
        timestamp created_at
    }
```

---

## 2. テーブル定義

### 2.1 products テーブル

プロダクト（複数のソースをまとめる単位）の基本情報を管理する。

```sql
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- インデックス
CREATE INDEX idx_products_name ON products(name);

-- コメント
COMMENT ON TABLE products IS 'プロダクト（複数のソースをまとめる単位）';
COMMENT ON COLUMN products.id IS 'プロダクトの一意識別子';
COMMENT ON COLUMN products.name IS 'プロダクト名（一意）';
COMMENT ON COLUMN products.description IS 'プロダクトの説明';
```

**使用例:**

```sql
-- ECサイトプロダクト
INSERT INTO products (name, description) VALUES (
  'my-ecommerce',
  'ECサイトプロダクト（フロントエンド、バックエンド、インフラ、ドキュメントを含む）'
);

-- 監視システムプロダクト
INSERT INTO products (name, description) VALUES (
  'monitoring-system',
  '監視システム（Prometheus、Grafana、アラート設定を含む）'
);
```

### 2.2 sources テーブル

情報ソース（Git、Confluence、PDF等）の基本情報を管理する。

```sql
CREATE TABLE sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID REFERENCES products(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL UNIQUE,
    source_type VARCHAR(50) NOT NULL CHECK (source_type IN ('git', 'confluence', 'pdf', 'redmine', 'notion', 'local')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- インデックス
CREATE INDEX idx_sources_name ON sources(name);
CREATE INDEX idx_sources_type ON sources(source_type);
CREATE INDEX idx_sources_product_id ON sources(product_id);

-- コメント
COMMENT ON TABLE sources IS 'ドキュメント・コードのソース情報（Git、Confluence、PDFなど）';
COMMENT ON COLUMN sources.id IS 'ソースの一意識別子';
COMMENT ON COLUMN sources.product_id IS '所属するプロダクトのID（NULLの場合は未分類）';
COMMENT ON COLUMN sources.name IS 'ソース名（一意）';
COMMENT ON COLUMN sources.source_type IS 'ソースタイプ（git/confluence/pdf/redmine/notion/local）';
COMMENT ON COLUMN sources.metadata IS 'ソースタイプ固有の情報（JSONBフォーマット）';
```

**metadata カラムの使用例:**

```sql
-- Gitリポジトリの場合（プロダクトに属する）
INSERT INTO sources (name, source_type, product_id, metadata) VALUES (
  'my-ecommerce-backend',
  'git',
  'ecommerce-product-uuid',
  '{"url": "git@github.com:example/backend.git", "default_branch": "main"}'::jsonb
);

-- Confluenceスペースの場合（プロダクトに属する）
INSERT INTO sources (name, source_type, product_id, metadata) VALUES (
  'my-ecommerce-docs',
  'confluence',
  'ecommerce-product-uuid',
  '{"base_url": "https://confluence.example.com", "space_key": "ECOM", "username": "bot@example.com"}'::jsonb
);

-- 未分類のソース（product_id = NULL）
INSERT INTO sources (name, source_type, metadata) VALUES (
  'shared-library',
  'git',
  '{"url": "git@github.com:example/shared-lib.git", "default_branch": "main"}'::jsonb
);
```

### 2.3 source_snapshots テーブル

ソースの特定バージョン時点のスナップショットを管理する。

```sql
CREATE TABLE source_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    version_identifier TEXT NOT NULL,
    indexed BOOLEAN NOT NULL DEFAULT FALSE,
    indexed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_source_snapshots_source_version UNIQUE (source_id, version_identifier)
);

-- インデックス
CREATE INDEX idx_source_snapshots_source_id ON source_snapshots(source_id);
CREATE INDEX idx_source_snapshots_version ON source_snapshots(version_identifier);
CREATE INDEX idx_source_snapshots_indexed ON source_snapshots(indexed) WHERE indexed = TRUE;

-- コメント
COMMENT ON TABLE source_snapshots IS 'ソースの特定バージョン時点のスナップショット';
COMMENT ON COLUMN source_snapshots.id IS 'スナップショットの一意識別子';
COMMENT ON COLUMN source_snapshots.source_id IS '対象ソースのID';
COMMENT ON COLUMN source_snapshots.version_identifier IS 'バージョン識別子（Gitの場合はcommit_hash、Confluenceの場合はpage_version等）';
COMMENT ON COLUMN source_snapshots.indexed IS 'インデックス完了フラグ';
COMMENT ON COLUMN source_snapshots.indexed_at IS 'インデックス完了日時';
```

**version_identifier の使い分け例:**

| ソースタイプ | version_identifier の例 |
|------------|------------------------|
| git | `abc123def456...` (commit hash) |
| confluence | `12` (page version number) |
| pdf | `sha256:abc123...` (file hash) |
| notion | `2024-01-15T10:30:00Z` (last edited timestamp) |
```

### 2.4 files テーブル

スナップショット内のファイル・ドキュメント情報を管理する。

```sql
CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id UUID NOT NULL REFERENCES source_snapshots(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    size BIGINT NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_files_snapshot_path UNIQUE (snapshot_id, path)
);

-- インデックス
CREATE INDEX idx_files_snapshot_id ON files(snapshot_id);
CREATE INDEX idx_files_content_type ON files(content_type);
CREATE INDEX idx_files_path ON files(path);
CREATE INDEX idx_files_content_hash ON files(content_hash);

-- コメント
COMMENT ON TABLE files IS 'スナップショット内のファイル・ドキュメント情報';
COMMENT ON COLUMN files.id IS 'ファイルの一意識別子';
COMMENT ON COLUMN files.snapshot_id IS '所属するスナップショットのID';
COMMENT ON COLUMN files.path IS 'ソースルートからの相対パス（またはドキュメント識別子）';
COMMENT ON COLUMN files.size IS 'ファイルサイズ（バイト）';
COMMENT ON COLUMN files.content_type IS 'MIMEタイプ形式のコンテンツ種別（例: text/x-go, text/x-python, text/markdown, application/pdf, text/html）';
COMMENT ON COLUMN files.content_hash IS 'ファイル内容のSHA-256ハッシュ';
```

**カラムの使い分け例:**

| ソースタイプ | path の例 | content_type の例 |
|------------|----------|------------------|
| git | `src/main.go`, `pkg/server/http.go` | `text/x-go` |
| git | `README.md`, `docs/api.md` | `text/markdown` |
| git | `src/index.js` | `text/javascript` |
| confluence | `TEAM/Engineering/Architecture` | `text/html` |
| pdf | `design-specs/system-architecture.pdf` | `application/pdf` |
| notion | `Engineering/RFCs/RFC-001` | `text/markdown` |

**主要なMIMEタイプ:**

| 言語/形式 | content_type |
|---------|--------------|
| Go | `text/x-go` |
| Python | `text/x-python` |
| JavaScript | `text/javascript` |
| TypeScript | `text/typescript` |
| Java | `text/x-java` |
| Markdown | `text/markdown` |
| HTML | `text/html` |
| JSON | `application/json` |
| YAML | `application/x-yaml` |
| PDF | `application/pdf` |
```

### 2.5 git_refs テーブル

Git専用の参照（ブランチ、タグ）管理テーブル。

```sql
CREATE TABLE git_refs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    ref_name VARCHAR(255) NOT NULL,
    snapshot_id UUID NOT NULL REFERENCES source_snapshots(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_git_refs_source_ref UNIQUE (source_id, ref_name)
);

-- インデックス
CREATE INDEX idx_git_refs_source_id ON git_refs(source_id);
CREATE INDEX idx_git_refs_snapshot_id ON git_refs(snapshot_id);
CREATE INDEX idx_git_refs_ref_name ON git_refs(ref_name);

-- コメント
COMMENT ON TABLE git_refs IS 'Git専用の参照（ブランチ、タグ）管理';
COMMENT ON COLUMN git_refs.id IS 'Git参照の一意識別子';
COMMENT ON COLUMN git_refs.source_id IS '対象ソースのID（source_type=gitのみ）';
COMMENT ON COLUMN git_refs.ref_name IS '参照名（ブランチ名またはタグ名: main, develop, v1.0.0 等）';
COMMENT ON COLUMN git_refs.snapshot_id IS '参照が指すスナップショットのID';
COMMENT ON COLUMN git_refs.created_at IS '参照の作成日時';
COMMENT ON COLUMN git_refs.updated_at IS '参照の更新日時（別のコミットを指すようになった時）';
```

**使用例:**

```sql
-- backend-api ソースの main ブランチを commit abc123 に設定
INSERT INTO git_refs (source_id, ref_name, snapshot_id)
VALUES (
    'backend-api-uuid',
    'main',
    'snapshot-abc123-uuid'
)
ON CONFLICT (source_id, ref_name) DO UPDATE
SET snapshot_id = EXCLUDED.snapshot_id,
    updated_at = CURRENT_TIMESTAMP;

-- 同じコミットに v1.0.0 タグを追加
INSERT INTO git_refs (source_id, ref_name, snapshot_id)
VALUES (
    'backend-api-uuid',
    'v1.0.0',
    'snapshot-abc123-uuid'  -- 同じスナップショットを指す
)
ON CONFLICT (source_id, ref_name) DO UPDATE
SET snapshot_id = EXCLUDED.snapshot_id,
    updated_at = CURRENT_TIMESTAMP;

-- backend-api ソースの main ブランチが指す最新スナップショットを取得
SELECT ss.*
FROM git_refs gr
JOIN source_snapshots ss ON ss.id = gr.snapshot_id
WHERE gr.source_id = 'backend-api-uuid'
  AND gr.ref_name = 'main'
  AND ss.indexed = TRUE;
```

**設計のポイント:**

- `UNIQUE (source_id, ref_name)`: 1つのソースの1つのブランチ/タグは1つのスナップショットのみを指す
- 同じコミット（スナップショット）を複数の参照（main と v1.0.0）で指すことが可能
- Git以外のソースタイプ（Confluence、PDF等）にはこのテーブルを使用しない

### 2.6 chunks テーブル

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

### 2.7 embeddings テーブル

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

### 2.8 wiki_metadata テーブル

Wiki生成の実行履歴とメタデータを管理する。

```sql
CREATE TABLE wiki_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    output_path TEXT NOT NULL,
    file_count INTEGER NOT NULL DEFAULT 0,
    generated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_wiki_metadata_product UNIQUE (product_id)
);

-- インデックス
CREATE INDEX idx_wiki_metadata_product_id ON wiki_metadata(product_id);
CREATE INDEX idx_wiki_metadata_generated_at ON wiki_metadata(generated_at DESC);

-- コメント
COMMENT ON TABLE wiki_metadata IS 'Wiki生成の実行履歴とメタデータ（プロダクト単位のみ）';
COMMENT ON COLUMN wiki_metadata.id IS 'Wiki生成レコードの一意識別子';
COMMENT ON COLUMN wiki_metadata.product_id IS '対象プロダクトのID';
COMMENT ON COLUMN wiki_metadata.output_path IS 'Wikiファイルの出力先パス（例: /var/lib/dev-rag/wikis/my-ecommerce/）';
COMMENT ON COLUMN wiki_metadata.file_count IS '生成されたWikiファイル数';
COMMENT ON COLUMN wiki_metadata.generated_at IS 'Wiki生成完了日時';
```

**使用パターン:**

**プロダクト単位でのWiki生成:**
```sql
INSERT INTO wiki_metadata (product_id, output_path, file_count) VALUES
  ('my-ecommerce-uuid', '/var/lib/dev-rag/wikis/my-ecommerce/', 15);
```
プロダクトに紐付く全ソース（backend、frontend、Confluence等）の情報を統合したWikiを生成。
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

-- productsテーブル
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_products_name ON products(name);

-- sourcesテーブル
CREATE TABLE sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID REFERENCES products(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL UNIQUE,
    source_type VARCHAR(50) NOT NULL CHECK (source_type IN ('git', 'confluence', 'pdf', 'redmine', 'notion', 'local')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sources_name ON sources(name);
CREATE INDEX idx_sources_type ON sources(source_type);
CREATE INDEX idx_sources_product_id ON sources(product_id);

-- source_snapshotsテーブル
CREATE TABLE source_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    version_identifier TEXT NOT NULL,
    indexed BOOLEAN NOT NULL DEFAULT FALSE,
    indexed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_source_snapshots_source_version UNIQUE (source_id, version_identifier)
);

CREATE INDEX idx_source_snapshots_source_id ON source_snapshots(source_id);
CREATE INDEX idx_source_snapshots_version ON source_snapshots(version_identifier);
CREATE INDEX idx_source_snapshots_indexed ON source_snapshots(indexed) WHERE indexed = TRUE;

-- git_refsテーブル
CREATE TABLE git_refs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    ref_name VARCHAR(255) NOT NULL,
    snapshot_id UUID NOT NULL REFERENCES source_snapshots(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_git_refs_source_ref UNIQUE (source_id, ref_name)
);

CREATE INDEX idx_git_refs_source_id ON git_refs(source_id);
CREATE INDEX idx_git_refs_snapshot_id ON git_refs(snapshot_id);
CREATE INDEX idx_git_refs_ref_name ON git_refs(ref_name);

-- filesテーブル
CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id UUID NOT NULL REFERENCES source_snapshots(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    size BIGINT NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_files_snapshot_path UNIQUE (snapshot_id, path)
);

CREATE INDEX idx_files_snapshot_id ON files(snapshot_id);
CREATE INDEX idx_files_content_type ON files(content_type);
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
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    output_path TEXT NOT NULL,
    file_count INTEGER NOT NULL DEFAULT 0,
    generated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_wiki_metadata_product UNIQUE (product_id)
);

CREATE INDEX idx_wiki_metadata_product_id ON wiki_metadata(product_id);
CREATE INDEX idx_wiki_metadata_generated_at ON wiki_metadata(generated_at DESC);
```

#### 001_init_schema.down.sql

```sql
DROP TABLE IF EXISTS wiki_metadata;
DROP TABLE IF EXISTS embeddings;
DROP TABLE IF EXISTS chunks;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS git_refs;
DROP TABLE IF EXISTS source_snapshots;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS products;
DROP EXTENSION IF EXISTS vector;
```

---

## 4. クエリ例

### 4.1 基本的なCRUD操作

#### プロダクト登録

```sql
INSERT INTO products (name, description)
VALUES (
  'my-ecommerce',
  'ECサイトプロダクト（フロントエンド、バックエンド、インフラ、ドキュメントを含む）'
)
RETURNING id, name, created_at;
```

#### ソース登録

```sql
-- バックエンドGitリポジトリ
INSERT INTO sources (name, source_type, metadata)
VALUES (
  'my-ecommerce-backend',
  'git',
  '{"url": "git@github.com:example/backend.git", "default_branch": "main"}'::jsonb
)
RETURNING id, name, created_at;

-- フロントエンドGitリポジトリ
INSERT INTO sources (name, source_type, metadata)
VALUES (
  'my-ecommerce-frontend',
  'git',
  '{"url": "git@github.com:example/frontend.git", "default_branch": "main"}'::jsonb
)
RETURNING id, name, created_at;

-- Confluenceドキュメント
INSERT INTO sources (name, source_type, metadata)
VALUES (
  'my-ecommerce-docs',
  'confluence',
  '{"base_url": "https://confluence.example.com", "space_key": "ECOM"}'::jsonb
)
RETURNING id, name, created_at;
```

#### プロダクトに属するソース一覧の取得

```sql
SELECT
    s.id,
    s.name,
    s.source_type,
    s.metadata
FROM sources s
WHERE s.product_id = $1  -- プロダクトID
ORDER BY s.name;
```

#### スナップショット作成とGit参照の設定

```sql
-- 1. Gitリポジトリのスナップショット作成
INSERT INTO source_snapshots (source_id, version_identifier)
VALUES ('550e8400-e29b-41d4-a716-446655440000', 'abc123def456...')
ON CONFLICT (source_id, version_identifier) DO NOTHING
RETURNING id;

-- 2. Git参照（main ブランチ）をスナップショットに紐付け
INSERT INTO git_refs (source_id, ref_name, snapshot_id)
VALUES (
    '550e8400-e29b-41d4-a716-446655440000',
    'main',
    'snapshot-abc123-uuid'  -- 上記で取得したスナップショットID
)
ON CONFLICT (source_id, ref_name) DO UPDATE
SET snapshot_id = EXCLUDED.snapshot_id,
    updated_at = CURRENT_TIMESTAMP;

-- 3. 同じコミットに v1.0.0 タグを追加（同じスナップショットを指す）
INSERT INTO git_refs (source_id, ref_name, snapshot_id)
VALUES (
    '550e8400-e29b-41d4-a716-446655440000',
    'v1.0.0',
    'snapshot-abc123-uuid'
)
ON CONFLICT (source_id, ref_name) DO UPDATE
SET snapshot_id = EXCLUDED.snapshot_id,
    updated_at = CURRENT_TIMESTAMP;

-- Confluenceページのスナップショット（git_refsは使用しない）
INSERT INTO source_snapshots (source_id, version_identifier)
VALUES ('660e8400-e29b-41d4-a716-446655440000', '42')
ON CONFLICT (source_id, version_identifier) DO NOTHING
RETURNING id;
```

#### ファイル登録

```sql
INSERT INTO files (snapshot_id, path, size, content_type, content_hash)
VALUES
    ('660e8400-e29b-41d4-a716-446655440000', 'src/main.go', 1024, 'text/x-go', 'hash123'),
    ('660e8400-e29b-41d4-a716-446655440000', 'README.md', 512, 'text/markdown', 'hash456')
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

#### 基本的なベクトル検索（Git参照を使用）

```sql
WITH target_snapshot AS (
    SELECT ss.id
    FROM git_refs gr
    JOIN source_snapshots ss ON ss.id = gr.snapshot_id
    JOIN sources s ON s.id = gr.source_id
    WHERE s.name = 'my-service'
      AND gr.ref_name = 'main'
      AND ss.indexed = TRUE
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

**注:** Git以外のソース（Confluence、PDF等）の場合は、git_refsを使わずに最新のスナップショットを直接取得します。

#### フィルタ付きベクトル検索（Git参照を使用）

```sql
WITH target_snapshot AS (
    SELECT ss.id
    FROM git_refs gr
    JOIN source_snapshots ss ON ss.id = gr.snapshot_id
    JOIN sources s ON s.id = gr.source_id
    WHERE s.name = $1  -- ソース名
      AND gr.ref_name = $2  -- 参照名（ブランチ/タグ）
      AND ss.indexed = TRUE
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
  AND ($5::text IS NULL OR f.content_type = $5)     -- コンテンツタイプ（MIMEタイプ）
ORDER BY e.vector <=> $3::vector
LIMIT $6;  -- 取得件数
```

**パラメータ:**
- `$1`: ソース名
- `$2`: 参照名（Gitの場合はブランチ/タグ名: main, v1.0.0 等）
- `$3`: クエリベクトル
- `$4`: パスプレフィックス（オプション）
- `$5`: コンテンツタイプ（オプション、MIMEタイプ: text/x-go, text/markdown 等）
- `$6`: 取得件数

#### 前後コンテキスト付き検索（Git参照を使用）

```sql
WITH target_snapshot AS (
    SELECT ss.id
    FROM git_refs gr
    JOIN source_snapshots ss ON ss.id = gr.snapshot_id
    JOIN sources s ON s.id = gr.source_id
    WHERE s.name = $1 AND gr.ref_name = $2 AND ss.indexed = TRUE
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

#### プロダクト単位でのWiki生成メタデータ登録

```sql
INSERT INTO wiki_metadata (product_id, output_path, file_count, generated_at)
VALUES (
    $1,  -- product_id
    $2,  -- output_path (例: /var/lib/dev-rag/wikis/my-ecommerce/)
    $3,  -- file_count
    CURRENT_TIMESTAMP
)
ON CONFLICT (product_id) DO UPDATE
SET output_path = EXCLUDED.output_path,
    file_count = EXCLUDED.file_count,
    generated_at = CURRENT_TIMESTAMP
RETURNING id, generated_at;
```


#### プロダクト横断検索（複数ソースから検索）

```sql
-- プロダクトに属する全ソースのスナップショットから検索
WITH target_snapshots AS (
    SELECT DISTINCT ON (ss.source_id) ss.id
    FROM source_snapshots ss
    JOIN sources s ON s.id = ss.source_id
    WHERE s.product_id = $1  -- プロダクトID
      AND ss.indexed = TRUE
    ORDER BY ss.source_id, ss.created_at DESC
)
SELECT
    f.path,
    c.start_line,
    c.end_line,
    c.content,
    1 - (e.vector <=> $2::vector) AS score
FROM embeddings e
JOIN chunks c ON c.id = e.chunk_id
JOIN files f ON f.id = c.file_id
WHERE f.snapshot_id IN (SELECT id FROM target_snapshots)
ORDER BY e.vector <=> $2::vector
LIMIT $3;  -- 取得件数
```

#### プロダクトの最新Wiki情報取得

```sql
SELECT
    p.name AS product_name,
    p.description,
    wm.output_path,
    wm.file_count,
    wm.generated_at,
    COUNT(s.id) AS source_count
FROM wiki_metadata wm
JOIN products p ON p.id = wm.product_id
LEFT JOIN sources s ON s.product_id = p.id
WHERE p.name = $1  -- プロダクト名
GROUP BY p.id, p.name, p.description, wm.id, wm.output_path, wm.file_count, wm.generated_at
ORDER BY wm.generated_at DESC
LIMIT 1;
```

#### プロダクト一覧とWiki生成状況

```sql
SELECT
    p.id,
    p.name,
    p.description,
    COUNT(s.id) AS source_count,
    wm.output_path,
    wm.file_count,
    wm.generated_at
FROM products p
LEFT JOIN sources s ON s.product_id = p.id
LEFT JOIN wiki_metadata wm ON wm.product_id = p.id
GROUP BY p.id, p.name, p.description, wm.id, wm.output_path, wm.file_count, wm.generated_at
ORDER BY p.name;
```

### 4.5 統計・メタデータクエリ

#### ソースごとのチャンク数

```sql
SELECT
    s.name,
    s.source_type,
    COUNT(c.id) AS chunk_count,
    SUM(c.token_count) AS total_tokens
FROM sources s
JOIN source_snapshots ss ON ss.source_id = s.id
JOIN files f ON f.snapshot_id = ss.id
JOIN chunks c ON c.file_id = f.id
WHERE ss.indexed = TRUE
GROUP BY s.id, s.name, s.source_type
ORDER BY chunk_count DESC;
```

#### 最新インデックス状況

```sql
SELECT
    s.name,
    s.source_type,
    ss.version_identifier,
    ss.ref_name,
    ss.indexed,
    ss.indexed_at,
    COUNT(DISTINCT f.id) AS file_count,
    COUNT(c.id) AS chunk_count
FROM sources s
JOIN source_snapshots ss ON ss.source_id = s.id
LEFT JOIN files f ON f.snapshot_id = ss.id
LEFT JOIN chunks c ON c.file_id = f.id
WHERE ss.id IN (
    SELECT DISTINCT ON (source_id) id
    FROM source_snapshots
    WHERE indexed = TRUE
    ORDER BY source_id, created_at DESC
)
GROUP BY s.id, s.name, s.source_type, ss.version_identifier, ss.ref_name, ss.indexed, ss.indexed_at
ORDER BY s.name;
```

#### インデックス・Wiki統合情報

```sql
SELECT
    s.name,
    s.source_type,
    ss.version_identifier,
    ss.ref_name,
    ss.indexed,
    ss.indexed_at,
    COUNT(DISTINCT f.id) AS file_count,
    COUNT(c.id) AS chunk_count,
    wm.output_path AS wiki_path,
    wm.file_count AS wiki_file_count,
    wm.generated_at AS wiki_generated_at
FROM sources s
JOIN source_snapshots ss ON ss.source_id = s.id
LEFT JOIN files f ON f.snapshot_id = ss.id
LEFT JOIN chunks c ON c.file_id = f.id
LEFT JOIN wiki_metadata wm ON wm.source_id = s.id
WHERE s.name = $1  -- ソース名
  AND ss.indexed = TRUE
ORDER BY ss.created_at DESC
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
