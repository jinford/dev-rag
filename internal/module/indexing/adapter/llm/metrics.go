package llm

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// LLMMetrics はLLM APIの使用状況メトリクスを管理する
type LLMMetrics struct {
	mu sync.RWMutex

	// APIコール回数
	totalAPIRequests     int
	requestsByType       map[string]int
	requestsByModel      map[string]int
	successfulRequests   int
	failedRequests       int
	retriedRequests      int

	// トークン使用量
	totalTokens      int
	promptTokens     int
	responseTokens   int
	tokensByModel    map[string]TokenUsage
	tokensByType     map[string]TokenUsage

	// コスト
	totalCost        float64
	costsByModel     map[string]float64
	costsByType      map[string]float64

	// レイテンシ
	totalLatency     time.Duration
	latencyByModel   map[string][]time.Duration
	latencyByType    map[string][]time.Duration

	// エラー
	errorsByType     map[string]int
	errorsByModel    map[string]int

	// タイムスタンプ
	startTime        time.Time
	lastRequestTime  time.Time
}

// NewLLMMetrics は新しいLLMMetricsを作成する
func NewLLMMetrics() *LLMMetrics {
	return &LLMMetrics{
		requestsByType:   make(map[string]int),
		requestsByModel:  make(map[string]int),
		tokensByModel:    make(map[string]TokenUsage),
		tokensByType:     make(map[string]TokenUsage),
		costsByModel:     make(map[string]float64),
		costsByType:      make(map[string]float64),
		latencyByModel:   make(map[string][]time.Duration),
		latencyByType:    make(map[string][]time.Duration),
		errorsByType:     make(map[string]int),
		errorsByModel:    make(map[string]int),
		startTime:        time.Now(),
	}
}

// RequestMetric は単一のリクエストのメトリクス
type RequestMetric struct {
	Model         string
	RequestType   string
	Usage         TokenUsage
	Cost          float64
	Latency       time.Duration
	Success       bool
	Retried       bool
	ErrorType     string
}

// RecordRequest はリクエストのメトリクスを記録する
func (m *LLMMetrics) RecordRequest(metric RequestMetric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// APIコール回数
	m.totalAPIRequests++
	m.requestsByType[metric.RequestType]++
	m.requestsByModel[metric.Model]++
	m.lastRequestTime = time.Now()

	if metric.Success {
		m.successfulRequests++
	} else {
		m.failedRequests++
		m.errorsByType[metric.ErrorType]++
		m.errorsByModel[metric.Model]++
	}

	if metric.Retried {
		m.retriedRequests++
	}

	// トークン使用量
	if metric.Success {
		m.totalTokens += metric.Usage.TotalTokens
		m.promptTokens += metric.Usage.PromptTokens
		m.responseTokens += metric.Usage.ResponseTokens

		// モデル別トークン
		modelUsage := m.tokensByModel[metric.Model]
		modelUsage.PromptTokens += metric.Usage.PromptTokens
		modelUsage.ResponseTokens += metric.Usage.ResponseTokens
		modelUsage.TotalTokens += metric.Usage.TotalTokens
		m.tokensByModel[metric.Model] = modelUsage

		// タイプ別トークン
		typeUsage := m.tokensByType[metric.RequestType]
		typeUsage.PromptTokens += metric.Usage.PromptTokens
		typeUsage.ResponseTokens += metric.Usage.ResponseTokens
		typeUsage.TotalTokens += metric.Usage.TotalTokens
		m.tokensByType[metric.RequestType] = typeUsage

		// コスト
		m.totalCost += metric.Cost
		m.costsByModel[metric.Model] += metric.Cost
		m.costsByType[metric.RequestType] += metric.Cost
	}

	// レイテンシ
	m.totalLatency += metric.Latency
	m.latencyByModel[metric.Model] = append(m.latencyByModel[metric.Model], metric.Latency)
	m.latencyByType[metric.RequestType] = append(m.latencyByType[metric.RequestType], metric.Latency)
}

