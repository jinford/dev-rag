# アーキテクチャ理解Wiki生成システム設計書（改訂版）

## 概要

gitリポジトリのコードを「木（断片）」ではなく「森（全体像）」として理解し、**Markdown形式のWikiとして自動生成**するシステム。

**本設計は現在の実装状況を反映し、完璧な状態を目指す最終設計である。**
DBには現在データが入っていないため、互換性を考慮せず理想的な設計を採用する。

**プロンプトテンプレート**: LLMに渡すプロンプトの詳細は [architecture-wiki-prompt-template.md](architecture-wiki-prompt-template.md) を参照してください。

### 重要な設計原則: 階層的集約

本システムの最も重要な設計原則は、**情報を階層的に集約する**ことです：

```
Code Chunks (葉)
  ↓ 集約
File Summary (枝)
  ↓ 集約
Directory Summary (木々)
  ↓ 集約
Architecture Summary (森全体)
```

この階層的アプローチにより：
- **情報の一貫性**: 下位レベルの要約から上位レベルを生成するため、矛盾がない
- **重複処理の削減**: 各レベルで既に生成された要約を再利用
- **自然な抽象化**: 詳細（葉）→ 枝 → 木々 → 森という自然な流れ

### シンプルな設計方針

本システムは、**言語やプロジェクト構造に依存しない完全に汎用的なアプローチ**を採用：

**設計方針**:
- **すべてのディレクトリを平等に扱う**（深さ制限なし、特別扱いなし）
- **すべてのファイルを処理対象とする**（除外ルールなし）
- **特別な判定やパターンマッチングを行わない**

**メリット**:
- 極めてシンプルで保守しやすい
- あらゆる言語、フレームワーク、プロジェクト構造に対応
- 特殊なケースへの対応が不要

### 既存システムとの関係

本システムは [requirements.md](requirements.md) および [design.md](design.md) で定義された「RAG基盤 + Wiki自動生成システム」の一部として、**Wiki生成機能を強化**するものである。

従来の課題：
- 通常のRAG：チャンク単位の検索 → 局所的な理解のみ
- 既存のWiki生成：平坦な構造 → アーキテクチャ全体像が伝わりにくい

解決策：
- **3層構造の要約生成**（アーキテクチャ/モジュール/ファイル）
- **階層的な情報をWikiページに統合**（全体像 → モジュール → 実装詳細）
- **既存技術スタックのみ使用**（PostgreSQL + pgvector、Go、OpenAI API）

---

## システムアーキテクチャ

### 全体フロー

```
1. インデックス構築フェーズ（拡張）
   Repository → Analyzer → [Architecture/Module/File] Summarizer → PostgreSQL
   ↓
   - 既存のチャンク化・Embedding生成に加えて
   - アーキテクチャ要約、モジュール要約、ファイル要約を生成

2. Wiki生成フェーズ（新規実装）
   Wiki Generator → 階層的検索 → 要約 + チャンク → LLM → Markdown Wiki
   ↓
   - 疑似クエリ方式を採用
   - 階層的要約を活用してコンテキストを強化
   - Mermaid図を含む構造化されたWikiページを生成
```

### 情報の階層構造

```
Layer 1: Architecture Summary (森全体)
├─ システム全体のアーキテクチャ概要（overview）
├─ 技術スタック（tech_stack）
├─ データフロー（data_flow）
└─ 主要コンポーネント（components）
→ architecture_summariesテーブルに保存
→ Wikiの「architecture.md」に反映
→ **Directory Summary（木々）から集約して生成**

Layer 2: Directory Summary (木々)
├─ 各ディレクトリの役割と責務
├─ 配下のサブディレクトリとファイル
├─ 親ディレクトリとの関係
└─ 主要な機能
→ すべてのディレクトリを平等に扱う
→ directory_summariesテーブルに保存
→ **File Summary（枝）から集約して生成**

Layer 3: File Summary (枝)
├─ 各ファイルの要約（FileSummarizer）
├─ 公開API（exports）
├─ 依存関係（imports）
└─ 主要コンポーネント
→ file_summariesテーブルに保存（file_idで紐付け）
→ **Code Chunks（葉）から集約して生成**

Layer 4: Code Chunks (葉)
└─ コードチャンク（実装詳細）
→ chunksテーブルに保存（level=1:親, 2:子, 3:孫の階層構造）
→ **最も細かい粒度の情報（実際のコード片）**
```

---

## データモデル

### 設計原則

既存のスキーマ（[schema.sql](../schema/schema.sql)）を**拡張**し、要約情報を追加する。

1. **バージョン管理**: 既存の `source_snapshots` を活用
2. **一意性保証**: 適切なユニーク制約でデータ整合性を担保
3. **Embedding次元の固定性**:
   - pgvectorの `VECTOR` 型は列全体で次元が固定される（最初の挿入時の次元で固定）
   - `VECTOR` と記述しても、最初にINSERTされたベクトルの次元数で列が固定される
   - **モデル変更時の対応**: 異なる次元のEmbeddingモデルに切り替える場合は以下の手順が必要:
     1. 新しいマイグレーションで列を再作成（例: `embedding_v2 VECTOR`）
     2. または、既存列を削除して再作成
     3. 全データを新モデルで再生成
   - 実際の次元数はメタデータ（`metadata.dim`）に保存し、モデル情報を記録
4. **冪等性**: UPSERT動作で重複を防止
5. **UUID使用**: 既存システムと統一してUUIDを主キーとして使用

### 既存テーブル（活用）

現在のスキーマには以下のテーブルが存在し、これらを最大限活用する：

- `products`: プロダクト管理（複数ソースをまとめる単位）
- `sources`: ソース管理（Git、Confluence等）
- `source_snapshots`: スナップショット管理（バージョン管理）
- `files`: ファイル情報（domain分類含む）
- `chunks`: チャンク情報（level, importance_score含む）
- `embeddings`: Embeddingベクトル
- `chunk_hierarchy`: チャンクの親子関係
- `chunk_dependencies`: チャンク間の依存関係

**既存機能の活用**:
- FileSummarizerは既に実装済み → 出力先をfile_summariesテーブルに変更
- 階層的チャンキング（chunk_hierarchy）は既に実装済み
- 依存関係抽出（chunk_dependencies）は既に実装済み
- ドメイン分類（files.domain）は既に実装済み

**設計方針**:
- chunksテーブル: ファイルの中身（コードチャンク）専用
- 3つの新規要約テーブル: 要約（アーキテクチャ/ディレクトリ/ファイル）専用
- 責務を明確に分離することで、拡張性と保守性を向上

