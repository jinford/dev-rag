package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/repository"
	"github.com/jinford/dev-rag/pkg/sqlc"
	"github.com/manifoldco/promptui"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v3"
)

// FeedbackCreateAction は新しいフィードバックを記録するコマンドのアクション
func FeedbackCreateAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")
	interactive := cmd.Bool("interactive")

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	repo := repository.NewQualityRepositoryRW(sqlc.New(appCtx.Database.Pool))

	var note *models.QualityNote

	if interactive {
		// インタラクティブモード
		note, err = promptQualityNote()
		if err != nil {
			return fmt.Errorf("入力エラー: %w", err)
		}
	} else {
		// フラグベースモード
		note, err = createNoteFromFlags(cmd)
		if err != nil {
			return fmt.Errorf("フラグパースエラー: %w", err)
		}
	}

	// バリデーション
	if err := validateQualityNote(note); err != nil {
		return fmt.Errorf("バリデーションエラー: %w", err)
	}

	// データベースに保存
	createdNote, err := repo.CreateQualityNote(ctx, note)
	if err != nil {
		return fmt.Errorf("品質ノートの作成に失敗: %w", err)
	}

	// 成功メッセージ
	fmt.Printf("\n✓ 品質フィードバックを記録しました\n")
	fmt.Printf("  ID: %s\n", createdNote.ID)
	fmt.Printf("  Note ID: %s\n", createdNote.NoteID)
	fmt.Printf("  Severity: %s\n", createdNote.Severity)
	fmt.Printf("  Reviewer: %s\n", createdNote.Reviewer)

	slog.Info("品質フィードバックを記録", "id", createdNote.ID, "noteID", createdNote.NoteID)

	return nil
}

// FeedbackListAction はフィードバックリストを表示するコマンドのアクション
func FeedbackListAction(ctx context.Context, cmd *cli.Command) error {
	envFile := cmd.String("env")
	severityStr := cmd.String("severity")
	statusStr := cmd.String("status")
	days := cmd.Int("days")
	reviewer := cmd.String("reviewer")

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	repo := repository.NewQualityRepositoryR(sqlc.New(appCtx.Database.Pool))

	// フィルタの構築
	filter := &models.QualityNoteFilter{}

	if severityStr != "" {
		severity := models.QualitySeverity(severityStr)
		filter.Severity = &severity
	}

	if statusStr != "" {
		status := models.QualityStatus(statusStr)
		filter.Status = &status
	}

	if days > 0 {
		startDate := time.Now().AddDate(0, 0, -days)
		filter.StartDate = &startDate
	}

	// フィードバックリストを取得
	notes, err := repo.ListQualityNotes(ctx, filter)
	if err != nil {
		return fmt.Errorf("品質ノートの取得に失敗: %w", err)
	}

	// レビュー者でフィルタ (RepositoryではサポートされていないのでGoコードでフィルタ)
	if reviewer != "" {
		filteredNotes := make([]*models.QualityNote, 0)
		for _, note := range notes {
			if note.Reviewer == reviewer {
				filteredNotes = append(filteredNotes, note)
			}
		}
		notes = filteredNotes
	}

	if len(notes) == 0 {
		fmt.Println("フィードバックはありません")
		return nil
	}

	// テーブル表示
	renderQualityNotesTable(notes)

	return nil
}

// FeedbackShowAction は特定のフィードバックを詳細表示するコマンドのアクション
func FeedbackShowAction(ctx context.Context, cmd *cli.Command) error {
	noteID := cmd.String("note-id")
	envFile := cmd.String("env")

	if noteID == "" {
		return fmt.Errorf("--note-id は必須です")
	}

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	repo := repository.NewQualityRepositoryR(sqlc.New(appCtx.Database.Pool))

	// フィードバックを取得
	note, err := repo.GetQualityNoteByNoteID(ctx, noteID)
	if err != nil {
		return fmt.Errorf("品質ノートの取得に失敗: %w", err)
	}

	// 詳細表示
	renderQualityNoteDetail(note)

	return nil
}

