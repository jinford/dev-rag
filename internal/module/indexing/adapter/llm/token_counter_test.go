package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenCounter(t *testing.T) {
	counter, err := NewTokenCounter()
	require.NoError(t, err)
	require.NotNil(t, counter)
	require.NotNil(t, counter.encoding)
}

func TestTokenCounter_CountTokens(t *testing.T) {
	counter, err := NewTokenCounter()
	require.NoError(t, err)

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "simple english",
			text:     "Hello, World!",
			expected: 4,
		},
		{
			name:     "longer text",
			text:     "This is a test sentence with multiple words.",
			expected: 9,
		},
		{
			name:     "japanese text",
			text:     "これはテストです",
			expected: 5,
		},
		{
			name:     "code snippet",
			text:     "func main() { fmt.Println(\"hello\") }",
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := counter.CountTokens(tt.text)
			assert.Equal(t, tt.expected, count, "token count mismatch for: %s", tt.text)
		})
	}
}

func TestTokenCounter_CountPromptAndResponse(t *testing.T) {
	counter, err := NewTokenCounter()
	require.NoError(t, err)

	prompt := "Summarize the following code:"
	response := "This code prints hello world."

	usage := counter.CountPromptAndResponse(prompt, response)

	assert.Greater(t, usage.PromptTokens, 0, "prompt tokens should be > 0")
	assert.Greater(t, usage.ResponseTokens, 0, "response tokens should be > 0")
	assert.Equal(t, usage.PromptTokens+usage.ResponseTokens, usage.TotalTokens, "total should equal sum")
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		minCount int
		maxCount int
	}{
		{
			name:     "empty string",
			text:     "",
			minCount: 0,
			maxCount: 0,
		},
		{
			name:     "short text",
			text:     "Hello",
			minCount: 1,
			maxCount: 2,
		},
		{
			name:     "medium text",
			text:     "This is a test sentence.",
			minCount: 7,
			maxCount: 9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := EstimateTokens(tt.text)
			assert.GreaterOrEqual(t, count, tt.minCount, "count should be >= minCount")
			assert.LessOrEqual(t, count, tt.maxCount, "count should be <= maxCount")
		})
	}
}

func TestTokenCounter_NilEncoding(t *testing.T) {
	// エンコーディングがnilの場合の動作をテスト
	counter := &TokenCounter{encoding: nil}
	count := counter.CountTokens("test")
	assert.Equal(t, 0, count, "should return 0 when encoding is nil")
}

func BenchmarkTokenCounter_CountTokens(b *testing.B) {
	counter, err := NewTokenCounter()
	require.NoError(b, err)

	text := "This is a test sentence with multiple words to benchmark token counting performance."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = counter.CountTokens(text)
	}
}