### 新規テーブル

#### 1. file_summaries（ファイル要約テーブル）

```sql
-- ファイル要約テーブル
CREATE TABLE file_summaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,

    -- 要約内容
    summary TEXT NOT NULL,
    embedding VECTOR NOT NULL,  -- 次元固定（最初のINSERT時に決定、実際の次元数はmetadata.dimに記録）

    -- メタデータ
    metadata JSONB DEFAULT '{}',  -- {model, dim, generated_at, llm_model, etc.}

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- 一意性制約
    CONSTRAINT uq_file_summaries_file_id UNIQUE(file_id)
);

-- インデックス
CREATE INDEX idx_file_summaries_embedding ON file_summaries
USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX idx_file_summaries_file_id ON file_summaries(file_id);

-- コメント
COMMENT ON TABLE file_summaries IS 'ファイルごとの要約（LLMが生成）';
COMMENT ON COLUMN file_summaries.id IS '要約の一意識別子';
COMMENT ON COLUMN file_summaries.file_id IS '対象ファイルのID';
COMMENT ON COLUMN file_summaries.summary IS 'LLMが生成した要約（Markdown形式）';
COMMENT ON COLUMN file_summaries.embedding IS 'Embeddingベクトル（pgvectorの制約により列全体で次元固定、最初のINSERT時の次元で確定）';
COMMENT ON COLUMN file_summaries.metadata IS 'メタデータ（モデル名、次元数、生成日時等。metadata.dimに実際の次元数を記録）';
```

#### 2. directory_summaries（ディレクトリ要約テーブル）

```sql
-- ディレクトリ要約テーブル
CREATE TABLE directory_summaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id UUID NOT NULL REFERENCES source_snapshots(id) ON DELETE CASCADE,

    -- ディレクトリ情報
    path VARCHAR(512) NOT NULL,
    parent_path VARCHAR(512),
    depth INTEGER NOT NULL,

    -- 要約内容
    summary TEXT NOT NULL,
    embedding VECTOR NOT NULL,  -- 次元固定（最初のINSERT時に決定、実際の次元数はmetadata.dimに記録）

    -- メタデータ
    metadata JSONB DEFAULT '{}',  -- {file_count, subdir_count, total_files, languages, dim, etc.}

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- 一意性制約
    CONSTRAINT uq_directory_summaries_snapshot_path UNIQUE(snapshot_id, path)
);

-- インデックス
CREATE INDEX idx_directory_summaries_embedding ON directory_summaries
USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX idx_directory_summaries_snapshot_id ON directory_summaries(snapshot_id);
CREATE INDEX idx_directory_summaries_path ON directory_summaries(path);
CREATE INDEX idx_directory_summaries_parent_path ON directory_summaries(parent_path);
CREATE INDEX idx_directory_summaries_depth ON directory_summaries(depth);

-- コメント
COMMENT ON TABLE directory_summaries IS 'ディレクトリごとの要約（LLMが生成）';
COMMENT ON COLUMN directory_summaries.id IS '要約の一意識別子';
COMMENT ON COLUMN directory_summaries.snapshot_id IS '対象スナップショットのID';
COMMENT ON COLUMN directory_summaries.path IS 'ディレクトリパス';
COMMENT ON COLUMN directory_summaries.parent_path IS '親ディレクトリパス（階層構造用）';
COMMENT ON COLUMN directory_summaries.depth IS 'ディレクトリの深さ（0=ルート）';
COMMENT ON COLUMN directory_summaries.summary IS 'LLMが生成した要約（Markdown形式）';
COMMENT ON COLUMN directory_summaries.embedding IS 'Embeddingベクトル（pgvectorの制約により列全体で次元固定、最初のINSERT時の次元で確定）';
COMMENT ON COLUMN directory_summaries.metadata IS 'メタデータ（ファイル数、言語統計、次元数等。metadata.dimに実際の次元数を記録）';
```

#### 3. architecture_summaries（アーキテクチャ要約テーブル）

```sql
-- アーキテクチャ要約テーブル
CREATE TABLE architecture_summaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id UUID NOT NULL REFERENCES source_snapshots(id) ON DELETE CASCADE,

    -- 要約の種類
    summary_type VARCHAR(50) NOT NULL CHECK (summary_type IN (
        'overview', 'tech_stack', 'data_flow', 'components'
    )),

    -- 要約内容
    summary TEXT NOT NULL,
    embedding VECTOR NOT NULL,  -- 次元固定（最初のINSERT時に決定、実際の次元数はmetadata.dimに記録）

    -- メタデータ
    metadata JSONB DEFAULT '{}',  -- {file_count, directory_count, llm_model, dim, etc.}

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- 一意性制約
    CONSTRAINT uq_architecture_summaries_snapshot_type UNIQUE(snapshot_id, summary_type)
);

-- インデックス
CREATE INDEX idx_architecture_summaries_embedding ON architecture_summaries
USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX idx_architecture_summaries_snapshot_id ON architecture_summaries(snapshot_id);
CREATE INDEX idx_architecture_summaries_type ON architecture_summaries(summary_type);

-- コメント
COMMENT ON TABLE architecture_summaries IS 'システム全体のアーキテクチャ要約（LLMが生成）';
COMMENT ON COLUMN architecture_summaries.id IS '要約の一意識別子';
COMMENT ON COLUMN architecture_summaries.snapshot_id IS '対象スナップショットのID';
COMMENT ON COLUMN architecture_summaries.summary_type IS '要約種別（overview/tech_stack/data_flow/components）';
COMMENT ON COLUMN architecture_summaries.summary IS 'LLMが生成した要約（Markdown形式）';
COMMENT ON COLUMN architecture_summaries.embedding IS 'Embeddingベクトル（pgvectorの制約により列全体で次元固定、最初のINSERT時の次元で確定）';
COMMENT ON COLUMN architecture_summaries.metadata IS 'メタデータ（統計情報、次元数等。metadata.dimに実際の次元数を記録）';
```

**設計の特徴**:
1. **シンプル**: 各テーブルが単一の責務を持つ
2. **明確**: テーブル名で役割が分かる
3. **型安全**: file_idとpathの使い分けで混乱しない
4. **データ整合性**: file_summariesはfilesとの外部キーで整合性保証
5. **拡張性**: 各テーブルに特化したカラムを追加しやすい

**生成ルール**:
- **すべてのディレクトリを走査**（深さ制限なし、除外なし）
- **階層構造の保持**: parent_pathで親子関係を記録
- **シンプルさ優先**: 特殊なルールや判定は一切なし

