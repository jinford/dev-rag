package prompts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jinford/dev-rag/pkg/indexer/llm"
)

const (
	// DomainClassificationPromptVersion はドメイン分類プロンプトのバージョン
	DomainClassificationPromptVersion = "1.1"

	// DomainClassificationTemperature はドメイン分類の温度設定
	// 完全に決定論的にする
	DomainClassificationTemperature = 0.0

	// DomainClassificationMaxTokens は生成する最大トークン数
	DomainClassificationMaxTokens = 300
)

// Domain はファイル/ディレクトリのドメイン分類
type Domain string

const (
	// DomainCode はアプリケーションコード、ライブラリコード
	DomainCode Domain = "code"
	// DomainArchitecture は設計文書、ADR、アーキテクチャ図
	DomainArchitecture Domain = "architecture"
	// DomainOps はCI/CD設定、監視設定、運用Runbook
	DomainOps Domain = "ops"
	// DomainTests はテストコード、テストデータ
	DomainTests Domain = "tests"
	// DomainInfra はインフラ定義（Terraform、Kubernetes YAML、Helm charts）
	DomainInfra Domain = "infra"
)

// DomainClassificationPrompt はドメイン分類プロンプトを構築します
const domainClassificationSystemPrompt = `You are a file classification assistant.

Your task is to classify files and directories into one of these domains:
- code: Application code, library code
- architecture: Design documents, ADRs, architecture diagrams
- ops: CI/CD config, monitoring config, runbooks
- tests: Test code, test data
- infra: Infrastructure definitions (Terraform, Kubernetes YAML, Helm charts)

Guidelines:
- Always return exactly one domain
- If information is insufficient, fall back to "code"
- Provide a rationale in 2 sentences or less, citing line numbers when relevant (e.g., L12)
- Consider directory hints when available
- Return a valid JSON response`

// DomainClassificationRequest はドメイン分類リクエスト
type DomainClassificationRequest struct {
	// NodePath はファイル/ディレクトリパス
	NodePath string
	// NodeType は "file" または "dir"
	NodeType string
	// DetectedLanguage は検出された言語
	DetectedLanguage string
	// LinesOfCode は行数
	LinesOfCode int
	// LastModified は最終更新日
	LastModified string
	// SampleLines はファイルのサンプル（最大50行: 先頭25行 + 末尾25行）
	SampleLines string
	// DirectoryHints はルールベース推定結果
	DirectoryHints *DirectoryHint
}

// DirectoryHint はルールベースのドメイン推定ヒント
type DirectoryHint struct {
	// Pattern はマッチしたパターン
	Pattern string `json:"pattern"`
	// SuggestedDomain は推定されたドメイン
	SuggestedDomain string `json:"suggested_domain"`
}

// DomainClassificationResponse はドメイン分類レスポンス
type DomainClassificationResponse struct {
	PromptVersion string  `json:"prompt_version"`
	Domain        string  `json:"domain"`
	Rationale     string  `json:"rationale"`
	Confidence    float64 `json:"confidence"`
}

// DomainClassifier はドメイン分類を担当します
type DomainClassifier struct {
	llmClient    llm.LLMClient
	tokenCounter *llm.TokenCounter
}

// NewDomainClassifier は新しいDomainClassifierを作成します
func NewDomainClassifier(llmClient llm.LLMClient, tokenCounter *llm.TokenCounter) *DomainClassifier {
	return &DomainClassifier{
		llmClient:    llmClient,
		tokenCounter: tokenCounter,
	}
}

