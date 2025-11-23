-- カバレッジマップ構築のためのsnapshot_filesテーブル
-- 全ファイルリスト（インデックス対象外含む）を永続化して正確なカバレッジ率を計算可能にする

CREATE TABLE snapshot_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id UUID NOT NULL REFERENCES source_snapshots(id) ON DELETE CASCADE,
    file_path VARCHAR(512) NOT NULL,
    file_size BIGINT NOT NULL,
    domain VARCHAR(50),          -- ドメイン分類 (code, architecture, ops, tests, infra)
    indexed BOOLEAN NOT NULL DEFAULT false,  -- インデックス済みか
    skip_reason VARCHAR(255),    -- インデックスしなかった理由（除外パターン、バイナリファイル等）
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_snapshot_file UNIQUE (snapshot_id, file_path)
);

-- パフォーマンス向上のためのインデックス
CREATE INDEX idx_snapshot_files_snapshot ON snapshot_files(snapshot_id);
CREATE INDEX idx_snapshot_files_domain ON snapshot_files(domain);
CREATE INDEX idx_snapshot_files_indexed ON snapshot_files(indexed);
