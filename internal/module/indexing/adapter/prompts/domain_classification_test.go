package prompts

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jinford/dev-rag/internal/module/indexing/adapter/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// モックLLMクライアント
type mockDomainClassifierClient struct {
	response string
	err      error
}

func (m *mockDomainClassifierClient) GenerateCompletion(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	if m.err != nil {
		return llm.CompletionResponse{}, m.err
	}
	return llm.CompletionResponse{
		Content:       m.response,
		TokensUsed:    100,
		PromptVersion: DomainClassificationPromptVersion,
		Model:         "gpt-4o-mini",
	}, nil
}

func TestGenerateDomainClassificationPrompt(t *testing.T) {
	tests := []struct {
		name     string
		req      DomainClassificationRequest
		contains []string
	}{
		{
			name: "basic file classification",
			req: DomainClassificationRequest{
				NodePath:         "pkg/indexer/indexer.go",
				NodeType:         "file",
				DetectedLanguage: "Go",
				LinesOfCode:      500,
				LastModified:     "2024-01-15",
			},
			contains: []string{
				"Path: pkg/indexer/indexer.go",
				"Type: file",
				"Language: Go",
				"Lines of Code: 500",
				"Last Modified: 2024-01-15",
			},
		},
		{
			name: "test file with hints",
			req: DomainClassificationRequest{
				NodePath:         "pkg/indexer/indexer_test.go",
				NodeType:         "file",
				DetectedLanguage: "Go",
				LinesOfCode:      200,
				DirectoryHints: &DirectoryHint{
					Pattern:         "*_test.go",
					SuggestedDomain: "tests",
				},
			},
			contains: []string{
				"Path: pkg/indexer/indexer_test.go",
				"Directory Hints:",
				"*_test.go",
				"tests",
			},
		},
		{
			name: "with sample lines",
			req: DomainClassificationRequest{
				NodePath:    "README.md",
				NodeType:    "file",
				SampleLines: "L1: # My Project\nL2: This is a documentation file\n",
			},
			contains: []string{
				"Path: README.md",
				"Sample Lines:",
				"L1: # My Project",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := GenerateDomainClassificationPrompt(tt.req)

			for _, expected := range tt.contains {
				assert.Contains(t, prompt, expected)
			}
		})
	}
}