// FeedbackResolveAction はフィードバックを解決済みにするコマンドのアクション
func FeedbackResolveAction(ctx context.Context, cmd *cli.Command) error {
	noteID := cmd.String("note-id")
	envFile := cmd.String("env")

	if noteID == "" {
		return fmt.Errorf("--note-id は必須です")
	}

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	repo := repository.NewQualityRepositoryRW(sqlc.New(appCtx.Database.Pool))

	// フィードバックを取得
	note, err := repo.GetQualityNoteByNoteID(ctx, noteID)
	if err != nil {
		return fmt.Errorf("品質ノートの取得に失敗: %w", err)
	}

	// すでに解決済みの場合
	if note.IsResolved() {
		fmt.Printf("品質ノート %s はすでに解決済みです\n", noteID)
		return nil
	}

	// ステータスを更新
	updatedNote, err := repo.UpdateQualityNoteStatus(ctx, note.ID, models.QualityStatusResolved)
	if err != nil {
		return fmt.Errorf("品質ノートのステータス更新に失敗: %w", err)
	}

	// 成功メッセージ
	fmt.Printf("\n✓ 品質ノートを解決済みにしました\n")
	fmt.Printf("  Note ID: %s\n", updatedNote.NoteID)
	fmt.Printf("  Resolved At: %s\n", updatedNote.ResolvedAt.Format(time.RFC3339))

	slog.Info("品質ノートを解決済みに設定", "noteID", noteID)

	return nil
}

// FeedbackImportAction はJSON形式から一括インポートするコマンドのアクション
func FeedbackImportAction(ctx context.Context, cmd *cli.Command) error {
	file := cmd.String("file")
	envFile := cmd.String("env")

	if file == "" {
		return fmt.Errorf("--file は必須です")
	}

	// 共通コンテキストの初期化
	appCtx, err := NewAppContext(ctx, envFile)
	if err != nil {
		return err
	}
	defer appCtx.Close()

	repo := repository.NewQualityRepositoryRW(sqlc.New(appCtx.Database.Pool))

	// JSONファイルを読み込み
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("ファイルの読み込みに失敗: %w", err)
	}

	// JSONパース
	type ImportNote struct {
		NoteID       string   `json:"note_id"`
		Severity     string   `json:"severity"`
		NoteText     string   `json:"note_text"`
		Reviewer     string   `json:"reviewer"`
		LinkedFiles  []string `json:"linked_files"`
		LinkedChunks []string `json:"linked_chunks"`
	}

	var importNotes []ImportNote
	if err := json.Unmarshal(data, &importNotes); err != nil {
		return fmt.Errorf("JSONのパースに失敗: %w", err)
	}

	// インポート処理
	successCount := 0
	errorCount := 0

	for i, in := range importNotes {
		note := &models.QualityNote{
			ID:           uuid.New(),
			NoteID:       in.NoteID,
			Severity:     models.QualitySeverity(in.Severity),
			NoteText:     in.NoteText,
			Reviewer:     in.Reviewer,
			LinkedFiles:  in.LinkedFiles,
			LinkedChunks: in.LinkedChunks,
			Status:       models.QualityStatusOpen,
			CreatedAt:    time.Now(),
		}

		if err := validateQualityNote(note); err != nil {
			fmt.Printf("⚠ 行 %d をスキップ: %v\n", i+1, err)
			errorCount++
			continue
		}

		if _, err := repo.CreateQualityNote(ctx, note); err != nil {
			fmt.Printf("⚠ 行 %d の保存に失敗: %v\n", i+1, err)
			errorCount++
			continue
		}

		successCount++
	}

	// サマリー表示
	fmt.Printf("\n✓ インポート完了\n")
	fmt.Printf("  成功: %d件\n", successCount)
	if errorCount > 0 {
		fmt.Printf("  エラー: %d件\n", errorCount)
	}

	slog.Info("品質ノートをインポート", "success", successCount, "error", errorCount)

	return nil
}

// === ヘルパー関数 ===

// promptQualityNote はインタラクティブに品質ノートの入力を受け付けます
func promptQualityNote() (*models.QualityNote, error) {
	note := &models.QualityNote{
		ID:        uuid.New(),
		Status:    models.QualityStatusOpen,
		CreatedAt: time.Now(),
	}

	// Note ID
	promptNoteID := promptui.Prompt{
		Label: "Note ID (例: QN-2025-001)",
	}
	noteID, err := promptNoteID.Run()
	if err != nil {
		return nil, err
	}
	note.NoteID = noteID

	// Severity
	promptSeverity := promptui.Select{
		Label: "Severity",
		Items: []string{"critical", "high", "medium", "low"},
	}
	_, severity, err := promptSeverity.Run()
	if err != nil {
		return nil, err
	}
	note.Severity = models.QualitySeverity(severity)

	// Note Text
	promptNoteText := promptui.Prompt{
		Label: "問題の詳細",
	}
	noteText, err := promptNoteText.Run()
	if err != nil {
		return nil, err
	}
	note.NoteText = noteText

	// Reviewer
	promptReviewer := promptui.Prompt{
		Label: "レビュー者名",
	}
	reviewer, err := promptReviewer.Run()
	if err != nil {
		return nil, err
	}
	note.Reviewer = reviewer

	// Linked Files (オプション)
	promptLinkedFiles := promptui.Prompt{
		Label:   "関連ファイル (カンマ区切り、オプション)",
		Default: "",
	}
	linkedFilesStr, err := promptLinkedFiles.Run()
	if err != nil {
		return nil, err
	}
	if linkedFilesStr != "" {
		note.LinkedFiles = splitAndTrim(linkedFilesStr)
	}

	// Linked Chunks (オプション)
	promptLinkedChunks := promptui.Prompt{
		Label:   "関連チャンクID (カンマ区切り、オプション)",
		Default: "",
	}
	linkedChunksStr, err := promptLinkedChunks.Run()
	if err != nil {
		return nil, err
	}
	if linkedChunksStr != "" {
		note.LinkedChunks = splitAndTrim(linkedChunksStr)
	}

	return note, nil
}

