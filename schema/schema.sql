-- pgvector拡張のインストール
CREATE EXTENSION IF NOT EXISTS vector;

-- repositoriesテーブル
CREATE TABLE IF NOT EXISTS repositories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    url TEXT NOT NULL,
    default_branch VARCHAR(100) NOT NULL DEFAULT 'main',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_repositories_name ON repositories(name);

COMMENT ON TABLE repositories IS 'Gitリポジトリの基本情報';
COMMENT ON COLUMN repositories.id IS 'リポジトリの一意識別子';
COMMENT ON COLUMN repositories.name IS 'リポジトリ名（一意）';
COMMENT ON COLUMN repositories.url IS 'GitリポジトリのURL（SSH/HTTPS）';
COMMENT ON COLUMN repositories.default_branch IS 'デフォルトブランチ名';

-- snapshotsテーブル
CREATE TABLE IF NOT EXISTS snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    commit_hash VARCHAR(40) NOT NULL,
    ref_name VARCHAR(255),
    indexed BOOLEAN NOT NULL DEFAULT FALSE,
    indexed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_snapshots_repo_commit UNIQUE (repository_id, commit_hash)
);

CREATE INDEX IF NOT EXISTS idx_snapshots_repository_id ON snapshots(repository_id);
CREATE INDEX IF NOT EXISTS idx_snapshots_commit_hash ON snapshots(commit_hash);
CREATE INDEX IF NOT EXISTS idx_snapshots_ref_name ON snapshots(ref_name);
CREATE INDEX IF NOT EXISTS idx_snapshots_indexed ON snapshots(indexed) WHERE indexed = TRUE;

COMMENT ON TABLE snapshots IS 'リポジトリの特定コミット時点のスナップショット';
COMMENT ON COLUMN snapshots.id IS 'スナップショットの一意識別子';
COMMENT ON COLUMN snapshots.repository_id IS '対象リポジトリのID';
COMMENT ON COLUMN snapshots.commit_hash IS 'Gitコミットハッシュ（40文字のSHA-1）';
COMMENT ON COLUMN snapshots.ref_name IS '参照名（ブランチ名またはタグ名）';
COMMENT ON COLUMN snapshots.indexed IS 'インデックス完了フラグ';
COMMENT ON COLUMN snapshots.indexed_at IS 'インデックス完了日時';

-- filesテーブル
CREATE TABLE IF NOT EXISTS files (
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

CREATE INDEX IF NOT EXISTS idx_files_snapshot_id ON files(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_files_source_type ON files(snapshot_id, source_type);
CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
CREATE INDEX IF NOT EXISTS idx_files_content_hash ON files(content_hash);

COMMENT ON TABLE files IS 'スナップショット内のファイル情報';
COMMENT ON COLUMN files.id IS 'ファイルの一意識別子';
COMMENT ON COLUMN files.snapshot_id IS '所属するスナップショットのID';
COMMENT ON COLUMN files.path IS 'リポジトリルートからの相対パス';
COMMENT ON COLUMN files.size IS 'ファイルサイズ（バイト）';
COMMENT ON COLUMN files.language IS 'プログラミング言語種別';
COMMENT ON COLUMN files.content_hash IS 'ファイル内容のSHA-256ハッシュ';
COMMENT ON COLUMN files.source_type IS 'ソース種別（code/doc/wiki）';

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
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_chunks_file_ordinal UNIQUE (file_id, ordinal),
    CONSTRAINT chk_chunks_lines CHECK (end_line >= start_line)
);

CREATE INDEX IF NOT EXISTS idx_chunks_file_id ON chunks(file_id);
CREATE INDEX IF NOT EXISTS idx_chunks_file_ordinal ON chunks(file_id, ordinal);
CREATE INDEX IF NOT EXISTS idx_chunks_content_hash ON chunks(content_hash);

COMMENT ON TABLE chunks IS 'ファイルを分割したチャンク';
COMMENT ON COLUMN chunks.id IS 'チャンクの一意識別子';
COMMENT ON COLUMN chunks.file_id IS '所属するファイルのID';
COMMENT ON COLUMN chunks.ordinal IS 'ファイル内でのチャンク序数（0始まり）';
COMMENT ON COLUMN chunks.start_line IS 'チャンクの開始行番号';
COMMENT ON COLUMN chunks.end_line IS 'チャンクの終了行番号';
COMMENT ON COLUMN chunks.content IS 'チャンクのテキスト内容';
COMMENT ON COLUMN chunks.content_hash IS 'チャンク内容のSHA-256ハッシュ';
COMMENT ON COLUMN chunks.token_count IS '推定トークン数';

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

-- wiki_metadataテーブル
CREATE TABLE IF NOT EXISTS wiki_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    snapshot_id UUID NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    output_path TEXT NOT NULL,
    file_count INTEGER NOT NULL DEFAULT 0,
    generated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_wiki_metadata_snapshot UNIQUE (snapshot_id)
);

CREATE INDEX IF NOT EXISTS idx_wiki_metadata_repository_id ON wiki_metadata(repository_id);
CREATE INDEX IF NOT EXISTS idx_wiki_metadata_snapshot_id ON wiki_metadata(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_wiki_metadata_generated_at ON wiki_metadata(generated_at DESC);

COMMENT ON TABLE wiki_metadata IS 'Wiki生成の実行履歴とメタデータ';
COMMENT ON COLUMN wiki_metadata.id IS 'Wiki生成レコードの一意識別子';
COMMENT ON COLUMN wiki_metadata.repository_id IS '対象リポジトリのID';
COMMENT ON COLUMN wiki_metadata.snapshot_id IS '対象スナップショットのID';
COMMENT ON COLUMN wiki_metadata.output_path IS 'Wikiファイルの出力先パス（例: /var/lib/dev-rag/wikis/myapp/）';
COMMENT ON COLUMN wiki_metadata.file_count IS '生成されたWikiファイル数';
COMMENT ON COLUMN wiki_metadata.generated_at IS 'Wiki生成完了日時';