#### 4. マイグレーション戦略

新規マイグレーションファイルを作成：

- `schema/migrations/009_add_summary_tables.up.sql`
- `schema/migrations/009_add_summary_tables.down.sql`

**移行手順**:
1. 3つの要約テーブルを作成（file_summaries, directory_summaries, architecture_summaries）
2. 各テーブルにインデックス、制約、コメントを追加
3. FileSummarizerを修正して、最初からfile_summariesテーブルに書き込むようにする
4. 既存データの移行は行わない（DBは空なので不要）

**Embeddingモデル変更時の移行手順**:

将来、異なる次元のEmbeddingモデル（例: text-embedding-3-small [1536次元] → text-embedding-3-large [3072次元]）に切り替える場合:

1. **新規マイグレーション作成**（例: `010_change_embedding_dimension.up.sql`）
   ```sql
   -- 既存のインデックスを削除
   DROP INDEX IF EXISTS idx_file_summaries_embedding;
   DROP INDEX IF EXISTS idx_directory_summaries_embedding;
   DROP INDEX IF EXISTS idx_architecture_summaries_embedding;

   -- 既存のEmbedding列を削除
   ALTER TABLE file_summaries DROP COLUMN embedding;
   ALTER TABLE directory_summaries DROP COLUMN embedding;
   ALTER TABLE architecture_summaries DROP COLUMN embedding;

   -- 新しい次元のEmbedding列を追加
   ALTER TABLE file_summaries ADD COLUMN embedding VECTOR;
   ALTER TABLE directory_summaries ADD COLUMN embedding VECTOR;
   ALTER TABLE architecture_summaries ADD COLUMN embedding VECTOR;

   -- インデックスを再作成
   CREATE INDEX idx_file_summaries_embedding ON file_summaries
   USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

   CREATE INDEX idx_directory_summaries_embedding ON directory_summaries
   USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

   CREATE INDEX idx_architecture_summaries_embedding ON architecture_summaries
   USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
   ```

2. **環境変数の更新**
   - Embeddingモデル名を新しいモデルに変更
   - コードが新しい次元数を認識するよう確認

3. **既存データの再生成**
   - 全てのfile_summariesを再生成（新しいEmbeddingモデルで）
   - その後、directory_summariesを再生成
   - 最後にarchitecture_summariesを再生成

4. **動作確認**
   - 新しい次元でのベクトル検索が正常に動作することを確認
   - metadata.dimが新しい次元数を記録していることを確認

### 既存テーブルとの連携

**役割分担の明確化**:

```
architecture_summaries テーブル（新規）:
└─ アーキテクチャ要約（overview/tech_stack/data_flow/components）
→ システム全体の要約

directory_summaries テーブル（新規）:
└─ ディレクトリ要約
→ ディレクトリごとの責務と機能

file_summaries テーブル（新規）:
└─ ファイル要約
→ ファイルごとの要約（file_idで外部キー結合）

files テーブル（既存）:
└─ ファイルのメタデータ（パス、サイズ、言語、ドメイン、ハッシュ等）
→ chunksとfile_summariesの親

chunks テーブル（既存）:
├─ level=1: 親チャンク（大きなコードブロック）
├─ level=2: 子チャンク（中程度のコードブロック）
└─ level=3: 孫チャンク（小さなコードブロック）
→ ファイルの「中身（コード）」専用、要約は一切含まない
→ levelは階層的チャンキングの深さを表す（数値が大きいほど詳細）

chunk_hierarchy テーブル（既存）:
└─ chunks間の親子関係を管理

chunk_dependencies テーブル（既存）:
└─ chunks間の依存関係を管理（call, import, type等）
```

**設計の明確化**:
1. **3つの要約テーブル**: 役割ごとに分離（architecture/directory/file）
2. **chunksテーブル**: 純粋に「コードチャンク」のみを保存、levelは階層化の深さ（1=親、2=子、3=孫）
3. **filesテーブル**: メタデータ保持とchunks/file_summariesの親として機能
4. **責務の完全分離**: 要約とコードを明確に分離することで、拡張性と保守性を向上

**データの流れ（階層的集約）**:
```
files (メタデータ)
  ├─→ file_summaries (ファイル要約) ← FileSummarizerが生成
  │     ↓
  │   directory_summaries (ディレクトリ要約) ← DirectorySummarizerがfile_summariesから集約
  │     ↓
  │   architecture_summaries (アーキテクチャ要約) ← ArchitectureSummarizerがdirectory_summariesから集約
  │
  └─→ chunks (コードチャンク) ← Chunkerが生成

重要: 要約は階層的に生成される（File → Directory → Architecture）
```

---

## コンポーネント設計

### 設計原則

1. **エラー復旧**: LLM呼び出し失敗時のリトライロジック
2. **部分実行**: 処理済みのデータをスキップ（冪等性）
3. **セキュリティ**: 秘匿情報のフィルタリング（.env, credentials, API keys等）
4. **既存フローとの統合**: 既存の `index` コマンドに統合

### 共通インターフェース

```go
// pkg/wiki/interfaces.go

// LLMクライアント（リトライ機能付き）
type LLMClient interface {
    Generate(ctx context.Context, prompt string) (string, error)
    GenerateWithRetry(ctx context.Context, prompt string, maxRetries int) (string, error)
    CreateEmbedding(ctx context.Context, text string) ([]float32, error)
    CreateEmbeddingWithRetry(ctx context.Context, text string, maxRetries int) ([]float32, error)
}

// セキュリティフィルタ
type SecurityFilter interface {
    ContainsSensitiveInfo(content string) bool
    MaskSensitiveInfo(content string) string
    FilterFiles(files []string) []string
}
```

### 実装方針

既存のコンポーネントを最大限活用：

1. **既存のLLMクライアント**: `pkg/indexer/llm/client.go` を拡張
2. **既存のEmbedder**: `pkg/indexer/embedder/embedder.go` を活用
3. **既存のChunker**: `pkg/indexer/chunker/chunker.go` を活用
4. **既存のDependency Analyzer**: `pkg/indexer/dependency/analyzer.go` を活用

---

### 1. RepositoryAnalyzer（リポジトリ解析エンジン）

**責務**: リポジトリ全体を走査し、ディレクトリ構造を抽出

**配置**: `pkg/wiki/analyzer/repository_analyzer.go`

