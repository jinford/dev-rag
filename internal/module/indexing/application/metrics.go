package application

import (
	"sort"
)

// IndexMetrics はインデックス化処理のメトリクスを収集します
type IndexMetrics struct {
	// AST解析メトリクス
	ASTParseAttempts int // AST解析を試行した回数
	ASTParseSuccesses int // AST解析に成功した回数
	ASTParseFailures  int // AST解析に失敗した回数

	// メタデータ抽出メトリクス
	MetadataExtractAttempts int // メタデータ抽出を試行した回数
	MetadataExtractSuccesses int // メタデータ抽出に成功した回数
	MetadataExtractFailures  int // メタデータ抽出に失敗した回数

	// チャンク品質メトリクス
	HighCommentRatioExcluded int   // コメント比率95%超過で除外されたチャンク数
	CyclomaticComplexities   []int // 循環的複雑度のリスト（分布計算用）
}

// NewIndexMetrics は新しいIndexMetricsを作成します
func NewIndexMetrics() *IndexMetrics {
	return &IndexMetrics{
		CyclomaticComplexities: make([]int, 0),
	}
}

// RecordASTParseAttempt はAST解析の試行を記録します
func (m *IndexMetrics) RecordASTParseAttempt() {
	m.ASTParseAttempts++
}

// RecordASTParseSuccess はAST解析の成功を記録します
func (m *IndexMetrics) RecordASTParseSuccess() {
	m.ASTParseSuccesses++
}

// RecordASTParseFailure はAST解析の失敗を記録します
func (m *IndexMetrics) RecordASTParseFailure() {
	m.ASTParseFailures++
}

// RecordMetadataExtractAttempt はメタデータ抽出の試行を記録します
func (m *IndexMetrics) RecordMetadataExtractAttempt() {
	m.MetadataExtractAttempts++
}

// RecordMetadataExtractSuccess はメタデータ抽出の成功を記録します
func (m *IndexMetrics) RecordMetadataExtractSuccess() {
	m.MetadataExtractSuccesses++
}

// RecordMetadataExtractFailure はメタデータ抽出の失敗を記録します
func (m *IndexMetrics) RecordMetadataExtractFailure() {
	m.MetadataExtractFailures++
}

// RecordHighCommentRatioExcluded はコメント比率95%超過で除外されたチャンクを記録します
func (m *IndexMetrics) RecordHighCommentRatioExcluded() {
	m.HighCommentRatioExcluded++
}

// RecordCyclomaticComplexity は循環的複雑度を記録します
func (m *IndexMetrics) RecordCyclomaticComplexity(complexity int) {
	m.CyclomaticComplexities = append(m.CyclomaticComplexities, complexity)
}

// ASTParseSuccessRate はAST解析の成功率を計算します
func (m *IndexMetrics) ASTParseSuccessRate() float64 {
	if m.ASTParseAttempts == 0 {
		return 0.0
	}
	return float64(m.ASTParseSuccesses) / float64(m.ASTParseAttempts)
}

// ASTParseFailureRate はAST解析の失敗率を計算します
func (m *IndexMetrics) ASTParseFailureRate() float64 {
	if m.ASTParseAttempts == 0 {
		return 0.0
	}
	return float64(m.ASTParseFailures) / float64(m.ASTParseAttempts)
}

// MetadataExtractSuccessRate はメタデータ抽出の成功率を計算します
func (m *IndexMetrics) MetadataExtractSuccessRate() float64 {
	if m.MetadataExtractAttempts == 0 {
		return 0.0
	}
	return float64(m.MetadataExtractSuccesses) / float64(m.MetadataExtractAttempts)
}

// CyclomaticComplexityP50 は循環的複雑度のP50（中央値）を計算します
func (m *IndexMetrics) CyclomaticComplexityP50() int {
	return m.calculatePercentile(50)
}

// CyclomaticComplexityP95 は循環的複雑度のP95を計算します
func (m *IndexMetrics) CyclomaticComplexityP95() int {
	return m.calculatePercentile(95)
}

// CyclomaticComplexityP99 は循環的複雑度のP99を計算します
func (m *IndexMetrics) CyclomaticComplexityP99() int {
	return m.calculatePercentile(99)
}

// calculatePercentile は指定されたパーセンタイルを計算します
func (m *IndexMetrics) calculatePercentile(percentile int) int {
	if len(m.CyclomaticComplexities) == 0 {
		return 0
	}

	// ソート済みのコピーを作成
	sorted := make([]int, len(m.CyclomaticComplexities))
	copy(sorted, m.CyclomaticComplexities)
	sort.Ints(sorted)

	// パーセンタイルのインデックスを計算
	index := int(float64(len(sorted)-1) * float64(percentile) / 100.0)
	return sorted[index]
}

// Merge は他のメトリクスをマージします（並行処理用）
func (m *IndexMetrics) Merge(other *IndexMetrics) {
	m.ASTParseAttempts += other.ASTParseAttempts
	m.ASTParseSuccesses += other.ASTParseSuccesses
	m.ASTParseFailures += other.ASTParseFailures

	m.MetadataExtractAttempts += other.MetadataExtractAttempts
	m.MetadataExtractSuccesses += other.MetadataExtractSuccesses
	m.MetadataExtractFailures += other.MetadataExtractFailures

	m.HighCommentRatioExcluded += other.HighCommentRatioExcluded
	m.CyclomaticComplexities = append(m.CyclomaticComplexities, other.CyclomaticComplexities...)
}