// MetricsSnapshot はメトリクスのスナップショット
type MetricsSnapshot struct {
	// タイムスタンプ
	CapturedAt      time.Time     `json:"captured_at"`
	StartTime       time.Time     `json:"start_time"`
	ElapsedTime     time.Duration `json:"elapsed_time"`
	LastRequestTime time.Time     `json:"last_request_time"`

	// APIコール回数
	TotalAPIRequests   int            `json:"total_api_requests"`
	SuccessfulRequests int            `json:"successful_requests"`
	FailedRequests     int            `json:"failed_requests"`
	RetriedRequests    int            `json:"retried_requests"`
	SuccessRate        float64        `json:"success_rate"`
	RequestsByType     map[string]int `json:"requests_by_type"`
	RequestsByModel    map[string]int `json:"requests_by_model"`

	// トークン使用量
	TotalTokens       int                    `json:"total_tokens"`
	PromptTokens      int                    `json:"prompt_tokens"`
	ResponseTokens    int                    `json:"response_tokens"`
	TokensByModel     map[string]TokenUsage  `json:"tokens_by_model"`
	TokensByType      map[string]TokenUsage  `json:"tokens_by_type"`

	// コスト
	TotalCost       float64            `json:"total_cost"`
	CostsByModel    map[string]float64 `json:"costs_by_model"`
	CostsByType     map[string]float64 `json:"costs_by_type"`

	// レイテンシ
	AverageLatency       time.Duration          `json:"average_latency"`
	LatencyByModel       map[string]LatencyStat `json:"latency_by_model"`
	LatencyByType        map[string]LatencyStat `json:"latency_by_type"`

	// エラー
	ErrorsByType  map[string]int `json:"errors_by_type"`
	ErrorsByModel map[string]int `json:"errors_by_model"`
}

// LatencyStat はレイテンシの統計情報
type LatencyStat struct {
	Min     time.Duration `json:"min"`
	Max     time.Duration `json:"max"`
	Average time.Duration `json:"average"`
	P50     time.Duration `json:"p50"`
	P95     time.Duration `json:"p95"`
	P99     time.Duration `json:"p99"`
}

// GetSnapshot は現在のメトリクスのスナップショットを返す
func (m *LLMMetrics) GetSnapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := MetricsSnapshot{
		CapturedAt:         time.Now(),
		StartTime:          m.startTime,
		ElapsedTime:        time.Since(m.startTime),
		LastRequestTime:    m.lastRequestTime,
		TotalAPIRequests:   m.totalAPIRequests,
		SuccessfulRequests: m.successfulRequests,
		FailedRequests:     m.failedRequests,
		RetriedRequests:    m.retriedRequests,
		TotalTokens:        m.totalTokens,
		PromptTokens:       m.promptTokens,
		ResponseTokens:     m.responseTokens,
		TotalCost:          m.totalCost,
	}

	// 成功率の計算
	if m.totalAPIRequests > 0 {
		snapshot.SuccessRate = float64(m.successfulRequests) / float64(m.totalAPIRequests) * 100.0
	}

	// 平均レイテンシの計算
	if m.successfulRequests > 0 {
		snapshot.AverageLatency = m.totalLatency / time.Duration(m.successfulRequests)
	}

	// マップのコピー
	snapshot.RequestsByType = copyIntMap(m.requestsByType)
	snapshot.RequestsByModel = copyIntMap(m.requestsByModel)
	snapshot.TokensByModel = copyTokenUsageMap(m.tokensByModel)
	snapshot.TokensByType = copyTokenUsageMap(m.tokensByType)
	snapshot.CostsByModel = copyFloat64Map(m.costsByModel)
	snapshot.CostsByType = copyFloat64Map(m.costsByType)
	snapshot.ErrorsByType = copyIntMap(m.errorsByType)
	snapshot.ErrorsByModel = copyIntMap(m.errorsByModel)

	// レイテンシ統計の計算
	snapshot.LatencyByModel = make(map[string]LatencyStat)
	for model, latencies := range m.latencyByModel {
		snapshot.LatencyByModel[model] = calculateLatencyStat(latencies)
	}

	snapshot.LatencyByType = make(map[string]LatencyStat)
	for reqType, latencies := range m.latencyByType {
		snapshot.LatencyByType[reqType] = calculateLatencyStat(latencies)
	}

	return snapshot
}