```go
// pkg/wiki/analyzer/repository_analyzer.go

type RepositoryAnalyzer struct {
    pool           *pgxpool.Pool
    llm            wiki.LLMClient
    embedder       *embedder.Embedder
    securityFilter wiki.SecurityFilter
}

func NewRepositoryAnalyzer(
    pool *pgxpool.Pool,
    llm wiki.LLMClient,
    embedder *embedder.Embedder,
    securityFilter wiki.SecurityFilter,
) *RepositoryAnalyzer {
    return &RepositoryAnalyzer{
        pool:           pool,
        llm:            llm,
        embedder:       embedder,
        securityFilter: securityFilter,
    }
}

// 設計方針:
// - RepositoryAnalyzer は pool のみを保持
// - 読み取り専用クエリは都度 sqlc.New(pool) を使用
// - サマライザは pool を受け取り、各自でトランザクション管理
// - この設計により、トランザクションのスコープが明確になる

type RepoStructure struct {
    SourceID     uuid.UUID
    SnapshotID   uuid.UUID
    RootPath     string            // リポジトリのルートパス
    Directories  []*DirectoryInfo  // すべてのディレクトリ
    Files        []*FileInfo       // すべてのファイル
}

type DirectoryInfo struct {
    Path            string
    ParentPath      string        // 親ディレクトリ（空文字列=ルート）
    Depth           int           // 深さ（0=ルート）
    Files           []string      // このディレクトリ直下のファイル
    Subdirectories  []string      // このディレクトリ直下のサブディレクトリ
    TotalFiles      int           // 配下すべてのファイル数
    Languages       map[string]int // 使用言語とファイル数
}

type FileInfo struct {
    FileID    uuid.UUID
    Path      string
    Size      int64
    Language  string
    Domain    string
    Hash      string
}

// メインエントリーポイント
func (a *RepositoryAnalyzer) AnalyzeRepository(ctx context.Context, sourceID, snapshotID uuid.UUID) error {
    // 1. 既に処理済みかチェック
    if a.isAlreadyAnalyzed(ctx, snapshotID) {
        return nil
    }

    // 2. ディレクトリ構造の収集（既存のfilesテーブルから構築）
    structure, err := a.collectStructure(ctx, sourceID, snapshotID)
    if err != nil {
        return fmt.Errorf("structure collection failed: %w", err)
    }

    // 3. ディレクトリサマライザーですべてのディレクトリの要約生成
    //    （File Summaryから集約）
    //    各ディレクトリで個別にトランザクションを管理
    dirSummarizer := summarizer.NewDirectorySummarizer(a.pool, a.llm, a.embedder, a.securityFilter)
    if err := dirSummarizer.GenerateSummaries(ctx, structure); err != nil {
        return fmt.Errorf("directory summaries generation failed: %w", err)
    }

    // 4. アーキテクチャサマライザーでリポジトリ全体の要約生成
    //    （Directory Summaryから集約）
    //    コミット済みのディレクトリ要約を読み込んで処理
    archSummarizer := summarizer.NewArchitectureSummarizer(a.pool, a.llm, a.embedder, a.securityFilter)
    if err := archSummarizer.GenerateSummaries(ctx, structure); err != nil {
        return fmt.Errorf("repository summary generation failed: %w", err)
    }

    return nil
}

func (a *RepositoryAnalyzer) isAlreadyAnalyzed(ctx context.Context, snapshotID uuid.UUID) bool {
    // 読み取り専用クエリなので、都度 sqlc.New(pool) を使用
    queries := sqlc.New(a.pool)
    count, err := queries.CountArchitectureSummaries(ctx, snapshotID)
    return err == nil && count > 0
}

func (a *RepositoryAnalyzer) collectStructure(ctx context.Context, sourceID, snapshotID uuid.UUID) (*RepoStructure, error) {
    // 読み取り専用クエリなので、都度 sqlc.New(pool) を使用
    queries := sqlc.New(a.pool)

    structure := &RepoStructure{
        SourceID:    sourceID,
        SnapshotID:  snapshotID,
        Directories: make([]*DirectoryInfo, 0),
        Files:       make([]*FileInfo, 0),
    }

    // 既存のfilesテーブルから情報を取得
    files, err := queries.ListFilesBySnapshot(ctx, snapshotID)
    if err != nil {
        return nil, err
    }

    // ファイル情報を構築
    for _, file := range files {
        fileInfo := &FileInfo{
            FileID:   file.ID,
            Path:     file.Path,
            Size:     file.Size,
            Language: file.Language,
            Domain:   file.Domain,
            Hash:     file.ContentHash,
        }
        structure.Files = append(structure.Files, fileInfo)
    }

    // ディレクトリ構造を構築
    structure.Directories = a.buildDirectoryStructure(structure.Files)

    return structure, nil
}

func (a *RepositoryAnalyzer) buildDirectoryStructure(files []*FileInfo) []*DirectoryInfo {
    // すべてのディレクトリを抽出して構造化
    dirMap := make(map[string]*DirectoryInfo)

    // すべてのファイルからディレクトリを抽出
    for _, file := range files {
        dir := filepath.Dir(file.Path)

        // ルートディレクトリの正規化
        if dir == "." || dir == "" {
            dir = "."
        }

        // ディレクトリパスを分割（"." を除く）
        parts := []string{}
        if dir != "." {
            parts = strings.Split(dir, string(filepath.Separator))
        }

        // ルートディレクトリを初期化
        if _, exists := dirMap["."]; !exists {
            dirMap["."] = &DirectoryInfo{
                Path:           ".",
                ParentPath:     "",
                Depth:          0,
                Files:          []string{},
                Subdirectories: []string{},
                Languages:      make(map[string]int),
            }
        }

        // ルート直下のファイル
        if dir == "." {
            dirMap["."].Files = append(dirMap["."].Files, file.Path)
            dirMap["."].Languages[file.Language]++
            continue
        }

        // すべての階層のディレクトリを記録
        for depth := 1; depth <= len(parts); depth++ {
            dirPath := filepath.Join(parts[:depth]...)
            parentPath := "."
            if depth > 1 {
                parentPath = filepath.Join(parts[:depth-1]...)
            }

            if _, exists := dirMap[dirPath]; !exists {
                dirMap[dirPath] = &DirectoryInfo{
                    Path:           dirPath,
                    ParentPath:     parentPath,
                    Depth:          depth,
                    Files:          []string{},
                    Subdirectories: []string{},
                    Languages:      make(map[string]int),
                }
            }

            // このディレクトリ直下のファイル
            if len(parts) == depth {
                dirMap[dirPath].Files = append(dirMap[dirPath].Files, file.Path)
                dirMap[dirPath].Languages[file.Language]++
            }
        }
    }

    // サブディレクトリの関係を構築
    for path, dir := range dirMap {
        if dir.ParentPath != "" {
            if parent, exists := dirMap[dir.ParentPath]; exists {
                parent.Subdirectories = append(parent.Subdirectories, path)
            }
        } else if path != "." {
            // ルート直下のディレクトリ
            if root, exists := dirMap["."]; exists {
                root.Subdirectories = append(root.Subdirectories, path)
            }
        }
    }

    // 配下のすべてのファイル数を計算
    for _, dir := range dirMap {
        dir.TotalFiles = a.countTotalFiles(dir, dirMap)
    }

    // すべてのディレクトリを返す（フィルタリングなし）
    directories := make([]*DirectoryInfo, 0, len(dirMap))
    for _, dir := range dirMap {
        directories = append(directories, dir)
    }

    return directories
}

func (a *RepositoryAnalyzer) countTotalFiles(dir *DirectoryInfo, dirMap map[string]*DirectoryInfo) int {
    total := len(dir.Files)
    for _, subdir := range dir.Subdirectories {
        if sd, exists := dirMap[subdir]; exists {
            total += a.countTotalFiles(sd, dirMap)
        }
    }
    return total
}
```

