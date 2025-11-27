package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jinford/dev-rag/internal/core/ingestion/summary"
	"github.com/jinford/dev-rag/internal/infra/postgres/sqlc"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/samber/mo"
)

// SummaryRepository は summary.Repository インターフェースを実装する PostgreSQL リポジトリ
type SummaryRepository struct {
	q sqlc.Querier
}

// NewSummaryRepository は新しい SummaryRepository を作成する
func NewSummaryRepository(q sqlc.Querier) *SummaryRepository {
	return &SummaryRepository{q: q}
}

// コンパイル時の型チェック
var _ summary.Repository = (*SummaryRepository)(nil)

// === Summary CRUD ===

func (r *SummaryRepository) CreateSummary(ctx context.Context, s *summary.Summary) (*summary.Summary, error) {
	metadataJSON, err := json.Marshal(s.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	sqlcSummary, err := r.q.CreateSummary(ctx, sqlc.CreateSummaryParams{
		SnapshotID:  UUIDToPgtype(s.SnapshotID),
		SummaryType: string(s.SummaryType),
		TargetPath:  s.TargetPath,
		Depth:       IntPtrToPgInt4(s.Depth),
		ParentPath:  StringPtrToPgtext(s.ParentPath),
		ArchType:    archTypeToPgtext(s.ArchType),
		Content:     s.Content,
		ContentHash: s.ContentHash,
		SourceHash:  s.SourceHash,
		Metadata:    metadataJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create summary: %w", err)
	}

	return convertSQLCSummary(sqlcSummary)
}

func (r *SummaryRepository) GetSummaryByID(ctx context.Context, id uuid.UUID) (mo.Option[*summary.Summary], error) {
	sqlcSummary, err := r.q.GetSummaryByID(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows || err == sql.ErrNoRows {
			return mo.None[*summary.Summary](), nil
		}
		return mo.None[*summary.Summary](), fmt.Errorf("failed to get summary: %w", err)
	}

	converted, err := convertSQLCSummary(sqlcSummary)
	if err != nil {
		return mo.None[*summary.Summary](), err
	}

	return mo.Some(converted), nil
}

func (r *SummaryRepository) GetFileSummary(ctx context.Context, snapshotID uuid.UUID, path string) (mo.Option[*summary.Summary], error) {
	sqlcSummary, err := r.q.GetFileSummary(ctx, sqlc.GetFileSummaryParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		TargetPath: path,
	})
	if err != nil {
		if err == pgx.ErrNoRows || err == sql.ErrNoRows {
			return mo.None[*summary.Summary](), nil
		}
		return mo.None[*summary.Summary](), fmt.Errorf("failed to get file summary: %w", err)
	}

	converted, err := convertSQLCSummary(sqlcSummary)
	if err != nil {
		return mo.None[*summary.Summary](), err
	}

	return mo.Some(converted), nil
}

func (r *SummaryRepository) GetDirectorySummary(ctx context.Context, snapshotID uuid.UUID, path string) (mo.Option[*summary.Summary], error) {
	sqlcSummary, err := r.q.GetDirectorySummary(ctx, sqlc.GetDirectorySummaryParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		TargetPath: path,
	})
	if err != nil {
		if err == pgx.ErrNoRows || err == sql.ErrNoRows {
			return mo.None[*summary.Summary](), nil
		}
		return mo.None[*summary.Summary](), fmt.Errorf("failed to get directory summary: %w", err)
	}

	converted, err := convertSQLCSummary(sqlcSummary)
	if err != nil {
		return mo.None[*summary.Summary](), err
	}

	return mo.Some(converted), nil
}

func (r *SummaryRepository) GetArchitectureSummary(ctx context.Context, snapshotID uuid.UUID, archType summary.ArchType) (mo.Option[*summary.Summary], error) {
	sqlcSummary, err := r.q.GetArchitectureSummary(ctx, sqlc.GetArchitectureSummaryParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		ArchType:   archTypeToPgtext(&archType),
	})
	if err != nil {
		if err == pgx.ErrNoRows || err == sql.ErrNoRows {
			return mo.None[*summary.Summary](), nil
		}
		return mo.None[*summary.Summary](), fmt.Errorf("failed to get architecture summary: %w", err)
	}

	converted, err := convertSQLCSummary(sqlcSummary)
	if err != nil {
		return mo.None[*summary.Summary](), err
	}

	return mo.Some(converted), nil
}

