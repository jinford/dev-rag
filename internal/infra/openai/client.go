package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/jinford/dev-rag/internal/core/wiki"
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
	ErrAPIKeyNotSet = errors.New("OpenAI API key not set: please set OPENAI_API_KEY environment variable")

	// ErrInvalidResponseFormat は不正なレスポンス形式のエラー
	ErrInvalidResponseFormat = errors.New("invalid response format")

	// ErrMaxRetriesExceeded は最大リトライ回数を超過した場合のエラー
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

// Client は OpenAI API を使用した LLM クライアント実装
type Client struct {
	client  openai.Client
	model   string
	timeout time.Duration
}

// NewClient は新しい Client を作成する
// APIキーは環境変数 OPENAI_API_KEY から読み込む
func NewClient() (*Client, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, ErrAPIKeyNotSet
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	return &Client{
		client:  client,
		model:   DefaultModel,
		timeout: DefaultTimeout,
	}, nil
}

// NewClientWithModel はモデルを指定して Client を作成する
func NewClientWithModel(model string) (*Client, error) {
	client, err := NewClient()
	if err != nil {
		return nil, err
	}
	client.model = model
	return client, nil
}

// NewClientWithAPIKey はAPIキーとモデルを指定して Client を作成する
func NewClientWithAPIKey(apiKey, model string) (*Client, error) {
	if apiKey == "" {
		return nil, ErrAPIKeyNotSet
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	return &Client{
		client:  client,
		model:   model,
		timeout: DefaultTimeout,
	}, nil
}

// SetTimeout はAPIコールのタイムアウトを設定する
func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// ModelName はモデル名を返す
func (c *Client) ModelName() string {
	return c.model
}

// GenerateCompletion は OpenAI API を使用してテキストを生成する
func (c *Client) GenerateCompletion(ctx context.Context, req wiki.CompletionRequest) (wiki.CompletionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	model := c.model
	if req.Model != "" {
		model = req.Model
	}

	var jsonParseRetries int
	for {
		resp, err := c.generateWithRetry(ctx, model, req)
		if err != nil {
			return wiki.CompletionResponse{}, err
		}

		if req.ResponseFormat == "json" {
			if !isValidJSON(resp.Content) {
				jsonParseRetries++
				if jsonParseRetries > JSONParseMaxRetries {
					return wiki.CompletionResponse{}, fmt.Errorf("%w: JSON parse failed after %d retries", ErrInvalidResponseFormat, JSONParseMaxRetries)
				}
				continue
			}
		}

		return resp, nil
	}
}

func (c *Client) generateWithRetry(ctx context.Context, model string, req wiki.CompletionRequest) (wiki.CompletionResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			backoffDuration := time.Duration(math.Pow(2, float64(attempt-1))) * BaseBackoff
			if backoffDuration > MaxBackoff {
				backoffDuration = MaxBackoff
			}

			select {
			case <-ctx.Done():
				return wiki.CompletionResponse{}, ctx.Err()
			case <-time.After(backoffDuration):
			}
		}

		params := openai.ChatCompletionNewParams{
			Model: shared.ChatModel(model),
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(req.Prompt),
			},
			Temperature: openai.Float(req.Temperature),
		}

		if req.MaxTokens > 0 {
			params.MaxTokens = openai.Int(int64(req.MaxTokens))
		}

		if req.ResponseFormat == "json" {
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: &shared.ResponseFormatJSONObjectParam{
					Type: "json_object",
				},
			}
		}

		completion, err := c.client.Chat.Completions.New(ctx, params)
		if err != nil {
			lastErr = err

			if isRateLimitError(err) {
				continue
			}

			return wiki.CompletionResponse{}, fmt.Errorf("OpenAI API call failed: %w", err)
		}

		if len(completion.Choices) == 0 {
			return wiki.CompletionResponse{}, fmt.Errorf("no completion choices returned")
		}

		content := completion.Choices[0].Message.Content
		tokensUsed := int(completion.Usage.TotalTokens)

		return wiki.CompletionResponse{
			Content:    content,
			TokensUsed: tokensUsed,
			Model:      string(completion.Model),
		}, nil
	}

	return wiki.CompletionResponse{}, fmt.Errorf("%w: %v", ErrMaxRetriesExceeded, lastErr)
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429
	}

	return false
}

func isValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

// インターフェース実装の確認
var _ wiki.Client = (*Client)(nil)
