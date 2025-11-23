-- 詳細な依存関係情報のロールバック

ALTER TABLE chunks DROP COLUMN IF EXISTS standard_imports;
ALTER TABLE chunks DROP COLUMN IF EXISTS external_imports;
ALTER TABLE chunks DROP COLUMN IF EXISTS internal_calls;
ALTER TABLE chunks DROP COLUMN IF EXISTS external_calls;
ALTER TABLE chunks DROP COLUMN IF EXISTS type_dependencies;
