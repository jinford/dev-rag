package prompts

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jinford/dev-rag/pkg/indexer/llm"
)

// MockLLMClient はLLMClientのモック実装
type MockLLMClient struct {
	response      string
	err           error
	callCount     int
	lastRequest   llm.CompletionRequest
	responseDelay int // レスポンスを返すまでの遅延回数（リトライテスト用）
}

func (m *MockLLMClient) GenerateCompletion(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	m.callCount++
	m.lastRequest = req

	// 遅延カウントがある場合は最初の数回はエラーを返す
	if m.responseDelay > 0 && m.callCount <= m.responseDelay {
		return llm.CompletionResponse{}, errors.New("temporary error")
	}

	if m.err != nil {
		return llm.CompletionResponse{}, m.err
	}

	return llm.CompletionResponse{
		Content:    m.response,
		TokensUsed: 100,
		Model:      "gpt-4o-mini",
	}, nil
}

func TestGenerateFileSummaryPrompt(t *testing.T) {
	tests := []struct {
		name     string
		req      FileSummaryRequest
		contains []string
	}{
		{
			name: "基本的なプロンプト生成",
			req: FileSummaryRequest{
				FilePath:    "pkg/indexer/indexer.go",
				Language:    "Go",
				FileContent: "package indexer\n\nfunc IndexSource() {}",
			},
			contains: []string{
				"pkg/indexer/indexer.go",
				"Go",
				"package indexer",
				"JSON response",
				"prompt_version",
			},
		},
		{
			name: "Markdownファイル",
			req: FileSummaryRequest{
				FilePath:    "README.md",
				Language:    "Markdown",
				FileContent: "# Project Title\n\nThis is a description.",
			},
			contains: []string{
				"README.md",
				"Markdown",
				"# Project Title",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := GenerateFileSummaryPrompt(tt.req)

			for _, substr := range tt.contains {
				if !strings.Contains(prompt, substr) {
					t.Errorf("プロンプトに '%s' が含まれていません", substr)
				}
			}
		})
	}
}

func TestFileSummaryGenerator_Generate(t *testing.T) {
	tests := []struct {
		name           string
		req            FileSummaryRequest
		mockResponse   string
		mockErr        error
		expectedErr    bool
		validateResult func(*testing.T, *FileSummaryResponse)
	}{
		{
			name: "正常なレスポンス",
			req: FileSummaryRequest{
				FilePath:    "pkg/indexer/indexer.go",
				Language:    "Go",
				FileContent: "package indexer\n\nfunc IndexSource() {}",
			},
			mockResponse: `{
				"prompt_version": "1.1",
				"summary": [
					"IndexSource関数がチャンク生成→埋め込み計算→ベクターストア登録を連鎖実行",
					"設定値はenvとconfigファイルをマージして解決"
				],
				"risks": [
					"embedサービスへの同期呼び出しでレイテンシが高い"
				],
				"metadata": {
					"primary_topics": ["indexing", "configuration"],
					"key_symbols": ["IndexSource", "Indexer"]
				}
			}`,
			expectedErr: false,
			validateResult: func(t *testing.T, resp *FileSummaryResponse) {
				if resp.PromptVersion != "1.1" {
					t.Errorf("PromptVersion = %s, want 1.1", resp.PromptVersion)
				}
				if len(resp.Summary) != 2 {
					t.Errorf("len(Summary) = %d, want 2", len(resp.Summary))
				}
				if len(resp.Risks) != 1 {
					t.Errorf("len(Risks) = %d, want 1", len(resp.Risks))
				}
				if len(resp.Metadata.PrimaryTopics) != 2 {
					t.Errorf("len(PrimaryTopics) = %d, want 2", len(resp.Metadata.PrimaryTopics))
				}
				if len(resp.Metadata.KeySymbols) != 2 {
					t.Errorf("len(KeySymbols) = %d, want 2", len(resp.Metadata.KeySymbols))
				}
			},
		},
		{
			name: "リスクなしのレスポンス",
			req: FileSummaryRequest{
				FilePath:    "pkg/models/models.go",
				Language:    "Go",
				FileContent: "package models\n\ntype User struct {}",
			},
			mockResponse: `{
				"prompt_version": "1.1",
				"summary": [
					"User構造体を定義"
				],
				"risks": [],
				"metadata": {
					"primary_topics": ["models"],
					"key_symbols": ["User"]
				}
			}`,
			expectedErr: false,
			validateResult: func(t *testing.T, resp *FileSummaryResponse) {
				if len(resp.Summary) != 1 {
					t.Errorf("len(Summary) = %d, want 1", len(resp.Summary))
				}
				if len(resp.Risks) != 0 {
					t.Errorf("len(Risks) = %d, want 0", len(resp.Risks))
				}
			},
		},
		{
			name: "不正なJSONレスポンス",
			req: FileSummaryRequest{
				FilePath:    "test.go",
				Language:    "Go",
				FileContent: "package test",
			},
			mockResponse: `invalid json`,
			expectedErr:  true,
		},
		{
			name: "LLMエラー",
			req: FileSummaryRequest{
				FilePath:    "test.go",
				Language:    "Go",
				FileContent: "package test",
			},
			mockErr:     errors.New("API rate limit exceeded"),
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockLLMClient{
				response: tt.mockResponse,
				err:      tt.mockErr,
			}

			tokenCounter, err := llm.NewTokenCounter()
			if err != nil {
				t.Fatalf("Failed to create token counter: %v", err)
			}

			generator := NewFileSummaryGenerator(mockClient, tokenCounter)

			resp, err := generator.Generate(context.Background(), tt.req)

			if tt.expectedErr {
				if err == nil {
					t.Error("エラーが期待されましたが、エラーが返されませんでした")
				}
				return
			}

			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}

			if tt.validateResult != nil {
				tt.validateResult(t, resp)
			}

			// リクエストパラメータの検証
			if mockClient.lastRequest.Temperature != FileSummaryTemperature {
				t.Errorf("Temperature = %f, want %f", mockClient.lastRequest.Temperature, FileSummaryTemperature)
			}
			if mockClient.lastRequest.MaxTokens != FileSummaryMaxTokens {
				t.Errorf("MaxTokens = %d, want %d", mockClient.lastRequest.MaxTokens, FileSummaryMaxTokens)
			}
			if mockClient.lastRequest.ResponseFormat != "json" {
				t.Errorf("ResponseFormat = %s, want json", mockClient.lastRequest.ResponseFormat)
			}
		})
	}
}

