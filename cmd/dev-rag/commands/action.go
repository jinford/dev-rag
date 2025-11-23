package commands

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/repository"
	"github.com/jinford/dev-rag/pkg/sqlc"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v3"
)

// ActionListAction はアクションリストを表示するコマンドのアクション
func ActionListAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")
	priorityStr := cmd.String("priority")
	actionTypeStr := cmd.String("type")
	statusStr := cmd.String("status")

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	repo := repository.NewActionRepositoryR(sqlc.New(appCtx.Database.Pool))

	// フィルタの構築
	filter := &models.ActionFilter{}

	if priorityStr != "" {
		priority := models.ActionPriority(priorityStr)
		filter.Priority = &priority
	}

	if actionTypeStr != "" {
		actionType := models.ActionType(actionTypeStr)
		filter.ActionType = &actionType
	}

	if statusStr != "" {
		status := models.ActionStatus(statusStr)
		filter.Status = &status
	}

	// アクションリストを取得
	actions, err := repo.ListActions(ctx, filter)
	if err != nil {
		return fmt.Errorf("アクションの取得に失敗: %w", err)
	}

	if len(actions) == 0 {
		fmt.Println("アクションはありません")
		return nil
	}

	// テーブル表示
	renderActionsTable(actions)

	return nil
}

// ActionShowAction は特定のアクションを詳細表示するコマンドのアクション
func ActionShowAction(ctx context.Context, cmd *cli.Command) error {
	actionID := cmd.String("action-id")
	envFile := cmd.String("env")

	if actionID == "" {
		return fmt.Errorf("--action-id は必須です")
	}

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	repo := repository.NewActionRepositoryR(sqlc.New(appCtx.Database.Pool))

	// アクションを取得
	action, err := repo.GetActionByActionID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("アクションの取得に失敗: %w", err)
	}

	// 詳細表示
	renderActionDetail(action)

	return nil
}

// ActionCompleteAction はアクションを完了にするコマンドのアクション
func ActionCompleteAction(ctx context.Context, cmd *cli.Command) error {
	actionID := cmd.String("action-id")
	envFile := cmd.String("env")

	if actionID == "" {
		return fmt.Errorf("--action-id は必須です")
	}

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	repo := repository.NewActionRepositoryRW(sqlc.New(appCtx.Database.Pool))

	// アクションを取得
	action, err := repo.GetActionByActionID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("アクションの取得に失敗: %w", err)
	}

	// すでに完了済みの場合
	if action.IsCompleted() {
		fmt.Printf("アクション %s はすでに完了済みです\n", actionID)
		return nil
	}

	// ステータスを更新
	updatedAction, err := repo.UpdateActionStatus(ctx, action.ID, models.ActionStatusCompleted)
	if err != nil {
		return fmt.Errorf("アクションのステータス更新に失敗: %w", err)
	}

	// 成功メッセージ
	fmt.Printf("\n✓ アクションを完了にしました\n")
	fmt.Printf("  Action ID: %s\n", updatedAction.ActionID)
	fmt.Printf("  Completed At: %s\n", updatedAction.CompletedAt.Format(time.RFC3339))

	slog.Info("アクションを完了に設定", "actionID", actionID)

	return nil
}

// ActionExportAction はバックログをエクスポートするコマンドのアクション
func ActionExportAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")
	formatStr := cmd.String("format")
	outputFile := cmd.String("output")

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	repo := repository.NewActionRepositoryR(sqlc.New(appCtx.Database.Pool))

	// 全アクションを取得
	actions, err := repo.ListActions(ctx, nil)
	if err != nil {
		return fmt.Errorf("アクションの取得に失敗: %w", err)
	}

	if len(actions) == 0 {
		fmt.Println("エクスポートするアクションがありません")
		return nil
	}

	// フォーマットに応じてエクスポート
	switch formatStr {
	case "json":
		if err := exportActionsJSON(actions, outputFile); err != nil {
			return fmt.Errorf("JSONエクスポートに失敗: %w", err)
		}
	case "csv":
		if err := exportActionsCSV(actions, outputFile); err != nil {
			return fmt.Errorf("CSVエクスポートに失敗: %w", err)
		}
	default:
		return fmt.Errorf("サポートされていないフォーマット: %s（json または csv を指定してください）", formatStr)
	}

	fmt.Printf("✓ %d件のアクションを %s にエクスポートしました\n", len(actions), outputFile)
	slog.Info("アクションをエクスポート", "count", len(actions), "format", formatStr, "output", outputFile)

	return nil
}

// === ヘルパー関数 ===

// renderActionsTable はテーブル形式でアクションリストを表示します
func renderActionsTable(actions []*models.Action) {
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("Action ID", "Priority", "Type", "Title", "Status", "Created At")

	for _, action := range actions {
		table.Append(
			action.ActionID,
			string(action.Priority),
			string(action.ActionType),
			truncateString(action.Title, 50),
			string(action.Status),
			action.CreatedAt.Format("2006-01-02 15:04"),
		)
	}

	table.Render()
}

