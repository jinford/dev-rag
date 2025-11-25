package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewErrorHandler(t *testing.T) {
	tests := []struct {
		name    string
		logDir  string
		wantErr bool
		enabled bool
	}{
		{
			name:    "有効なログディレクトリ",
			logDir:  t.TempDir(),
			wantErr: false,
			enabled: true,
		},
		{
			name:    "空のログディレクトリ（無効化）",
			logDir:  "",
			wantErr: false,
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewErrorHandler(tt.logDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewErrorHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if handler == nil {
				t.Fatal("handler is nil")
			}
			if handler.enabled != tt.enabled {
				t.Errorf("handler.enabled = %v, want %v", handler.enabled, tt.enabled)
			}

			// クリーンアップ
			if handler.logFile != nil {
				handler.Close()
			}
		})
	}
}

func TestErrorHandler_LogError(t *testing.T) {
	// 一時ディレクトリを作成
	logDir := t.TempDir()

	handler, err := NewErrorHandler(logDir)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer handler.Close()

	// エラーレコードを作成
	record := ErrorRecord{
		Timestamp:     time.Now(),
		ErrorType:     ErrorTypeJSONParseFailed,
		PromptSection: PromptSectionFileSummary,
		Prompt:        "test prompt",
		Response:      "test response",
		ErrorMessage:  "test error",
		RetryCount:    1,
	}

	// ログに記録
	if err := handler.LogError(record); err != nil {
		t.Errorf("LogError() error = %v", err)
	}

	// ログファイルを閉じる
	handler.Close()

	// ログファイルが存在することを確認
	logFileName := "llm_errors_" + time.Now().Format("2006-01-02") + ".jsonl"
	logFilePath := filepath.Join(logDir, logFileName)

	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		t.Errorf("Log file does not exist: %s", logFilePath)
	}

	// ログファイルの内容を確認
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// JSON形式であることを確認
	var loggedRecord ErrorRecord
	if err := json.Unmarshal(content, &loggedRecord); err != nil {
		t.Errorf("Failed to unmarshal log content: %v", err)
	}

	// 記録された内容を確認
	if loggedRecord.ErrorType != ErrorTypeJSONParseFailed {
		t.Errorf("ErrorType = %v, want %v", loggedRecord.ErrorType, ErrorTypeJSONParseFailed)
	}
	if loggedRecord.PromptSection != PromptSectionFileSummary {
		t.Errorf("PromptSection = %v, want %v", loggedRecord.PromptSection, PromptSectionFileSummary)
	}
	if loggedRecord.Prompt != "test prompt" {
		t.Errorf("Prompt = %v, want %v", loggedRecord.Prompt, "test prompt")
	}
}

func TestErrorHandler_LogError_Disabled(t *testing.T) {
	// ログが無効化されている場合
	handler, err := NewErrorHandler("")
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	record := ErrorRecord{
		Timestamp:     time.Now(),
		ErrorType:     ErrorTypeJSONParseFailed,
		PromptSection: PromptSectionFileSummary,
		Prompt:        "test prompt",
		Response:      "test response",
		ErrorMessage:  "test error",
		RetryCount:    1,
	}

	// ログに記録（エラーが発生しないことを確認）
	if err := handler.LogError(record); err != nil {
		t.Errorf("LogError() error = %v, expected nil", err)
	}
}

func TestCreateErrorResponse(t *testing.T) {
	tests := []struct {
		name    string
		section PromptSection
	}{
		{
			name:    "ファイルサマリー",
			section: PromptSectionFileSummary,
		},
		{
			name:    "チャンク要約",
			section: PromptSectionChunkSummary,
		},
		{
			name:    "ドメイン分類",
			section: PromptSectionDomainClassification,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := CreateErrorResponse(tt.section)

			// JSON形式であることを確認
			var parsed map[string]string
			if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
			}

			// 必須フィールドの確認
			if parsed["error"] != string(ErrorTypeJSONParseFailed) {
				t.Errorf("error = %v, want %v", parsed["error"], ErrorTypeJSONParseFailed)
			}
			if parsed["prompt_section"] != string(tt.section) {
				t.Errorf("prompt_section = %v, want %v", parsed["prompt_section"], tt.section)
			}
		})
	}
}