func TestGenerateSummaryText(t *testing.T) {
	tests := []struct {
		name     string
		resp     *FileSummaryResponse
		contains []string
	}{
		{
			name: "完全なサマリーテキスト",
			resp: &FileSummaryResponse{
				PromptVersion: "1.1",
				Summary: []string{
					"IndexSource関数がチャンク生成を実行",
					"設定値はconfigファイルで管理",
				},
				Risks: []string{
					"レイテンシが高い可能性",
				},
				Metadata: FileSummaryMetadata{
					PrimaryTopics: []string{"indexing", "configuration"},
					KeySymbols:    []string{"IndexSource"},
				},
			},
			contains: []string{
				"## Summary",
				"IndexSource関数がチャンク生成を実行",
				"設定値はconfigファイルで管理",
				"## Risks",
				"レイテンシが高い可能性",
				"## Metadata",
				"Primary Topics",
				"Key Symbols",
			},
		},
		{
			name: "リスクなしのサマリー",
			resp: &FileSummaryResponse{
				PromptVersion: "1.1",
				Summary: []string{
					"User構造体を定義",
				},
				Risks: []string{},
				Metadata: FileSummaryMetadata{
					PrimaryTopics: []string{"models"},
					KeySymbols:    []string{"User"},
				},
			},
			contains: []string{
				"## Summary",
				"User構造体を定義",
				"## Metadata",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := GenerateSummaryText(tt.resp)

			for _, substr := range tt.contains {
				if !strings.Contains(text, substr) {
					t.Errorf("サマリーテキストに '%s' が含まれていません\nGenerated text:\n%s", substr, text)
				}
			}
		})
	}
}

func TestFileSummaryResponse_JSON(t *testing.T) {
	// JSON形式のシリアライズ/デシリアライズが正しく動作するかテスト
	original := &FileSummaryResponse{
		PromptVersion: "1.1",
		Summary: []string{
			"項目1",
			"項目2",
		},
		Risks: []string{
			"リスク1",
		},
		Metadata: FileSummaryMetadata{
			PrimaryTopics: []string{"topic1", "topic2"},
			KeySymbols:    []string{"Symbol1", "Symbol2"},
		},
	}

	// シリアライズ
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	// デシリアライズ
	var restored FileSummaryResponse
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// 比較
	if restored.PromptVersion != original.PromptVersion {
		t.Errorf("PromptVersion mismatch: got %s, want %s", restored.PromptVersion, original.PromptVersion)
	}
	if len(restored.Summary) != len(original.Summary) {
		t.Errorf("Summary length mismatch: got %d, want %d", len(restored.Summary), len(original.Summary))
	}
	if len(restored.Risks) != len(original.Risks) {
		t.Errorf("Risks length mismatch: got %d, want %d", len(restored.Risks), len(original.Risks))
	}
	if len(restored.Metadata.PrimaryTopics) != len(original.Metadata.PrimaryTopics) {
		t.Errorf("PrimaryTopics length mismatch: got %d, want %d", len(restored.Metadata.PrimaryTopics), len(original.Metadata.PrimaryTopics))
	}
}
