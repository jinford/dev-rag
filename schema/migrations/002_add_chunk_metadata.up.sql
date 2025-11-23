-- chunksテーブルにメタデータカラムを追加

-- 構造メタデータカラム
ALTER TABLE chunks ADD COLUMN chunk_type VARCHAR(50);        -- function, class, method, etc.
ALTER TABLE chunks ADD COLUMN chunk_name VARCHAR(255);        -- 関数名、クラス名
ALTER TABLE chunks ADD COLUMN parent_name VARCHAR(255);       -- 所属構造体、パッケージ
ALTER TABLE chunks ADD COLUMN signature TEXT;                 -- 関数シグネチャ
ALTER TABLE chunks ADD COLUMN doc_comment TEXT;               -- ドキュメントコメント
ALTER TABLE chunks ADD COLUMN imports JSONB;                  -- インポート情報
ALTER TABLE chunks ADD COLUMN calls JSONB;                    -- 呼び出し関数リスト
ALTER TABLE chunks ADD COLUMN lines_of_code INTEGER;          -- コード行数
ALTER TABLE chunks ADD COLUMN comment_ratio NUMERIC(3,2);     -- コメント比率
ALTER TABLE chunks ADD COLUMN cyclomatic_complexity INTEGER;  -- 循環的複雑度
ALTER TABLE chunks ADD COLUMN embedding_context TEXT;         -- Embedding用拡張テキスト

-- トレーサビリティ・バージョン管理カラム
ALTER TABLE chunks ADD COLUMN source_snapshot_id UUID REFERENCES source_snapshots(id) ON DELETE CASCADE;
ALTER TABLE chunks ADD COLUMN git_commit_hash VARCHAR(40);    -- Gitコミットハッシュ
ALTER TABLE chunks ADD COLUMN author VARCHAR(255);            -- 最終更新者
ALTER TABLE chunks ADD COLUMN updated_at TIMESTAMP;           -- ファイル最終更新日時
ALTER TABLE chunks ADD COLUMN indexed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP; -- インデックス作成日時
ALTER TABLE chunks ADD COLUMN file_version VARCHAR(100);      -- ファイルバージョン識別子
ALTER TABLE chunks ADD COLUMN is_latest BOOLEAN NOT NULL DEFAULT true; -- 最新バージョンフラグ

-- 決定的な識別子
-- フォーマット: {product_name}/{source_name}/{file_path}#L{start_line}-L{end_line}@{commit_hash}
-- 例: myproduct/backend-repo/src/main.go#L10-L25@abc123def456
ALTER TABLE chunks ADD COLUMN chunk_key VARCHAR(512) NOT NULL DEFAULT '';

-- UNIQUE制約の追加（chunk_key）
ALTER TABLE chunks ADD CONSTRAINT uq_chunks_chunk_key UNIQUE (chunk_key);

-- インデックス作成
CREATE INDEX IF NOT EXISTS idx_chunks_source_snapshot ON chunks(source_snapshot_id);
CREATE INDEX IF NOT EXISTS idx_chunks_git_commit_hash ON chunks(git_commit_hash);
CREATE INDEX IF NOT EXISTS idx_chunks_is_latest ON chunks(is_latest);
CREATE INDEX IF NOT EXISTS idx_chunks_indexed_at ON chunks(indexed_at);
CREATE INDEX IF NOT EXISTS idx_chunks_updated_at ON chunks(updated_at);
CREATE INDEX IF NOT EXISTS idx_chunks_chunk_type ON chunks(chunk_type);

-- コメント追加
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