// createNoteFromFlags はフラグから品質ノートを作成します
func createNoteFromFlags(cmd *cli.Command) (*models.QualityNote, error) {
	noteID := cmd.String("note-id")
	severity := cmd.String("severity")
	text := cmd.String("text")
	reviewer := cmd.String("reviewer")
	linkedFilesStr := cmd.String("linked-files")
	linkedChunksStr := cmd.String("linked-chunks")

	if noteID == "" || severity == "" || text == "" || reviewer == "" {
		return nil, fmt.Errorf("--note-id, --severity, --text, --reviewer は必須です")
	}

	note := &models.QualityNote{
		ID:        uuid.New(),
		NoteID:    noteID,
		Severity:  models.QualitySeverity(severity),
		NoteText:  text,
		Reviewer:  reviewer,
		Status:    models.QualityStatusOpen,
		CreatedAt: time.Now(),
	}

	if linkedFilesStr != "" {
		note.LinkedFiles = splitAndTrim(linkedFilesStr)
	}

	if linkedChunksStr != "" {
		note.LinkedChunks = splitAndTrim(linkedChunksStr)
	}

	return note, nil
}

// validateQualityNote は品質ノートをバリデーションします
func validateQualityNote(note *models.QualityNote) error {
	if note.NoteID == "" {
		return fmt.Errorf("note_id は必須です")
	}

	validSeverities := map[models.QualitySeverity]bool{
		models.QualitySeverityCritical: true,
		models.QualitySeverityHigh:     true,
		models.QualitySeverityMedium:   true,
		models.QualitySeverityLow:      true,
	}

	if !validSeverities[note.Severity] {
		return fmt.Errorf("無効な severity: %s", note.Severity)
	}

	if note.NoteText == "" {
		return fmt.Errorf("note_text は必須です")
	}

	if note.Reviewer == "" {
		return fmt.Errorf("reviewer は必須です")
	}

	return nil
}

// renderQualityNotesTable はテーブル形式で品質ノートリストを表示します
func renderQualityNotesTable(notes []*models.QualityNote) {
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("Note ID", "Severity", "Status", "Reviewer", "Created At")

	for _, note := range notes {
		table.Append(
			note.NoteID,
			string(note.Severity),
			string(note.Status),
			note.Reviewer,
			note.CreatedAt.Format("2006-01-02 15:04"),
		)
	}

	table.Render()
}

// renderQualityNoteDetail は品質ノートの詳細を表示します
func renderQualityNoteDetail(note *models.QualityNote) {
	fmt.Printf("\n=== 品質フィードバック詳細 ===\n\n")
	fmt.Printf("ID:            %s\n", note.ID)
	fmt.Printf("Note ID:       %s\n", note.NoteID)
	fmt.Printf("Severity:      %s\n", note.Severity)
	fmt.Printf("Status:        %s\n", note.Status)
	fmt.Printf("Reviewer:      %s\n", note.Reviewer)
	fmt.Printf("Created At:    %s\n", note.CreatedAt.Format(time.RFC3339))

	if note.ResolvedAt != nil {
		fmt.Printf("Resolved At:   %s\n", note.ResolvedAt.Format(time.RFC3339))
	}

	fmt.Printf("\n問題の詳細:\n%s\n", note.NoteText)

	if len(note.LinkedFiles) > 0 {
		fmt.Printf("\n関連ファイル:\n")
		for _, file := range note.LinkedFiles {
			fmt.Printf("  - %s\n", file)
		}
	}

	if len(note.LinkedChunks) > 0 {
		fmt.Printf("\n関連チャンクID:\n")
		for _, chunk := range note.LinkedChunks {
			fmt.Printf("  - %s\n", chunk)
		}
	}

	fmt.Println()
}

// splitAndTrim は文字列をカンマで分割してトリムします
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
