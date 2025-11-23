-- 依存グラフの構築
-- チャンク間の依存関係を管理するテーブル

CREATE TABLE IF NOT EXISTS chunk_dependencies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_chunk_id UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    to_chunk_id UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    dep_type VARCHAR(50) NOT NULL,  -- 'call', 'import', 'type'
    symbol VARCHAR(255),             -- 依存の対象シンボル（関数名、型名など）
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_chunk_dependency UNIQUE (from_chunk_id, to_chunk_id, dep_type, symbol)
);

-- インデックス作成
CREATE INDEX IF NOT EXISTS idx_chunk_dependencies_from ON chunk_dependencies(from_chunk_id);
CREATE INDEX IF NOT EXISTS idx_chunk_dependencies_to ON chunk_dependencies(to_chunk_id);
CREATE INDEX IF NOT EXISTS idx_chunk_dependencies_type ON chunk_dependencies(dep_type);

-- カラムコメント追加
COMMENT ON TABLE chunk_dependencies IS 'チャンク間の依存関係を管理するテーブル';
COMMENT ON COLUMN chunk_dependencies.from_chunk_id IS '依存元のチャンクID';
COMMENT ON COLUMN chunk_dependencies.to_chunk_id IS '依存先のチャンクID';
COMMENT ON COLUMN chunk_dependencies.dep_type IS '依存関係の種類（call: 関数呼び出し、import: インポート、type: 型依存）';
COMMENT ON COLUMN chunk_dependencies.symbol IS '依存の対象シンボル名';
