package llm

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrorType はエラーの種類を表します
type ErrorType string

const (
	// ErrorTypeJSONParseFailed はJSON解析エラー
	ErrorTypeJSONParseFailed ErrorType = "parse_failed"
	// ErrorTypeRateLimitExceeded はレート制限エラー
	ErrorTypeRateLimitExceeded ErrorType = "rate_limit_exceeded"
	// ErrorTypeTimeout はタイムアウトエラー
	ErrorTypeTimeout ErrorType = "timeout"
	// ErrorTypeUnknown は不明なエラー
	ErrorTypeUnknown ErrorType = "unknown"
)

// PromptSection はプロンプトのセクションを表します
type PromptSection string

const (
	// PromptSectionFileSummary はファイルサマリー生成プロンプト（9.2節）
	PromptSectionFileSummary PromptSection = "9.2"
	// PromptSectionChunkSummary はチャンク要約生成プロンプト（9.3節）
	PromptSectionChunkSummary PromptSection = "9.3"
	// PromptSectionDomainClassification はドメイン分類プロンプト（9.4節）
	PromptSectionDomainClassification PromptSection = "9.4"
	// PromptSectionActionGeneration はアクション生成プロンプト（9.5節）
	PromptSectionActionGeneration PromptSection = "9.5"
)

// ErrorRecord は失敗したLLM呼び出しのログレコードです
type ErrorRecord struct {
	// Timestamp はエラー発生時刻
	Timestamp time.Time `json:"timestamp"`
	// ErrorType はエラーの種類
	ErrorType ErrorType `json:"error_type"`
	// PromptSection はプロンプトのセクション
	PromptSection PromptSection `json:"prompt_section"`
	// Prompt は実際に送信されたプロンプト
	Prompt string `json:"prompt"`
	// Response はLLMから返されたレスポンス
	Response string `json:"response"`
	// ErrorMessage はエラーメッセージ
	ErrorMessage string `json:"error_message"`
	// RetryCount はリトライ回数
	RetryCount int `json:"retry_count"`
}

// ErrorHandler はLLMエラーハンドリングを一元管理します
type ErrorHandler struct {
	logFile  *os.File
	logMutex sync.Mutex
	enabled  bool
}

// NewErrorHandler は新しいErrorHandlerを作成します
func NewErrorHandler(logDir string) (*ErrorHandler, error) {
	if logDir == "" {
		// ログが無効化されている場合
		return &ErrorHandler{enabled: false}, nil
	}

	// ログディレクトリを作成
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// ログファイルを作成（日付でローテーション）
	logFileName := fmt.Sprintf("llm_errors_%s.jsonl", time.Now().Format("2006-01-02"))
	logFilePath := filepath.Join(logDir, logFileName)

	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &ErrorHandler{
		logFile: logFile,
		enabled: true,
	}, nil
}

// Close はログファイルを閉じます
func (h *ErrorHandler) Close() error {
	if h.logFile != nil {
		return h.logFile.Close()
	}
	return nil
}

// LogError はエラーをログに記録します
func (h *ErrorHandler) LogError(record ErrorRecord) error {
	if !h.enabled {
		// ログが無効化されている場合はスキップ
		return nil
	}

	h.logMutex.Lock()
	defer h.logMutex.Unlock()

	// JSON形式でログに書き込み
	jsonBytes, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal error record: %w", err)
	}

	if _, err := h.logFile.Write(append(jsonBytes, '\n')); err != nil {
		return fmt.Errorf("failed to write log: %w", err)
	}

	// 標準エラー出力にも警告を出力
	log.Printf("[LLM Error] %s (section %s): %s", record.ErrorType, record.PromptSection, record.ErrorMessage)

	return nil
}

// CreateErrorResponse はJSON解析失敗時のエラーレスポンスを生成します
func CreateErrorResponse(section PromptSection) string {
	errorResp := map[string]string{
		"error":          string(ErrorTypeJSONParseFailed),
		"prompt_section": string(section),
	}
	jsonBytes, _ := json.Marshal(errorResp)
	return string(jsonBytes)
}

// ParseErrorResponse はエラーレスポンスかどうかを判定します
func ParseErrorResponse(content string) (bool, ErrorType, PromptSection) {
	var resp map[string]string
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return false, "", ""
	}

	if errorType, ok := resp["error"]; ok {
		section := PromptSection(resp["prompt_section"])
		return true, ErrorType(errorType), section
	}

	return false, "", ""
}

// TruncateString は文字列を指定された長さに切り詰めます（ログ記録用）
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}

// GlobalErrorHandler はグローバルなエラーハンドラーのインスタンス
var GlobalErrorHandler *ErrorHandler

// InitGlobalErrorHandler はグローバルなエラーハンドラーを初期化します
func InitGlobalErrorHandler(logDir string) error {
	handler, err := NewErrorHandler(logDir)
	if err != nil {
		return err
	}
	GlobalErrorHandler = handler
	return nil
}

// CloseGlobalErrorHandler はグローバルなエラーハンドラーを閉じます
func CloseGlobalErrorHandler() error {
	if GlobalErrorHandler != nil {
		return GlobalErrorHandler.Close()
	}
	return nil
}