func (r *SummaryRepository) UpdateSummary(ctx context.Context, s *summary.Summary) error {
	metadataJSON, err := json.Marshal(s.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = r.q.UpdateSummary(ctx, sqlc.UpdateSummaryParams{
		ID:          UUIDToPgtype(s.ID),
		Content:     s.Content,
		ContentHash: s.ContentHash,
		SourceHash:  s.SourceHash,
		Metadata:    metadataJSON,
	})
	if err != nil {
		if err == pgx.ErrNoRows || err == sql.ErrNoRows {
			return fmt.Errorf("summary not found: %w", sql.ErrNoRows)
		}
		return fmt.Errorf("failed to update summary: %w", err)
	}

	return nil
}

func (r *SummaryRepository) DeleteSummary(ctx context.Context, id uuid.UUID) error {
	err := r.q.DeleteSummary(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows || err == sql.ErrNoRows {
			return fmt.Errorf("summary not found: %w", sql.ErrNoRows)
		}
		return fmt.Errorf("failed to delete summary: %w", err)
	}
	return nil
}

// === Summary一覧取得 ===

func (r *SummaryRepository) ListFileSummariesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*summary.Summary, error) {
	sqlcSummaries, err := r.q.ListFileSummariesBySnapshot(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to list file summaries: %w", err)
	}

	summaries := make([]*summary.Summary, 0, len(sqlcSummaries))
	for _, s := range sqlcSummaries {
		converted, err := convertSQLCSummary(s)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, converted)
	}

	return summaries, nil
}

func (r *SummaryRepository) ListDirectorySummariesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*summary.Summary, error) {
	sqlcSummaries, err := r.q.ListDirectorySummariesBySnapshot(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to list directory summaries: %w", err)
	}

	summaries := make([]*summary.Summary, 0, len(sqlcSummaries))
	for _, s := range sqlcSummaries {
		converted, err := convertSQLCSummary(s)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, converted)
	}

	return summaries, nil
}

func (r *SummaryRepository) ListDirectorySummariesByDepth(ctx context.Context, snapshotID uuid.UUID, depth int) ([]*summary.Summary, error) {
	sqlcSummaries, err := r.q.ListDirectorySummariesByDepth(ctx, sqlc.ListDirectorySummariesByDepthParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		Depth:      IntPtrToPgInt4(&depth),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list directory summaries by depth: %w", err)
	}

	summaries := make([]*summary.Summary, 0, len(sqlcSummaries))
	for _, s := range sqlcSummaries {
		converted, err := convertSQLCSummary(s)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, converted)
	}

	return summaries, nil
}

func (r *SummaryRepository) ListArchitectureSummariesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*summary.Summary, error) {
	sqlcSummaries, err := r.q.ListArchitectureSummariesBySnapshot(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to list architecture summaries: %w", err)
	}

	summaries := make([]*summary.Summary, 0, len(sqlcSummaries))
	for _, s := range sqlcSummaries {
		converted, err := convertSQLCSummary(s)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, converted)
	}

	return summaries, nil
}

// === 差分検知用 ===

func (r *SummaryRepository) GetMaxDirectoryDepth(ctx context.Context, snapshotID uuid.UUID) (int, error) {
	depth, err := r.q.GetMaxDirectoryDepth(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return 0, fmt.Errorf("failed to get max directory depth: %w", err)
	}

	return int(depth), nil
}

// === Embedding ===

func (r *SummaryRepository) CreateSummaryEmbedding(ctx context.Context, e *summary.SummaryEmbedding) error {
	_, err := r.q.CreateSummaryEmbedding(ctx, sqlc.CreateSummaryEmbeddingParams{
		SummaryID: UUIDToPgtype(e.SummaryID),
		Vector:    pgvector.NewVector(e.Vector),
		Model:     e.Model,
	})
	if err != nil {
		return fmt.Errorf("failed to create summary embedding: %w", err)
	}
	return nil
}

