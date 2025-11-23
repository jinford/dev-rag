-- 既存データのバックフィル用SQL
-- このSQLは既存データとの互換性を確保するために使用します
-- マイグレーション実行後、必要に応じて手動で実行してください

-- 既存チャンクのsource_snapshot_idをバックフィル
-- (ファイルが所属するスナップショットIDを設定)
UPDATE chunks c
SET source_snapshot_id = f.snapshot_id
FROM files f
WHERE c.file_id = f.id
  AND c.source_snapshot_id IS NULL;

-- 既存チャンクのchunk_keyを仮生成
-- (既存データには完全な情報がないため、仮のchunk_keyを生成)
-- 注意: このchunk_keyは完全なフォーマットではないため、後で再インデックスが必要
UPDATE chunks c
SET chunk_key = CONCAT('unknown/unknown/', f.path, '#L', c.start_line, '-L', c.end_line, '@unknown')
FROM files f
WHERE c.file_id = f.id
  AND c.chunk_key = '';

-- 既存チャンクのindexed_atをバックフィル
-- (created_atと同じ値を設定)
-- 注意: 既にALTER TABLEでDEFAULT値が設定されているため、このクエリは不要な場合があります
-- UPDATE chunks
-- SET indexed_at = created_at
-- WHERE indexed_at IS NULL;

-- 実行結果の確認
-- SELECT COUNT(*) FROM chunks WHERE source_snapshot_id IS NOT NULL; -- バックフィル成功件数
-- SELECT COUNT(*) FROM chunks WHERE chunk_key LIKE 'unknown/unknown/%'; -- 仮chunk_key件数
