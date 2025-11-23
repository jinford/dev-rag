-- 階層関係と重要度スコアの追加

-- 1. chunksテーブルへのレベルと重要度スコアカラム追加
ALTER TABLE chunks ADD COLUMN level INTEGER NOT NULL DEFAULT 2;  -- 1:ファイルサマリー, 2:関数/クラス, 3:ロジック単位
ALTER TABLE chunks ADD COLUMN importance_score NUMERIC(5,4);     -- 0.0000〜1.0000

-- インデックス追加
CREATE INDEX IF NOT EXISTS idx_chunks_level ON chunks(level);
CREATE INDEX IF NOT EXISTS idx_chunks_importance_score ON chunks(importance_score);

-- コメント追加
COMMENT ON COLUMN chunks.level IS '階層レベル（1:ファイルサマリー, 2:関数/クラス, 3:ロジック単位）';
COMMENT ON COLUMN chunks.importance_score IS '重要度スコア（0.0000〜1.0000、参照回数・中心性・編集頻度から算出）';

-- 2. 階層関係を管理する中間テーブルの作成
CREATE TABLE chunk_hierarchy (
    parent_chunk_id UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    child_chunk_id UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,  -- 子の順序
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (parent_chunk_id, child_chunk_id),
    CONSTRAINT uq_child_ordinal UNIQUE (parent_chunk_id, ordinal),
    CONSTRAINT chk_no_self_reference CHECK (parent_chunk_id != child_chunk_id)
);

-- インデックス追加
CREATE INDEX IF NOT EXISTS idx_hierarchy_parent ON chunk_hierarchy(parent_chunk_id);
CREATE INDEX IF NOT EXISTS idx_hierarchy_child ON chunk_hierarchy(child_chunk_id);

-- コメント追加
COMMENT ON TABLE chunk_hierarchy IS 'チャンクの親子関係を管理する中間テーブル（階層構造の単一の真実源）';
COMMENT ON COLUMN chunk_hierarchy.parent_chunk_id IS '親チャンクのID';
COMMENT ON COLUMN chunk_hierarchy.child_chunk_id IS '子チャンクのID';
COMMENT ON COLUMN chunk_hierarchy.ordinal IS '同一親配下での子チャンクの順序（0始まり）';
COMMENT ON COLUMN chunk_hierarchy.created_at IS '関係の作成日時';
