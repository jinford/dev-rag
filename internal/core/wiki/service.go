package wiki

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// LLMClient はLLMとの通信インターフェース
type LLMClient interface {
	// Generate はプロンプトを受け取り、LLMによる応答を生成する
	Generate(ctx context.Context, prompt string) (string, error)

	// GenerateWithRetry はプロンプトを受け取り、失敗時にリトライしながらLLMによる応答を生成する
	GenerateWithRetry(ctx context.Context, prompt string, maxRetries int) (string, error)
}

// WikiService はWiki生成のビジネスロジックを提供する
type WikiService struct {
	repo      Repository
	llmClient LLMClient
}

// NewWikiService は新しいWikiServiceを作成する
func NewWikiService(repo Repository, llmClient LLMClient) *WikiService {
	return &WikiService{
		repo:      repo,
		llmClient: llmClient,
	}
}

// GenerateWikiParams はWiki生成のパラメータを表す
type GenerateWikiParams struct {
	ProductID  uuid.UUID
	OutputPath string
}

// GenerateWiki はプロダクトのWikiを生成する
func (s *WikiService) GenerateWiki(ctx context.Context, params GenerateWikiParams) error {
	// バリデーション
	if params.ProductID == uuid.Nil {
		return fmt.Errorf("productID is required")
	}
	if params.OutputPath == "" {
		return fmt.Errorf("outputPath is required")
	}

	// TODO: Wiki生成の実装
	// 1. リポジトリ構造の取得
	// 2. ファイル要約の取得/生成
	// 3. Wiki構造の設計
	// 4. LLMを使った各セクションの生成
	// 5. Markdownファイルの出力

	return fmt.Errorf("not implemented yet")
}

// RegenerateWiki は既存のWikiを再生成する
func (s *WikiService) RegenerateWiki(ctx context.Context, productID uuid.UUID) error {
	// バリデーション
	if productID == uuid.Nil {
		return fmt.Errorf("productID is required")
	}

	// TODO: Wiki再生成の実装
	// 既存のWikiメタデータを取得して再生成

	return fmt.Errorf("not implemented yet")
}

// GetWikiStatus は指定されたプロダクトのWiki生成状況を取得する
func (s *WikiService) GetWikiStatus(ctx context.Context, productID uuid.UUID) (*WikiMetadata, error) {
	if productID == uuid.Nil {
		return nil, fmt.Errorf("productID is required")
	}

	metadata, err := s.repo.GetWikiMetadata(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wiki metadata: %w", err)
	}

	return metadata, nil
}
