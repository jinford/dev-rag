-- 詳細な依存関係情報の追加
-- 標準/外部依存の区別、内部/外部呼び出しの区別、型依存を追加

ALTER TABLE chunks ADD COLUMN standard_imports JSONB;
ALTER TABLE chunks ADD COLUMN external_imports JSONB;
ALTER TABLE chunks ADD COLUMN internal_calls JSONB;
ALTER TABLE chunks ADD COLUMN external_calls JSONB;
ALTER TABLE chunks ADD COLUMN type_dependencies JSONB;

COMMENT ON COLUMN chunks.standard_imports IS '標準ライブラリのインポートリスト（JSON配列）';
COMMENT ON COLUMN chunks.external_imports IS '外部依存のインポートリスト（JSON配列）';
COMMENT ON COLUMN chunks.internal_calls IS 'プロジェクト内部の関数呼び出しリスト（JSON配列）';
COMMENT ON COLUMN chunks.external_calls IS '外部ライブラリの関数呼び出しリスト（JSON配列）';
COMMENT ON COLUMN chunks.type_dependencies IS '型依存リスト（JSON配列）';
