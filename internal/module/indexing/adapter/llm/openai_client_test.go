package llm

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewOpenAIClient はOpenAIClientの初期化をテストする
func TestNewOpenAIClient(t *testing.T) {
	tests := []struct {
		name      string
		setupEnv  func()
		wantError bool
		errType   error
	}{
		{
			name: "API keyが設定されている場合は成功する",
			setupEnv: func() {
				os.Setenv("OPENAI_API_KEY", "test-api-key")
			},
			wantError: false,
		},
		{
			name: "API keyが設定されていない場合はエラーを返す",
			setupEnv: func() {
				os.Unsetenv("OPENAI_API_KEY")
			},
			wantError: true,
			errType:   ErrAPIKeyNotSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer os.Unsetenv("OPENAI_API_KEY")

			client, err := NewOpenAIClient()

			if tt.wantError {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, DefaultModel, client.model)
				assert.Equal(t, DefaultTimeout, client.timeout)
			}
		})
	}
}

// TestNewOpenAIClientWithModel はカスタムモデルでの初期化をテストする
func TestNewOpenAIClientWithModel(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "test-api-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	customModel := "gpt-4o"
	client, err := NewOpenAIClientWithModel(customModel)

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, customModel, client.model)
}

// TestSetTimeout はタイムアウト設定をテストする
func TestSetTimeout(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "test-api-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	client, err := NewOpenAIClient()
	require.NoError(t, err)

	customTimeout := 30 * time.Second
	client.SetTimeout(customTimeout)

	assert.Equal(t, customTimeout, client.timeout)
}