// ExportJSON はメトリクスをJSON形式でエクスポートする
func (m *LLMMetrics) ExportJSON() ([]byte, error) {
	snapshot := m.GetSnapshot()
	return json.MarshalIndent(snapshot, "", "  ")
}

// Reset はメトリクスをリセットする
func (m *LLMMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalAPIRequests = 0
	m.requestsByType = make(map[string]int)
	m.requestsByModel = make(map[string]int)
	m.successfulRequests = 0
	m.failedRequests = 0
	m.retriedRequests = 0
	m.totalTokens = 0
	m.promptTokens = 0
	m.responseTokens = 0
	m.tokensByModel = make(map[string]TokenUsage)
	m.tokensByType = make(map[string]TokenUsage)
	m.totalCost = 0
	m.costsByModel = make(map[string]float64)
	m.costsByType = make(map[string]float64)
	m.totalLatency = 0
	m.latencyByModel = make(map[string][]time.Duration)
	m.latencyByType = make(map[string][]time.Duration)
	m.errorsByType = make(map[string]int)
	m.errorsByModel = make(map[string]int)
	m.startTime = time.Now()
	m.lastRequestTime = time.Time{}
}

// PrintSummary は簡潔なサマリーを標準出力に表示する
func (m *LLMMetrics) PrintSummary() {
	snapshot := m.GetSnapshot()

	fmt.Println("\n=== LLM API Metrics Summary ===")
	fmt.Printf("Period: %s to %s (elapsed: %s)\n",
		snapshot.StartTime.Format("2006-01-02 15:04:05"),
		snapshot.CapturedAt.Format("2006-01-02 15:04:05"),
		snapshot.ElapsedTime.Round(time.Second))
	fmt.Printf("\nAPI Calls:\n")
	fmt.Printf("  Total: %d\n", snapshot.TotalAPIRequests)
	fmt.Printf("  Successful: %d (%.2f%%)\n", snapshot.SuccessfulRequests, snapshot.SuccessRate)
	fmt.Printf("  Failed: %d\n", snapshot.FailedRequests)
	fmt.Printf("  Retried: %d\n", snapshot.RetriedRequests)

	fmt.Printf("\nTokens:\n")
	fmt.Printf("  Total: %d\n", snapshot.TotalTokens)
	fmt.Printf("  Prompt: %d\n", snapshot.PromptTokens)
	fmt.Printf("  Response: %d\n", snapshot.ResponseTokens)

	fmt.Printf("\nCost:\n")
	fmt.Printf("  Total: $%.4f\n", snapshot.TotalCost)

	if snapshot.AverageLatency > 0 {
		fmt.Printf("\nLatency:\n")
		fmt.Printf("  Average: %s\n", snapshot.AverageLatency.Round(time.Millisecond))
	}

	fmt.Println("===============================")
}

// ヘルパー関数

func copyIntMap(src map[string]int) map[string]int {
	dst := make(map[string]int)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyFloat64Map(src map[string]float64) map[string]float64 {
	dst := make(map[string]float64)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyTokenUsageMap(src map[string]TokenUsage) map[string]TokenUsage {
	dst := make(map[string]TokenUsage)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func calculateLatencyStat(latencies []time.Duration) LatencyStat {
	if len(latencies) == 0 {
		return LatencyStat{}
	}

	// ソート
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	// 簡易的なソート（バブルソート）
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// 統計値の計算
	stat := LatencyStat{
		Min: sorted[0],
		Max: sorted[len(sorted)-1],
	}

	// 平均
	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}
	stat.Average = sum / time.Duration(len(sorted))

	// パーセンタイル
	stat.P50 = sorted[len(sorted)*50/100]
	if len(sorted) > 1 {
		stat.P95 = sorted[len(sorted)*95/100]
		stat.P99 = sorted[len(sorted)*99/100]
	} else {
		stat.P95 = stat.Max
		stat.P99 = stat.Max
	}

	return stat
}
