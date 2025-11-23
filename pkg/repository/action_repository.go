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

// ActionRepositoryR はアクションに対する読み取り専用のデータベース操作を提供します
type ActionRepositoryR struct {
	q sqlc.Querier
}

// NewActionRepositoryR は新しい読み取り専用リポジトリを作成します
func NewActionRepositoryR(q sqlc.Querier) *ActionRepositoryR {
	return &ActionRepositoryR{q: q}
}

// ActionRepositoryRW は ActionRepositoryR を埋め込み、書き込み操作を提供します
type ActionRepositoryRW struct {
	*ActionRepositoryR
}

// NewActionRepositoryRW は読み書き可能なリポジトリを作成します
func NewActionRepositoryRW(q sqlc.Querier) *ActionRepositoryRW {
	return &ActionRepositoryRW{ActionRepositoryR: NewActionRepositoryR(q)}
}

// === Create操作 ===

// CreateAction は新しいアクションを作成します
func (rw *ActionRepositoryRW) CreateAction(ctx context.Context, action *models.Action) (*models.Action, error) {
	// JSONBフィールドの準備
	linkedFiles, err := json.Marshal(action.LinkedFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal linked_files: %w", err)
	}

	sqlcAction, err := rw.q.CreateAction(ctx, sqlc.CreateActionParams{
		ActionID:           action.ActionID,
		PromptVersion:      action.PromptVersion,
		Priority:           string(action.Priority),
		ActionType:         string(action.ActionType),
		Title:              action.Title,
		Description:        action.Description,
		LinkedFiles:        linkedFiles,
		OwnerHint:          StringToNullableText(action.OwnerHint),
		AcceptanceCriteria: action.AcceptanceCriteria,
		Status:             string(action.Status),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create action: %w", err)
	}

	return convertSQLCAction(sqlcAction)
}

// === Read操作 ===

// GetActionByID はIDでアクションを取得します
func (r *ActionRepositoryR) GetActionByID(ctx context.Context, id uuid.UUID) (*models.Action, error) {
	sqlcAction, err := r.q.GetAction(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("action not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get action: %w", err)
	}

	return convertSQLCAction(sqlcAction)
}

// GetActionByActionID はビジネスIDでアクションを取得します
func (r *ActionRepositoryR) GetActionByActionID(ctx context.Context, actionID string) (*models.Action, error) {
	sqlcAction, err := r.q.GetActionByActionID(ctx, actionID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("action not found: %s", actionID)
		}
		return nil, fmt.Errorf("failed to get action: %w", err)
	}

	return convertSQLCAction(sqlcAction)
}

// ListActions はフィルタ条件に基づいてアクションのリストを取得します
func (r *ActionRepositoryR) ListActions(ctx context.Context, filter *models.ActionFilter) ([]*models.Action, error) {
	var params sqlc.ListActionsParams

	// フィルタ条件を設定
	if filter != nil {
		if filter.Priority != nil {
			params.Priority = string(*filter.Priority)
		}
		if filter.ActionType != nil {
			params.ActionType = string(*filter.ActionType)
		}
		if filter.Status != nil {
			params.Status = string(*filter.Status)
		}
		if filter.Limit != nil {
			params.LimitCount = *filter.Limit
		}
	}

	rows, err := r.q.ListActions(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}

	actions := make([]*models.Action, 0, len(rows))
	for _, row := range rows {
		action, err := convertSQLCAction(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert action: %w", err)
		}
		actions = append(actions, action)
	}

	return actions, nil
}

// ListPendingActions は未完了のアクションのリストを取得します
func (r *ActionRepositoryR) ListPendingActions(ctx context.Context) ([]*models.Action, error) {
	rows, err := r.q.ListPendingActions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending actions: %w", err)
	}

	actions := make([]*models.Action, 0, len(rows))
	for _, row := range rows {
		action, err := convertSQLCAction(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert action: %w", err)
		}
		actions = append(actions, action)
	}

	return actions, nil
}

