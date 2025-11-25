package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	indexingpg "github.com/jinford/dev-rag/internal/module/indexing/adapter/pg"
	indexingsqlc "github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
	wikipg "github.com/jinford/dev-rag/internal/module/wiki/adapter/pg"
	wikisqlc "github.com/jinford/dev-rag/internal/module/wiki/adapter/pg/sqlc"
)

// TransactionProvider follows the pattern described in https://threedots.tech/post/database-transactions-in-go/
// It hides pgx transactions behind a callback that receives data-access adapters.
type TransactionProvider struct {
	pool *pgxpool.Pool
}

// NewTransactionProvider は新しいTransactionProviderを作成します
func NewTransactionProvider(pool *pgxpool.Pool) *TransactionProvider {
	return &TransactionProvider{pool: pool}
}

// Adapter bundles repository adapters that operate inside a single transaction.
type Adapter struct {
	Products *indexingpg.ProductRepository
	Sources  *indexingpg.SourceRepository
	Index    *indexingpg.IndexRepositoryRW
	Wiki     *wikipg.WikiRepositoryRW
	Locks    *Manager
}

func newAdapter(tx pgx.Tx) *Adapter {
	indexingQueries := indexingsqlc.New(tx)
	wikiQueries := wikisqlc.New(tx)
	return &Adapter{
		Products: indexingpg.NewProductRepository(indexingQueries),
		Sources:  indexingpg.NewSourceRepository(indexingQueries),
		Index:    indexingpg.NewIndexRepositoryRW(indexingQueries),
		Wiki:     wikipg.NewWikiRepositoryRW(wikiQueries),
		Locks:    NewManager(tx),
	}
}

// Transact opens a transaction, builds adapters, and passes them to fn.
func Transact[T any](ctx context.Context, p *TransactionProvider, fn func(*Adapter) (T, error)) (T, error) {
	var zero T
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return zero, fmt.Errorf("failed to begin transaction: %w", err)
	}

	adapters := newAdapter(tx)

	result, err := fn(adapters)
	if err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return zero, fmt.Errorf("tx rollback failed: %v (original err: %w)", rbErr, err)
		}
		return zero, err
	}

	if err := tx.Commit(ctx); err != nil {
		return zero, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}
