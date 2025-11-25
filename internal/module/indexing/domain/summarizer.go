package domain

import (
	"context"

	"github.com/google/uuid"
)

// FileSummaryMetadata はファイルサマリーのメタデータを表します
type FileSummaryMetadata struct {
	PrimaryTopics []string `json:"primary_topics"`
	KeySymbols    []string `json:"key_symbols"`
}

// FileSummaryResponse はファイルサマリー生成の結果を表します
type FileSummaryResponse struct {
	PromptVersion string                  `json:"prompt_version"`
	Summary       []string                `json:"summary"`
	Risks         []string                `json:"risks"`
	Metadata      FileSummaryMetadata     `json:"metadata"`
}

// FileSummaryRequest はファイルサマリー生成のリクエストを表します
type FileSummaryRequest struct {
	FilePath    string
	Language    string
	FileContent string
}

// FileSummary は保存用のファイルサマリーを表します
type FileSummary struct {
	FileID       uuid.UUID
	SummaryText  string
	Embedding    []float32
	MetadataJSON []byte
}

// FileSummaryGenerator はLLMを使用してファイルサマリーを生成するポートです
type FileSummaryGenerator interface {
	// Generate はLLMを使用してファイルサマリーを生成します
	Generate(ctx context.Context, req FileSummaryRequest) (*FileSummaryResponse, error)
}

// FileSummaryRepository はファイルサマリーの永続化ポートです
type FileSummaryRepository interface {
	// Upsert はファイルサマリーをUPSERTします（冪等性保証）
	Upsert(ctx context.Context, summary *FileSummary) error
}

// GenerateSummaryText はFileSummaryResponseからMarkdown形式のサマリーテキストを生成します
// この関数は純粋計算であり、副作用がありません
func GenerateSummaryText(resp *FileSummaryResponse) string {
	var text string

	// サマリー項目を箇条書きで追加
	if len(resp.Summary) > 0 {
		text += "## Summary\n\n"
		for _, item := range resp.Summary {
			text += "- " + item + "\n"
		}
		text += "\n"
	}

	// リスクを追加
	if len(resp.Risks) > 0 {
		text += "## Risks\n\n"
		for _, risk := range resp.Risks {
			text += "- " + risk + "\n"
		}
		text += "\n"
	}

	// メタデータを追加
	if len(resp.Metadata.PrimaryTopics) > 0 || len(resp.Metadata.KeySymbols) > 0 {
		text += "## Metadata\n\n"
		if len(resp.Metadata.PrimaryTopics) > 0 {
			text += "**Primary Topics:** "
			for i, topic := range resp.Metadata.PrimaryTopics {
				if i > 0 {
					text += ", "
				}
				text += topic
			}
			text += "\n\n"
		}
		if len(resp.Metadata.KeySymbols) > 0 {
			text += "**Key Symbols:** "
			for i, symbol := range resp.Metadata.KeySymbols {
				if i > 0 {
					text += ", "
				}
				text += symbol
			}
			text += "\n\n"
		}
	}

	return text
}
