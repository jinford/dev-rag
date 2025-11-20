package indexer

import (
	"testing"
)

func TestIndexMetrics_ASTParseMetrics(t *testing.T) {
	metrics := NewIndexMetrics()

	// AST解析を3回試行、2回成功、1回失敗
	metrics.RecordASTParseAttempt()
	metrics.RecordASTParseSuccess()

	metrics.RecordASTParseAttempt()
	metrics.RecordASTParseSuccess()

	metrics.RecordASTParseAttempt()
	metrics.RecordASTParseFailure()

	if metrics.ASTParseAttempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", metrics.ASTParseAttempts)
	}

	if metrics.ASTParseSuccesses != 2 {
		t.Errorf("Expected 2 successes, got %d", metrics.ASTParseSuccesses)
	}

	if metrics.ASTParseFailures != 1 {
		t.Errorf("Expected 1 failure, got %d", metrics.ASTParseFailures)
	}

	successRate := metrics.ASTParseSuccessRate()
	if successRate < 0.66 || successRate > 0.67 {
		t.Errorf("Expected success rate ~0.67, got %f", successRate)
	}

	failureRate := metrics.ASTParseFailureRate()
	if failureRate < 0.33 || failureRate > 0.34 {
		t.Errorf("Expected failure rate ~0.33, got %f", failureRate)
	}
}

func TestIndexMetrics_MetadataExtractMetrics(t *testing.T) {
	metrics := NewIndexMetrics()

	// メタデータ抽出を5回試行、4回成功、1回失敗
	for i := 0; i < 4; i++ {
		metrics.RecordMetadataExtractAttempt()
		metrics.RecordMetadataExtractSuccess()
	}

	metrics.RecordMetadataExtractAttempt()
	metrics.RecordMetadataExtractFailure()

	if metrics.MetadataExtractAttempts != 5 {
		t.Errorf("Expected 5 attempts, got %d", metrics.MetadataExtractAttempts)
	}

	if metrics.MetadataExtractSuccesses != 4 {
		t.Errorf("Expected 4 successes, got %d", metrics.MetadataExtractSuccesses)
	}

	if metrics.MetadataExtractFailures != 1 {
		t.Errorf("Expected 1 failure, got %d", metrics.MetadataExtractFailures)
	}

	successRate := metrics.MetadataExtractSuccessRate()
	if successRate != 0.8 {
		t.Errorf("Expected success rate 0.8, got %f", successRate)
	}
}

func TestIndexMetrics_HighCommentRatioExcluded(t *testing.T) {
	metrics := NewIndexMetrics()

	// コメント比率が高いチャンクを3個除外
	metrics.RecordHighCommentRatioExcluded()
	metrics.RecordHighCommentRatioExcluded()
	metrics.RecordHighCommentRatioExcluded()

	if metrics.HighCommentRatioExcluded != 3 {
		t.Errorf("Expected 3 excluded chunks, got %d", metrics.HighCommentRatioExcluded)
	}
}

func TestIndexMetrics_CyclomaticComplexityDistribution(t *testing.T) {
	metrics := NewIndexMetrics()

	// 複数の循環的複雑度を記録
	complexities := []int{1, 2, 3, 5, 8, 13, 21, 34, 55, 89}
	for _, c := range complexities {
		metrics.RecordCyclomaticComplexity(c)
	}

	if len(metrics.CyclomaticComplexities) != 10 {
		t.Errorf("Expected 10 complexities, got %d", len(metrics.CyclomaticComplexities))
	}

	// P50（中央値）を確認
	p50 := metrics.CyclomaticComplexityP50()
	if p50 != 8 && p50 != 13 {
		t.Errorf("Expected P50 to be 8 or 13, got %d", p50)
	}

	// P95を確認（10要素の場合、95%は9.5番目の要素 = index 8 or 9）
	p95 := metrics.CyclomaticComplexityP95()
	// index 8 = 55, index 9 = 89
	if p95 != 55 && p95 != 89 {
		t.Errorf("Expected P95 to be 55 or 89, got %d", p95)
	}

	// P99を確認（10要素の場合、99%は9.9番目の要素 = index 9）
	p99 := metrics.CyclomaticComplexityP99()
	if p99 != 55 && p99 != 89 {
		t.Errorf("Expected P99 to be 55 or 89, got %d", p99)
	}
}

func TestIndexMetrics_EmptyMetrics(t *testing.T) {
	metrics := NewIndexMetrics()

	// 空のメトリクスでの動作を確認
	if metrics.ASTParseSuccessRate() != 0.0 {
		t.Errorf("Expected 0.0 success rate for empty metrics, got %f", metrics.ASTParseSuccessRate())
	}

	if metrics.ASTParseFailureRate() != 0.0 {
		t.Errorf("Expected 0.0 failure rate for empty metrics, got %f", metrics.ASTParseFailureRate())
	}

	if metrics.MetadataExtractSuccessRate() != 0.0 {
		t.Errorf("Expected 0.0 success rate for empty metrics, got %f", metrics.MetadataExtractSuccessRate())
	}

	if metrics.CyclomaticComplexityP50() != 0 {
		t.Errorf("Expected 0 for P50 of empty metrics, got %d", metrics.CyclomaticComplexityP50())
	}
}

func TestIndexMetrics_Merge(t *testing.T) {
	metrics1 := NewIndexMetrics()
	metrics1.RecordASTParseAttempt()
	metrics1.RecordASTParseSuccess()
	metrics1.RecordHighCommentRatioExcluded()
	metrics1.RecordCyclomaticComplexity(5)

	metrics2 := NewIndexMetrics()
	metrics2.RecordASTParseAttempt()
	metrics2.RecordASTParseFailure()
	metrics2.RecordHighCommentRatioExcluded()
	metrics2.RecordHighCommentRatioExcluded()
	metrics2.RecordCyclomaticComplexity(10)

	// マージ
	metrics1.Merge(metrics2)

	if metrics1.ASTParseAttempts != 2 {
		t.Errorf("Expected 2 attempts after merge, got %d", metrics1.ASTParseAttempts)
	}

	if metrics1.ASTParseSuccesses != 1 {
		t.Errorf("Expected 1 success after merge, got %d", metrics1.ASTParseSuccesses)
	}

	if metrics1.ASTParseFailures != 1 {
		t.Errorf("Expected 1 failure after merge, got %d", metrics1.ASTParseFailures)
	}

	if metrics1.HighCommentRatioExcluded != 3 {
		t.Errorf("Expected 3 excluded chunks after merge, got %d", metrics1.HighCommentRatioExcluded)
	}

	if len(metrics1.CyclomaticComplexities) != 2 {
		t.Errorf("Expected 2 complexities after merge, got %d", len(metrics1.CyclomaticComplexities))
	}
}
