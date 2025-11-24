-- 009_add_summary_tables.up.sql
-- アーキテクチャ理解Wiki生成システムの要約テーブルを追加

-- 1. file_summariesテーブル（ファイル要約）
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

-- file_summariesのインデックス
CREATE INDEX idx_file_summaries_embedding ON file_summaries
USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX idx_file_summaries_file_id ON file_summaries(file_id);

-- file_summariesのコメント
COMMENT ON TABLE file_summaries IS 'ファイルごとの要約（LLMが生成）';
COMMENT ON COLUMN file_summaries.id IS '要約の一意識別子';
COMMENT ON COLUMN file_summaries.file_id IS '対象ファイルのID';
COMMENT ON COLUMN file_summaries.summary IS 'LLMが生成した要約（Markdown形式）';
COMMENT ON COLUMN file_summaries.embedding IS 'Embeddingベクトル（pgvectorの制約により列全体で次元固定、最初のINSERT時の次元で確定）';
COMMENT ON COLUMN file_summaries.metadata IS 'メタデータ（モデル名、次元数、生成日時等。metadata.dimに実際の次元数を記録）';

-- 2. directory_summariesテーブル（ディレクトリ要約）
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

-- directory_summariesのインデックス
CREATE INDEX idx_directory_summaries_embedding ON directory_summaries
USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX idx_directory_summaries_snapshot_id ON directory_summaries(snapshot_id);
CREATE INDEX idx_directory_summaries_path ON directory_summaries(path);
CREATE INDEX idx_directory_summaries_parent_path ON directory_summaries(parent_path);
CREATE INDEX idx_directory_summaries_depth ON directory_summaries(depth);

-- directory_summariesのコメント
COMMENT ON TABLE directory_summaries IS 'ディレクトリごとの要約（LLMが生成）';
COMMENT ON COLUMN directory_summaries.id IS '要約の一意識別子';
COMMENT ON COLUMN directory_summaries.snapshot_id IS '対象スナップショットのID';
COMMENT ON COLUMN directory_summaries.path IS 'ディレクトリパス';
COMMENT ON COLUMN directory_summaries.parent_path IS '親ディレクトリパス（階層構造用）';
COMMENT ON COLUMN directory_summaries.depth IS 'ディレクトリの深さ（0=ルート）';
COMMENT ON COLUMN directory_summaries.summary IS 'LLMが生成した要約（Markdown形式）';
COMMENT ON COLUMN directory_summaries.embedding IS 'Embeddingベクトル（pgvectorの制約により列全体で次元固定、最初のINSERT時の次元で確定）';
COMMENT ON COLUMN directory_summaries.metadata IS 'メタデータ（ファイル数、言語統計、次元数等。metadata.dimに実際の次元数を記録）';

-- 3. architecture_summariesテーブル（アーキテクチャ要約）
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

-- architecture_summariesのインデックス
CREATE INDEX idx_architecture_summaries_embedding ON architecture_summaries
USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX idx_architecture_summaries_snapshot_id ON architecture_summaries(snapshot_id);
CREATE INDEX idx_architecture_summaries_type ON architecture_summaries(summary_type);

-- architecture_summariesのコメント
COMMENT ON TABLE architecture_summaries IS 'システム全体のアーキテクチャ要約（LLMが生成）';
COMMENT ON COLUMN architecture_summaries.id IS '要約の一意識別子';
COMMENT ON COLUMN architecture_summaries.snapshot_id IS '対象スナップショットのID';
COMMENT ON COLUMN architecture_summaries.summary_type IS '要約種別（overview/tech_stack/data_flow/components）';
COMMENT ON COLUMN architecture_summaries.summary IS 'LLMが生成した要約（Markdown形式）';
COMMENT ON COLUMN architecture_summaries.embedding IS 'Embeddingベクトル（pgvectorの制約により列全体で次元固定、最初のINSERT時の次元で確定）';
COMMENT ON COLUMN architecture_summaries.metadata IS 'メタデータ（統計情報、次元数等。metadata.dimに実際の次元数を記録）';
