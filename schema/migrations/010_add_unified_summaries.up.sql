-- 統合要約テーブル（summaries）の作成
CREATE TABLE IF NOT EXISTS summaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id UUID NOT NULL REFERENCES source_snapshots(id) ON DELETE CASCADE,

    -- 要約の種類と対象
    summary_type VARCHAR(20) NOT NULL,  -- 'file' | 'directory' | 'architecture'
    target_path TEXT NOT NULL,          -- ファイル/ディレクトリパス、アーキテクチャは空文字

    -- 階層情報（ディレクトリ要約用）
    depth INTEGER,                      -- ディレクトリの深さ（0=ルート）
    parent_path TEXT,                   -- 親ディレクトリパス

    -- アーキテクチャ要約の種類
    arch_type VARCHAR(20),              -- 'overview' | 'tech_stack' | 'data_flow' | 'components'

    -- 要約内容
    content TEXT NOT NULL,
    content_hash VARCHAR(64) NOT NULL,  -- 要約内容のハッシュ（変更検知用）

    -- 生成元情報のハッシュ（差分検知用）
    source_hash VARCHAR(64) NOT NULL,   -- 入力データのハッシュ

    -- メタデータ
    metadata JSONB DEFAULT '{}',

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- チェック制約
    CONSTRAINT chk_summary_type CHECK (summary_type IN ('file', 'directory', 'architecture')),
    CONSTRAINT chk_arch_type CHECK (
        (summary_type = 'architecture' AND arch_type IN ('overview', 'tech_stack', 'data_flow', 'components'))
        OR (summary_type != 'architecture' AND arch_type IS NULL)
    )
);

-- インデックス
CREATE INDEX IF NOT EXISTS idx_summaries_snapshot ON summaries(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_summaries_type ON summaries(summary_type);
CREATE INDEX IF NOT EXISTS idx_summaries_path ON summaries(target_path);
CREATE INDEX IF NOT EXISTS idx_summaries_source_hash ON summaries(source_hash);

-- 一意性制約（部分インデックス）
CREATE UNIQUE INDEX IF NOT EXISTS uq_summaries_file ON summaries(snapshot_id, target_path)
    WHERE summary_type = 'file';
CREATE UNIQUE INDEX IF NOT EXISTS uq_summaries_directory ON summaries(snapshot_id, target_path)
    WHERE summary_type = 'directory';
CREATE UNIQUE INDEX IF NOT EXISTS uq_summaries_architecture ON summaries(snapshot_id, arch_type)
    WHERE summary_type = 'architecture';

-- コメント追加
COMMENT ON TABLE summaries IS '階層的要約（ファイル/ディレクトリ/アーキテクチャ）';
COMMENT ON COLUMN summaries.summary_type IS '要約の種類（file/directory/architecture）';
COMMENT ON COLUMN summaries.target_path IS '対象パス（ファイルパス/ディレクトリパス、architectureは空文字）';
COMMENT ON COLUMN summaries.source_hash IS '入力データのハッシュ（差分検知用）';
COMMENT ON COLUMN summaries.content_hash IS '要約内容のハッシュ';

-- 要約のEmbeddingベクトルテーブル
CREATE TABLE IF NOT EXISTS summary_embeddings (
    summary_id UUID PRIMARY KEY REFERENCES summaries(id) ON DELETE CASCADE,
    vector VECTOR(1536) NOT NULL,
    model VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_summary_embeddings_vector ON summary_embeddings
USING ivfflat (vector vector_cosine_ops) WITH (lists = 100);

COMMENT ON TABLE summary_embeddings IS '要約のEmbeddingベクトル';
