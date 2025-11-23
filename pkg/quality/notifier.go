package quality

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
)

// Notifier はアクション生成結果を通知するインターフェースです
type Notifier interface {
	Notify(actions []models.Action) error
}

// StandardOutputNotifier は標準出力に通知するNotifierです
type StandardOutputNotifier struct{}

// NewStandardOutputNotifier は新しいStandardOutputNotifierを作成します
func NewStandardOutputNotifier() *StandardOutputNotifier {
	return &StandardOutputNotifier{}
}

// Notify は標準出力にアクションを表示します
func (n *StandardOutputNotifier) Notify(actions []models.Action) error {
	fmt.Println("\n========================================")
	fmt.Println("週次レビュー結果")
	fmt.Printf("実行日時: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("========================================")

	if len(actions) == 0 {
		fmt.Println("生成されたアクションはありません。")
		return nil
	}

	fmt.Printf("生成されたアクション数: %d\n\n", len(actions))

	for i, action := range actions {
		fmt.Printf("[%d] %s\n", i+1, action.Title)
		fmt.Printf("    ID: %s\n", action.ActionID)
		fmt.Printf("    優先度: %s\n", action.Priority)
		fmt.Printf("    種別: %s\n", action.ActionType)
		fmt.Printf("    担当者: %s\n", action.OwnerHint)
		fmt.Printf("    ステータス: %s\n", action.Status)
		if action.Description != "" {
			desc := action.Description
			if len(desc) > 100 {
				desc = desc[:100] + "..."
			}
			fmt.Printf("    説明: %s\n", desc)
		}
		fmt.Println()
	}

	fmt.Println("========================================")
	return nil
}

// FileNotifier はファイルに通知するNotifierです
type FileNotifier struct {
	FilePath string
}

// NewFileNotifier は新しいFileNotifierを作成します
func NewFileNotifier(filePath string) *FileNotifier {
	return &FileNotifier{
		FilePath: filePath,
	}
}

// Notify はファイルにアクションを書き込みます
func (n *FileNotifier) Notify(actions []models.Action) error {
	var sb strings.Builder

	sb.WriteString("========================================\n")
	sb.WriteString("週次レビュー結果\n")
	sb.WriteString(fmt.Sprintf("実行日時: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("========================================\n\n")

	if len(actions) == 0 {
		sb.WriteString("生成されたアクションはありません。\n")
	} else {
		sb.WriteString(fmt.Sprintf("生成されたアクション数: %d\n\n", len(actions)))

		for i, action := range actions {
			sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, action.Title))
			sb.WriteString(fmt.Sprintf("    ID: %s\n", action.ActionID))
			sb.WriteString(fmt.Sprintf("    優先度: %s\n", action.Priority))
			sb.WriteString(fmt.Sprintf("    種別: %s\n", action.ActionType))
			sb.WriteString(fmt.Sprintf("    担当者: %s\n", action.OwnerHint))
			sb.WriteString(fmt.Sprintf("    ステータス: %s\n", action.Status))
			sb.WriteString(fmt.Sprintf("    説明: %s\n", action.Description))
			sb.WriteString(fmt.Sprintf("    受け入れ基準: %s\n", action.AcceptanceCriteria))
			if len(action.LinkedFiles) > 0 {
				sb.WriteString(fmt.Sprintf("    関連ファイル: %s\n", strings.Join(action.LinkedFiles, ", ")))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("========================================\n")

	// ファイルに書き込み（追記モード）
	f, err := os.OpenFile(n.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("ファイルを開けませんでした: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(sb.String()); err != nil {
		return fmt.Errorf("ファイルへの書き込みに失敗: %w", err)
	}

	return nil
}

// MultiNotifier は複数のNotifierに通知するNotifierです
type MultiNotifier struct {
	Notifiers []Notifier
}

// NewMultiNotifier は新しいMultiNotifierを作成します
func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	return &MultiNotifier{
		Notifiers: notifiers,
	}
}

// Notify はすべてのNotifierに通知します
func (n *MultiNotifier) Notify(actions []models.Action) error {
	var errors []string

	for _, notifier := range n.Notifiers {
		if err := notifier.Notify(actions); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("一部の通知に失敗しました: %s", strings.Join(errors, "; "))
	}

	return nil
}
