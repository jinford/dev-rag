package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jinford/dev-rag/internal/core/ingestion"
	"github.com/jinford/dev-rag/internal/infra/postgres/sqlc"
	pgvector "github.com/pgvector/pgvector-go"
)

// Repository は ingestion.Repository インターフェースを実装する PostgreSQL リポジトリです
type Repository struct {
	q sqlc.Querier
}

// NewRepository は新しい Repository を作成します
func NewRepository(q sqlc.Querier) *Repository {
	return &Repository{q: q}
}

// コンパイル時の型チェック
var _ ingestion.Repository = (*Repository)(nil)

// === Product ===

func (r *Repository) GetProductByID(ctx context.Context, id uuid.UUID) (*ingestion.Product, error) {
	product, err := r.q.GetProduct(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return &ingestion.Product{
		ID:          PgtypeToUUID(product.ID),
		Name:        product.Name,
		Description: PgtextToStringPtr(product.Description),
		CreatedAt:   PgtypeToTime(product.CreatedAt),
		UpdatedAt:   PgtypeToTime(product.UpdatedAt),
	}, nil
}

func (r *Repository) GetProductByName(ctx context.Context, name string) (*ingestion.Product, error) {
	product, err := r.q.GetProductByName(ctx, name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return &ingestion.Product{
		ID:          PgtypeToUUID(product.ID),
		Name:        product.Name,
		Description: PgtextToStringPtr(product.Description),
		CreatedAt:   PgtypeToTime(product.CreatedAt),
		UpdatedAt:   PgtypeToTime(product.UpdatedAt),
	}, nil
}

func (r *Repository) ListProducts(ctx context.Context) ([]*ingestion.Product, error) {
	products, err := r.q.ListProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	result := make([]*ingestion.Product, 0, len(products))
	for _, p := range products {
		result = append(result, &ingestion.Product{
			ID:          PgtypeToUUID(p.ID),
			Name:        p.Name,
			Description: PgtextToStringPtr(p.Description),
			CreatedAt:   PgtypeToTime(p.CreatedAt),
			UpdatedAt:   PgtypeToTime(p.UpdatedAt),
		})
	}

	return result, nil
}

func (r *Repository) ListProductsWithStats(ctx context.Context) ([]*ingestion.ProductWithStats, error) {
	rows, err := r.q.ListProductsWithStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list products with stats: %w", err)
	}

	products := make([]*ingestion.ProductWithStats, 0, len(rows))
	for _, row := range rows {
		product := &ingestion.ProductWithStats{
			ID:          PgtypeToUUID(row.ID),
			Name:        row.Name,
			Description: PgtextToStringPtr(row.Description),
			CreatedAt:   PgtypeToTime(row.CreatedAt),
			UpdatedAt:   PgtypeToTime(row.UpdatedAt),
			SourceCount: int(row.SourceCount),
		}

		if lastIndexed, ok := row.LastIndexedAt.(pgtype.Timestamp); ok && lastIndexed.Valid {
			product.LastIndexedAt = PgtypeToTimePtr(lastIndexed)
		}
		if wikiGenerated, ok := row.WikiGeneratedAt.(pgtype.Timestamp); ok && wikiGenerated.Valid {
			product.WikiGeneratedAt = PgtypeToTimePtr(wikiGenerated)
		}

		products = append(products, product)
	}

	return products, nil
}

func (r *Repository) CreateProductIfNotExists(ctx context.Context, name string, description *string) (*ingestion.Product, error) {
	existing, err := r.q.GetProductByName(ctx, name)
	if err == nil {
		return &ingestion.Product{
			ID:          PgtypeToUUID(existing.ID),
			Name:        existing.Name,
			Description: PgtextToStringPtr(existing.Description),
			CreatedAt:   PgtypeToTime(existing.CreatedAt),
			UpdatedAt:   PgtypeToTime(existing.UpdatedAt),
		}, nil
	}
	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing product: %w", err)
	}

	product, err := r.q.CreateProduct(ctx, sqlc.CreateProductParams{
		Name:        name,
		Description: StringPtrToPgtext(description),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	return &ingestion.Product{
		ID:          PgtypeToUUID(product.ID),
		Name:        product.Name,
		Description: PgtextToStringPtr(product.Description),
		CreatedAt:   PgtypeToTime(product.CreatedAt),
		UpdatedAt:   PgtypeToTime(product.UpdatedAt),
	}, nil
}

func (r *Repository) UpdateProduct(ctx context.Context, id uuid.UUID, name string, description *string) (*ingestion.Product, error) {
	product, err := r.q.UpdateProduct(ctx, sqlc.UpdateProductParams{
		ID:          UUIDToPgtype(id),
		Name:        name,
		Description: StringPtrToPgtext(description),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found: %s", id)
		}
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	return &ingestion.Product{
		ID:          PgtypeToUUID(product.ID),
		Name:        product.Name,
		Description: PgtextToStringPtr(product.Description),
		CreatedAt:   PgtypeToTime(product.CreatedAt),
		UpdatedAt:   PgtypeToTime(product.UpdatedAt),
	}, nil
}

func (r *Repository) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	err := r.q.DeleteProduct(ctx, UUIDToPgtype(id))
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	return nil
}

// === Source ===

func (r *Repository) GetSourceByID(ctx context.Context, id uuid.UUID) (*ingestion.Source, error) {
	sqlcSource, err := r.q.GetSource(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("source not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	var metadata ingestion.SourceMetadata
	if err := json.Unmarshal(sqlcSource.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &ingestion.Source{
		ID:         PgtypeToUUID(sqlcSource.ID),
		ProductID:  PgtypeToUUID(sqlcSource.ProductID),
		Name:       sqlcSource.Name,
		SourceType: ingestion.SourceType(sqlcSource.SourceType),
		Metadata:   metadata,
		CreatedAt:  PgtypeToTime(sqlcSource.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcSource.UpdatedAt),
	}, nil
}

func (r *Repository) GetSourceByName(ctx context.Context, name string) (*ingestion.Source, error) {
	sqlcSource, err := r.q.GetSourceByName(ctx, name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("source not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	var metadata ingestion.SourceMetadata
	if err := json.Unmarshal(sqlcSource.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &ingestion.Source{
		ID:         PgtypeToUUID(sqlcSource.ID),
		ProductID:  PgtypeToUUID(sqlcSource.ProductID),
		Name:       sqlcSource.Name,
		SourceType: ingestion.SourceType(sqlcSource.SourceType),
		Metadata:   metadata,
		CreatedAt:  PgtypeToTime(sqlcSource.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcSource.UpdatedAt),
	}, nil
}

func (r *Repository) ListSourcesByProductID(ctx context.Context, productID uuid.UUID) ([]*ingestion.Source, error) {
	sqlcSources, err := r.q.ListSourcesByProduct(ctx, UUIDToPgtype(productID))
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}

	sources := make([]*ingestion.Source, 0, len(sqlcSources))
	for _, sqlcSource := range sqlcSources {
		var metadata ingestion.SourceMetadata
		if err := json.Unmarshal(sqlcSource.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		sources = append(sources, &ingestion.Source{
			ID:         PgtypeToUUID(sqlcSource.ID),
			ProductID:  PgtypeToUUID(sqlcSource.ProductID),
			Name:       sqlcSource.Name,
			SourceType: ingestion.SourceType(sqlcSource.SourceType),
			Metadata:   metadata,
			CreatedAt:  PgtypeToTime(sqlcSource.CreatedAt),
			UpdatedAt:  PgtypeToTime(sqlcSource.UpdatedAt),
		})
	}

	return sources, nil
}

func (r *Repository) CreateSourceIfNotExists(ctx context.Context, name string, sourceType ingestion.SourceType, productID uuid.UUID, metadata ingestion.SourceMetadata) (*ingestion.Source, error) {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	sqlcSource, err := r.q.CreateSourceIfNotExists(ctx, sqlc.CreateSourceIfNotExistsParams{
		Name:       name,
		SourceType: string(sourceType),
		ProductID:  UUIDToPgtype(productID),
		Metadata:   metadataJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create source: %w", err)
	}

	return &ingestion.Source{
		ID:         PgtypeToUUID(sqlcSource.ID),
		ProductID:  PgtypeToUUID(sqlcSource.ProductID),
		Name:       sqlcSource.Name,
		SourceType: ingestion.SourceType(sqlcSource.SourceType),
		Metadata:   metadata,
		CreatedAt:  PgtypeToTime(sqlcSource.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcSource.UpdatedAt),
	}, nil
}

// === SourceSnapshot ===

func (r *Repository) GetSnapshotByVersion(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*ingestion.SourceSnapshot, error) {
	sqlcSnapshot, err := r.q.GetSourceSnapshotByVersion(ctx, sqlc.GetSourceSnapshotByVersionParams{
		SourceID:          UUIDToPgtype(sourceID),
		VersionIdentifier: versionIdentifier,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("snapshot not found: %s@%s", sourceID, versionIdentifier)
		}
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	return &ingestion.SourceSnapshot{
		ID:                PgtypeToUUID(sqlcSnapshot.ID),
		SourceID:          PgtypeToUUID(sqlcSnapshot.SourceID),
		VersionIdentifier: sqlcSnapshot.VersionIdentifier,
		Indexed:           sqlcSnapshot.Indexed,
		IndexedAt:         PgtypeToTimePtr(sqlcSnapshot.IndexedAt),
		CreatedAt:         PgtypeToTime(sqlcSnapshot.CreatedAt),
	}, nil
}

func (r *Repository) GetLatestIndexedSnapshot(ctx context.Context, sourceID uuid.UUID) (*ingestion.SourceSnapshot, error) {
	sqlcSnapshot, err := r.q.GetLatestIndexedSnapshot(ctx, UUIDToPgtype(sourceID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("no indexed snapshot found for source: %s", sourceID)
		}
		return nil, fmt.Errorf("failed to get latest indexed snapshot: %w", err)
	}

	return &ingestion.SourceSnapshot{
		ID:                PgtypeToUUID(sqlcSnapshot.ID),
		SourceID:          PgtypeToUUID(sqlcSnapshot.SourceID),
		VersionIdentifier: sqlcSnapshot.VersionIdentifier,
		Indexed:           sqlcSnapshot.Indexed,
		IndexedAt:         PgtypeToTimePtr(sqlcSnapshot.IndexedAt),
		CreatedAt:         PgtypeToTime(sqlcSnapshot.CreatedAt),
	}, nil
}

func (r *Repository) ListSnapshotsBySource(ctx context.Context, sourceID uuid.UUID) ([]*ingestion.SourceSnapshot, error) {
	sqlcSnapshots, err := r.q.ListSourceSnapshotsBySource(ctx, UUIDToPgtype(sourceID))
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	snapshots := make([]*ingestion.SourceSnapshot, 0, len(sqlcSnapshots))
	for _, sqlcSnapshot := range sqlcSnapshots {
		snapshots = append(snapshots, &ingestion.SourceSnapshot{
			ID:                PgtypeToUUID(sqlcSnapshot.ID),
			SourceID:          PgtypeToUUID(sqlcSnapshot.SourceID),
			VersionIdentifier: sqlcSnapshot.VersionIdentifier,
			Indexed:           sqlcSnapshot.Indexed,
			IndexedAt:         PgtypeToTimePtr(sqlcSnapshot.IndexedAt),
			CreatedAt:         PgtypeToTime(sqlcSnapshot.CreatedAt),
		})
	}

	return snapshots, nil
}

func (r *Repository) CreateSnapshot(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*ingestion.SourceSnapshot, error) {
	sqlcSnapshot, err := r.q.CreateSourceSnapshot(ctx, sqlc.CreateSourceSnapshotParams{
		SourceID:          UUIDToPgtype(sourceID),
		VersionIdentifier: versionIdentifier,
	})
	if err != nil {
		// PostgreSQLのユニーク制約違反エラー（23505）をチェック
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("failed to create snapshot: %w", ingestion.ErrSnapshotVersionConflict)
		}
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	return &ingestion.SourceSnapshot{
		ID:                PgtypeToUUID(sqlcSnapshot.ID),
		SourceID:          PgtypeToUUID(sqlcSnapshot.SourceID),
		VersionIdentifier: sqlcSnapshot.VersionIdentifier,
		Indexed:           sqlcSnapshot.Indexed,
		IndexedAt:         PgtypeToTimePtr(sqlcSnapshot.IndexedAt),
		CreatedAt:         PgtypeToTime(sqlcSnapshot.CreatedAt),
	}, nil
}

func (r *Repository) MarkSnapshotIndexed(ctx context.Context, snapshotID uuid.UUID) error {
	_, err := r.q.MarkSnapshotIndexed(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return fmt.Errorf("failed to mark snapshot as indexed: %w", err)
	}
	return nil
}

// === GitRef ===

func (r *Repository) GetGitRefByName(ctx context.Context, sourceID uuid.UUID, refName string) (*ingestion.GitRef, error) {
	sqlcRef, err := r.q.GetGitRefByName(ctx, sqlc.GetGitRefByNameParams{
		SourceID: UUIDToPgtype(sourceID),
		RefName:  refName,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("git ref not found: %s/%s", sourceID, refName)
		}
		return nil, fmt.Errorf("failed to get git ref: %w", err)
	}

	return &ingestion.GitRef{
		ID:         PgtypeToUUID(sqlcRef.ID),
		SourceID:   PgtypeToUUID(sqlcRef.SourceID),
		RefName:    sqlcRef.RefName,
		SnapshotID: PgtypeToUUID(sqlcRef.SnapshotID),
		CreatedAt:  PgtypeToTime(sqlcRef.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcRef.UpdatedAt),
	}, nil
}

func (r *Repository) ListGitRefsBySource(ctx context.Context, sourceID uuid.UUID) ([]*ingestion.GitRef, error) {
	sqlcRefs, err := r.q.ListGitRefsBySource(ctx, UUIDToPgtype(sourceID))
	if err != nil {
		return nil, fmt.Errorf("failed to list git refs: %w", err)
	}

	refs := make([]*ingestion.GitRef, 0, len(sqlcRefs))
	for _, sqlcRef := range sqlcRefs {
		refs = append(refs, &ingestion.GitRef{
			ID:         PgtypeToUUID(sqlcRef.ID),
			SourceID:   PgtypeToUUID(sqlcRef.SourceID),
			RefName:    sqlcRef.RefName,
			SnapshotID: PgtypeToUUID(sqlcRef.SnapshotID),
			CreatedAt:  PgtypeToTime(sqlcRef.CreatedAt),
			UpdatedAt:  PgtypeToTime(sqlcRef.UpdatedAt),
		})
	}

	return refs, nil
}

func (r *Repository) UpsertGitRef(ctx context.Context, sourceID uuid.UUID, refName string, snapshotID uuid.UUID) (*ingestion.GitRef, error) {
	sqlcRef, err := r.q.CreateGitRef(ctx, sqlc.CreateGitRefParams{
		SourceID:   UUIDToPgtype(sourceID),
		RefName:    refName,
		SnapshotID: UUIDToPgtype(snapshotID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert git ref: %w", err)
	}

	return &ingestion.GitRef{
		ID:         PgtypeToUUID(sqlcRef.ID),
		SourceID:   PgtypeToUUID(sqlcRef.SourceID),
		RefName:    sqlcRef.RefName,
		SnapshotID: PgtypeToUUID(sqlcRef.SnapshotID),
		CreatedAt:  PgtypeToTime(sqlcRef.CreatedAt),
		UpdatedAt:  PgtypeToTime(sqlcRef.UpdatedAt),
	}, nil
}

// === File ===

func (r *Repository) GetFileByID(ctx context.Context, id uuid.UUID) (*ingestion.File, error) {
	file, err := r.q.GetFile(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("file not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return &ingestion.File{
		ID:          PgtypeToUUID(file.ID),
		SnapshotID:  PgtypeToUUID(file.SnapshotID),
		Path:        file.Path,
		Size:        file.Size,
		ContentType: file.ContentType,
		ContentHash: file.ContentHash,
		Language:    PgtextToStringPtr(file.Language),
		Domain:      PgtextToStringPtr(file.Domain),
		CreatedAt:   PgtypeToTime(file.CreatedAt),
	}, nil
}

func (r *Repository) ListFilesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*ingestion.File, error) {
	rows, err := r.q.ListFilesBySnapshot(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := make([]*ingestion.File, 0, len(rows))
	for _, row := range rows {
		files = append(files, &ingestion.File{
			ID:          PgtypeToUUID(row.ID),
			SnapshotID:  PgtypeToUUID(row.SnapshotID),
			Path:        row.Path,
			Size:        row.Size,
			ContentType: row.ContentType,
			ContentHash: row.ContentHash,
			Language:    PgtextToStringPtr(row.Language),
			Domain:      PgtextToStringPtr(row.Domain),
			CreatedAt:   PgtypeToTime(row.CreatedAt),
		})
	}

	return files, nil
}

func (r *Repository) GetFileHashesBySnapshot(ctx context.Context, snapshotID uuid.UUID) (map[string]string, error) {
	rows, err := r.q.GetFileHashesBySnapshot(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to get file hashes: %w", err)
	}

	hashes := make(map[string]string, len(rows))
	for _, row := range rows {
		hashes[row.Path] = row.ContentHash
	}

	return hashes, nil
}

func (r *Repository) GetFilesByDomain(ctx context.Context, snapshotID uuid.UUID, domain string) ([]*ingestion.File, error) {
	rows, err := r.q.GetFilesByDomain(ctx, sqlc.GetFilesByDomainParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		Domain:     StringToNullableText(domain),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get files by domain: %w", err)
	}

	files := make([]*ingestion.File, 0, len(rows))
	for _, row := range rows {
		files = append(files, &ingestion.File{
			ID:          PgtypeToUUID(row.ID),
			SnapshotID:  PgtypeToUUID(row.SnapshotID),
			Path:        row.Path,
			Size:        row.Size,
			ContentType: row.ContentType,
			ContentHash: row.ContentHash,
			Language:    PgtextToStringPtr(row.Language),
			Domain:      PgtextToStringPtr(row.Domain),
			CreatedAt:   PgtypeToTime(row.CreatedAt),
		})
	}

	return files, nil
}

func (r *Repository) CreateFile(ctx context.Context, snapshotID uuid.UUID, path string, size int64, contentType string, contentHash string, language *string, domain *string) (*ingestion.File, error) {
	file, err := r.q.CreateFile(ctx, sqlc.CreateFileParams{
		SnapshotID:  UUIDToPgtype(snapshotID),
		Path:        path,
		Size:        size,
		ContentType: contentType,
		ContentHash: contentHash,
		Language:    StringPtrToPgtext(language),
		Domain:      StringPtrToPgtext(domain),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return &ingestion.File{
		ID:          PgtypeToUUID(file.ID),
		SnapshotID:  PgtypeToUUID(file.SnapshotID),
		Path:        file.Path,
		Size:        file.Size,
		ContentType: file.ContentType,
		ContentHash: file.ContentHash,
		Language:    PgtextToStringPtr(file.Language),
		Domain:      PgtextToStringPtr(file.Domain),
		CreatedAt:   PgtypeToTime(file.CreatedAt),
	}, nil
}

func (r *Repository) DeleteFileByID(ctx context.Context, id uuid.UUID) error {
	if _, err := r.q.GetFile(ctx, UUIDToPgtype(id)); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("file not found: %s", id)
		}
		return fmt.Errorf("failed to get file: %w", err)
	}

	if err := r.q.DeleteFile(ctx, UUIDToPgtype(id)); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

func (r *Repository) DeleteFilesByPaths(ctx context.Context, snapshotID uuid.UUID, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	if err := r.q.DeleteFilesByPaths(ctx, sqlc.DeleteFilesByPathsParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		Column2:    paths,
	}); err != nil {
		return fmt.Errorf("failed to delete files by paths: %w", err)
	}

	return nil
}

// === Chunk ===

func (r *Repository) GetChunkByID(ctx context.Context, id uuid.UUID) (*ingestion.Chunk, error) {
	chunk, err := r.q.GetChunk(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("chunk not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get chunk: %w", err)
	}

	return convertSQLCChunk(chunk), nil
}

func (r *Repository) ListChunksByFile(ctx context.Context, fileID uuid.UUID) ([]*ingestion.Chunk, error) {
	rows, err := r.q.ListChunksByFile(ctx, UUIDToPgtype(fileID))
	if err != nil {
		return nil, fmt.Errorf("failed to list chunks: %w", err)
	}

	chunks := make([]*ingestion.Chunk, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, convertSQLCChunk(row))
	}

	return chunks, nil
}

func (r *Repository) GetChunkContext(ctx context.Context, chunkID uuid.UUID, beforeCount int, afterCount int) ([]*ingestion.Chunk, error) {
	target, err := r.q.GetChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("chunk not found: %s", chunkID)
		}
		return nil, fmt.Errorf("failed to get target chunk: %w", err)
	}

	minOrdinal := target.Ordinal - int32(beforeCount)
	if minOrdinal < 0 {
		minOrdinal = 0
	}
	maxOrdinal := target.Ordinal + int32(afterCount)

	rows, err := r.q.ListChunksByOrdinalRange(ctx, sqlc.ListChunksByOrdinalRangeParams{
		FileID:    target.FileID,
		Ordinal:   minOrdinal,
		Ordinal_2: maxOrdinal,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get context chunks: %w", err)
	}

	chunks := make([]*ingestion.Chunk, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, convertSQLCChunk(row))
	}

	return chunks, nil
}

func (r *Repository) GetChunkChildren(ctx context.Context, parentID uuid.UUID) ([]*ingestion.Chunk, error) {
	rows, err := r.q.GetChildChunks(ctx, UUIDToPgtype(parentID))
	if err != nil {
		return nil, fmt.Errorf("failed to get child chunks: %w", err)
	}

	chunks := make([]*ingestion.Chunk, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, convertSQLCChunk(row))
	}

	return chunks, nil
}

func (r *Repository) GetChunkParent(ctx context.Context, chunkID uuid.UUID) (*ingestion.Chunk, error) {
	chunk, err := r.q.GetParentChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // 親がいない場合
		}
		return nil, fmt.Errorf("failed to get parent chunk: %w", err)
	}

	return convertSQLCChunk(chunk), nil
}

func (r *Repository) GetChunkTree(ctx context.Context, rootID uuid.UUID, maxDepth int) ([]*ingestion.Chunk, error) {
	result := make([]*ingestion.Chunk, 0)
	visited := make(map[uuid.UUID]bool)

	var traverse func(parentID uuid.UUID, depth int) error
	traverse = func(parentID uuid.UUID, depth int) error {
		if depth > maxDepth {
			return nil
		}
		if visited[parentID] {
			return nil // 循環参照を防止
		}
		visited[parentID] = true

		parent, err := r.GetChunkByID(ctx, parentID)
		if err != nil {
			return err
		}
		result = append(result, parent)

		children, err := r.GetChunkChildren(ctx, parentID)
		if err != nil {
			return err
		}

		for _, child := range children {
			if err := traverse(child.ID, depth+1); err != nil {
				return err
			}
		}

		return nil
	}

	if err := traverse(rootID, 1); err != nil {
		return nil, fmt.Errorf("failed to get chunk tree: %w", err)
	}

	return result, nil
}

func (r *Repository) CreateChunk(ctx context.Context, fileID uuid.UUID, ordinal int, startLine int, endLine int, content string, contentHash string, tokenCount int, metadata *ingestion.ChunkMetadata) (*ingestion.Chunk, error) {
	if metadata == nil {
		metadata = &ingestion.ChunkMetadata{
			Level:    2,
			IsLatest: true,
			ChunkKey: "",
		}
	}

	imports := JSONBFromStringSlice(metadata.Imports)
	calls := JSONBFromStringSlice(metadata.Calls)
	standardImports := JSONBFromStringSlice(metadata.StandardImports)
	externalImports := JSONBFromStringSlice(metadata.ExternalImports)
	internalCalls := JSONBFromStringSlice(metadata.InternalCalls)
	externalCalls := JSONBFromStringSlice(metadata.ExternalCalls)
	typeDependencies := JSONBFromStringSlice(metadata.TypeDependencies)

	chunk, err := r.q.CreateChunk(ctx, sqlc.CreateChunkParams{
		FileID:      UUIDToPgtype(fileID),
		Ordinal:     int32(ordinal),
		StartLine:   int32(startLine),
		EndLine:     int32(endLine),
		Content:     content,
		ContentHash: contentHash,
		TokenCount:  IntToPgtype(tokenCount),
		// 構造メタデータ
		ChunkType:            StringPtrToPgtext(metadata.Type),
		ChunkName:            StringPtrToPgtext(metadata.Name),
		ParentName:           StringPtrToPgtext(metadata.ParentName),
		Signature:            StringPtrToPgtext(metadata.Signature),
		DocComment:           StringPtrToPgtext(metadata.DocComment),
		Imports:              imports,
		Calls:                calls,
		LinesOfCode:          IntPtrToPgInt4(metadata.LinesOfCode),
		CommentRatio:         Float64PtrToPgNumeric(metadata.CommentRatio),
		CyclomaticComplexity: IntPtrToPgInt4(metadata.CyclomaticComplexity),
		EmbeddingContext:     StringPtrToPgtext(metadata.EmbeddingContext),
		// 階層関係と重要度
		Level:           int32(metadata.Level),
		ImportanceScore: Float64PtrToPgNumeric(metadata.ImportanceScore),
		// 詳細な依存関係情報
		StandardImports:  standardImports,
		ExternalImports:  externalImports,
		InternalCalls:    internalCalls,
		ExternalCalls:    externalCalls,
		TypeDependencies: typeDependencies,
		// トレーサビリティ・バージョン管理
		SourceSnapshotID: UUIDPtrToPgtype(metadata.SourceSnapshotID),
		GitCommitHash:    StringPtrToPgtext(metadata.GitCommitHash),
		Author:           StringPtrToPgtext(metadata.Author),
		UpdatedAt:        TimePtrToPgtype(metadata.UpdatedAt),
		IndexedAt:        TimeToPgtype(time.Now()),
		FileVersion:      StringPtrToPgtext(metadata.FileVersion),
		IsLatest:         metadata.IsLatest,
		ChunkKey:         metadata.ChunkKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create chunk: %w", err)
	}

	return convertSQLCChunk(chunk), nil
}

func (r *Repository) BatchCreateChunks(ctx context.Context, chunks []*ingestion.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	rows := make([]sqlc.CreateChunkBatchParams, 0, len(chunks))
	for _, chunk := range chunks {
		rows = append(rows, sqlc.CreateChunkBatchParams{
			FileID:      UUIDToPgtype(chunk.FileID),
			Ordinal:     int32(chunk.Ordinal),
			StartLine:   int32(chunk.StartLine),
			EndLine:     int32(chunk.EndLine),
			Content:     chunk.Content,
			ContentHash: chunk.ContentHash,
			TokenCount:  IntToPgtype(chunk.TokenCount),
		})
	}

	if _, err := r.q.CreateChunkBatch(ctx, rows); err != nil {
		return fmt.Errorf("failed to batch create chunks: %w", err)
	}

	return nil
}

func (r *Repository) DeleteChunksByFileID(ctx context.Context, fileID uuid.UUID) error {
	if err := r.q.DeleteChunksByFile(ctx, UUIDToPgtype(fileID)); err != nil {
		return fmt.Errorf("failed to delete chunks by file: %w", err)
	}
	return nil
}

func (r *Repository) AddChunkRelation(ctx context.Context, parentID, childID uuid.UUID, ordinal int) error {
	if err := r.q.AddChunkRelation(ctx, sqlc.AddChunkRelationParams{
		ParentChunkID: UUIDToPgtype(parentID),
		ChildChunkID:  UUIDToPgtype(childID),
		Ordinal:       int32(ordinal),
	}); err != nil {
		return fmt.Errorf("failed to add chunk relation: %w", err)
	}
	return nil
}

func (r *Repository) UpdateChunkImportanceScore(ctx context.Context, chunkID uuid.UUID, score float64) error {
	err := r.q.UpdateChunkImportanceScore(ctx, sqlc.UpdateChunkImportanceScoreParams{
		ID:              UUIDToPgtype(chunkID),
		ImportanceScore: Float64ToNullableNumeric(score),
	})
	if err != nil {
		return fmt.Errorf("failed to update chunk importance score: %w", err)
	}
	return nil
}

func (r *Repository) BatchUpdateChunkImportanceScores(ctx context.Context, scores map[uuid.UUID]float64) error {
	for chunkID, score := range scores {
		if err := r.UpdateChunkImportanceScore(ctx, chunkID, score); err != nil {
			return err
		}
	}
	return nil
}

// === Embedding ===

func (r *Repository) CreateEmbedding(ctx context.Context, chunkID uuid.UUID, vector []float32, model string) error {
	_, err := r.q.CreateEmbedding(ctx, sqlc.CreateEmbeddingParams{
		ChunkID: UUIDToPgtype(chunkID),
		Vector:  pgvector.NewVector(vector),
		Model:   model,
	})
	if err != nil {
		return fmt.Errorf("failed to create embedding: %w", err)
	}
	return nil
}

func (r *Repository) BatchCreateEmbeddings(ctx context.Context, embeddings []*ingestion.Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	rows := make([]sqlc.CreateEmbeddingBatchParams, 0, len(embeddings))
	for _, embedding := range embeddings {
		rows = append(rows, sqlc.CreateEmbeddingBatchParams{
			ChunkID: UUIDToPgtype(embedding.ChunkID),
			Vector:  pgvector.NewVector(embedding.Vector),
			Model:   embedding.Model,
		})
	}

	var batchErr error
	results := r.q.CreateEmbeddingBatch(ctx, rows)
	results.Exec(func(i int, err error) {
		if err != nil && batchErr == nil {
			batchErr = fmt.Errorf("failed to insert embedding at index %d: %w", i, err)
		}
	})

	if batchErr != nil {
		return fmt.Errorf("failed to batch create embeddings: %w", batchErr)
	}

	return nil
}

// === ChunkDependency ===

func (r *Repository) GetDependenciesByChunk(ctx context.Context, chunkID uuid.UUID) ([]*ingestion.ChunkDependency, error) {
	rows, err := r.q.GetDependenciesByChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies: %w", err)
	}

	deps := make([]*ingestion.ChunkDependency, 0, len(rows))
	for _, row := range rows {
		deps = append(deps, &ingestion.ChunkDependency{
			ID:          PgtypeToUUID(row.ID),
			FromChunkID: PgtypeToUUID(row.FromChunkID),
			ToChunkID:   PgtypeToUUID(row.ToChunkID),
			DepType:     row.DepType,
			Symbol:      PgtextToStringPtr(row.Symbol),
			CreatedAt:   PgtypeToTime(row.CreatedAt),
		})
	}

	return deps, nil
}

func (r *Repository) GetIncomingDependenciesByChunk(ctx context.Context, chunkID uuid.UUID) ([]*ingestion.ChunkDependency, error) {
	rows, err := r.q.GetIncomingDependenciesByChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		return nil, fmt.Errorf("failed to get incoming dependencies: %w", err)
	}

	deps := make([]*ingestion.ChunkDependency, 0, len(rows))
	for _, row := range rows {
		deps = append(deps, &ingestion.ChunkDependency{
			ID:          PgtypeToUUID(row.ID),
			FromChunkID: PgtypeToUUID(row.FromChunkID),
			ToChunkID:   PgtypeToUUID(row.ToChunkID),
			DepType:     row.DepType,
			Symbol:      PgtextToStringPtr(row.Symbol),
			CreatedAt:   PgtypeToTime(row.CreatedAt),
		})
	}

	return deps, nil
}

func (r *Repository) CreateDependency(ctx context.Context, fromChunkID, toChunkID uuid.UUID, depType, symbol string) error {
	return r.q.CreateDependency(ctx, sqlc.CreateDependencyParams{
		FromChunkID: UUIDToPgtype(fromChunkID),
		ToChunkID:   UUIDToPgtype(toChunkID),
		DepType:     depType,
		Symbol:      StringToNullableText(symbol),
	})
}

func (r *Repository) DeleteDependenciesByChunk(ctx context.Context, chunkID uuid.UUID) error {
	return r.q.DeleteDependenciesByChunk(ctx, UUIDToPgtype(chunkID))
}

// === SnapshotFile ===

func (r *Repository) GetSnapshotFiles(ctx context.Context, snapshotID uuid.UUID) ([]*ingestion.SnapshotFile, error) {
	rows, err := r.q.GetSnapshotFilesBySnapshot(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot files: %w", err)
	}

	files := make([]*ingestion.SnapshotFile, 0, len(rows))
	for _, row := range rows {
		files = append(files, &ingestion.SnapshotFile{
			ID:         PgtypeToUUID(row.ID),
			SnapshotID: PgtypeToUUID(row.SnapshotID),
			FilePath:   row.FilePath,
			FileSize:   row.FileSize,
			Domain:     PgtextToStringPtr(row.Domain),
			Indexed:    row.Indexed,
			SkipReason: PgtextToStringPtr(row.SkipReason),
			CreatedAt:  PgtypeToTime(row.CreatedAt),
		})
	}

	return files, nil
}

func (r *Repository) GetDomainCoverageStats(ctx context.Context, snapshotID uuid.UUID) ([]*ingestion.DomainCoverage, error) {
	rows, err := r.q.GetDomainCoverageStats(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to get domain coverage stats: %w", err)
	}

	coverages := make([]*ingestion.DomainCoverage, 0, len(rows))
	for _, row := range rows {
		coverages = append(coverages, &ingestion.DomainCoverage{
			Domain:           row.Domain,
			TotalFiles:       int(row.TotalFiles),
			IndexedFiles:     int(row.IndexedFiles),
			IndexedChunks:    int(row.IndexedChunks),
			CoverageRate:     PgnumericToFloat64(row.CoverageRate),
			AvgCommentRatio:  PgnumericToFloat64(row.AvgCommentRatio),
			AvgComplexity:    PgnumericToFloat64(row.AvgComplexity),
		})
	}

	return coverages, nil
}

func (r *Repository) CreateSnapshotFile(ctx context.Context, snapshotID uuid.UUID, filePath string, fileSize int64, domain *string, indexed bool, skipReason *string) (*ingestion.SnapshotFile, error) {
	sf, err := r.q.CreateSnapshotFile(ctx, sqlc.CreateSnapshotFileParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		FilePath:   filePath,
		FileSize:   fileSize,
		Domain:     StringPtrToPgtext(domain),
		Indexed:    indexed,
		SkipReason: StringPtrToPgtext(skipReason),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot file: %w", err)
	}

	return &ingestion.SnapshotFile{
		ID:         PgtypeToUUID(sf.ID),
		SnapshotID: PgtypeToUUID(sf.SnapshotID),
		FilePath:   sf.FilePath,
		FileSize:   sf.FileSize,
		Domain:     PgtextToStringPtr(sf.Domain),
		Indexed:    sf.Indexed,
		SkipReason: PgtextToStringPtr(sf.SkipReason),
		CreatedAt:  PgtypeToTime(sf.CreatedAt),
	}, nil
}

func (r *Repository) UpdateSnapshotFileIndexed(ctx context.Context, snapshotID uuid.UUID, filePath string, indexed bool) error {
	err := r.q.UpdateSnapshotFileIndexed(ctx, sqlc.UpdateSnapshotFileIndexedParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		FilePath:   filePath,
		Indexed:    indexed,
	})
	if err != nil {
		return fmt.Errorf("failed to update snapshot file indexed status: %w", err)
	}
	return nil
}

// === Helper functions ===

func convertSQLCChunk(row sqlc.Chunk) *ingestion.Chunk {
	return &ingestion.Chunk{
		ID:          PgtypeToUUID(row.ID),
		FileID:      PgtypeToUUID(row.FileID),
		Ordinal:     int(row.Ordinal),
		StartLine:   int(row.StartLine),
		EndLine:     int(row.EndLine),
		Content:     row.Content,
		ContentHash: row.ContentHash,
		TokenCount:  PgtypeToInt(row.TokenCount),
		CreatedAt:   PgtypeToTime(row.CreatedAt),
		// 構造メタデータ
		Type:                 PgtextToStringPtr(row.ChunkType),
		Name:                 PgtextToStringPtr(row.ChunkName),
		ParentName:           PgtextToStringPtr(row.ParentName),
		Signature:            PgtextToStringPtr(row.Signature),
		DocComment:           PgtextToStringPtr(row.DocComment),
		Imports:              StringSliceFromJSONB(row.Imports),
		Calls:                StringSliceFromJSONB(row.Calls),
		LinesOfCode:          PgtypeToIntPtr(row.LinesOfCode),
		CommentRatio:         PgtypeToFloat64Ptr(row.CommentRatio),
		CyclomaticComplexity: PgtypeToIntPtr(row.CyclomaticComplexity),
		EmbeddingContext:     PgtextToStringPtr(row.EmbeddingContext),
		// 階層関係と重要度
		Level:           int(row.Level),
		ImportanceScore: PgtypeToFloat64Ptr(row.ImportanceScore),
		// 詳細な依存関係情報
		StandardImports:  StringSliceFromJSONB(row.StandardImports),
		ExternalImports:  StringSliceFromJSONB(row.ExternalImports),
		InternalCalls:    StringSliceFromJSONB(row.InternalCalls),
		ExternalCalls:    StringSliceFromJSONB(row.ExternalCalls),
		TypeDependencies: StringSliceFromJSONB(row.TypeDependencies),
		// トレーサビリティ・バージョン管理
		SourceSnapshotID: PgtypeToUUIDPtr(row.SourceSnapshotID),
		GitCommitHash:    PgtextToStringPtr(row.GitCommitHash),
		Author:           PgtextToStringPtr(row.Author),
		UpdatedAt:        PgtypeToTimePtr(row.UpdatedAt),
		IndexedAt:        PgtypeToTime(row.IndexedAt),
		FileVersion:      PgtextToStringPtr(row.FileVersion),
		IsLatest:         row.IsLatest,
		// 決定的な識別子
		ChunkKey: row.ChunkKey,
	}
}
