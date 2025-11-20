-- Phase 2 タスク7: snapshot_filesテーブルの削除

DROP INDEX IF EXISTS idx_snapshot_files_indexed;
DROP INDEX IF EXISTS idx_snapshot_files_domain;
DROP INDEX IF EXISTS idx_snapshot_files_snapshot;
DROP TABLE IF EXISTS snapshot_files;
