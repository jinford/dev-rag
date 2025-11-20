package prompts

import (
	"context"
	"os"
	"testing"

	"github.com/jinford/dev-rag/pkg/indexer/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDomainClassifier_Integration は実際のLLM APIを使用した統合テストです
// 環境変数 OPENAI_API_KEY が設定されている場合のみ実行されます
func TestDomainClassifier_Integration(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	// OpenAIクライアントを作成
	client, err := llm.NewOpenAIClientWithModel("gpt-4o-mini")
	require.NoError(t, err)

	tokenCounter, err := llm.NewTokenCounter()
	require.NoError(t, err)

	classifier := NewDomainClassifier(client, tokenCounter)

	tests := []struct {
		name           string
		req            DomainClassificationRequest
		expectedDomain string
		minConfidence  float64
	}{
		{
			name: "test file classification",
			req: DomainClassificationRequest{
				NodePath:         "pkg/indexer/indexer_test.go",
				NodeType:         "file",
				DetectedLanguage: "Go",
				LinesOfCode:      200,
				SampleLines: `L1: package indexer
L2:
L3: import (
L4:     "testing"
L5:     "github.com/stretchr/testify/assert"
L6: )
L7:
L8: func TestIndexer_Process(t *testing.T) {
L9:     // test implementation
L10: }`,
				DirectoryHints: &DirectoryHint{
					Pattern:         "*_test.go",
					SuggestedDomain: "tests",
				},
			},
			expectedDomain: "tests",
			minConfidence:  0.8,
		},
		{
			name: "ADR document classification",
			req: DomainClassificationRequest{
				NodePath:    "docs/adr/ADR-003.md",
				NodeType:    "file",
				LinesOfCode: 100,
				SampleLines: `L1: # ADR-003: Use PostgreSQL for Vector Storage
L2:
L3: ## Status
L4: Accepted
L5:
L6: ## Context
L7: We need to store vector embeddings for our RAG system.
L8:
L9: ## Decision
L10: We will use PostgreSQL with pgvector extension.`,
			},
			expectedDomain: "architecture",
			minConfidence:  0.7,
		},
		{
			name: "CI/CD workflow classification",
			req: DomainClassificationRequest{
				NodePath:    ".github/workflows/ci.yml",
				NodeType:    "file",
				LinesOfCode: 50,
				SampleLines: `L1: name: CI
L2:
L3: on:
L4:   push:
L5:     branches: [main]
L6:   pull_request:
L7:
L8: jobs:
L9:   test:
L10:     runs-on: ubuntu-latest`,
				DirectoryHints: &DirectoryHint{
					Pattern:         ".github/workflows/",
					SuggestedDomain: "ops",
				},
			},
			expectedDomain: "ops",
			minConfidence:  0.7,
		},
		{
			name: "Terraform infrastructure classification",
			req: DomainClassificationRequest{
				NodePath:         "infra/terraform/main.tf",
				NodeType:         "file",
				DetectedLanguage: "HCL",
				LinesOfCode:      150,
				SampleLines: `L1: resource "aws_instance" "web" {
L2:   ami           = "ami-0c55b159cbfafe1f0"
L3:   instance_type = "t2.micro"
L4:
L5:   tags = {
L6:     Name = "WebServer"
L7:   }
L8: }`,
				DirectoryHints: &DirectoryHint{
					Pattern:         "*.tf",
					SuggestedDomain: "infra",
				},
			},
			expectedDomain: "infra",
			minConfidence:  0.7,
		},
		{
			name: "application code classification",
			req: DomainClassificationRequest{
				NodePath:         "pkg/indexer/indexer.go",
				NodeType:         "file",
				DetectedLanguage: "Go",
				LinesOfCode:      500,
				SampleLines: `L1: package indexer
L2:
L3: import (
L4:     "context"
L5:     "fmt"
L6: )
L7:
L8: // Indexer はファイルをインデックス化します
L9: type Indexer struct {
L10:     db Database`,
				DirectoryHints: &DirectoryHint{
					Pattern:         "default",
					SuggestedDomain: "code",
				},
			},
			expectedDomain: "code",
			minConfidence:  0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			resp, err := classifier.Classify(ctx, tt.req)
			require.NoError(t, err)

			// ドメイン分類が期待通りか確認
			assert.Equal(t, tt.expectedDomain, resp.Domain,
				"Expected domain %s but got %s. Rationale: %s",
				tt.expectedDomain, resp.Domain, resp.Rationale)

			// 信頼度が閾値以上か確認
			assert.GreaterOrEqual(t, resp.Confidence, tt.minConfidence,
				"Confidence %f is below minimum %f", resp.Confidence, tt.minConfidence)

			// レスポンスの基本的な検証
			assert.NotEmpty(t, resp.Rationale, "Rationale should not be empty")
			assert.Equal(t, DomainClassificationPromptVersion, resp.PromptVersion)

			t.Logf("Domain: %s, Confidence: %.2f, Rationale: %s",
				resp.Domain, resp.Confidence, resp.Rationale)
		})
	}
}

// TestDomainClassifier_FallbackToRuleBased は信頼度が低い場合のフォールバック動作をテストします
func TestDomainClassifier_FallbackToRuleBased(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	client, err := llm.NewOpenAIClientWithModel("gpt-4o-mini")
	require.NoError(t, err)

	tokenCounter, err := llm.NewTokenCounter()
	require.NoError(t, err)

	classifier := NewDomainClassifier(client, tokenCounter)

	// 曖昧なファイルで分類を試みる
	req := DomainClassificationRequest{
		NodePath:    "scripts/data.txt",
		NodeType:    "file",
		LinesOfCode: 10,
		SampleLines: "L1: some data\nL2: more data\n",
	}

	ctx := context.Background()
	resp, err := classifier.Classify(ctx, req)
	require.NoError(t, err)

	t.Logf("Ambiguous file classified as: %s (confidence: %.2f)", resp.Domain, resp.Confidence)
	assert.NotEmpty(t, resp.Domain)
	assert.GreaterOrEqual(t, resp.Confidence, 0.2)
}

// TestExtractSampleLinesWithRealFile は実際のファイルからサンプル行を抽出するテストです
func TestExtractSampleLinesWithRealFile(t *testing.T) {
	// テスト用のファイルコンテンツを作成
	content := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

// This is a comment
// Another comment

func helper() {
	// Some logic
}
`

	sampleLines := ExtractSampleLines(content, 5)

	// サンプル行が正しく抽出されているか確認
	assert.Contains(t, sampleLines, "L1: package main")
	assert.Contains(t, sampleLines, "import \"fmt\"")

	t.Logf("Extracted sample lines:\n%s", sampleLines)
}
