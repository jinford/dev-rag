package models

import (
	"time"

	"github.com/google/uuid"
)

// ActionPriority はアクションの優先度を表します
type ActionPriority string

const (
	ActionPriorityP1 ActionPriority = "P1"
	ActionPriorityP2 ActionPriority = "P2"
	ActionPriorityP3 ActionPriority = "P3"
)

// ActionType はアクションの種類を表します
type ActionType string

const (
	ActionTypeReindex      ActionType = "reindex"
	ActionTypeDocFix       ActionType = "doc_fix"
	ActionTypeTestUpdate   ActionType = "test_update"
	ActionTypeInvestigate  ActionType = "investigate"
)

// ActionStatus はアクションのステータスを表します
type ActionStatus string

const (
	ActionStatusOpen      ActionStatus = "open"
	ActionStatusNoop      ActionStatus = "noop"
	ActionStatusCompleted ActionStatus = "completed"
)

// Action は品質フィードバックから生成された改善アクションを表します
// Phase 4タスク4: アクション自動生成プロンプトの実装
type Action struct {
	ID                 uuid.UUID      `json:"id"`
	ActionID           string         `json:"actionID" validate:"required,max=100"`     // ビジネス識別子（例: ACT-2024-001）
	PromptVersion      string         `json:"promptVersion" validate:"required"`         // プロンプトバージョン
	Priority           ActionPriority `json:"priority" validate:"required,oneof=P1 P2 P3"`
	ActionType         ActionType     `json:"actionType" validate:"required,oneof=reindex doc_fix test_update investigate"`
	Title              string         `json:"title" validate:"required"`                 // アクションタイトル
	Description        string         `json:"description" validate:"required"`           // 詳細説明
	LinkedFiles        []string       `json:"linkedFiles,omitempty"`                     // 関連ファイルリスト
	OwnerHint          string         `json:"ownerHint" validate:"required"`             // 担当者ヒント
	AcceptanceCriteria string         `json:"acceptanceCriteria" validate:"required"`    // 受入基準
	Status             ActionStatus   `json:"status" validate:"required,oneof=open noop completed"`
	CreatedAt          time.Time      `json:"createdAt"`
	CompletedAt        *time.Time     `json:"completedAt,omitempty"`
}

// IsOpen はアクションがオープン状態かどうかを返します
func (a *Action) IsOpen() bool {
	return a.Status == ActionStatusOpen
}

// IsCompleted はアクションが完了しているかどうかを返します
func (a *Action) IsCompleted() bool {
	return a.Status == ActionStatusCompleted
}

// IsNoop はアクションが不要かどうかを返します
func (a *Action) IsNoop() bool {
	return a.Status == ActionStatusNoop
}

// IsHighPriority はアクションが高優先度（P1）かどうかを返します
func (a *Action) IsHighPriority() bool {
	return a.Priority == ActionPriorityP1
}

// IsPending はアクションが未完了かどうかを返します
func (a *Action) IsPending() bool {
	return a.Status == ActionStatusOpen
}

// IsP1 はアクションがP1優先度かどうかを返します
func (a *Action) IsP1() bool {
	return a.Priority == ActionPriorityP1
}

// ActionFilter はアクションのフィルタ条件を表します
type ActionFilter struct {
	Priority  *ActionPriority `json:"priority,omitempty"`
	ActionType *ActionType     `json:"actionType,omitempty"`
	Status    *ActionStatus   `json:"status,omitempty"`
	StartDate *time.Time      `json:"startDate,omitempty"`
	EndDate   *time.Time      `json:"endDate,omitempty"`
	Limit     *int            `json:"limit,omitempty"`
}
