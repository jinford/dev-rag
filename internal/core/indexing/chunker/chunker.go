package chunker

import (
	"context"

	"github.com/google/uuid"
)

// Chunker はファイルをチャンクに分割する戦略インターフェース
type Chunker interface {
	// Chunk はファイルの内容をチャンクに分割します
	// path: ファイルパス、content: ファイル内容
	Chunk(ctx context.Context, path string, content string) ([]*ChunkResult, error)
}

// ChunkerFactory は言語に応じた適切な Chunker を取得するファクトリインターフェース
type ChunkerFactory interface {
	// GetChunker は指定された言語に対応する Chunker を取得します
	GetChunker(language string) (Chunker, error)
}

// Chunk はチャンクを表します
type Chunk struct {
	Content   string
	StartLine int
	EndLine   int
	Tokens    int
}

// ChunkWithMetadata はチャンクとメタデータをセットで保持します
type ChunkWithMetadata struct {
	Chunk    *Chunk
	Metadata *ChunkMetadata
}

// ChunkResult はチャンク分割の結果を表します
type ChunkResult struct {
	// 基本情報
	Content   string // チャンクの内容
	StartLine int    // 開始行
	EndLine   int    // 終了行
	Tokens    int    // トークン数

	// メタデータ
	Metadata *ChunkMetadata
}

// ChunkMetadata はチャンクの構造メタデータを表します
type ChunkMetadata struct {
	// 構造情報
	Type       *string // チャンクの種別（function, method, class, package など）
	Name       *string // チャンク名（関数名、クラス名など）
	ParentName *string // 親要素の名前（メソッドの場合はクラス名など）
	Signature  *string // シグネチャ（関数・メソッドの場合）
	DocComment *string // ドキュメントコメント

	// 依存関係情報
	Imports           []string // インポート一覧
	Calls             []string // 関数呼び出し一覧
	StandardImports   []string // 標準ライブラリインポート
	ExternalImports   []string // 外部依存インポート
	InternalCalls     []string // 内部関数呼び出し
	ExternalCalls     []string // 外部関数呼び出し
	TypeDependencies  []string // 型依存

	// コード品質メトリクス
	LinesOfCode          *int     // コード行数
	CommentRatio         *float64 // コメント率
	CyclomaticComplexity *int     // 循環的複雑度

	// 階層と重要度
	Level           int      // 階層レベル（1: ファイル全体、2: 関数/クラス、3: ロジックブロック）
	ImportanceScore *float64 // 重要度スコア

	// Embedding用コンテキスト
	EmbeddingContext *string // Embedding生成時に使用する追加コンテキスト

	// トレーサビリティ
	SourceSnapshotID *uuid.UUID // ソーススナップショットID
	GitCommitHash    *string    // Gitコミットハッシュ
	Author           *string    // 作成者
	FileVersion      *string    // ファイルバージョン
	IsLatest         bool       // 最新版かどうか
	ChunkKey         string     // 決定的な識別子
}

// LanguageDetector はファイルの言語を検出するインターフェース
type LanguageDetector interface {
	// DetectLanguage はファイルパスと内容から言語を検出します
	DetectLanguage(path string, content []byte) (string, error)
}

// TokenCounter はテキストのトークン数をカウントするインターフェース
type TokenCounter interface {
	// CountTokens はテキストのトークン数をカウントします
	CountTokens(text string) int

	// TrimToTokenLimit はテキストを指定されたトークン数に収まるようトリミングします
	TrimToTokenLimit(text string, maxTokens int) string
}

// ChunkerConfig はChunkerの設定を表します
type ChunkerConfig struct {
	// トークン設定
	TargetTokens int // 目標トークン数（デフォルト: 800）
	MaxTokens    int // 最大トークン数（デフォルト: 1600）
	MinTokens    int // 最小トークン数（デフォルト: 100）
	Overlap      int // オーバーラップトークン数（デフォルト: 200）

	// ロジック分割設定
	LineThreshold       int // 行数閾値（デフォルト: 100行）
	ComplexityThreshold int // 循環的複雑度閾値（デフォルト: 15）

	// メタデータ抽出設定
	ExtractMetadata      bool // メタデータを抽出するかどうか
	ExtractDependencies  bool // 依存関係を抽出するかどうか
	CalculateComplexity  bool // 循環的複雑度を計算するかどうか
	GenerateEmbedContext bool // Embeddingコンテキストを生成するかどうか
}

// DefaultChunkerConfig はデフォルトのChunker設定を返します
func DefaultChunkerConfig() *ChunkerConfig {
	return &ChunkerConfig{
		TargetTokens:         800,
		MaxTokens:            1600,
		MinTokens:            100,
		Overlap:              200,
		LineThreshold:        100,
		ComplexityThreshold:  15,
		ExtractMetadata:      true,
		ExtractDependencies:  true,
		CalculateComplexity:  true,
		GenerateEmbedContext: true,
	}
}

// MetricsCollector はチャンク化のメトリクスを収集するインターフェース
type MetricsCollector interface {
	// AST解析メトリクス
	RecordASTParseAttempt()
	RecordASTParseSuccess()
	RecordASTParseFailure()

	// メタデータ抽出メトリクス
	RecordMetadataExtractAttempt()
	RecordMetadataExtractSuccess()
	RecordMetadataExtractFailure()

	// チャンク品質メトリクス
	RecordHighCommentRatioExcluded()
	RecordCyclomaticComplexity(complexity int)
}

// Logger はログ出力のインターフェース
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}