---

### 2. ArchitectureSummarizer（アーキテクチャ要約生成）

**責務**: システム全体のアーキテクチャを要約（**Directory Summaryから集約**）

**配置**: `pkg/wiki/summarizer/architecture_summarizer.go`

**重要な設計変更**:
- **Directory Summaryから集約して生成**（ディレクトリ構造から直接生成しない）
- ディレクトリ要約を読み込み、それを抽象化してアーキテクチャレベルの要約を作成
- 階層的な情報の流れを保つ（File → Directory → Architecture）

```go
// pkg/wiki/summarizer/architecture_summarizer.go

type ArchitectureSummarizer struct {
    pool           *pgxpool.Pool
    llm            wiki.LLMClient
    embedder       *embedder.Embedder
    securityFilter wiki.SecurityFilter
}

func NewArchitectureSummarizer(
    pool *pgxpool.Pool,
    llm wiki.LLMClient,
    embedder *embedder.Embedder,
    securityFilter wiki.SecurityFilter,
) *ArchitectureSummarizer {
    return &ArchitectureSummarizer{
        pool:           pool,
        llm:            llm,
        embedder:       embedder,
        securityFilter: securityFilter,
    }
}

func (s *ArchitectureSummarizer) GenerateSummaries(
    ctx context.Context,
    structure *analyzer.RepoStructure,
) error {
    // 複数種類の要約を生成
    summaryTypes := []string{"overview", "tech_stack", "data_flow", "components"}

    for _, summaryType := range summaryTypes {
        if err := s.generateSummary(ctx, structure, summaryType); err != nil {
            return fmt.Errorf("failed to generate %s summary: %w", summaryType, err)
        }
    }

    return nil
}

func (s *ArchitectureSummarizer) generateSummary(
    ctx context.Context,
    structure *analyzer.RepoStructure,
    summaryType string,
) error {
    // トランザクション開始
    tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback(ctx)

    // sqlc.Queriesをトランザクションでラップ
    queries := sqlc.New(tx)

    // Directory Summaryを全て取得（重要: ディレクトリ構造ではなく、既に生成された要約を使う）
    // コミット済みのディレクトリ要約を読み込む
    directorySummaries, err := s.collectAllDirectorySummaries(ctx, queries, structure.SnapshotID)
    if err != nil {
        return fmt.Errorf("failed to collect directory summaries: %w", err)
    }

    // プロンプト構築（Directory Summaryを元に）
    prompt := s.buildPrompt(structure, directorySummaries, summaryType)

    // セキュリティチェック
    if s.securityFilter.ContainsSensitiveInfo(prompt) {
        prompt = s.securityFilter.MaskSensitiveInfo(prompt)
    }

    // LLMで要約生成（リトライ付き）
    summary, err := s.llm.GenerateWithRetry(ctx, prompt, 3)
    if err != nil {
        return fmt.Errorf("LLM generation failed: %w", err)
    }

    // Embedding生成
    embedding, err := s.embedder.CreateEmbeddingWithRetry(ctx, summary, 3)
    if err != nil {
        return fmt.Errorf("embedding creation failed: %w", err)
    }

    // メタデータ構築
    metadata := map[string]interface{}{
        "model":              "text-embedding-3-small",
        "dim":                len(embedding),
        "generated_at":       time.Now().Format(time.RFC3339),
        "file_count":         len(structure.Files),
        "directory_count":    len(structure.Directories),
        "llm_model":          "gpt-4o-mini", // 環境変数から取得
        "prompt_version":     "3.0",         // トークンベース + 階層的集約
        "aggregation_source": "directory_summaries",
    }
    metadataJSON, _ := json.Marshal(metadata)

    // architecture_summariesテーブルにUPSERT（冪等性保証）
    _, err = tx.Exec(ctx, `
        INSERT INTO architecture_summaries
        (snapshot_id, summary_type, summary, embedding, metadata)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (snapshot_id, summary_type)
        DO UPDATE SET
            summary = EXCLUDED.summary,
            embedding = EXCLUDED.embedding,
            metadata = EXCLUDED.metadata,
            updated_at = CURRENT_TIMESTAMP
    `, structure.SnapshotID, summaryType, summary, embedding, metadataJSON)
    if err != nil {
        return fmt.Errorf("failed to upsert architecture summary: %w", err)
    }

    // コミット
    return tx.Commit(ctx)
}

func (s *ArchitectureSummarizer) collectAllDirectorySummaries(
    ctx context.Context,
    queries *sqlc.Queries,
    snapshotID uuid.UUID,
) (string, error) {
    // directory_summariesテーブルから全てのディレクトリ要約を取得
    const maxContextTokens = 8000 // トークンベースで管理
    var summaries []string
    totalTokens := 0

    // ディレクトリを取得（深さでソート）
    rows, err := queries.ListDirectorySummariesBySnapshot(ctx, snapshotID)
    if err != nil {
        return "", fmt.Errorf("failed to list directory summaries: %w", err)
    }

    for _, row := range rows {
        // ディレクトリ要約を整形
        summaryText := fmt.Sprintf("## %s (深さ: %d)\n%s\n", row.Path, row.Depth, row.Summary)

        // トークン数を推定（文字数 / 4 で概算）
        estimatedTokens := len(summaryText) / 4

        // コンテキスト長チェック（安全マージン20%）
        if totalTokens+estimatedTokens > int(float64(maxContextTokens)*0.8) {
            log.Printf("warning: context limit reached, truncating at %d directories", len(summaries))
            summaries = append(summaries, fmt.Sprintf("... (残り %d ディレクトリは省略されました)", len(rows)-len(summaries)))
            break
        }

        summaries = append(summaries, summaryText)
        totalTokens += estimatedTokens
    }

    if len(summaries) == 0 {
        return "", fmt.Errorf("no directory summaries found")
    }

    return strings.Join(summaries, "\n\n"), nil
}

func (s *ArchitectureSummarizer) buildPrompt(
    structure *analyzer.RepoStructure,
    directorySummariesContent string,
    summaryType string,
) string {
    // プロンプトテンプレートの詳細は以下のドキュメントを参照
    // docs/architecture-wiki-prompt-template.md - セクションA: ArchitectureSummarizer用プロンプト

    // ディレクトリ要約を統合して、summary_type（overview/tech_stack/data_flow/components）に応じた
    // アーキテクチャレベルの要約を生成する
    // 実装の詳細はプロンプトテンプレートドキュメントを参照

    return buildArchitectureSummaryPrompt(structure, directorySummariesContent, summaryType)
}
```

