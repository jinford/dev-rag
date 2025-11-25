package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// FileSummaryService はファイル要約の生成フローを管理します
type FileSummaryService struct {
	generator      domain.FileSummaryGenerator
	embedder       domain.Embedder
	repository     domain.FileSummaryRepository
	logger         *slog.Logger
	maxRetries     int
	llmModelName   string
	promptVersion  string
}

// NewFileSummaryService は新しいFileSummaryServiceを作成します
func NewFileSummaryService(
	generator domain.FileSummaryGenerator,
	embedder domain.Embedder,
	repository domain.FileSummaryRepository,
	logger *slog.Logger,
	llmModelName string,
	promptVersion string,
) *FileSummaryService {
	return &FileSummaryService{
		generator:     generator,
		embedder:      embedder,
		repository:    repository,
		logger:        logger,
		maxRetries:    3,
		llmModelName:  llmModelName,
		promptVersion: promptVersion,
	}
}

// GenerateAndSave はファイル要約を生成してリポジトリに保存します
func (s *FileSummaryService) GenerateAndSave(
	ctx context.Context,
	fileID uuid.UUID,
	filePath string,
	language string,
	content string,
) error {
	// 1. LLMでファイルサマリーを生成（リトライ付き）
	summaryResp, err := s.generateFileSummaryWithRetry(ctx, filePath, language, content)
	if err != nil {
		return fmt.Errorf("failed to generate file summary: %w", err)
	}

	// 2. サマリーテキストを構築（純粋計算なので domain 層のヘルパーを使用）
	summaryText := domain.GenerateSummaryText(summaryResp)

	// 3. Embeddingを生成（リトライ付き）
	embedding, err := s.createEmbeddingWithRetry(ctx, summaryText)
	if err != nil {
		return fmt.Errorf("failed to create embedding: %w", err)
	}

	// 4. メタデータを構築
	embeddingMetadata, err := s.embedder.GetMetadata()
	if err != nil {
		return fmt.Errorf("failed to get embedder metadata: %w", err)
	}

	metadata := map[string]interface{}{
		"model":          embeddingMetadata.ModelName,
		"dim":            embeddingMetadata.Dimension,
		"generated_at":   time.Now().Format(time.RFC3339),
		"llm_model":      s.llmModelName,
		"prompt_version": s.promptVersion,
		"primary_topics": summaryResp.Metadata.PrimaryTopics,
		"key_symbols":    summaryResp.Metadata.KeySymbols,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// 5. リポジトリに保存（冪等性保証）
	summary := &domain.FileSummary{
		FileID:       fileID,
		SummaryText:  summaryText,
		Embedding:    embedding,
		MetadataJSON: metadataJSON,
	}

	err = s.repository.Upsert(ctx, summary)
	if err != nil {
		return fmt.Errorf("failed to save file summary: %w", err)
	}

	s.logger.Debug("File summary generated and saved",
		"fileID", fileID,
		"filePath", filePath,
		"embeddingDim", len(embedding),
	)

	return nil
}

// generateFileSummaryWithRetry はリトライ付きでLLMファイルサマリーを生成します
func (s *FileSummaryService) generateFileSummaryWithRetry(
	ctx context.Context,
	filePath string,
	language string,
	content string,
) (*domain.FileSummaryResponse, error) {
	var lastErr error

	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		req := domain.FileSummaryRequest{
			FilePath:    filePath,
			Language:    language,
			FileContent: content,
		}

		resp, err := s.generator.Generate(ctx, req)
		if err != nil {
			lastErr = err
			s.logger.Warn("LLM file summary generation failed",
				"attempt", attempt,
				"maxRetries", s.maxRetries,
				"filePath", filePath,
				"error", err,
			)

			// 最後の試行でない場合は再試行
			if attempt < s.maxRetries {
				// Exponential Backoff
				backoff := time.Duration(1<<uint(attempt-1)) * time.Second
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
					continue
				}
			}
			continue
		}

		// 成功
		if attempt > 1 {
			s.logger.Info("LLM file summary generation succeeded after retry",
				"attempt", attempt,
				"filePath", filePath,
			)
		}
		return resp, nil
	}

	return nil, fmt.Errorf("failed to generate file summary after %d retries: %w", s.maxRetries, lastErr)
}

// createEmbeddingWithRetry はリトライ付きでEmbeddingを生成します
func (s *FileSummaryService) createEmbeddingWithRetry(
	ctx context.Context,
	text string,
) ([]float32, error) {
	var lastErr error

	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		embedding, err := s.embedder.Embed(ctx, text)
		if err != nil {
			lastErr = err
			s.logger.Warn("Embedding generation failed",
				"attempt", attempt,
				"maxRetries", s.maxRetries,
				"error", err,
			)

			// 最後の試行でない場合は再試行
			if attempt < s.maxRetries {
				// Exponential Backoff
				backoff := time.Duration(1<<uint(attempt-1)) * time.Second
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
					continue
				}
			}
			continue
		}

		// 成功
		if attempt > 1 {
			s.logger.Info("Embedding generation succeeded after retry",
				"attempt", attempt,
			)
		}
		return embedding, nil
	}

	return nil, fmt.Errorf("failed to create embedding after %d retries: %w", s.maxRetries, lastErr)
}
