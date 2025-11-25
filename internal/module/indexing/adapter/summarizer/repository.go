package summarizer

import (
	"context"
	"fmt"

	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	"github.com/jinford/dev-rag/internal/platform/database"
)

// fileSummaryRepository は domain.FileSummaryRepository の実装です
type fileSummaryRepository struct {
	txProvider *database.TransactionProvider
}

// NewFileSummaryRepository は新しい domain.FileSummaryRepository を作成します
func NewFileSummaryRepository(txProvider *database.TransactionProvider) domain.FileSummaryRepository {
	return &fileSummaryRepository{
		txProvider: txProvider,
	}
}

// Upsert はファイルサマリーをUPSERTします（冪等性保証）
func (r *fileSummaryRepository) Upsert(ctx context.Context, summary *domain.FileSummary) error {
	_, err := database.Transact(ctx, r.txProvider, func(adapters *database.Adapter) (struct{}, error) {
		_, err := adapters.Wiki.UpsertFileSummary(
			ctx,
			summary.FileID,
			summary.SummaryText,
			summary.Embedding,
			summary.MetadataJSON,
		)
		if err != nil {
			return struct{}{}, fmt.Errorf("failed to upsert file summary: %w", err)
		}
		return struct{}{}, nil
	})

	return err
}