---

### 3. DirectorySummarizer（ディレクトリ要約生成）

**責務**: 各ディレクトリの責務と役割を要約

**配置**: `pkg/wiki/summarizer/directory_summarizer.go`

**アプローチ**:
- ディレクトリ直下の**全ファイルの要約を取得**（file_summariesテーブルから）
- 恣意的な「重要ファイル選択」は行わない
- FileSummarizerが生成した要約を活用
- LLMにファイル要約の集合を渡し、ディレクトリレベルで抽象化

```go
// pkg/wiki/summarizer/directory_summarizer.go

type DirectorySummarizer struct {
    pool           *pgxpool.Pool
    llm            wiki.LLMClient
    embedder       *embedder.Embedder
    securityFilter wiki.SecurityFilter
}

func NewDirectorySummarizer(
    pool *pgxpool.Pool,
    llm wiki.LLMClient,
    embedder *embedder.Embedder,
    securityFilter wiki.SecurityFilter,
) *DirectorySummarizer {
    return &DirectorySummarizer{
        pool:           pool,
        llm:            llm,
        embedder:       embedder,
        securityFilter: securityFilter,
    }
}

func (s *DirectorySummarizer) GenerateSummaries(
    ctx context.Context,
    structure *analyzer.RepoStructure,
) error {
    // ディレクトリを深さごとにグループ化
    depthMap := make(map[int][]*analyzer.DirectoryInfo)
    maxDepth := 0

    for _, dir := range structure.Directories {
        depthMap[dir.Depth] = append(depthMap[dir.Depth], dir)
        if dir.Depth > maxDepth {
            maxDepth = dir.Depth
        }
    }

    // 深い階層から順番に処理（葉から幹へ）
    // 各階層内では並列処理、階層間では同期処理
    for depth := maxDepth; depth >= 0; depth-- {
        directories := depthMap[depth]
        if len(directories) == 0 {
            continue
        }

        log.Printf("processing directories at depth %d (%d directories)", depth, len(directories))

        // 同じ階層のディレクトリは並列処理可能
        sem := make(chan struct{}, 5) // 最大5並列
        errCh := make(chan error, len(directories))
        var wg sync.WaitGroup

        for _, directory := range directories {
            wg.Add(1)
            go func(dir *analyzer.DirectoryInfo) {
                defer wg.Done()
                sem <- struct{}{}        // 並列数制限
                defer func() { <-sem }() // 解放

                // 各ディレクトリで個別にトランザクション開始・コミット
                if err := s.GenerateSummary(ctx, structure.SnapshotID, dir); err != nil {
                    log.Printf("directory summary failed for %s: %v", dir.Path, err)
                    errCh <- err
                }
            }(directory)
        }

        // この階層の全ディレクトリ処理完了を待つ
        wg.Wait()
        close(errCh)

        // エラー集約（この階層の30%以上失敗したら致命的とみなす）
        var errors []error
        for err := range errCh {
            errors = append(errors, err)
        }

        if len(errors) > len(directories)/3 {
            return fmt.Errorf("too many directory summary failures at depth %d: %d/%d", depth, len(errors), len(directories))
        }

        log.Printf("completed directories at depth %d (failures: %d/%d)", depth, len(errors), len(directories))
    }

    return nil
}

func (s *DirectorySummarizer) GenerateSummary(
    ctx context.Context,
    snapshotID uuid.UUID,
    directory *analyzer.DirectoryInfo,
) error {
    // トランザクション開始
    tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback(ctx)

    // sqlc.Queriesをトランザクションでラップ
    queries := sqlc.New(tx)

    // ディレクトリ直下の全ファイルの要約を取得
    fileSummaries, err := s.collectAllFileSummaries(ctx, queries, snapshotID, directory.Files)
    if err != nil && len(directory.Files) > 0 {
        // ファイルがあるのに要約が取得できない場合はエラー
        return fmt.Errorf("failed to collect file summaries: %w", err)
    }

    // サブディレクトリの要約を取得（階層的集約）
    subdirSummaries, err := s.collectSubdirectorySummaries(ctx, queries, snapshotID, directory.Subdirectories)
    if err != nil {
        // サブディレクトリ要約が取得できない場合は警告のみ
        log.Printf("warning: failed to collect subdirectory summaries for %s: %v", directory.Path, err)
        subdirSummaries = ""
    }

    // ファイル要約もサブディレクトリ要約もない場合はスキップ
    if fileSummaries == "" && subdirSummaries == "" {
        log.Printf("info: skipping directory %s (no files or subdirectories)", directory.Path)
        return nil
    }

    // プロンプト構築（ファイル要約 + サブディレクトリ要約）
    prompt := s.buildPrompt(directory, fileSummaries, subdirSummaries)

    // セキュリティフィルタ（プロンプト全体に適用）
    if s.securityFilter.ContainsSensitiveInfo(prompt) {
        prompt = s.securityFilter.MaskSensitiveInfo(prompt)
    }

    // LLM生成
    summary, err := s.llm.GenerateWithRetry(ctx, prompt, 3)
    if err != nil {
        return err
    }

    // Embedding生成
    embedding, err := s.embedder.CreateEmbeddingWithRetry(ctx, summary, 3)
    if err != nil {
        return err
    }

    // メタデータ構築
    metadata := map[string]interface{}{
        "model":             "text-embedding-3-small",
        "dim":               len(embedding),
        "file_count":        len(directory.Files),
        "subdir_count":      len(directory.Subdirectories),
        "total_files":       directory.TotalFiles,
        "languages":         directory.Languages,
        "llm_model":         "gpt-4o-mini",
        "prompt_version":    "2.0",
        "aggregation_mode":  "hierarchical", // 階層的集約を明示
        "generated_at":      time.Now().Format(time.RFC3339),
    }
    metadataJSON, _ := json.Marshal(metadata)

    // directory_summariesテーブルにUPSERT
    _, err = tx.Exec(ctx, `
        INSERT INTO directory_summaries
        (snapshot_id, path, parent_path, depth, summary, embedding, metadata)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (snapshot_id, path)
        DO UPDATE SET
            parent_path = EXCLUDED.parent_path,
            depth = EXCLUDED.depth,
            summary = EXCLUDED.summary,
            embedding = EXCLUDED.embedding,
            metadata = EXCLUDED.metadata,
            updated_at = CURRENT_TIMESTAMP
    `,
        snapshotID,
        directory.Path,
        directory.ParentPath,
        directory.Depth,
        summary,
        embedding,
        metadataJSON,
    )
    if err != nil {
        return fmt.Errorf("failed to upsert directory summary: %w", err)
    }

    // コミット
    return tx.Commit(ctx)
}

func (s *DirectorySummarizer) collectAllFileSummaries(
    ctx context.Context,
    queries *sqlc.Queries,
    snapshotID uuid.UUID,
    filePaths []string,
) (string, error) {
    // ディレクトリ直下の全ファイルの要約を取得
    const maxContextTokens = 8000 // トークンベースで管理
    var summaries []string
    totalTokens := 0

    for _, filePath := range filePaths {
        // file_summariesテーブルからファイル要約を取得
        // まずfilesテーブルからfile_idを取得し、それを使ってfile_summariesを検索
        summary, err := queries.GetFileSummaryByPath(ctx, snapshotID, filePath)
        if err != nil {
            log.Printf("warning: failed to get file summary for %s: %v", filePath, err)
            continue
        }

        if summary == "" {
            continue
        }

        // ファイルサマリーを整形
        summaryText := fmt.Sprintf("## %s\n%s\n", filepath.Base(filePath), summary)

        // トークン数を推定（文字数 / 4 で概算）
        estimatedTokens := len(summaryText) / 4

        // コンテキスト長チェック（安全マージン20%）
        if totalTokens+estimatedTokens > int(float64(maxContextTokens)*0.8) {
            log.Printf("warning: context limit reached for directory, truncating at %d files", len(summaries))
            summaries = append(summaries, fmt.Sprintf("... (残り %d ファイルは省略されました)", len(filePaths)-len(summaries)))
            break
        }

        summaries = append(summaries, summaryText)
        totalTokens += estimatedTokens
    }

    if len(summaries) == 0 {
        return "", nil // エラーではなく空文字列を返す（サブディレクトリのみの場合もある）
    }

    return strings.Join(summaries, "\n\n"), nil
}

func (s *DirectorySummarizer) collectSubdirectorySummaries(
    ctx context.Context,
    queries *sqlc.Queries,
    snapshotID uuid.UUID,
    subdirectories []string,
) (string, error) {
    // サブディレクトリの要約を取得（階層的集約）
    const maxContextTokens = 8000
    var summaries []string
    totalTokens := 0

    for _, subdirPath := range subdirectories {
        // directory_summariesテーブルからサブディレクトリ要約を取得
        summary, err := queries.GetDirectorySummaryByPath(ctx, snapshotID, subdirPath)
        if err != nil {
            log.Printf("warning: failed to get subdirectory summary for %s: %v", subdirPath, err)
            continue
        }

        if summary == "" {
            continue
        }

        // サブディレクトリ要約を整形
        summaryText := fmt.Sprintf("### サブディレクトリ: %s\n%s\n", filepath.Base(subdirPath), summary)

        // トークン数を推定
        estimatedTokens := len(summaryText) / 4

        // コンテキスト長チェック（安全マージン20%）
        if totalTokens+estimatedTokens > int(float64(maxContextTokens)*0.8) {
            log.Printf("warning: context limit reached for subdirectories, truncating at %d subdirs", len(summaries))
            summaries = append(summaries, fmt.Sprintf("... (残り %d サブディレクトリは省略されました)", len(subdirectories)-len(summaries)))
            break
        }

        summaries = append(summaries, summaryText)
        totalTokens += estimatedTokens
    }

    if len(summaries) == 0 {
        return "", nil // エラーではなく空文字列を返す
    }

    return strings.Join(summaries, "\n\n"), nil
}

func (s *DirectorySummarizer) buildPrompt(directory *analyzer.DirectoryInfo, filesContent, subdirContent string) string {
    // プロンプトテンプレートの詳細は以下のドキュメントを参照
    // docs/architecture-wiki-prompt-template.md - セクションB: DirectorySummarizer用プロンプト

    // ディレクトリ直下のファイル要約 + サブディレクトリ要約を統合して、
    // ディレクトリレベルの要約を生成する（階層的集約）
    // 実装の詳細はプロンプトテンプレートドキュメントを参照

    return buildDirectorySummaryPrompt(directory, filesContent, subdirContent)
}
```

