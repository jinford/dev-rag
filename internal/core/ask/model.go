package ask

import (
	"github.com/google/uuid"
	"github.com/samber/mo"
)

// AskParams は質問応答のパラメータを表す
type AskParams struct {
	ProductID    mo.Option[uuid.UUID] // プロダクトID
	Query        string               // ユーザーの質問文
	ChunkLimit   int                  // チャンク検索の上限（デフォルト: 10）
	SummaryLimit int                  // 要約検索の上限（デフォルト: 5）
}

// AskResult は質問応答の結果を表す
type AskResult struct {
	Answer  string            // LLMによる回答
	Sources []SourceReference // 参照したソース情報
}

// SourceReference は回答の根拠となったソース参照を表す
type SourceReference struct {
	FilePath  string  // ファイルパス
	StartLine int     // 開始行
	EndLine   int     // 終了行
	Score     float64 // 関連度スコア
}
