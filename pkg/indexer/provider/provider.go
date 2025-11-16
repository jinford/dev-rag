package provider

import (
	"context"

	"github.com/jinford/dev-rag/pkg/models"
)

// IndexParams はインデックス化の共通パラメータです
type IndexParams struct {
	ProductName string         // プロダクト名
	Identifier  string         // ソース識別子（GitならURL、ConfluenceならSpaceKey等）
	Options     map[string]any // ソースタイプ固有のオプション
	ForceInit   bool           // 強制初期化（既存データを削除）
}

// SourceDocument はソースから取得されたドキュメントを表します
type SourceDocument struct {
	Path        string // ドキュメントのパス（識別子）
	Content     string // ドキュメントの内容
	Size        int64  // ドキュメントのサイズ（バイト）
	ContentHash string // ドキュメント内容のハッシュ
}

// SourceProvider はソースタイプごとの具体的な実装を提供するインターフェースです
type SourceProvider interface {
	// GetSourceType はソースタイプを返します
	GetSourceType() models.SourceType

	// ExtractSourceName はソース識別子からソース名を抽出します
	ExtractSourceName(identifier string) string

	// FetchDocuments はソースからドキュメント一覧を取得します
	// 戻り値: ドキュメント一覧, バージョン識別子, エラー
	FetchDocuments(ctx context.Context, params IndexParams) ([]*SourceDocument, string, error)

	// CreateMetadata はソースメタデータを作成します
	CreateMetadata(params IndexParams) models.SourceMetadata

	// ShouldIgnore はドキュメントを除外すべきかを判定します
	ShouldIgnore(doc *SourceDocument) bool
}
