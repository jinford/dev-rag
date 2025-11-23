package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/sqlc"
)

// QualityRepositoryR は品質ノートに対する読み取り専用のデータベース操作を提供します
type QualityRepositoryR struct {
	q sqlc.Querier
}

// NewQualityRepositoryR は新しい読み取り専用リポジトリを作成します
func NewQualityRepositoryR(q sqlc.Querier) *QualityRepositoryR {
	return &QualityRepositoryR{q: q}
}

// QualityRepositoryRW は QualityRepositoryR を埋め込み、書き込み操作を提供します
type QualityRepositoryRW struct {
	*QualityRepositoryR
}

// NewQualityRepositoryRW は読み書き可能なリポジトリを作成します
func NewQualityRepositoryRW(q sqlc.Querier) *QualityRepositoryRW {
	return &QualityRepositoryRW{QualityRepositoryR: NewQualityRepositoryR(q)}
}

// === Create操作 ===

// CreateQualityNote は新しい品質ノートを作成します
func (rw *QualityRepositoryRW) CreateQualityNote(ctx context.Context, note *models.QualityNote) (*models.QualityNote, error) {
	// JSONBフィールドの準備
	linkedFiles, err := json.Marshal(note.LinkedFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal linked_files: %w", err)
	}

	linkedChunks, err := json.Marshal(note.LinkedChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal linked_chunks: %w", err)
	}

	sqlcNote, err := rw.q.CreateQualityNote(ctx, sqlc.CreateQualityNoteParams{
		NoteID:       note.NoteID,
		Severity:     string(note.Severity),
		NoteText:     note.NoteText,
		LinkedFiles:  linkedFiles,
		LinkedChunks: linkedChunks,
		Reviewer:     note.Reviewer,
		Status:       string(note.Status),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create quality note: %w", err)
	}

	return convertSQLCQualityNote(sqlcNote)
}

// === Read操作 ===

// GetQualityNoteByID はIDで品質ノートを取得します
func (r *QualityRepositoryR) GetQualityNoteByID(ctx context.Context, id uuid.UUID) (*models.QualityNote, error) {
	sqlcNote, err := r.q.GetQualityNote(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("quality note not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get quality note: %w", err)
	}

	return convertSQLCQualityNote(sqlcNote)
}

// GetQualityNoteByNoteID はビジネスIDで品質ノートを取得します
func (r *QualityRepositoryR) GetQualityNoteByNoteID(ctx context.Context, noteID string) (*models.QualityNote, error) {
	sqlcNote, err := r.q.GetQualityNoteByNoteID(ctx, noteID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("quality note not found: %s", noteID)
		}
		return nil, fmt.Errorf("failed to get quality note: %w", err)
	}

	return convertSQLCQualityNote(sqlcNote)
}

// ListQualityNotes はフィルタ条件に基づいて品質ノートのリストを取得します
func (r *QualityRepositoryR) ListQualityNotes(ctx context.Context, filter *models.QualityNoteFilter) ([]*models.QualityNote, error) {
	var params sqlc.ListQualityNotesParams

	// フィルタ条件を設定
	if filter != nil {
		if filter.Severity != nil {
			params.Severity = string(*filter.Severity)
		}
		if filter.Status != nil {
			params.Status = string(*filter.Status)
		}
		if filter.StartDate != nil {
			params.StartDate = TimePtrToPgtimestamp(filter.StartDate)
		}
		if filter.EndDate != nil {
			params.EndDate = TimePtrToPgtimestamp(filter.EndDate)
		}
		if filter.Limit != nil {
			params.LimitCount = *filter.Limit
		}
	}

	rows, err := r.q.ListQualityNotes(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list quality notes: %w", err)
	}

	notes := make([]*models.QualityNote, 0, len(rows))
	for _, row := range rows {
		note, err := convertSQLCQualityNote(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert quality note: %w", err)
		}
		notes = append(notes, note)
	}

	return notes, nil
}

// ListQualityNotesBySeverity は深刻度でフィルタして品質ノートのリストを取得します
func (r *QualityRepositoryR) ListQualityNotesBySeverity(ctx context.Context, severity models.QualitySeverity) ([]*models.QualityNote, error) {
	rows, err := r.q.ListQualityNotesBySeverity(ctx, string(severity))
	if err != nil {
		return nil, fmt.Errorf("failed to list quality notes by severity: %w", err)
	}

	notes := make([]*models.QualityNote, 0, len(rows))
	for _, row := range rows {
		note, err := convertSQLCQualityNote(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert quality note: %w", err)
		}
		notes = append(notes, note)
	}

	return notes, nil
}

// ListQualityNotesByStatus はステータスでフィルタして品質ノートのリストを取得します
func (r *QualityRepositoryR) ListQualityNotesByStatus(ctx context.Context, status models.QualityStatus) ([]*models.QualityNote, error) {
	rows, err := r.q.ListQualityNotesByStatus(ctx, string(status))
	if err != nil {
		return nil, fmt.Errorf("failed to list quality notes by status: %w", err)
	}

	notes := make([]*models.QualityNote, 0, len(rows))
	for _, row := range rows {
		note, err := convertSQLCQualityNote(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert quality note: %w", err)
		}
		notes = append(notes, note)
	}

	return notes, nil
}

