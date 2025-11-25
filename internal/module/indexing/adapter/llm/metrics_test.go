package llm

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLLMMetrics(t *testing.T) {
	metrics := NewLLMMetrics()
	require.NotNil(t, metrics)
	assert.NotZero(t, metrics.startTime)
}

func TestLLMMetrics_RecordRequest_Success(t *testing.T) {
	metrics := NewLLMMetrics()

	metric := RequestMetric{
		Model:       "gpt-4o-mini",
		RequestType: "summarization",
		Usage: TokenUsage{
			PromptTokens:   1000,
			ResponseTokens: 500,
			TotalTokens:    1500,
		},
		Cost:    0.00045,
		Latency: 2 * time.Second,
		Success: true,
	}

	metrics.RecordRequest(metric)

	snapshot := metrics.GetSnapshot()
	assert.Equal(t, 1, snapshot.TotalAPIRequests)
	assert.Equal(t, 1, snapshot.SuccessfulRequests)
	assert.Equal(t, 0, snapshot.FailedRequests)
	assert.Equal(t, 1500, snapshot.TotalTokens)
	assert.Equal(t, 1000, snapshot.PromptTokens)
	assert.Equal(t, 500, snapshot.ResponseTokens)
	assert.InDelta(t, 0.00045, snapshot.TotalCost, 0.0001)
	assert.Equal(t, 2*time.Second, snapshot.AverageLatency)
	assert.Equal(t, 100.0, snapshot.SuccessRate)
}

func TestLLMMetrics_RecordRequest_Failure(t *testing.T) {
	metrics := NewLLMMetrics()

	metric := RequestMetric{
		Model:       "gpt-4o-mini",
		RequestType: "summarization",
		Latency:     1 * time.Second,
		Success:     false,
		ErrorType:   "rate_limit",
	}

	metrics.RecordRequest(metric)

	snapshot := metrics.GetSnapshot()
	assert.Equal(t, 1, snapshot.TotalAPIRequests)
	assert.Equal(t, 0, snapshot.SuccessfulRequests)
	assert.Equal(t, 1, snapshot.FailedRequests)
	assert.Equal(t, 0, snapshot.TotalTokens)
	assert.Equal(t, 0.0, snapshot.TotalCost)
	assert.Equal(t, 0.0, snapshot.SuccessRate)
	assert.Equal(t, 1, snapshot.ErrorsByType["rate_limit"])
	assert.Equal(t, 1, snapshot.ErrorsByModel["gpt-4o-mini"])
}

func TestLLMMetrics_RecordRequest_Retry(t *testing.T) {
	metrics := NewLLMMetrics()

	metric := RequestMetric{
		Model:       "gpt-4o",
		RequestType: "classification",
		Usage:       TokenUsage{TotalTokens: 500},
		Cost:        0.001,
		Latency:     3 * time.Second,
		Success:     true,
		Retried:     true,
	}

	metrics.RecordRequest(metric)

	snapshot := metrics.GetSnapshot()
	assert.Equal(t, 1, snapshot.TotalAPIRequests)
	assert.Equal(t, 1, snapshot.RetriedRequests)
}

func TestLLMMetrics_MultipleRequests(t *testing.T) {
	metrics := NewLLMMetrics()

	// 成功リクエスト1
	metrics.RecordRequest(RequestMetric{
		Model:       "gpt-4o-mini",
		RequestType: "summarization",
		Usage:       TokenUsage{PromptTokens: 1000, ResponseTokens: 500, TotalTokens: 1500},
		Cost:        0.00045,
		Latency:     2 * time.Second,
		Success:     true,
	})

	// 成功リクエスト2
	metrics.RecordRequest(RequestMetric{
		Model:       "gpt-4o",
		RequestType: "classification",
		Usage:       TokenUsage{PromptTokens: 500, ResponseTokens: 200, TotalTokens: 700},
		Cost:        0.003,
		Latency:     1 * time.Second,
		Success:     true,
	})

	// 失敗リクエスト
	metrics.RecordRequest(RequestMetric{
		Model:       "gpt-4o-mini",
		RequestType: "summarization",
		Latency:     500 * time.Millisecond,
		Success:     false,
		ErrorType:   "timeout",
	})

	snapshot := metrics.GetSnapshot()
	assert.Equal(t, 3, snapshot.TotalAPIRequests)
	assert.Equal(t, 2, snapshot.SuccessfulRequests)
	assert.Equal(t, 1, snapshot.FailedRequests)
	assert.InDelta(t, 66.67, snapshot.SuccessRate, 0.1)
	assert.Equal(t, 2200, snapshot.TotalTokens)
	assert.InDelta(t, 0.00345, snapshot.TotalCost, 0.0001)

	// モデル別の統計
	assert.Equal(t, 2, snapshot.RequestsByModel["gpt-4o-mini"])
	assert.Equal(t, 1, snapshot.RequestsByModel["gpt-4o"])

	// タイプ別の統計
	assert.Equal(t, 2, snapshot.RequestsByType["summarization"])
	assert.Equal(t, 1, snapshot.RequestsByType["classification"])

	// トークン統計
	assert.Equal(t, 1500, snapshot.TokensByModel["gpt-4o-mini"].TotalTokens)
	assert.Equal(t, 700, snapshot.TokensByModel["gpt-4o"].TotalTokens)
	assert.Equal(t, 1500, snapshot.TokensByType["summarization"].TotalTokens)
	assert.Equal(t, 700, snapshot.TokensByType["classification"].TotalTokens)

	// コスト統計
	assert.InDelta(t, 0.00045, snapshot.CostsByModel["gpt-4o-mini"], 0.0001)
	assert.InDelta(t, 0.003, snapshot.CostsByModel["gpt-4o"], 0.0001)
}

