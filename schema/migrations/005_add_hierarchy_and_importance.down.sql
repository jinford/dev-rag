-- 階層関係と重要度スコアのロールバック

-- 1. 階層関係テーブルの削除
DROP TABLE IF EXISTS chunk_hierarchy;

-- 2. chunksテーブルから追加カラムを削除
DROP INDEX IF EXISTS idx_chunks_importance_score;
DROP INDEX IF EXISTS idx_chunks_level;

ALTER TABLE chunks DROP COLUMN IF EXISTS importance_score;
ALTER TABLE chunks DROP COLUMN IF EXISTS level;
