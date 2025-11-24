-- 009_add_summary_tables.down.sql
-- アーキテクチャ理解Wiki生成システムの要約テーブルをロールバック

-- テーブルを削除（外部キー制約の依存関係順に削除）
DROP TABLE IF EXISTS architecture_summaries;
DROP TABLE IF EXISTS directory_summaries;
DROP TABLE IF EXISTS file_summaries;