---

### 4. WikiGenerator（Wiki生成エンジン）

**責務**: 階層的要約を活用してMarkdown Wikiを生成

**配置**: `pkg/wiki/generator/wiki_generator.go`

```go
// pkg/wiki/generator/wiki_generator.go

type WikiGenerator struct {
    db            *sqlc.Queries
    llm           wiki.LLMClient
    searcher      *search.VectorSearcher
    promptBuilder *PromptBuilder
}

// GenerateArchitecturePage はarchitecture.mdページを生成
func (g *WikiGenerator) GenerateArchitecturePage(
    ctx context.Context,
    productID uuid.UUID,
    outputDir string,
) error {
    // 1. プロダクトの全ソースを取得
    sources, err := g.db.ListSourcesByProduct(ctx, productID)
    if err != nil {
        return err
    }

    // 2. 各ソースの最新スナップショットを取得
    var snapshots []uuid.UUID
    for _, source := range sources {
        snapshot, err := g.db.GetLatestSnapshot(ctx, source.ID)
        if err != nil {
            continue
        }
        snapshots = append(snapshots, snapshot.ID)
    }

    // 3. アーキテクチャ要約を取得（優先）
    var architectureSummaries []string
    for _, snapshotID := range snapshots {
        // architecture_summariesテーブルから各種アーキテクチャ要約を取得
        summaryTypes := []string{"overview", "tech_stack", "data_flow", "components"}
        for _, summaryType := range summaryTypes {
            summary, err := g.db.GetArchitectureSummary(ctx, snapshotID, summaryType)
            if err == nil && summary != "" {
                architectureSummaries = append(architectureSummaries, summary)
            }
        }
    }

    // 4. 疑似クエリでRAG検索（補足情報）
    pseudoQuery := "システムアーキテクチャ、コンポーネント間の依存関係、データフロー、技術的な設計判断を説明する"

    searchResults, err := g.searcher.SearchByProduct(ctx, productID, pseudoQuery, 25)
    if err != nil {
        return err
    }

    // 5. プロンプト構築（階層的コンテキスト）
    prompt := g.promptBuilder.BuildArchitecturePrompt(
        architectureSummaries,  // 最優先コンテキスト
        searchResults,           // 補足コンテキスト
        pseudoQuery,
    )

    // 6. LLM生成
    markdown, err := g.llm.Generate(ctx, prompt)
    if err != nil {
        return err
    }

    // 7. ファイル出力
    outputPath := filepath.Join(outputDir, "architecture.md")
    return os.WriteFile(outputPath, []byte(markdown), 0644)
}

// GenerateDirectoryPage はdirectories/<directory>.mdページを生成
func (g *WikiGenerator) GenerateDirectoryPage(
    ctx context.Context,
    sourceID uuid.UUID,
    directoryPath string,
    outputDir string,
) error {
    // 1. 最新スナップショットを取得
    snapshot, err := g.db.GetLatestSnapshot(ctx, sourceID)
    if err != nil {
        return err
    }

    // 2. ディレクトリ要約を取得（優先）
    summaryContent, err := g.db.GetDirectorySummary(ctx, snapshot.ID, directoryPath)
    if err != nil {
        log.Printf("warning: failed to get directory summary for %s: %v", directoryPath, err)
        summaryContent = ""
    }

    // 3. 疑似クエリでRAG検索（パスフィルタ適用）
    pseudoQuery := fmt.Sprintf("%sの責務、実装詳細、関連ファイル、処理フローを説明する", directoryPath)

    searchResults, err := g.searcher.SearchBySourceWithPathFilter(
        ctx,
        sourceID,
        pseudoQuery,
        15,
        directoryPath, // パスプレフィックス
    )
    if err != nil {
        return err
    }

    // 4. プロンプト構築
    prompt := g.promptBuilder.BuildDirectoryPrompt(
        summaryContent,   // 優先コンテキスト
        searchResults,    // 補足コンテキスト
        pseudoQuery,
    )

    // 5. LLM生成
    markdown, err := g.llm.Generate(ctx, prompt)
    if err != nil {
        return err
    }

    // 6. ファイル出力
    dirName := strings.ReplaceAll(directoryPath, "/", "_")
    outputPath := filepath.Join(outputDir, "directories", dirName+".md")

    os.MkdirAll(filepath.Dir(outputPath), 0755)
    return os.WriteFile(outputPath, []byte(markdown), 0644)
}
```

