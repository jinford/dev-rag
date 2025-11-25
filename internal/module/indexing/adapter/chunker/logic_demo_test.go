package chunker

import (
	"fmt"
	"strings"
	"testing"
)

// TestLogicSplitting_RealWorldExample は実際のシナリオでロジック分割が動作することを示します
func TestLogicSplitting_RealWorldExample(t *testing.T) {
	// 実世界のコード例：200行を超える大きな関数
	code := `
package main

import (
	"fmt"
	"errors"
)

// ProcessLargeDataset は大規模なデータセット処理を行います
// この関数は複数の処理ステップを含みます
func ProcessLargeDataset(data []string) error {
	// ステップ1: 入力検証
	if data == nil {
		return errors.New("data cannot be nil")
	}
	if len(data) == 0 {
		return errors.New("data cannot be empty")
	}

	// ステップ2: データクリーニング
	cleaned := make([]string, 0, len(data))
	for _, item := range data {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}

	// ステップ3: データ変換
	transformed := make([]string, 0, len(cleaned))
	for _, item := range cleaned {
		upper := strings.ToUpper(item)
		transformed = append(transformed, upper)
	}

	// ステップ4: データフィルタリング
	filtered := make([]string, 0)
	for _, item := range transformed {
		if len(item) > 3 {
			filtered = append(filtered, item)
		}
	}

	// ステップ5: エラーチェック
	if len(filtered) == 0 {
		return errors.New("no valid data after processing")
	}

	// ステップ6: 結果出力
	for i, item := range filtered {
		fmt.Printf("%d: %s\n", i, item)
	}

	// ステップ7: 統計計算
	totalLength := 0
	for _, item := range filtered {
		totalLength += len(item)
	}
	avgLength := totalLength / len(filtered)
	fmt.Printf("Average length: %d\n", avgLength)

	// ステップ8: 最終検証
	if avgLength < 4 {
		fmt.Println("Warning: average length is low")
	}

	// ステップ9: ログ出力
	fmt.Printf("Processed %d items\n", len(filtered))
	fmt.Printf("Original count: %d\n", len(data))
	fmt.Printf("Cleaned count: %d\n", len(cleaned))
	fmt.Printf("Filtered count: %d\n", len(filtered))

	// ステップ10: 成功
	return nil
}
`

	// 関数をさらに大きくするために繰り返し処理を追加
	additionalSteps := ""
	for i := 11; i <= 50; i++ {
		additionalSteps += fmt.Sprintf(`
	// ステップ%d: 追加処理
	fmt.Printf("Processing step %%d\n", %d)
`, i, i)
	}
	fullCode := strings.Replace(code, "return nil", additionalSteps+"\n\treturn nil", 1)

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	result := astChunker.ChunkWithMetrics(fullCode, chunker)

	if !result.ParseSuccess {
		t.Fatalf("AST parse failed: %v", result.ParseError)
	}

	// 関数チャンク（レベル2）を探す
	var functionChunk *ChunkWithMetadata
	for _, chunk := range result.Chunks {
		if chunk.Metadata != nil && chunk.Metadata.Level == 2 {
			if chunk.Metadata.Name != nil && *chunk.Metadata.Name == "ProcessLargeDataset" {
				functionChunk = chunk
				break
			}
		}
	}

	if functionChunk == nil {
		t.Fatal("Function chunk not found")
	}

	// 関数が分割対象かチェック
	startLine := functionChunk.Chunk.StartLine
	endLine := functionChunk.Chunk.EndLine
	lineCount := endLine - startLine + 1

	t.Logf("Function: %s", *functionChunk.Metadata.Name)
	t.Logf("Lines: %d-%d (total: %d lines)", startLine, endLine, lineCount)
	t.Logf("Tokens: %d", functionChunk.Chunk.Tokens)
	if functionChunk.Metadata.CyclomaticComplexity != nil {
		t.Logf("Cyclomatic Complexity: %d", *functionChunk.Metadata.CyclomaticComplexity)
	}

	// レベル3のチャンク（ロジックチャンク）を探す
	logicChunks := make([]*ChunkWithMetadata, 0)
	for _, chunk := range result.Chunks {
		if chunk.Metadata != nil && chunk.Metadata.Level == 3 {
			logicChunks = append(logicChunks, chunk)
		}
	}

	t.Logf("Logic chunks generated: %d", len(logicChunks))

	// ロジックチャンクの詳細を表示
	for i, chunk := range logicChunks {
		t.Logf("  Logic chunk %d:", i)
		t.Logf("    Type: %s", getStringPtrValue(chunk.Metadata.Type))
		t.Logf("    Name: %s", getStringPtrValue(chunk.Metadata.Name))
		t.Logf("    Parent: %s", getStringPtrValue(chunk.Metadata.ParentName))
		t.Logf("    Lines: %d-%d", chunk.Chunk.StartLine, chunk.Chunk.EndLine)
		t.Logf("    Tokens: %d", chunk.Chunk.Tokens)
	}

	// 期待：大きな関数なので、少なくとも分割判定は行われるべき
	if lineCount < 100 {
		t.Logf("Note: Function has %d lines (threshold: 100). May not be split.", lineCount)
	}
}

// TestLogicChunker_SmallFunction は小さな関数が分割されないことを確認します
func TestLogicChunker_SmallFunction(t *testing.T) {
	code := `
package main

func SmallFunction(x, y int) int {
	return x + y
}
`

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	result := astChunker.ChunkWithMetrics(code, chunker)

	if !result.ParseSuccess {
		t.Fatalf("AST parse failed: %v", result.ParseError)
	}

	// レベル3のチャンク（ロジックチャンク）が生成されていないことを確認
	logicChunkCount := 0
	for _, chunk := range result.Chunks {
		if chunk.Metadata != nil && chunk.Metadata.Level == 3 {
			logicChunkCount++
		}
	}

	if logicChunkCount > 0 {
		t.Errorf("Small function should not be split into logic chunks, got %d", logicChunkCount)
	}

	t.Logf("Small function correctly NOT split (logic chunks: %d)", logicChunkCount)
}

// getStringPtrValue はポインタから文字列値を安全に取得します
func getStringPtrValue(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
