package ingestion

import (
	"context"
	"time"
)

// IndexParams はインデックス化の共通パラメータ
type IndexParams struct {
	ProductName string         // プロダクト名
	Identifier  string         // ソース識別子（GitならURL、ConfluenceならSpaceKey等）
	Options     map[string]any // ソースタイプ固有のオプション
	ForceInit   bool           // 強制初期化（既存データを削除）
}

// SourceDocument はソースから取得されたドキュメントを表す
type SourceDocument struct {
	Path        string // ドキュメントのパス（識別子）
	Content     string // ドキュメントの内容
	Size        int64  // ドキュメントのサイズ（バイト）
	ContentHash string // ドキュメント内容のハッシュ

	// コミットメタデータ
	CommitHash string    // Gitコミットハッシュ
	Author     string    // 最終更新者
	UpdatedAt  time.Time // ファイル最終更新日時
}

// SourceProvider はソースタイプごとの具体的な実装を提供するインターフェース
// Git、Confluence、Redmine など複数のソースタイプに対応するための拡張ポイント
type SourceProvider interface {
	// GetSourceType はソースタイプを返す
	GetSourceType() SourceType

	// ExtractSourceName はソース識別子からソース名を抽出する
	ExtractSourceName(identifier string) string

	// FetchDocuments はソースからドキュメント一覧を取得する
	// 戻り値: ドキュメント一覧, バージョン識別子, エラー
	FetchDocuments(ctx context.Context, params IndexParams) ([]*SourceDocument, string, error)

	// CreateMetadata はソースメタデータを作成する
	CreateMetadata(params IndexParams) SourceMetadata

	// ShouldIgnore はドキュメントを除外すべきかを判定する
	ShouldIgnore(doc *SourceDocument) bool
}