// ListActionsByPriority は優先度でフィルタしてアクションのリストを取得します
func (r *ActionRepositoryR) ListActionsByPriority(ctx context.Context, priority models.ActionPriority) ([]*models.Action, error) {
	rows, err := r.q.ListActionsByPriority(ctx, string(priority))
	if err != nil {
		return nil, fmt.Errorf("failed to list actions by priority: %w", err)
	}

	actions := make([]*models.Action, 0, len(rows))
	for _, row := range rows {
		action, err := convertSQLCAction(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert action: %w", err)
		}
		actions = append(actions, action)
	}

	return actions, nil
}

// ListActionsByType は種別でフィルタしてアクションのリストを取得します
func (r *ActionRepositoryR) ListActionsByType(ctx context.Context, actionType models.ActionType) ([]*models.Action, error) {
	rows, err := r.q.ListActionsByType(ctx, string(actionType))
	if err != nil {
		return nil, fmt.Errorf("failed to list actions by type: %w", err)
	}

	actions := make([]*models.Action, 0, len(rows))
	for _, row := range rows {
		action, err := convertSQLCAction(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert action: %w", err)
		}
		actions = append(actions, action)
	}

	return actions, nil
}

// ListActionsByStatus はステータスでフィルタしてアクションのリストを取得します
func (r *ActionRepositoryR) ListActionsByStatus(ctx context.Context, status models.ActionStatus) ([]*models.Action, error) {
	rows, err := r.q.ListActionsByStatus(ctx, string(status))
	if err != nil {
		return nil, fmt.Errorf("failed to list actions by status: %w", err)
	}

	actions := make([]*models.Action, 0, len(rows))
	for _, row := range rows {
		action, err := convertSQLCAction(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert action: %w", err)
		}
		actions = append(actions, action)
	}

	return actions, nil
}

// === Update操作 ===

// UpdateActionStatus はアクションのステータスを更新します
func (rw *ActionRepositoryRW) UpdateActionStatus(ctx context.Context, id uuid.UUID, status models.ActionStatus) (*models.Action, error) {
	var completedAt *time.Time
	if status == models.ActionStatusCompleted {
		now := time.Now()
		completedAt = &now
	}

	sqlcAction, err := rw.q.UpdateActionStatus(ctx, sqlc.UpdateActionStatusParams{
		ID:          UUIDToPgtype(id),
		Status:      string(status),
		CompletedAt: TimePtrToPgtimestamp(completedAt),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("action not found: %s", id)
		}
		return nil, fmt.Errorf("failed to update action status: %w", err)
	}

	return convertSQLCAction(sqlcAction)
}

// === Delete操作 ===

// DeleteAction はアクションを削除します
func (rw *ActionRepositoryRW) DeleteAction(ctx context.Context, id uuid.UUID) error {
	// 存在確認
	if _, err := rw.q.GetAction(ctx, UUIDToPgtype(id)); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("action not found: %s", id)
		}
		return fmt.Errorf("failed to get action: %w", err)
	}

	if err := rw.q.DeleteAction(ctx, UUIDToPgtype(id)); err != nil {
		return fmt.Errorf("failed to delete action: %w", err)
	}

	return nil
}

// === Private helpers ===

// convertSQLCAction は sqlc.ActionBacklog を models.Action に変換します
func convertSQLCAction(row sqlc.ActionBacklog) (*models.Action, error) {
	// JSONBフィールドのパース
	var linkedFiles []string
	if len(row.LinkedFiles) > 0 {
		if err := json.Unmarshal(row.LinkedFiles, &linkedFiles); err != nil {
			return nil, fmt.Errorf("failed to unmarshal linked_files: %w", err)
		}
	}

	ownerHint := ""
	if ownerHintPtr := PgtextToStringPtr(row.OwnerHint); ownerHintPtr != nil {
		ownerHint = *ownerHintPtr
	}

	return &models.Action{
		ID:                 PgtypeToUUID(row.ID),
		ActionID:           row.ActionID,
		PromptVersion:      row.PromptVersion,
		Priority:           models.ActionPriority(row.Priority),
		ActionType:         models.ActionType(row.ActionType),
		Title:              row.Title,
		Description:        row.Description,
		LinkedFiles:        linkedFiles,
		OwnerHint:          ownerHint,
		AcceptanceCriteria: row.AcceptanceCriteria,
		Status:             models.ActionStatus(row.Status),
		CreatedAt:          PgtypeToTime(row.CreatedAt),
		CompletedAt:        PgtypeToTimePtr(row.CompletedAt),
	}, nil
}