// GenerateDomainClassificationPrompt はドメイン分類プロンプトを構築します
func GenerateDomainClassificationPrompt(req DomainClassificationRequest) string {
	var sb strings.Builder

	sb.WriteString("Classify the following file or directory into one of these domains:\n")
	sb.WriteString("- code: Application code, library code\n")
	sb.WriteString("- architecture: Design documents, ADRs, architecture diagrams\n")
	sb.WriteString("- ops: CI/CD config, monitoring config, runbooks\n")
	sb.WriteString("- tests: Test code, test data\n")
	sb.WriteString("- infra: Infrastructure definitions (Terraform, Kubernetes YAML, Helm charts)\n\n")

	sb.WriteString(fmt.Sprintf("Path: %s\n", req.NodePath))
	sb.WriteString(fmt.Sprintf("Type: %s\n", req.NodeType))
	if req.DetectedLanguage != "" {
		sb.WriteString(fmt.Sprintf("Language: %s\n", req.DetectedLanguage))
	}
	if req.LinesOfCode > 0 {
		sb.WriteString(fmt.Sprintf("Lines of Code: %d\n", req.LinesOfCode))
	}
	if req.LastModified != "" {
		sb.WriteString(fmt.Sprintf("Last Modified: %s\n", req.LastModified))
	}

	// サンプル行があれば追加
	if req.SampleLines != "" {
		sb.WriteString("\nSample Lines:\n")
		sb.WriteString(req.SampleLines)
		sb.WriteString("\n")
	}

	// ディレクトリヒントがあれば追加
	if req.DirectoryHints != nil {
		hintJSON, _ := json.Marshal(req.DirectoryHints)
		sb.WriteString(fmt.Sprintf("\nDirectory Hints: %s\n", string(hintJSON)))
	}

	sb.WriteString("\nAdditional classification hints:\n")
	sb.WriteString("- tests: *_test.*, *.spec.*, /tests/ directory\n")
	sb.WriteString("- architecture: docs/adr/, docs/design/, docs/decisions/\n")
	sb.WriteString("- ops: .github/workflows/, ci/, monitoring/\n")
	sb.WriteString("- infra: infra/, terraform/, k8s/, helm/\n")

	sb.WriteString("\nReturn a JSON response with the following structure:\n")
	sb.WriteString("{\n")
	sb.WriteString(`  "prompt_version": "1.1",` + "\n")
	sb.WriteString(`  "domain": "code",` + "\n")
	sb.WriteString(`  "rationale": "...",` + "\n")
	sb.WriteString(`  "confidence": 0.81` + "\n")
	sb.WriteString("}")

	return sb.String()
}

// Classify はLLMを使用してドメインを分類します
// エラー発生時にはエラーログを記録し、エラーを返します
// フォールバック: ドメイン分類失敗時はルールベース結果を使用すべき（呼び出し側で制御）
func (c *DomainClassifier) Classify(ctx context.Context, req DomainClassificationRequest) (*DomainClassificationResponse, error) {
	// プロンプトを構築
	userPrompt := GenerateDomainClassificationPrompt(req)

	// システムプロンプトとユーザープロンプトを結合
	fullPrompt := fmt.Sprintf("%s\n\n%s", domainClassificationSystemPrompt, userPrompt)

	// トークン数を計算
	tokens := c.tokenCounter.CountTokens(fullPrompt)

	// トークン数が多すぎる場合はエラー
	const maxInputTokens = 50000
	if tokens > maxInputTokens {
		return nil, fmt.Errorf("prompt too long: %d tokens (max: %d)", tokens, maxInputTokens)
	}

	// LLMを呼び出し
	llmReq := llm.CompletionRequest{
		Prompt:         fullPrompt,
		Temperature:    DomainClassificationTemperature,
		MaxTokens:      DomainClassificationMaxTokens,
		ResponseFormat: "json",
	}

	resp, err := c.llmClient.GenerateCompletion(ctx, llmReq)
	if err != nil {
		// エラーログを記録
		c.logError(fullPrompt, "", err, 0)
		return nil, fmt.Errorf("failed to generate completion: %w", err)
	}

	// JSON応答をパース
	var classificationResp DomainClassificationResponse
	if err := json.Unmarshal([]byte(resp.Content), &classificationResp); err != nil {
		// JSON解析エラーをログに記録
		c.logError(fullPrompt, resp.Content, err, 1)
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// プロンプトバージョンを検証
	llm.DefaultPromptVersionRegistry.ValidateVersion(llm.PromptTypeDomainClassification, classificationResp.PromptVersion)

	// ドメインの妥当性を検証
	validDomains := map[string]bool{
		"code":         true,
		"architecture": true,
		"ops":          true,
		"tests":        true,
		"infra":        true,
	}
	if !validDomains[classificationResp.Domain] {
		return nil, fmt.Errorf("invalid domain returned: %s", classificationResp.Domain)
	}

	return &classificationResp, nil
}

// logError はエラーをグローバルエラーハンドラーに記録します
func (c *DomainClassifier) logError(prompt, response string, err error, retryCount int) {
	if llm.GlobalErrorHandler == nil {
		return
	}

	// エラータイプを判定
	errorType := llm.ErrorTypeUnknown
	if errors.Is(err, context.DeadlineExceeded) {
		errorType = llm.ErrorTypeTimeout
	} else if errors.Is(err, llm.ErrMaxRetriesExceeded) {
		errorType = llm.ErrorTypeRateLimitExceeded
	} else if strings.Contains(err.Error(), "parse") || strings.Contains(err.Error(), "unmarshal") {
		errorType = llm.ErrorTypeJSONParseFailed
	}

	record := llm.ErrorRecord{
		Timestamp:     time.Now(),
		ErrorType:     errorType,
		PromptSection: llm.PromptSectionDomainClassification,
		Prompt:        llm.TruncateString(prompt, 5000),
		Response:      llm.TruncateString(response, 5000),
		ErrorMessage:  err.Error(),
		RetryCount:    retryCount,
	}

	_ = llm.GlobalErrorHandler.LogError(record)
}

// ExtractSampleLines はファイルから先頭25行と末尾25行を抽出します
func ExtractSampleLines(content string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 25
	}

	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// ファイルが短い場合はすべての行を返す
	if totalLines <= maxLines*2 {
		return content
	}

	// 先頭25行を取得
	var sb strings.Builder
	for i := 0; i < maxLines && i < totalLines; i++ {
		sb.WriteString(fmt.Sprintf("L%d: %s\n", i+1, lines[i]))
	}

	// 省略を示す行を追加
	sb.WriteString("\n... (omitted) ...\n\n")

	// 末尾25行を取得
	startIdx := totalLines - maxLines
	for i := startIdx; i < totalLines; i++ {
		sb.WriteString(fmt.Sprintf("L%d: %s\n", i+1, lines[i]))
	}

	return sb.String()
}

