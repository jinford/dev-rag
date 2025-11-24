package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jinford/dev-rag/internal/module/llm/domain"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

const (
	// DefaultModel はデフォルトで使用するOpenAIモデル
	DefaultModel = "gpt-4o-mini"

	// DefaultTimeout はAPI呼び出しのデフォルトタイムアウト
	DefaultTimeout = 60 * time.Second

	// MaxRetries はレート制限エラー時の最大リトライ回数
	MaxRetries = 3

	// BaseBackoff はExponential Backoffの基底時間
	BaseBackoff = 2 * time.Second

	// MaxBackoff はExponential Backoffの最大待機時間
	MaxBackoff = 32 * time.Second

	// JSONParseMaxRetries はJSON解析エラー時の最大リトライ回数
	JSONParseMaxRetries = 1
)

var (
	// ErrAPIKeyNotSet はAPIキーが設定されていない場合のエラー
	ErrAPIKeyNotSet = errors.New("OpenAI API key not set")
)

// OpenAIClient はOpenAI APIを使用したLLMクライアント実装
type OpenAIClient struct {
	client  openai.Client
	model   string
	timeout time.Duration
}

// NewOpenAIClient はAPIキーとモデルを指定してOpenAIClientを作成する
func NewOpenAIClient(apiKey, model string) (*OpenAIClient, error) {
	if apiKey == "" {
		return nil, ErrAPIKeyNotSet
	}

	if model == "" {
		model = DefaultModel
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	return &OpenAIClient{
		client:  client,
		model:   model,
		timeout: DefaultTimeout,
	}, nil
}

// SetTimeout はAPIコールのタイムアウトを設定する
func (c *OpenAIClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// GetModelName はモデル名を返す
func (c *OpenAIClient) GetModelName() string {
	return c.model
}

// GenerateCompletion はOpenAI APIを使用してテキストを生成する
// domain.Clientインターフェースを実装
func (c *OpenAIClient) GenerateCompletion(ctx context.Context, req domain.CompletionRequest) (domain.CompletionResponse, error) {
	// タイムアウト付きコンテキストの作成
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	model := c.model
	if req.Model != "" {
		model = req.Model
	}

	// JSON形式のレスポンスが要求された場合の処理
	var jsonParseRetries int
	for {
		// レート制限エラー用のリトライループ
		resp, err := c.generateWithRetry(ctx, model, req)
		if err != nil {
			return domain.CompletionResponse{}, err
		}

		// JSON形式が要求されている場合は妥当性を検証
		if req.ResponseFormat == "json" {
			if !isValidJSON(resp.Content) {
				jsonParseRetries++
				if jsonParseRetries > JSONParseMaxRetries {
					return domain.CompletionResponse{}, fmt.Errorf("JSON parse failed after %d retries", JSONParseMaxRetries)
				}
				// JSON解析に失敗した場合は再試行
				continue
			}
		}

		return resp, nil
	}
}

// generateWithRetry はレート制限エラー時にExponential Backoffでリトライする
func (c *OpenAIClient) generateWithRetry(ctx context.Context, model string, req domain.CompletionRequest) (domain.CompletionResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential Backoff
			backoffDuration := time.Duration(math.Pow(2, float64(attempt-1))) * BaseBackoff
			if backoffDuration > MaxBackoff {
				backoffDuration = MaxBackoff
			}

			select {
			case <-ctx.Done():
				return domain.CompletionResponse{}, ctx.Err()
			case <-time.After(backoffDuration):
				// バックオフ後、再試行
			}
		}

		// ChatCompletion APIパラメータの構築
		params := openai.ChatCompletionNewParams{
			Model: shared.ChatModel(model),
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(req.Prompt),
			},
			Temperature: openai.Float(req.Temperature),
		}

		// MaxTokensを設定
		if req.MaxTokens > 0 {
			params.MaxTokens = openai.Int(int64(req.MaxTokens))
		}

		// JSON形式が要求された場合
		if req.ResponseFormat == "json" {
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: &shared.ResponseFormatJSONObjectParam{
					Type: "json_object",
				},
			}
		}

		// API呼び出し
		completion, err := c.client.Chat.Completions.New(ctx, params)
		if err != nil {
			lastErr = err

			// レート制限エラーの判定
			if isRateLimitError(err) {
				// リトライ対象: 次のループへ
				continue
			}

			// その他のエラーは即座に返す
			return domain.CompletionResponse{}, fmt.Errorf("OpenAI API call failed: %w", err)
		}

		// レスポンスの解析
		if len(completion.Choices) == 0 {
			return domain.CompletionResponse{}, fmt.Errorf("no completion choices returned")
		}

		content := completion.Choices[0].Message.Content
		tokensUsed := int(completion.Usage.TotalTokens)

		return domain.CompletionResponse{
			Content:    content,
			TokensUsed: tokensUsed,
			Model:      string(completion.Model),
		}, nil
	}

	// 最大リトライ回数を超過
	return domain.CompletionResponse{}, fmt.Errorf("%w: %v", domain.ErrMaxRetriesExceeded, lastErr)
}

// isRateLimitError はエラーがレート制限エラーかどうかを判定する
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	// OpenAI SDKのエラー型を確認
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		// ステータスコード429はレート制限エラー
		return apiErr.StatusCode == 429
	}

	return false
}

// isValidJSON は文字列が有効なJSONかどうかを判定する
func isValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

// インターフェース実装の確認
var _ domain.Client = (*OpenAIClient)(nil)