---

### 5. PromptBuilder（階層的プロンプト構築）

**責務**: 階層的要約 + RAG検索結果を統合したプロンプトを構築

**配置**: `pkg/wiki/generator/prompt_builder.go`

```go
// pkg/wiki/generator/prompt_builder.go

type PromptBuilder struct {
    maxContextTokens int // デフォルト: 8000トークン
}

func NewPromptBuilder() *PromptBuilder {
    return &PromptBuilder{
        maxContextTokens: 8000,
    }
}

func (p *PromptBuilder) BuildArchitecturePrompt(
    architectureSummaries []string,
    searchResults []*search.SearchResult,
    pseudoQuery string,
) string {
    // プロンプトテンプレートの詳細は以下のドキュメントを参照
    // docs/architecture-wiki-prompt-template.md - セクション5: アーキテクチャページ生成用プロンプト

    // アーキテクチャ要約（最優先）とRAG検索結果（補足）を統合して
    // Markdown形式のアーキテクチャWikiページを生成するプロンプトを構築
    // 実装の詳細はプロンプトテンプレートドキュメントを参照

    return buildArchitectureWikiPrompt(architectureSummaries, searchResults, pseudoQuery)
}

func (p *PromptBuilder) BuildDirectoryPrompt(
    directorySummary string,
    searchResults []*search.SearchResult,
    pseudoQuery string,
) string {
    // プロンプトテンプレートの詳細は以下のドキュメントを参照
    // docs/architecture-wiki-prompt-template.md - セクションC: Directoryページ生成用プロンプト

    // ディレクトリ要約（優先）とRAG検索結果（実装詳細）を統合して
    // Markdown形式のディレクトリWikiページを生成するプロンプトを構築
    // 実装の詳細はプロンプトテンプレートドキュメントを参照

    return buildDirectoryWikiPrompt(directorySummary, searchResults, pseudoQuery)
}

func (p *PromptBuilder) groupByFilePath(results []*search.SearchResult) map[string][]*search.SearchResult {
    grouped := make(map[string][]*search.SearchResult)
    for _, result := range results {
        grouped[result.FilePath] = append(grouped[result.FilePath], result)
    }
    return grouped
}
```

---

## 実装計画

### マイグレーション優先

```bash
1. スキーママイグレーション作成
   - schema/migrations/009_add_summary_tables.up.sql
   - schema/migrations/009_add_summary_tables.down.sql

2. sqlcクエリ定義
   - queries/file_summaries.sql
   - queries/directory_summaries.sql
   - queries/architecture_summaries.sql

3. sqlc生成実行
   - sqlc generate
```

**マイグレーション内容**:

`009_add_summary_tables.up.sql`:
- file_summariesテーブルを作成
- directory_summariesテーブルを作成
- architecture_summariesテーブルを作成
- 各テーブルにインデックスを作成（ivfflat、B-tree等）
- コメントを追加
- UNIQUE制約を設定

**既存データについて**:
- DBには現在データが入っていないため、既存データの移行は不要
- chunksテーブルは最初からコードチャンク専用として使用される


---

## 参考資料

- 既存設計: [requirements.md](requirements.md), [design.md](design.md)
- 既存スキーマ: [schema.sql](../schema/schema.sql)
- pgvector: https://github.com/pgvector/pgvector
- PostgreSQL UPSERT: https://www.postgresql.org/docs/current/sql-insert.html
- OpenAI Embeddings: https://platform.openai.com/docs/guides/embeddings
- Go AST Parser: https://pkg.go.dev/go/parser