// CreateDirectoryHintFromRuleBased はルールベースの分類結果からDirectoryHintを生成します
func CreateDirectoryHintFromRuleBased(path string, ruleBasedDomain *string) *DirectoryHint {
	if ruleBasedDomain == nil {
		return nil
	}

	lowerPath := strings.ToLower(path)

	// パターンマッチングでヒントを生成
	var pattern string
	switch *ruleBasedDomain {
	case "tests":
		if strings.Contains(lowerPath, "_test.go") {
			pattern = "*_test.go"
		} else if strings.Contains(lowerPath, "_test.") {
			pattern = "*_test.*"
		} else if strings.Contains(lowerPath, "/test/") || strings.Contains(lowerPath, "/tests/") {
			pattern = "/tests/"
		}
	case "architecture":
		if strings.HasSuffix(lowerPath, ".md") {
			pattern = "*.md"
		} else if strings.Contains(lowerPath, "/docs/") {
			pattern = "/docs/"
		}
	case "ops":
		if strings.HasSuffix(lowerPath, ".sh") {
			pattern = "*.sh"
		} else if strings.Contains(lowerPath, "/scripts/") {
			pattern = "/scripts/"
		} else if strings.Contains(lowerPath, "/.github/workflows/") {
			pattern = ".github/workflows/"
		}
	case "infra":
		if strings.HasSuffix(lowerPath, ".tf") {
			pattern = "*.tf"
		} else if strings.HasSuffix(lowerPath, ".yaml") || strings.HasSuffix(lowerPath, ".yml") {
			pattern = "*.yaml"
		} else if strings.Contains(lowerPath, "/terraform/") {
			pattern = "/terraform/"
		} else if strings.Contains(lowerPath, "/k8s/") {
			pattern = "/k8s/"
		}
	case "code":
		pattern = "default"
	}

	if pattern == "" {
		pattern = "unknown"
	}

	return &DirectoryHint{
		Pattern:         pattern,
		SuggestedDomain: *ruleBasedDomain,
	}
}
