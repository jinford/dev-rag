package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/indexer/llm"
	"github.com/jinford/dev-rag/pkg/models"
)

const (
	// DefaultTemperature はアクション生成時のデフォルト温度パラメータ
	DefaultTemperature = 0.5

	// DefaultModel はアクション生成時のデフォルトモデル
	DefaultModel = "gpt-4o-mini"

	// MaxTokens はレスポンスの最大トークン数
	MaxTokens = 2000
)

// ActionGenerator はアクション生成サービス
type ActionGenerator struct {
	llmClient llm.LLMClient
	prompt    *ActionGenerationPrompt
}

// NewActionGenerator は新しいActionGeneratorを作成します
func NewActionGenerator(llmClient llm.LLMClient) *ActionGenerator {
	return &ActionGenerator{
		llmClient: llmClient,
		prompt:    NewActionGenerationPrompt(),
	}
}

// GenerateActions は週次レビューデータからアクションを生成します
func (g *ActionGenerator) GenerateActions(ctx context.Context, data ActionGenerationData) ([]models.Action, error) {
	// プロンプト生成
	promptText, err := g.prompt.GeneratePrompt(data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate prompt: %w", err)
	}

	// LLMリクエスト作成
	req := llm.CompletionRequest{
		Prompt:         promptText,
		Temperature:    DefaultTemperature,
		MaxTokens:      MaxTokens,
		ResponseFormat: "json",
		Model:          DefaultModel,
	}

	// LLM実行
	resp, err := g.llmClient.GenerateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate completion: %w", err)
	}

	// レスポンスパース
	actions, err := ParseActionResponse(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse action response: %w", err)
	}

	// メタデータの追加
	now := time.Now()
	for i := range actions {
		actions[i].ID = uuid.New()
		actions[i].CreatedAt = now

		// ActionIDが空の場合は自動生成
		if actions[i].ActionID == "" {
			actions[i].ActionID = fmt.Sprintf("ACT-%s-%03d", now.Format("2006-01"), i+1)
		}
	}

	return actions, nil
}

// actionResponseItem はLLMレスポンスの各アイテムを表します
type actionResponseItem struct {
	PromptVersion      string   `json:"prompt_version"`
	Priority           string   `json:"priority"`
	ActionType         string   `json:"action_type"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	LinkedFiles        []string `json:"linked_files"`
	OwnerHint          string   `json:"owner_hint"`
	AcceptanceCriteria string   `json:"acceptance_criteria"`
	Status             string   `json:"status"`
}

// ParseActionResponse はJSON文字列をパースしてActionリストを生成します
func ParseActionResponse(jsonStr string) ([]models.Action, error) {
	// JSON配列のパース
	var items []actionResponseItem
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Actionリストの生成
	actions := make([]models.Action, 0, len(items))
	for i, item := range items {
		// バリデーション: action_type
		actionType := models.ActionType(item.ActionType)
		if !isValidActionType(actionType) {
			return nil, fmt.Errorf("invalid action_type at index %d: %s", i, item.ActionType)
		}

		// バリデーション: priority
		priority := models.ActionPriority(item.Priority)
		if !isValidPriority(priority) {
			return nil, fmt.Errorf("invalid priority at index %d: %s", i, item.Priority)
		}

		// バリデーション: status
		status := models.ActionStatus(item.Status)
		if !isValidStatus(status) {
			return nil, fmt.Errorf("invalid status at index %d: %s", i, item.Status)
		}

		// Actionの生成
		action := models.Action{
			PromptVersion:      item.PromptVersion,
			Priority:           priority,
			ActionType:         actionType,
			Title:              item.Title,
			Description:        item.Description,
			LinkedFiles:        item.LinkedFiles,
			OwnerHint:          item.OwnerHint,
			AcceptanceCriteria: item.AcceptanceCriteria,
			Status:             status,
		}

		actions = append(actions, action)
	}

	return actions, nil
}

// isValidActionType はアクションタイプが有効かどうかを確認します
func isValidActionType(actionType models.ActionType) bool {
	switch actionType {
	case models.ActionTypeReindex,
		models.ActionTypeDocFix,
		models.ActionTypeTestUpdate,
		models.ActionTypeInvestigate:
		return true
	default:
		return false
	}
}

// isValidPriority は優先度が有効かどうかを確認します
func isValidPriority(priority models.ActionPriority) bool {
	switch priority {
	case models.ActionPriorityP1,
		models.ActionPriorityP2,
		models.ActionPriorityP3:
		return true
	default:
		return false
	}
}

// isValidStatus はステータスが有効かどうかを確認します
func isValidStatus(status models.ActionStatus) bool {
	switch status {
	case models.ActionStatusOpen,
		models.ActionStatusNoop,
		models.ActionStatusCompleted:
		return true
	default:
		return false
	}
}