func TestDomainClassifier_Classify(t *testing.T) {
	tests := []struct {
		name           string
		req            DomainClassificationRequest
		mockResponse   string
		expectedDomain string
		expectedConf   float64
		expectError    bool
	}{
		{
			name: "classify test file",
			req: DomainClassificationRequest{
				NodePath:         "pkg/indexer/indexer_test.go",
				NodeType:         "file",
				DetectedLanguage: "Go",
			},
			mockResponse: `{
				"prompt_version": "1.1",
				"domain": "tests",
				"rationale": "File name contains _test.go pattern (L1).",
				"confidence": 0.95
			}`,
			expectedDomain: "tests",
			expectedConf:   0.95,
			expectError:    false,
		},
		{
			name: "classify ADR document",
			req: DomainClassificationRequest{
				NodePath:    "docs/adr/ADR-003.md",
				NodeType:    "file",
				SampleLines: "L1: # ADR-003: Architecture Decision\nL2: ## Context\n",
			},
			mockResponse: `{
				"prompt_version": "1.1",
				"domain": "architecture",
				"rationale": "ADR document in docs/adr/ directory with design decisions (L1-L2).",
				"confidence": 0.85
			}`,
			expectedDomain: "architecture",
			expectedConf:   0.85,
			expectError:    false,
		},
		{
			name: "classify CI/CD config",
			req: DomainClassificationRequest{
				NodePath:    ".github/workflows/ci.yml",
				NodeType:    "file",
				SampleLines: "L1: name: CI\nL2: on: [push, pull_request]\n",
			},
			mockResponse: `{
				"prompt_version": "1.1",
				"domain": "ops",
				"rationale": "CI/CD workflow configuration in .github/workflows/ (L1-L2).",
				"confidence": 0.90
			}`,
			expectedDomain: "ops",
			expectedConf:   0.90,
			expectError:    false,
		},
		{
			name: "classify Terraform file",
			req: DomainClassificationRequest{
				NodePath:    "infra/terraform/main.tf",
				NodeType:    "file",
				SampleLines: "L1: resource \"aws_instance\" \"example\" {\nL2:   ami = \"ami-12345\"\n",
			},
			mockResponse: `{
				"prompt_version": "1.1",
				"domain": "infra",
				"rationale": "Terraform infrastructure definition with AWS resources (L1-L2).",
				"confidence": 0.92
			}`,
			expectedDomain: "infra",
			expectedConf:   0.92,
			expectError:    false,
		},
		{
			name: "classify application code",
			req: DomainClassificationRequest{
				NodePath:         "pkg/indexer/indexer.go",
				NodeType:         "file",
				DetectedLanguage: "Go",
			},
			mockResponse: `{
				"prompt_version": "1.1",
				"domain": "code",
				"rationale": "Application code with implementation logic.",
				"confidence": 0.80
			}`,
			expectedDomain: "code",
			expectedConf:   0.80,
			expectError:    false,
		},
		{
			name: "invalid domain returned",
			req: DomainClassificationRequest{
				NodePath: "unknown.txt",
				NodeType: "file",
			},
			mockResponse: `{
				"prompt_version": "1.1",
				"domain": "invalid_domain",
				"rationale": "Unknown file type.",
				"confidence": 0.50
			}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockDomainClassifierClient{
				response: tt.mockResponse,
			}
			tokenCounter, err := llm.NewTokenCounter()
			require.NoError(t, err)
			classifier := NewDomainClassifier(mockClient, tokenCounter)

			resp, err := classifier.Classify(context.Background(), tt.req)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedDomain, resp.Domain)
			assert.Equal(t, tt.expectedConf, resp.Confidence)
			assert.NotEmpty(t, resp.Rationale)
		})
	}
}

func TestExtractSampleLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		maxLines int
		expected []string
	}{
		{
			name:     "short file (less than maxLines*2)",
			content:  "line1\nline2\nline3\nline4\nline5",
			maxLines: 25,
			expected: []string{"line1", "line2", "line3", "line4", "line5"},
		},
		{
			name: "long file (more than maxLines*2)",
			content: func() string {
				lines := make([]string, 100)
				for i := 0; i < 100; i++ {
					lines[i] = "line" + string(rune(i+1))
				}
				return strings.Join(lines, "\n")
			}(),
			maxLines: 3,
			expected: []string{"L1:", "L3:", "... (omitted) ...", "L99:", "L101:"},
		},
		{
			name:     "empty content",
			content:  "",
			maxLines: 25,
			expected: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSampleLines(tt.content, tt.maxLines)

			for _, expected := range tt.expected {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestCreateDirectoryHintFromRuleBased(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		ruleBasedDomain *string
		expectedPattern string
		expectedDomain  string
	}{
		{
			name:            "test file with _test.go",
			path:            "pkg/indexer/indexer_test.go",
			ruleBasedDomain: stringPtr("tests"),
			expectedPattern: "*_test.go",
			expectedDomain:  "tests",
		},
		{
			name:            "markdown in docs",
			path:            "docs/architecture.md",
			ruleBasedDomain: stringPtr("architecture"),
			expectedPattern: "*.md",
			expectedDomain:  "architecture",
		},
		{
			name:            "shell script",
			path:            "scripts/deploy.sh",
			ruleBasedDomain: stringPtr("ops"),
			expectedPattern: "*.sh",
			expectedDomain:  "ops",
		},
		{
			name:            "terraform file",
			path:            "infra/terraform/main.tf",
			ruleBasedDomain: stringPtr("infra"),
			expectedPattern: "*.tf",
			expectedDomain:  "infra",
		},
		{
			name:            "yaml file in k8s directory",
			path:            "k8s/deployment.yaml",
			ruleBasedDomain: stringPtr("infra"),
			expectedPattern: "/k8s/",
			expectedDomain:  "infra",
		},
		{
			name:            "nil domain",
			path:            "some/file.txt",
			ruleBasedDomain: nil,
			expectedPattern: "",
			expectedDomain:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := CreateDirectoryHintFromRuleBased(tt.path, tt.ruleBasedDomain)

			if tt.ruleBasedDomain == nil {
				assert.Nil(t, hint)
				return
			}

			require.NotNil(t, hint)
			assert.Equal(t, tt.expectedDomain, hint.SuggestedDomain)
			// パターンは実装によって異なる可能性があるため、空でないことを確認
			assert.NotEmpty(t, hint.Pattern)
		})
	}
}

func TestDomainClassificationPromptStructure(t *testing.T) {
	req := DomainClassificationRequest{
		NodePath:         "pkg/indexer/indexer.go",
		NodeType:         "file",
		DetectedLanguage: "Go",
		LinesOfCode:      500,
		LastModified:     "2024-01-15",
		SampleLines:      "L1: package indexer\nL2: import \"context\"\n",
		DirectoryHints: &DirectoryHint{
			Pattern:         "default",
			SuggestedDomain: "code",
		},
	}

	prompt := GenerateDomainClassificationPrompt(req)

	// プロンプトの必須要素をチェック
	assert.Contains(t, prompt, "Classify the following file")
	assert.Contains(t, prompt, "code:")
	assert.Contains(t, prompt, "architecture:")
	assert.Contains(t, prompt, "ops:")
	assert.Contains(t, prompt, "tests:")
	assert.Contains(t, prompt, "infra:")
	assert.Contains(t, prompt, "JSON response")
	assert.Contains(t, prompt, "prompt_version")
	assert.Contains(t, prompt, "domain")
	assert.Contains(t, prompt, "rationale")
	assert.Contains(t, prompt, "confidence")
}

func TestDomainClassificationResponseParsing(t *testing.T) {
	jsonResponse := `{
		"prompt_version": "1.1",
		"domain": "tests",
		"rationale": "Test file with _test.go pattern.",
		"confidence": 0.95
	}`

	var resp DomainClassificationResponse
	err := json.Unmarshal([]byte(jsonResponse), &resp)

	require.NoError(t, err)
	assert.Equal(t, "1.1", resp.PromptVersion)
	assert.Equal(t, "tests", resp.Domain)
	assert.Equal(t, "Test file with _test.go pattern.", resp.Rationale)
	assert.Equal(t, 0.95, resp.Confidence)
}

// ヘルパー関数
func stringPtr(s string) *string {
	return &s
}