func (r *SummaryRepository) UpsertSummaryEmbedding(ctx context.Context, e *summary.SummaryEmbedding) error {
	_, err := r.q.UpsertSummaryEmbedding(ctx, sqlc.UpsertSummaryEmbeddingParams{
		SummaryID: UUIDToPgtype(e.SummaryID),
		Vector:    pgvector.NewVector(e.Vector),
		Model:     e.Model,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert summary embedding: %w", err)
	}
	return nil
}

func (r *SummaryRepository) GetSummaryEmbedding(ctx context.Context, summaryID uuid.UUID) (mo.Option[*summary.SummaryEmbedding], error) {
	sqlcEmbedding, err := r.q.GetSummaryEmbedding(ctx, UUIDToPgtype(summaryID))
	if err != nil {
		if err == pgx.ErrNoRows || err == sql.ErrNoRows {
			return mo.None[*summary.SummaryEmbedding](), nil
		}
		return mo.None[*summary.SummaryEmbedding](), fmt.Errorf("failed to get summary embedding: %w", err)
	}

	return mo.Some(&summary.SummaryEmbedding{
		SummaryID: PgtypeToUUID(sqlcEmbedding.SummaryID),
		Vector:    sqlcEmbedding.Vector.Slice(),
		Model:     sqlcEmbedding.Model,
		CreatedAt: PgtypeToTime(sqlcEmbedding.CreatedAt),
	}), nil
}

// === 検索 ===

// SummarySearchByProductParams はプロダクト横断検索のパラメータ
type SummarySearchByProductParams struct {
	ProductID    uuid.UUID
	QueryVector  []float32
	SummaryTypes []string
	PathPrefix   *string
	Limit        int
}

// SummarySearchResult は要約検索の結果
type SummarySearchResult struct {
	ID          uuid.UUID
	SnapshotID  uuid.UUID
	SummaryType string
	TargetPath  string
	ArchType    *summary.ArchType
	Content     string
	Score       float64
}

// SearchSummariesByProduct はプロダクト横断で要約を検索する
func (r *SummaryRepository) SearchSummariesByProduct(ctx context.Context, params SummarySearchByProductParams) ([]*SummarySearchResult, error) {
	// pgvectorのベクトル型を作成
	queryVector := pgvector.NewVector(params.QueryVector)

	// PathPrefixをpgtype.Textに変換
	var pathPrefix pgtype.Text
	if params.PathPrefix != nil {
		pathPrefix = pgtype.Text{String: *params.PathPrefix, Valid: true}
	}

	// sqlc生成コードを呼び出し
	rows, err := r.q.SearchSummariesByProduct(ctx, sqlc.SearchSummariesByProductParams{
		QueryVector:  queryVector,
		ProductID:    UUIDToPgtype(params.ProductID),
		SummaryTypes: params.SummaryTypes,
		PathPrefix:   pathPrefix,
		LimitVal:     int32(params.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search summaries by product: %w", err)
	}

	// 結果を変換
	results := make([]*SummarySearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &SummarySearchResult{
			ID:          PgtypeToUUID(row.ID),
			SnapshotID:  PgtypeToUUID(row.SnapshotID),
			SummaryType: row.SummaryType,
			TargetPath:  row.TargetPath,
			ArchType:    pgtextToArchType(row.ArchType),
			Content:     row.Content,
			Score:       row.Score,
		})
	}

	return results, nil
}

// === Helper functions ===

// convertSQLCSummary は sqlc.Summary を summary.Summary に変換する
func convertSQLCSummary(s sqlc.Summary) (*summary.Summary, error) {
	var metadata map[string]any
	if len(s.Metadata) > 0 {
		if err := json.Unmarshal(s.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &summary.Summary{
		ID:          PgtypeToUUID(s.ID),
		SnapshotID:  PgtypeToUUID(s.SnapshotID),
		SummaryType: summary.SummaryType(s.SummaryType),
		TargetPath:  s.TargetPath,
		Depth:       PgtypeToIntPtr(s.Depth),
		ParentPath:  PgtextToStringPtr(s.ParentPath),
		ArchType:    pgtextToArchType(s.ArchType),
		Content:     s.Content,
		ContentHash: s.ContentHash,
		SourceHash:  s.SourceHash,
		Metadata:    metadata,
		CreatedAt:   PgtypeToTime(s.CreatedAt),
		UpdatedAt:   PgtypeToTime(s.UpdatedAt),
	}, nil
}

// archTypeToPgtext は *summary.ArchType を pgtype.Text に変換する
func archTypeToPgtext(archType *summary.ArchType) pgtype.Text {
	if archType == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: string(*archType), Valid: true}
}

// pgtextToArchType は pgtype.Text を *summary.ArchType に変換する
func pgtextToArchType(t pgtype.Text) *summary.ArchType {
	if !t.Valid {
		return nil
	}
	archType := summary.ArchType(t.String)
	return &archType
}