func TestLLMMetrics_LatencyStats(t *testing.T) {
	metrics := NewLLMMetrics()

	// 複数のリクエストを記録（レイテンシが異なる）
	latencies := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond,
	}

	for _, latency := range latencies {
		metrics.RecordRequest(RequestMetric{
			Model:       "gpt-4o-mini",
			RequestType: "test",
			Usage:       TokenUsage{TotalTokens: 100},
			Cost:        0.0001,
			Latency:     latency,
			Success:     true,
		})
	}

	snapshot := metrics.GetSnapshot()
	assert.Equal(t, 300*time.Millisecond, snapshot.AverageLatency)

	// モデル別レイテンシ統計
	modelStat := snapshot.LatencyByModel["gpt-4o-mini"]
	assert.Equal(t, 100*time.Millisecond, modelStat.Min)
	assert.Equal(t, 500*time.Millisecond, modelStat.Max)
	assert.Equal(t, 300*time.Millisecond, modelStat.Average)
	assert.Equal(t, 300*time.Millisecond, modelStat.P50)
}

func TestLLMMetrics_ExportJSON(t *testing.T) {
	metrics := NewLLMMetrics()

	metrics.RecordRequest(RequestMetric{
		Model:       "gpt-4o-mini",
		RequestType: "summarization",
		Usage:       TokenUsage{PromptTokens: 1000, ResponseTokens: 500, TotalTokens: 1500},
		Cost:        0.00045,
		Latency:     2 * time.Second,
		Success:     true,
	})

	data, err := metrics.ExportJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// JSONが有効かチェック
	var snapshot MetricsSnapshot
	err = json.Unmarshal(data, &snapshot)
	require.NoError(t, err)
	assert.Equal(t, 1, snapshot.TotalAPIRequests)
	assert.Equal(t, 1500, snapshot.TotalTokens)
}

func TestLLMMetrics_Reset(t *testing.T) {
	metrics := NewLLMMetrics()

	// データを記録
	metrics.RecordRequest(RequestMetric{
		Model:       "gpt-4o-mini",
		RequestType: "test",
		Usage:       TokenUsage{TotalTokens: 1000},
		Cost:        0.001,
		Latency:     1 * time.Second,
		Success:     true,
	})

	snapshot := metrics.GetSnapshot()
	assert.Equal(t, 1, snapshot.TotalAPIRequests)

	// リセット
	metrics.Reset()

	snapshot = metrics.GetSnapshot()
	assert.Equal(t, 0, snapshot.TotalAPIRequests)
	assert.Equal(t, 0, snapshot.TotalTokens)
	assert.Equal(t, 0.0, snapshot.TotalCost)
	assert.Empty(t, snapshot.RequestsByType)
	assert.Empty(t, snapshot.RequestsByModel)
}

func TestLLMMetrics_ConcurrentRecording(t *testing.T) {
	metrics := NewLLMMetrics()

	// 並行で記録
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			metrics.RecordRequest(RequestMetric{
				Model:       "gpt-4o-mini",
				RequestType: "concurrent",
				Usage:       TokenUsage{TotalTokens: 100},
				Cost:        0.0001,
				Latency:     100 * time.Millisecond,
				Success:     true,
			})
			done <- true
		}()
	}

	// 全ての goroutine の完了を待つ
	for i := 0; i < 10; i++ {
		<-done
	}

	snapshot := metrics.GetSnapshot()
	assert.Equal(t, 10, snapshot.TotalAPIRequests)
	assert.Equal(t, 1000, snapshot.TotalTokens)
}

func TestLLMMetrics_PrintSummary(t *testing.T) {
	metrics := NewLLMMetrics()

	metrics.RecordRequest(RequestMetric{
		Model:       "gpt-4o-mini",
		RequestType: "summarization",
		Usage:       TokenUsage{PromptTokens: 1000, ResponseTokens: 500, TotalTokens: 1500},
		Cost:        0.00045,
		Latency:     2 * time.Second,
		Success:     true,
	})

	// PrintSummary() should not panic
	assert.NotPanics(t, func() {
		metrics.PrintSummary()
	})
}

func TestCalculateLatencyStat(t *testing.T) {
	tests := []struct {
		name      string
		latencies []time.Duration
		expected  LatencyStat
	}{
		{
			name:      "empty",
			latencies: []time.Duration{},
			expected:  LatencyStat{},
		},
		{
			name:      "single value",
			latencies: []time.Duration{100 * time.Millisecond},
			expected: LatencyStat{
				Min:     100 * time.Millisecond,
				Max:     100 * time.Millisecond,
				Average: 100 * time.Millisecond,
				P50:     100 * time.Millisecond,
				P95:     100 * time.Millisecond,
				P99:     100 * time.Millisecond,
			},
		},
		{
			name: "multiple values",
			latencies: []time.Duration{
				100 * time.Millisecond,
				200 * time.Millisecond,
				300 * time.Millisecond,
			},
			expected: LatencyStat{
				Min:     100 * time.Millisecond,
				Max:     300 * time.Millisecond,
				Average: 200 * time.Millisecond,
				P50:     200 * time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateLatencyStat(tt.latencies)
			assert.Equal(t, tt.expected.Min, result.Min)
			assert.Equal(t, tt.expected.Max, result.Max)
			if len(tt.latencies) > 0 {
				assert.Equal(t, tt.expected.Average, result.Average)
			}
		})
	}
}

func BenchmarkLLMMetrics_RecordRequest(b *testing.B) {
	metrics := NewLLMMetrics()
	metric := RequestMetric{
		Model:       "gpt-4o-mini",
		RequestType: "test",
		Usage:       TokenUsage{TotalTokens: 1000},
		Cost:        0.001,
		Latency:     100 * time.Millisecond,
		Success:     true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordRequest(metric)
	}
}
