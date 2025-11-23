-- chunksテーブルのメタデータカラムを削除（ロールバック用）

-- UNIQUE制約の削除
ALTER TABLE chunks DROP CONSTRAINT IF EXISTS uq_chunks_chunk_key;

-- インデックスの削除
DROP INDEX IF EXISTS idx_chunks_source_snapshot;
DROP INDEX IF EXISTS idx_chunks_git_commit_hash;
DROP INDEX IF EXISTS idx_chunks_is_latest;
DROP INDEX IF EXISTS idx_chunks_indexed_at;
DROP INDEX IF EXISTS idx_chunks_updated_at;
DROP INDEX IF EXISTS idx_chunks_chunk_type;

-- カラムの削除（逆順）
ALTER TABLE chunks DROP COLUMN IF EXISTS chunk_key;
ALTER TABLE chunks DROP COLUMN IF EXISTS is_latest;
ALTER TABLE chunks DROP COLUMN IF EXISTS file_version;
ALTER TABLE chunks DROP COLUMN IF EXISTS indexed_at;
ALTER TABLE chunks DROP COLUMN IF EXISTS updated_at;
ALTER TABLE chunks DROP COLUMN IF EXISTS author;
ALTER TABLE chunks DROP COLUMN IF EXISTS git_commit_hash;
ALTER TABLE chunks DROP COLUMN IF EXISTS source_snapshot_id;
ALTER TABLE chunks DROP COLUMN IF EXISTS embedding_context;
ALTER TABLE chunks DROP COLUMN IF EXISTS cyclomatic_complexity;
ALTER TABLE chunks DROP COLUMN IF EXISTS comment_ratio;
ALTER TABLE chunks DROP COLUMN IF EXISTS lines_of_code;
ALTER TABLE chunks DROP COLUMN IF EXISTS calls;
ALTER TABLE chunks DROP COLUMN IF EXISTS imports;
ALTER TABLE chunks DROP COLUMN IF EXISTS doc_comment;
ALTER TABLE chunks DROP COLUMN IF EXISTS signature;
ALTER TABLE chunks DROP COLUMN IF EXISTS parent_name;
ALTER TABLE chunks DROP COLUMN IF EXISTS chunk_name;
ALTER TABLE chunks DROP COLUMN IF EXISTS chunk_type;
