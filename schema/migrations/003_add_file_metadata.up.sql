-- filesテーブルにlanguageとdomainカラムを追加

-- メタデータカラムの追加
ALTER TABLE files ADD COLUMN language VARCHAR(50);     -- プログラミング言語（Go, Python, TypeScript等）
ALTER TABLE files ADD COLUMN domain VARCHAR(50);       -- ドメイン（code, architecture, ops, tests, infra）

-- インデックス作成
CREATE INDEX IF NOT EXISTS idx_files_language ON files(language);
CREATE INDEX IF NOT EXISTS idx_files_domain ON files(domain);

-- コメント追加
COMMENT ON COLUMN files.language IS 'プログラミング言語（go-enryによる自動検出）';
COMMENT ON COLUMN files.domain IS 'ドメイン分類（code, architecture, ops, tests, infra）';
