-- pgvector拡張のインストール
CREATE EXTENSION IF NOT EXISTS vector;

-- productsテーブル
CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);

COMMENT ON TABLE products IS 'プロダクト（複数のソースをまとめる単位）';
COMMENT ON COLUMN products.id IS 'プロダクトの一意識別子';
COMMENT ON COLUMN products.name IS 'プロダクト名（一意）';
COMMENT ON COLUMN products.description IS 'プロダクトの説明';

-- sourcesテーブル（repositoriesを抽象化）
CREATE TABLE IF NOT EXISTS sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL UNIQUE,
    source_type VARCHAR(50) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sources_name ON sources(name);
CREATE INDEX IF NOT EXISTS idx_sources_type ON sources(source_type);
CREATE INDEX IF NOT EXISTS idx_sources_product_id ON sources(product_id);

COMMENT ON TABLE sources IS 'ドキュメント・コードのソース情報（Git、Confluence、PDFなど）';
COMMENT ON COLUMN sources.id IS 'ソースの一意識別子';
COMMENT ON COLUMN sources.product_id IS '所属するプロダクトのID（必須）';
COMMENT ON COLUMN sources.name IS 'ソース名（一意）';
COMMENT ON COLUMN sources.source_type IS 'ソースタイプ（git/confluence/pdf/redmine/notion/local）';
COMMENT ON COLUMN sources.metadata IS 'ソースタイプ固有の情報（JSONBフォーマット）。例: Gitの場合 {"url": "git@github.com:...", "default_branch": "main"}、Confluenceの場合 {"base_url": "https://...", "space_key": "..."}';

-- source_snapshotsテーブル（snapshotsを抽象化）
CREATE TABLE IF NOT EXISTS source_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    version_identifier TEXT NOT NULL,
    indexed BOOLEAN NOT NULL DEFAULT FALSE,
    indexed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_source_snapshots_source_version UNIQUE (source_id, version_identifier)
);

CREATE INDEX IF NOT EXISTS idx_source_snapshots_source_id ON source_snapshots(source_id);
CREATE INDEX IF NOT EXISTS idx_source_snapshots_version ON source_snapshots(version_identifier);
CREATE INDEX IF NOT EXISTS idx_source_snapshots_indexed ON source_snapshots(indexed) WHERE indexed = TRUE;

COMMENT ON TABLE source_snapshots IS 'ソースの特定バージョン時点のスナップショット';
COMMENT ON COLUMN source_snapshots.id IS 'スナップショットの一意識別子';
COMMENT ON COLUMN source_snapshots.source_id IS '対象ソースのID';
COMMENT ON COLUMN source_snapshots.version_identifier IS 'バージョン識別子（Gitの場合はcommit_hash、Confluenceの場合はpage_version、PDFの場合はfile_hash等）';
COMMENT ON COLUMN source_snapshots.indexed IS 'インデックス完了フラグ';
COMMENT ON COLUMN source_snapshots.indexed_at IS 'インデックス完了日時';

-- git_refsテーブル（Git専用の参照管理）
CREATE TABLE IF NOT EXISTS git_refs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    ref_name VARCHAR(255) NOT NULL,
    snapshot_id UUID NOT NULL REFERENCES source_snapshots(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_git_refs_source_ref UNIQUE (source_id, ref_name)
);