func TestParseErrorResponse(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		wantIsError     bool
		wantErrorType   ErrorType
		wantSection     PromptSection
	}{
		{
			name:            "エラーレスポンス",
			content:         `{"error":"parse_failed","prompt_section":"9.2"}`,
			wantIsError:     true,
			wantErrorType:   ErrorTypeJSONParseFailed,
			wantSection:     PromptSectionFileSummary,
		},
		{
			name:            "正常なレスポンス",
			content:         `{"prompt_version":"1.1","summary":["test"]}`,
			wantIsError:     false,
			wantErrorType:   "",
			wantSection:     "",
		},
		{
			name:            "不正なJSON",
			content:         `invalid json`,
			wantIsError:     false,
			wantErrorType:   "",
			wantSection:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isError, errorType, section := ParseErrorResponse(tt.content)

			if isError != tt.wantIsError {
				t.Errorf("isError = %v, want %v", isError, tt.wantIsError)
			}
			if errorType != tt.wantErrorType {
				t.Errorf("errorType = %v, want %v", errorType, tt.wantErrorType)
			}
			if section != tt.wantSection {
				t.Errorf("section = %v, want %v", section, tt.wantSection)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "短い文字列",
			input:  "short",
			maxLen: 10,
			want:   "short",
		},
		{
			name:   "長い文字列",
			input:  "this is a very long string that should be truncated",
			maxLen: 20,
			want:   "this is a very long ... (truncated)",
		},
		{
			name:   "ちょうどの長さ",
			input:  "exact",
			maxLen: 5,
			want:   "exact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("TruncateString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobalErrorHandler(t *testing.T) {
	// 元のグローバルハンドラーを保存
	originalHandler := GlobalErrorHandler
	defer func() {
		GlobalErrorHandler = originalHandler
	}()

	// 一時ディレクトリを作成
	logDir := t.TempDir()

	// グローバルハンドラーを初期化
	if err := InitGlobalErrorHandler(logDir); err != nil {
		t.Fatalf("InitGlobalErrorHandler() error = %v", err)
	}

	if GlobalErrorHandler == nil {
		t.Fatal("GlobalErrorHandler is nil")
	}

	// クリーンアップ
	if err := CloseGlobalErrorHandler(); err != nil {
		t.Errorf("CloseGlobalErrorHandler() error = %v", err)
	}
}

func TestErrorHandler_MultipleRecords(t *testing.T) {
	// 一時ディレクトリを作成
	logDir := t.TempDir()

	handler, err := NewErrorHandler(logDir)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer handler.Close()

	// 複数のエラーレコードを記録
	records := []ErrorRecord{
		{
			Timestamp:     time.Now(),
			ErrorType:     ErrorTypeJSONParseFailed,
			PromptSection: PromptSectionFileSummary,
			Prompt:        "prompt1",
			Response:      "response1",
			ErrorMessage:  "error1",
			RetryCount:    1,
		},
		{
			Timestamp:     time.Now(),
			ErrorType:     ErrorTypeTimeout,
			PromptSection: PromptSectionChunkSummary,
			Prompt:        "prompt2",
			Response:      "response2",
			ErrorMessage:  "error2",
			RetryCount:    0,
		},
		{
			Timestamp:     time.Now(),
			ErrorType:     ErrorTypeRateLimitExceeded,
			PromptSection: PromptSectionDomainClassification,
			Prompt:        "prompt3",
			Response:      "response3",
			ErrorMessage:  "error3",
			RetryCount:    3,
		},
	}

	for _, record := range records {
		if err := handler.LogError(record); err != nil {
			t.Errorf("LogError() error = %v", err)
		}
	}

	// ログファイルを閉じる
	handler.Close()

	// ログファイルを読み込む
	logFileName := "llm_errors_" + time.Now().Format("2006-01-02") + ".jsonl"
	logFilePath := filepath.Join(logDir, logFileName)

	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// 行数を確認
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != len(records) {
		t.Errorf("Expected %d log lines, got %d", len(records), len(lines))
	}

	// 各行がJSON形式であることを確認
	for i, line := range lines {
		var loggedRecord ErrorRecord
		if err := json.Unmarshal([]byte(line), &loggedRecord); err != nil {
			t.Errorf("Failed to unmarshal line %d: %v", i+1, err)
		}
	}
}