// renderActionDetail はアクションの詳細を表示します
func renderActionDetail(action *models.Action) {
	fmt.Printf("\n=== アクション詳細 ===\n\n")
	fmt.Printf("ID:                  %s\n", action.ID)
	fmt.Printf("Action ID:           %s\n", action.ActionID)
	fmt.Printf("Prompt Version:      %s\n", action.PromptVersion)
	fmt.Printf("Priority:            %s\n", action.Priority)
	fmt.Printf("Action Type:         %s\n", action.ActionType)
	fmt.Printf("Title:               %s\n", action.Title)
	fmt.Printf("Status:              %s\n", action.Status)
	fmt.Printf("Created At:          %s\n", action.CreatedAt.Format(time.RFC3339))

	if action.CompletedAt != nil {
		fmt.Printf("Completed At:        %s\n", action.CompletedAt.Format(time.RFC3339))
	}

	if action.OwnerHint != "" {
		fmt.Printf("Owner Hint:          %s\n", action.OwnerHint)
	}

	fmt.Printf("\n説明:\n%s\n", action.Description)

	fmt.Printf("\n受け入れ基準:\n%s\n", action.AcceptanceCriteria)

	if len(action.LinkedFiles) > 0 {
		fmt.Printf("\n関連ファイル:\n")
		for _, file := range action.LinkedFiles {
			fmt.Printf("  - %s\n", file)
		}
	}

	fmt.Println()
}

// exportActionsJSON はアクションをJSON形式でエクスポートします
func exportActionsJSON(actions []*models.Action, outputFile string) error {
	data, err := json.MarshalIndent(actions, "", "  ")
	if err != nil {
		return fmt.Errorf("JSONシリアライズに失敗: %w", err)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("ファイル書き込みに失敗: %w", err)
	}

	return nil
}

// exportActionsCSV はアクションをCSV形式でエクスポートします
func exportActionsCSV(actions []*models.Action, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("ファイル作成に失敗: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// ヘッダー
	header := []string{
		"ID",
		"Action ID",
		"Prompt Version",
		"Priority",
		"Action Type",
		"Title",
		"Description",
		"Linked Files",
		"Owner Hint",
		"Acceptance Criteria",
		"Status",
		"Created At",
		"Completed At",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("ヘッダー書き込みに失敗: %w", err)
	}

	// データ行
	for _, action := range actions {
		linkedFilesJSON, _ := json.Marshal(action.LinkedFiles)

		completedAt := ""
		if action.CompletedAt != nil {
			completedAt = action.CompletedAt.Format(time.RFC3339)
		}

		row := []string{
			action.ID.String(),
			action.ActionID,
			action.PromptVersion,
			string(action.Priority),
			string(action.ActionType),
			action.Title,
			action.Description,
			string(linkedFilesJSON),
			action.OwnerHint,
			action.AcceptanceCriteria,
			string(action.Status),
			action.CreatedAt.Format(time.RFC3339),
			completedAt,
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("データ行書き込みに失敗: %w", err)
		}
	}

	return nil
}

// truncateString は文字列を指定された長さに切り詰めます
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ActionCreateAction は新しいアクションを手動で作成するコマンドのアクション
func ActionCreateAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")
	actionID := cmd.String("action-id")
	promptVersion := cmd.String("prompt-version")
	priority := cmd.String("priority")
	actionType := cmd.String("type")
	title := cmd.String("title")
	description := cmd.String("description")
	ownerHint := cmd.String("owner-hint")
	acceptanceCriteria := cmd.String("acceptance-criteria")

	// 必須パラメータの検証
	if actionID == "" || promptVersion == "" || priority == "" || actionType == "" ||
		title == "" || description == "" || acceptanceCriteria == "" {
		return fmt.Errorf("--action-id, --prompt-version, --priority, --type, --title, --description, --acceptance-criteria は必須です")
	}

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	repo := repository.NewActionRepositoryRW(sqlc.New(appCtx.Database.Pool))

	// アクションオブジェクトを作成
	action := &models.Action{
		ID:                 uuid.New(),
		ActionID:           actionID,
		PromptVersion:      promptVersion,
		Priority:           models.ActionPriority(priority),
		ActionType:         models.ActionType(actionType),
		Title:              title,
		Description:        description,
		OwnerHint:          ownerHint,
		AcceptanceCriteria: acceptanceCriteria,
		Status:             models.ActionStatusOpen,
		CreatedAt:          time.Now(),
	}

	// データベースに保存
	createdAction, err := repo.CreateAction(ctx, action)
	if err != nil {
		return fmt.Errorf("アクションの作成に失敗: %w", err)
	}

	// 成功メッセージ
	fmt.Printf("\n✓ アクションを作成しました\n")
	fmt.Printf("  ID: %s\n", createdAction.ID)
	fmt.Printf("  Action ID: %s\n", createdAction.ActionID)
	fmt.Printf("  Priority: %s\n", createdAction.Priority)
	fmt.Printf("  Type: %s\n", createdAction.ActionType)

	slog.Info("アクションを作成", "id", createdAction.ID, "actionID", createdAction.ActionID)

	return nil
}
