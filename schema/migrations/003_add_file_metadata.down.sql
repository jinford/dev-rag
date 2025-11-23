-- filesテーブルのメタデータカラムを削除（ロールバック用）

-- インデックスの削除
DROP INDEX IF EXISTS idx_files_language;
DROP INDEX IF EXISTS idx_files_domain;

-- カラムの削除
ALTER TABLE files DROP COLUMN IF EXISTS domain;
ALTER TABLE files DROP COLUMN IF EXISTS language;