// TestIsValidJSON はJSON妥当性検証をテストする
func TestIsValidJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "有効なJSONオブジェクト",
			input: `{"key": "value"}`,
			want:  true,
		},
		{
			name:  "有効なJSON配列",
			input: `["item1", "item2"]`,
			want:  true,
		},
		{
			name:  "有効なJSON (null)",
			input: `null`,
			want:  true,
		},
		{
			name:  "不正なJSON (閉じ括弧なし)",
			input: `{"key": "value"`,
			want:  false,
		},
		{
			name:  "不正なJSON (プレーンテキスト)",
			input: `This is plain text`,
			want:  false,
		},
		{
			name:  "空文字列",
			input: ``,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidJSON(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestIsRateLimitError はレート制限エラーの判定をテストする
func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		want  bool
	}{
		{
			name: "nilエラーはfalse",
			err:  nil,
			want: false,
		},
		{
			name: "通常のエラーはfalse",
			err:  errors.New("some error"),
			want: false,
		},
		// Note: OpenAI SDKの実際のエラーを使用した統合テストは別途実施する必要がある
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRateLimitError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestCompletionRequest はCompletionRequestの構造をテストする
func TestCompletionRequest(t *testing.T) {
	req := CompletionRequest{
		Prompt:         "Test prompt",
		Temperature:    0.5,
		MaxTokens:      100,
		ResponseFormat: "json",
		Model:          "gpt-4o",
	}

	assert.Equal(t, "Test prompt", req.Prompt)
	assert.Equal(t, 0.5, req.Temperature)
	assert.Equal(t, 100, req.MaxTokens)
	assert.Equal(t, "json", req.ResponseFormat)
	assert.Equal(t, "gpt-4o", req.Model)
}

// TestCompletionResponse はCompletionResponseの構造をテストする
func TestCompletionResponse(t *testing.T) {
	resp := CompletionResponse{
		Content:       "Test content",
		TokensUsed:    50,
		PromptVersion: "1.1",
		Model:         "gpt-4o-mini",
	}

	assert.Equal(t, "Test content", resp.Content)
	assert.Equal(t, 50, resp.TokensUsed)
	assert.Equal(t, "1.1", resp.PromptVersion)
	assert.Equal(t, "gpt-4o-mini", resp.Model)
}

// MockLLMClient はテスト用のモッククライアント
type MockLLMClient struct {
	GenerateCompletionFunc func(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

func (m *MockLLMClient) GenerateCompletion(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	if m.GenerateCompletionFunc != nil {
		return m.GenerateCompletionFunc(ctx, req)
	}
	return CompletionResponse{}, nil
}

// TestMockLLMClient はモッククライアントが正しく動作することをテストする
func TestMockLLMClient(t *testing.T) {
	t.Run("成功レスポンスを返す", func(t *testing.T) {
		expectedResp := CompletionResponse{
			Content:       "Mock response",
			TokensUsed:    10,
			PromptVersion: "1.1",
			Model:         "mock-model",
		}

		mock := &MockLLMClient{
			GenerateCompletionFunc: func(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
				return expectedResp, nil
			},
		}

		req := CompletionRequest{
			Prompt:      "Test prompt",
			Temperature: 0.5,
			MaxTokens:   100,
		}

		resp, err := mock.GenerateCompletion(context.Background(), req)
		assert.NoError(t, err)
		assert.Equal(t, expectedResp, resp)
	})

	t.Run("エラーを返す", func(t *testing.T) {
		expectedErr := errors.New("mock error")

		mock := &MockLLMClient{
			GenerateCompletionFunc: func(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
				return CompletionResponse{}, expectedErr
			},
		}

		req := CompletionRequest{
			Prompt: "Test prompt",
		}

		resp, err := mock.GenerateCompletion(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, CompletionResponse{}, resp)
	})

	t.Run("JSON形式のレスポンスを返す", func(t *testing.T) {
		jsonContent := `{"prompt_version": "1.1", "summary": ["item1", "item2"]}`

		mock := &MockLLMClient{
			GenerateCompletionFunc: func(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
				// ResponseFormatがjsonの場合はJSON形式のコンテンツを返す
				if req.ResponseFormat == "json" {
					return CompletionResponse{
						Content:    jsonContent,
						TokensUsed: 20,
						Model:      "mock-model",
					}, nil
				}
				return CompletionResponse{
					Content:    "Plain text response",
					TokensUsed: 10,
					Model:      "mock-model",
				}, nil
			},
		}

		req := CompletionRequest{
			Prompt:         "Test prompt",
			ResponseFormat: "json",
		}

		resp, err := mock.GenerateCompletion(context.Background(), req)
		assert.NoError(t, err)
		assert.Equal(t, jsonContent, resp.Content)
		assert.True(t, isValidJSON(resp.Content))

		// JSON内容の検証
		var jsonData map[string]interface{}
		err = json.Unmarshal([]byte(resp.Content), &jsonData)
		assert.NoError(t, err)
		assert.Equal(t, "1.1", jsonData["prompt_version"])
	})

	t.Run("タイムアウトをテストする", func(t *testing.T) {
		mock := &MockLLMClient{
			GenerateCompletionFunc: func(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
				// コンテキストのキャンセルをシミュレート
				select {
				case <-ctx.Done():
					return CompletionResponse{}, ctx.Err()
				case <-time.After(100 * time.Millisecond):
					return CompletionResponse{Content: "Success"}, nil
				}
			},
		}

		// タイムアウトが短いコンテキスト
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		req := CompletionRequest{
			Prompt: "Test prompt",
		}

		_, err := mock.GenerateCompletion(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, context.DeadlineExceeded, err)
	})
}

// TestLLMClientInterface はLLMClientインターフェースの実装をテストする
func TestLLMClientInterface(t *testing.T) {
	// OpenAIClientがLLMClientインターフェースを実装していることを確認
	var _ LLMClient = (*OpenAIClient)(nil)

	// MockLLMClientがLLMClientインターフェースを実装していることを確認
	var _ LLMClient = (*MockLLMClient)(nil)
}

// 統合テスト用の例 (実際のAPI呼び出しが必要なため、通常はスキップ)
// go test -v -tags=integration を実行した場合のみ実行される
//
// func TestOpenAIClientIntegration(t *testing.T) {
//     if os.Getenv("INTEGRATION_TEST") != "true" {
//         t.Skip("Skipping integration test")
//     }
//
//     client, err := NewOpenAIClient()
//     require.NoError(t, err)
//
//     req := CompletionRequest{
//         Prompt:         "Say 'Hello, World!' in JSON format with a 'message' field.",
//         Temperature:    0.0,
//         MaxTokens:      50,
//         ResponseFormat: "json",
//     }
//
//     resp, err := client.GenerateCompletion(context.Background(), req)
//     require.NoError(t, err)
//
//     assert.NotEmpty(t, resp.Content)
//     assert.True(t, isValidJSON(resp.Content))
//     assert.Greater(t, resp.TokensUsed, 0)
//     assert.NotEmpty(t, resp.Model)
// }
