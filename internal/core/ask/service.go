package ask

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jinford/dev-rag/internal/core/search"
)

// LLMClient はLLM通信インターフェース
type LLMClient interface {
	GenerateCompletion(ctx context.Context, prompt string) (string, error)
}

// AskService は質問応答のビジネスロジックを提供する
type AskService struct {
	searchService *search.SearchService
	llm           LLMClient
	logger        *slog.Logger
}

type AskServiceOption func(*AskService)

// WithAskLogger は AskService にロガーを設定する
func WithAskLogger(logger *slog.Logger) AskServiceOption {
	return func(s *AskService) {
		s.logger = logger
	}
}

// NewAskService は新しいAskServiceを作成する
func NewAskService(
	searchService *search.SearchService,
	llm LLMClient,
	opts ...AskServiceOption,
) *AskService {
	svc := &AskService{
		searchService: searchService,
		llm:           llm,
		logger:        slog.Default(),
	}

	for _, opt := range opts {
		opt(svc)
	}

	if svc.logger == nil {
		svc.logger = slog.Default()
	}

	return svc
}

// Ask は質問に対してRAGベースで回答を生成する
func (s *AskService) Ask(ctx context.Context, params AskParams) (*AskResult, error) {
	// 1. バリデーション
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if params.ProductID.IsAbsent() {
		return nil, fmt.Errorf("productID is required")
	}

	// 2. デフォルト値の設定
	chunkLimit := params.ChunkLimit
	if chunkLimit <= 0 {
		chunkLimit = 10
	}
	summaryLimit := params.SummaryLimit
	if summaryLimit <= 0 {
		summaryLimit = 5
	}

	// 3. HybridSearch実行（ProductID指定でプロダクト横断検索）
	searchParams := search.HybridSearchParams{
		ProductID:    params.ProductID,
		Query:        params.Query,
		ChunkLimit:   chunkLimit,
		SummaryLimit: summaryLimit,
	}

	s.logger.Info("executing hybrid search",
		"productID", params.ProductID.MustGet().String(),
		"query", params.Query,
		"chunkLimit", chunkLimit,
		"summaryLimit", summaryLimit,
	)

	hybridResult, err := s.searchService.HybridSearch(ctx, searchParams)
	if err != nil {
		return nil, fmt.Errorf("hybrid search failed: %w", err)
	}

	s.logger.Info("hybrid search completed",
		"chunks", len(hybridResult.Chunks),
		"summaries", len(hybridResult.Summaries),
	)

	// 4. プロンプト構築
	prompt := BuildAskPrompt(params.Query, hybridResult.Summaries, hybridResult.Chunks)

	// 5. LLMで回答生成
	s.logger.Info("generating answer with LLM")
	answer, err := s.llm.GenerateCompletion(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate answer: %w", err)
	}

	// 6. SourceReferenceを整形して返却
	sources := make([]SourceReference, 0, len(hybridResult.Chunks))
	for _, chunk := range hybridResult.Chunks {
		sources = append(sources, SourceReference{
			FilePath:  chunk.FilePath,
			StartLine: chunk.StartLine,
			EndLine:   chunk.EndLine,
			Score:     chunk.Score,
		})
	}

	s.logger.Info("ask completed successfully",
		"answerLength", len(answer),
		"sources", len(sources),
	)

	return &AskResult{
		Answer:  answer,
		Sources: sources,
	}, nil
}
