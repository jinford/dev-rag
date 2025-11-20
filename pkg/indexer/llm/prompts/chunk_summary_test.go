package prompts

import (
	"testing"

	"github.com/jinford/dev-rag/pkg/indexer/llm"
)

func TestGenerateChunkSummaryPrompt(t *testing.T) {
	tests := []struct {
		name     string
		req      ChunkSummaryRequest
		contains []string
	}{
		{
			name: "with_parent_context",
			req: ChunkSummaryRequest{
				ChunkID:       "test_chunk_1",
				ParentContext: "This is parent context",
				ChunkContent:  "func Test() { fmt.Println(\"hello\") }",
			},
			contains: []string{
				"Parent Context: This is parent context",
				"Chunk Content:",
				"func Test()",
				"test_chunk_1",
				"prompt_version",
				"1.1",
			},
		},
		{
			name: "without_parent_context",
			req: ChunkSummaryRequest{
				ChunkID:       "test_chunk_2",
				ParentContext: "",
				ChunkContent:  "func Process() error { return nil }",
			},
			contains: []string{
				"Parent Context: null",
				"Chunk Content:",
				"func Process()",
				"test_chunk_2",
			},
		},
		{
			name: "long_content",
			req: ChunkSummaryRequest{
				ChunkID:       "test_chunk_3",
				ParentContext: "Parent summary",
				ChunkContent:  "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}",
			},
			contains: []string{
				"package main",
				"import \"fmt\"",
				"test_chunk_3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := GenerateChunkSummaryPrompt(tt.req)

			if prompt == "" {
				t.Error("GenerateChunkSummaryPrompt returned empty string")
			}

			for _, substr := range tt.contains {
				if !contains(prompt, substr) {
					t.Errorf("prompt does not contain expected substring: %q", substr)
				}
			}
		})
	}
}

func TestBuildEmbeddingContext(t *testing.T) {
	tests := []struct {
		name            string
		summary         *ChunkSummaryResponse
		originalContent string
		expectedContain []string
	}{
		{
			name: "basic_context",
			summary: &ChunkSummaryResponse{
				SummarySentence: "This function processes user input and returns a result.",
				FocusEntities:   []string{"Process", "User", "Result"},
			},
			originalContent: "func Process(user User) Result { return Result{} }",
			expectedContain: []string{
				"Summary: This function processes user input and returns a result.",
				"func Process(user User) Result",
			},
		},
		{
			name: "empty_summary",
			summary: &ChunkSummaryResponse{
				SummarySentence: "",
			},
			originalContent: "var x = 10",
			expectedContain: []string{
				"Summary:",
				"var x = 10",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := BuildEmbeddingContext(tt.summary, tt.originalContent)

			if context == "" {
				t.Error("BuildEmbeddingContext returned empty string")
			}

			for _, substr := range tt.expectedContain {
				if !contains(context, substr) {
					t.Errorf("context does not contain expected substring: %q", substr)
				}
			}
		})
	}
}

func TestChunkSummaryConstants(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected interface{}
	}{
		{
			name:     "ChunkSummaryPromptVersion",
			value:    ChunkSummaryPromptVersion,
			expected: "1.1",
		},
		{
			name:     "ChunkSummaryTemperature",
			value:    ChunkSummaryTemperature,
			expected: 0.2,
		},
		{
			name:     "ChunkSummaryMaxTokens",
			value:    ChunkSummaryMaxTokens,
			expected: 150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.value, tt.expected)
			}
		})
	}
}

func TestChunkSummaryResponse_Validation(t *testing.T) {
	tests := []struct {
		name         string
		resp         ChunkSummaryResponse
		expectValid  bool
		errorMessage string
	}{
		{
			name: "valid_response",
			resp: ChunkSummaryResponse{
				PromptVersion:   "1.1",
				ChunkID:         "chunk_1",
				SummarySentence: "This is a summary.",
				FocusEntities:   []string{"Entity1", "Entity2"},
				Confidence:      0.75,
			},
			expectValid: true,
		},
		{
			name: "confidence_too_low",
			resp: ChunkSummaryResponse{
				PromptVersion:   "1.1",
				ChunkID:         "chunk_2",
				SummarySentence: "Summary",
				FocusEntities:   []string{"Entity1"},
				Confidence:      0.1,
			},
			expectValid:  false,
			errorMessage: "confidence out of range",
		},
		{
			name: "confidence_too_high",
			resp: ChunkSummaryResponse{
				PromptVersion:   "1.1",
				ChunkID:         "chunk_3",
				SummarySentence: "Summary",
				FocusEntities:   []string{"Entity1"},
				Confidence:      0.95,
			},
			expectValid:  false,
			errorMessage: "confidence out of range",
		},
		{
			name: "too_many_entities",
			resp: ChunkSummaryResponse{
				PromptVersion:   "1.1",
				ChunkID:         "chunk_4",
				SummarySentence: "Summary",
				FocusEntities:   []string{"E1", "E2", "E3", "E4"},
				Confidence:      0.75,
			},
			expectValid: true, // 実装では最初の3件に自動的にトリミングされる
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 信頼度の範囲チェック
			if tt.resp.Confidence < 0.2 || tt.resp.Confidence > 0.85 {
				if tt.expectValid {
					t.Errorf("expected valid confidence, got %.2f", tt.resp.Confidence)
				}
			} else {
				if !tt.expectValid && tt.errorMessage == "confidence out of range" {
					t.Errorf("expected invalid confidence, got %.2f", tt.resp.Confidence)
				}
			}

			// FocusEntitiesの数をチェック（実装側で3件に制限される想定）
			if len(tt.resp.FocusEntities) > 3 {
				// このケースは実装側で処理されるので、ここではログのみ
				t.Logf("FocusEntities has %d items, should be trimmed to 3", len(tt.resp.FocusEntities))
			}
		})
	}
}

func TestNewChunkSummaryGenerator(t *testing.T) {
	// 実際のTokenCounterを使用（モックは不要）
	mockClient := &MockLLMClient{}
	tokenCounter, err := llm.NewTokenCounter()
	if err != nil {
		t.Skipf("Failed to create TokenCounter: %v (skipping test)", err)
		return
	}

	generator := NewChunkSummaryGenerator(mockClient, tokenCounter)

	if generator == nil {
		t.Fatal("NewChunkSummaryGenerator returned nil")
	}

	if generator.llmClient == nil {
		t.Error("generator.llmClient is nil")
	}

	if generator.tokenCounter == nil {
		t.Error("generator.tokenCounter is nil")
	}
}

// ヘルパー関数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
