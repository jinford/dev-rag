package domain

import (
	"context"

	"github.com/google/uuid"
)

// Indexer はソースをインデックス化するビジネスロジックインターフェース
type Indexer interface {
	// IndexSource はソースの最新のコミットをインデックス化します
	// ファイル検出 → チャンク化 → 埋め込み生成 → 保存の一連の流れを実行
	IndexSource(ctx context.Context, sourceID uuid.UUID) error
}