// ListQualityNotesByDateRange は期間でフィルタして品質ノートのリストを取得します
func (r *QualityRepositoryR) ListQualityNotesByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*models.QualityNote, error) {
	rows, err := r.q.ListQualityNotesByDateRange(ctx, sqlc.ListQualityNotesByDateRangeParams{
		CreatedAt:   TimeToPgtimestamp(startDate),
		CreatedAt_2: TimeToPgtimestamp(endDate),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list quality notes by date range: %w", err)
	}

	notes := make([]*models.QualityNote, 0, len(rows))
	for _, row := range rows {
		note, err := convertSQLCQualityNote(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert quality note: %w", err)
		}
		notes = append(notes, note)
	}

	return notes, nil
}

// GetRecentQualityNotes は過去7日間の品質ノートを取得します
func (r *QualityRepositoryR) GetRecentQualityNotes(ctx context.Context) ([]*models.QualityNote, error) {
	rows, err := r.q.GetRecentQualityNotes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent quality notes: %w", err)
	}

	notes := make([]*models.QualityNote, 0, len(rows))
	for _, row := range rows {
		note, err := convertSQLCQualityNote(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert quality note: %w", err)
		}
		notes = append(notes, note)
	}

	return notes, nil
}

// === Update操作 ===

// UpdateQualityNoteStatus は品質ノートのステータスを更新します
func (rw *QualityRepositoryRW) UpdateQualityNoteStatus(ctx context.Context, id uuid.UUID, status models.QualityStatus) (*models.QualityNote, error) {
	var resolvedAt *time.Time
	if status == models.QualityStatusResolved {
		now := time.Now()
		resolvedAt = &now
	}

	sqlcNote, err := rw.q.UpdateQualityNoteStatus(ctx, sqlc.UpdateQualityNoteStatusParams{
		ID:         UUIDToPgtype(id),
		Status:     string(status),
		ResolvedAt: TimePtrToPgtimestamp(resolvedAt),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("quality note not found: %s", id)
		}
		return nil, fmt.Errorf("failed to update quality note status: %w", err)
	}

	return convertSQLCQualityNote(sqlcNote)
}

// UpdateQualityNote は品質ノートの内容を更新します
func (rw *QualityRepositoryRW) UpdateQualityNote(ctx context.Context, id uuid.UUID, note *models.QualityNote) (*models.QualityNote, error) {
	// JSONBフィールドの準備
	linkedFiles, err := json.Marshal(note.LinkedFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal linked_files: %w", err)
	}

	linkedChunks, err := json.Marshal(note.LinkedChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal linked_chunks: %w", err)
	}

	sqlcNote, err := rw.q.UpdateQualityNote(ctx, sqlc.UpdateQualityNoteParams{
		ID:           UUIDToPgtype(id),
		NoteText:     note.NoteText,
		LinkedFiles:  linkedFiles,
		LinkedChunks: linkedChunks,
		Severity:     string(note.Severity),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("quality note not found: %s", id)
		}
		return nil, fmt.Errorf("failed to update quality note: %w", err)
	}

	return convertSQLCQualityNote(sqlcNote)
}

// === Delete操作 ===

// DeleteQualityNote は品質ノートを削除します
func (rw *QualityRepositoryRW) DeleteQualityNote(ctx context.Context, id uuid.UUID) error {
	// 存在確認
	if _, err := rw.q.GetQualityNote(ctx, UUIDToPgtype(id)); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("quality note not found: %s", id)
		}
		return fmt.Errorf("failed to get quality note: %w", err)
	}

	if err := rw.q.DeleteQualityNote(ctx, UUIDToPgtype(id)); err != nil {
		return fmt.Errorf("failed to delete quality note: %w", err)
	}

	return nil
}

// === Private helpers ===

// convertSQLCQualityNote は sqlc.QualityNote を models.QualityNote に変換します
func convertSQLCQualityNote(row sqlc.QualityNote) (*models.QualityNote, error) {
	// JSONBフィールドのパース
	var linkedFiles []string
	if len(row.LinkedFiles) > 0 {
		if err := json.Unmarshal(row.LinkedFiles, &linkedFiles); err != nil {
			return nil, fmt.Errorf("failed to unmarshal linked_files: %w", err)
		}
	}

	var linkedChunks []string
	if len(row.LinkedChunks) > 0 {
		if err := json.Unmarshal(row.LinkedChunks, &linkedChunks); err != nil {
			return nil, fmt.Errorf("failed to unmarshal linked_chunks: %w", err)
		}
	}

	return &models.QualityNote{
		ID:           PgtypeToUUID(row.ID),
		NoteID:       row.NoteID,
		Severity:     models.QualitySeverity(row.Severity),
		NoteText:     row.NoteText,
		LinkedFiles:  linkedFiles,
		LinkedChunks: linkedChunks,
		Reviewer:     row.Reviewer,
		Status:       models.QualityStatus(row.Status),
		CreatedAt:    PgtypeToTime(row.CreatedAt),
		ResolvedAt:   PgtypeToTimePtr(row.ResolvedAt),
	}, nil
}
