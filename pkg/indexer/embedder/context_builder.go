package embedder

import (
	"fmt"
	"strings"

	"github.com/jinford/dev-rag/pkg/indexer/chunker"
	"github.com/jinford/dev-rag/pkg/repository"
	"github.com/pkoukk/tiktoken-go"
)

// ContextBuilder はEmbedding生成用のコンテキストを構築します
type ContextBuilder struct {
	encoder *tiktoken.Tiktoken
	// text-embedding-3-smallの最大トークン数は8191
	maxTokens int
}

// NewContextBuilder は新しいContextBuilderを作成します
func NewContextBuilder() (*ContextBuilder, error) {
	// cl100k_baseエンコーダを使用（OpenAIのtext-embedding-3-smallと互換）
	encoder, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoder: %w", err)
	}

	return &ContextBuilder{
		encoder:   encoder,
		maxTokens: 8191,
	}, nil
}

// BuildContext はチャンクとメタデータからEmbedding用の拡張コンテキストを構築します
//
// フォーマット:
//
//	File: {file_path}
//	Package: {package_name}
//	Function: {function_name}
//
//	{チャンク本体}
//
// トークン制限:
//   - 拡張コンテキストを含めた総トークン数がEmbeddingモデルの上限(8191トークン)を超える場合
//   - チャンク本体を優先し、コンテキスト情報を削減する
func (b *ContextBuilder) BuildContext(chunk *chunker.Chunk, metadata *repository.ChunkMetadata, filePath string) string {
	// メタデータがない場合はチャンク本体のみを返す
	if metadata == nil {
		return chunk.Content
	}

	// コンテキスト情報を構築
	var contextLines []string

	// ファイルパスは必須
	if filePath != "" {
		contextLines = append(contextLines, fmt.Sprintf("File: %s", filePath))
	}

	// コードの場合はPackage/Function/Parentを優先
	if metadata.ParentName != nil && *metadata.ParentName != "" {
		contextLines = append(contextLines, fmt.Sprintf("Package: %s", *metadata.ParentName))
	}

	if metadata.Name != nil && *metadata.Name != "" {
		if metadata.Type != nil && *metadata.Type != "" {
			contextLines = append(contextLines, fmt.Sprintf("%s: %s", capitalizeFirst(*metadata.Type), *metadata.Name))
		} else {
			contextLines = append(contextLines, fmt.Sprintf("Name: %s", *metadata.Name))
		}
	}

	if metadata.Signature != nil && *metadata.Signature != "" {
		contextLines = append(contextLines, fmt.Sprintf("Signature: %s", *metadata.Signature))
	}

	// コンテキスト情報がない場合はチャンク本体のみを返す
	if len(contextLines) == 0 {
		return chunk.Content
	}

	// コンテキスト付きテキストを構築
	contextHeader := strings.Join(contextLines, "\n")
	fullContext := fmt.Sprintf("%s\n\n%s", contextHeader, chunk.Content)

	// トークン数をチェック
	tokens := b.countTokens(fullContext)

	// トークン制限を超えない場合はそのまま返す
	if tokens <= b.maxTokens {
		return fullContext
	}

	// トークン制限を超える場合は、チャンク本体を優先してコンテキスト情報を削減
	return b.trimContext(contextLines, chunk.Content)
}

// trimContext はコンテキスト情報を削減してトークン制限内に収めます
func (b *ContextBuilder) trimContext(contextLines []string, content string) string {
	// まずチャンク本体のトークン数を確認
	contentTokens := b.countTokens(content)

	// チャンク本体だけでトークン制限を超える場合は、チャンク本体をトリミング
	if contentTokens > b.maxTokens {
		return b.trimToTokenLimit(content, b.maxTokens)
	}

	// 残りのトークン数を計算
	remainingTokens := b.maxTokens - contentTokens - 2 // 改行2つ分

	// コンテキスト情報を後ろから削っていく
	for i := len(contextLines) - 1; i >= 0; i-- {
		contextHeader := strings.Join(contextLines[:i+1], "\n")
		contextTokens := b.countTokens(contextHeader)

		if contextTokens <= remainingTokens {
			// トークン制限内に収まる場合
			return fmt.Sprintf("%s\n\n%s", contextHeader, content)
		}
	}

	// すべてのコンテキスト情報を削除してもトークン制限を超える場合は、チャンク本体のみを返す
	return content
}

// countTokens はテキストのトークン数をカウントします
func (b *ContextBuilder) countTokens(text string) int {
	tokens := b.encoder.Encode(text, nil, nil)
	return len(tokens)
}

// trimToTokenLimit はテキストを指定されたトークン数に収まるようトリミングします
func (b *ContextBuilder) trimToTokenLimit(text string, maxTokens int) string {
	// 現在のトークン数をチェック
	tokens := b.encoder.Encode(text, nil, nil)
	if len(tokens) <= maxTokens {
		return text
	}

	// 指定トークン数でトリミング
	trimmedTokens := tokens[:maxTokens]
	decoded := b.encoder.Decode(trimmedTokens)
	return decoded
}

// capitalizeFirst は文字列の最初の文字を大文字にします
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	// ASCIIの場合のみ対応（英語のみ）
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}