CREATE INDEX IF NOT EXISTS idx_git_refs_source_id ON git_refs(source_id);
CREATE INDEX IF NOT EXISTS idx_git_refs_snapshot_id ON git_refs(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_git_refs_ref_name ON git_refs(ref_name);

COMMENT ON TABLE git_refs IS 'Git専用の参照（ブランチ、タグ）管理';
COMMENT ON COLUMN git_refs.id IS 'Git参照の一意識別子';
COMMENT ON COLUMN git_refs.source_id IS '対象ソースのID（source_type=gitのみ）';
COMMENT ON COLUMN git_refs.ref_name IS '参照名（ブランチ名またはタグ名: main, develop, v1.0.0 等）';
COMMENT ON COLUMN git_refs.snapshot_id IS '参照が指すスナップショットのID';
COMMENT ON COLUMN git_refs.created_at IS '参照の作成日時';
COMMENT ON COLUMN git_refs.updated_at IS '参照の更新日時（別のコミットを指すようになった時）';

-- filesテーブル
CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id UUID NOT NULL REFERENCES source_snapshots(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    size BIGINT NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    language VARCHAR(50),
    domain VARCHAR(50),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_files_snapshot_path UNIQUE (snapshot_id, path)
);

CREATE INDEX IF NOT EXISTS idx_files_snapshot_id ON files(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
CREATE INDEX IF NOT EXISTS idx_files_content_type ON files(content_type);
CREATE INDEX IF NOT EXISTS idx_files_content_hash ON files(content_hash);
CREATE INDEX IF NOT EXISTS idx_files_language ON files(language);
CREATE INDEX IF NOT EXISTS idx_files_domain ON files(domain);

COMMENT ON TABLE files IS 'スナップショット内のファイル・ドキュメント情報';
COMMENT ON COLUMN files.id IS 'ファイルの一意識別子';
COMMENT ON COLUMN files.snapshot_id IS '所属するスナップショットのID';
COMMENT ON COLUMN files.path IS 'ソースルートからの相対パス（またはドキュメント識別子）';
COMMENT ON COLUMN files.size IS 'ファイルサイズ（バイト）';
COMMENT ON COLUMN files.content_type IS 'MIMEタイプ形式のコンテンツ種別（例: text/x-go, text/x-python, text/markdown, application/pdf, text/html）';
COMMENT ON COLUMN files.content_hash IS 'ファイル内容のSHA-256ハッシュ';
COMMENT ON COLUMN files.language IS 'プログラミング言語（go-enryによる自動検出）';
COMMENT ON COLUMN files.domain IS 'ドメイン分類（code, architecture, ops, tests, infra）';

-- chunksテーブル
CREATE TABLE IF NOT EXISTS chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,
    start_line INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    content TEXT NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    token_count INTEGER,
    -- 構造メタデータ (Phase 1追加)
    chunk_type VARCHAR(50),
    chunk_name VARCHAR(255),
    parent_name VARCHAR(255),
    signature TEXT,
    doc_comment TEXT,
    imports JSONB,
    calls JSONB,
    lines_of_code INTEGER,
    comment_ratio NUMERIC(3,2),
    cyclomatic_complexity INTEGER,
    embedding_context TEXT,
    -- トレーサビリティ・バージョン管理 (Phase 1追加)
    source_snapshot_id UUID REFERENCES source_snapshots(id) ON DELETE CASCADE,
    git_commit_hash VARCHAR(40),
    author VARCHAR(255),
    updated_at TIMESTAMP,
    indexed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    file_version VARCHAR(100),
    is_latest BOOLEAN NOT NULL DEFAULT true,
    chunk_key VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_chunks_file_ordinal UNIQUE (file_id, ordinal),
    CONSTRAINT uq_chunks_chunk_key UNIQUE (chunk_key),
    CONSTRAINT chk_chunks_lines CHECK (end_line >= start_line)
);

CREATE INDEX IF NOT EXISTS idx_chunks_file_id ON chunks(file_id);
CREATE INDEX IF NOT EXISTS idx_chunks_file_ordinal ON chunks(file_id, ordinal);
CREATE INDEX IF NOT EXISTS idx_chunks_content_hash ON chunks(content_hash);
CREATE INDEX IF NOT EXISTS idx_chunks_source_snapshot ON chunks(source_snapshot_id);
CREATE INDEX IF NOT EXISTS idx_chunks_git_commit_hash ON chunks(git_commit_hash);
CREATE INDEX IF NOT EXISTS idx_chunks_is_latest ON chunks(is_latest);
CREATE INDEX IF NOT EXISTS idx_chunks_indexed_at ON chunks(indexed_at);
CREATE INDEX IF NOT EXISTS idx_chunks_updated_at ON chunks(updated_at);
CREATE INDEX IF NOT EXISTS idx_chunks_chunk_type ON chunks(chunk_type);

COMMENT ON TABLE chunks IS 'ファイルを分割したチャンク';
COMMENT ON COLUMN chunks.id IS 'チャンクの一意識別子';
COMMENT ON COLUMN chunks.file_id IS '所属するファイルのID';
COMMENT ON COLUMN chunks.ordinal IS 'ファイル内でのチャンク序数（0始まり）';
COMMENT ON COLUMN chunks.start_line IS 'チャンクの開始行番号';
COMMENT ON COLUMN chunks.end_line IS 'チャンクの終了行番号';
COMMENT ON COLUMN chunks.content IS 'チャンクのテキスト内容';
COMMENT ON COLUMN chunks.content_hash IS 'チャンク内容のSHA-256ハッシュ';
COMMENT ON COLUMN chunks.token_count IS '推定トークン数';
COMMENT ON COLUMN chunks.chunk_type IS 'チャンクの種類（function, method, struct, interface, const, var等）';
COMMENT ON COLUMN chunks.chunk_name IS '関数名、クラス名、メソッド名等';
COMMENT ON COLUMN chunks.parent_name IS '所属する構造体名、パッケージ名等';
COMMENT ON COLUMN chunks.signature IS '関数シグネチャ（引数、戻り値）';
COMMENT ON COLUMN chunks.doc_comment IS 'ドキュメントコメント（GoDocコメント等）';
COMMENT ON COLUMN chunks.imports IS 'インポートされているパッケージリスト（JSON配列）';
COMMENT ON COLUMN chunks.calls IS '呼び出されている関数リスト（JSON配列）';
COMMENT ON COLUMN chunks.lines_of_code IS 'コード行数（コメント・空行を除く）';
COMMENT ON COLUMN chunks.comment_ratio IS 'コメント比率（0.00〜1.00）';
COMMENT ON COLUMN chunks.cyclomatic_complexity IS '循環的複雑度（McCabe複雑度）';
COMMENT ON COLUMN chunks.embedding_context IS 'Embedding生成用の拡張コンテキスト';
COMMENT ON COLUMN chunks.source_snapshot_id IS '所属するスナップショットID（トレーサビリティ用）';
COMMENT ON COLUMN chunks.git_commit_hash IS 'Gitコミットハッシュ（トレーサビリティ用）';
COMMENT ON COLUMN chunks.author IS '最終更新者（Git author）';
COMMENT ON COLUMN chunks.updated_at IS 'ファイル最終更新日時（コミット日時）';
COMMENT ON COLUMN chunks.indexed_at IS 'インデックス作成日時';
COMMENT ON COLUMN chunks.file_version IS 'ファイルバージョン識別子（オプション）';
COMMENT ON COLUMN chunks.is_latest IS '最新バージョンフラグ（true=最新、false=過去バージョン）';
COMMENT ON COLUMN chunks.chunk_key IS '決定的な識別子（{product_name}/{source_name}/{file_path}#L{start}-L{end}@{commit_hash}）';

-- embeddingsテーブル
CREATE TABLE IF NOT EXISTS embeddings (
    chunk_id UUID PRIMARY KEY REFERENCES chunks(id) ON DELETE CASCADE,
    vector VECTOR(1536) NOT NULL,
    model VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ベクトル検索用インデックス（IVFFlat）
-- lists パラメータは総チャンク数に応じて調整（目安: sqrt(総行数)）
CREATE INDEX IF NOT EXISTS idx_embeddings_vector_cosine ON embeddings
USING ivfflat (vector vector_cosine_ops)
WITH (lists = 100);

COMMENT ON TABLE embeddings IS 'チャンクのEmbeddingベクトル';
COMMENT ON COLUMN embeddings.chunk_id IS 'チャンクID（主キー兼外部キー）';
COMMENT ON COLUMN embeddings.vector IS 'Embeddingベクトル（1536次元）';
COMMENT ON COLUMN embeddings.model IS '使用したEmbeddingモデル名';

-- wiki_metadataテーブル（プロダクト単位のみ）
CREATE TABLE IF NOT EXISTS wiki_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    output_path TEXT NOT NULL,
    file_count INTEGER NOT NULL DEFAULT 0,
    generated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_wiki_metadata_product UNIQUE (product_id)
);

CREATE INDEX IF NOT EXISTS idx_wiki_metadata_product_id ON wiki_metadata(product_id);
CREATE INDEX IF NOT EXISTS idx_wiki_metadata_generated_at ON wiki_metadata(generated_at DESC);

COMMENT ON TABLE wiki_metadata IS 'Wiki生成の実行履歴とメタデータ（プロダクト単位のみ）';
COMMENT ON COLUMN wiki_metadata.id IS 'Wiki生成レコードの一意識別子';
COMMENT ON COLUMN wiki_metadata.product_id IS '対象プロダクトのID';
COMMENT ON COLUMN wiki_metadata.output_path IS 'Wikiファイルの出力先パス（例: /var/lib/dev-rag/wikis/my-ecommerce/）';
COMMENT ON COLUMN wiki_metadata.file_count IS '生成されたWikiファイル数';
COMMENT ON COLUMN wiki_metadata.generated_at IS 'Wiki生成完了日時';
