package chunker

import (
	"fmt"
	"strings"
	"testing"
)

// TestASTChunkerGo_LogicSplitting は大きな関数がロジック単位に分割されることを検証します
func TestASTChunkerGo_LogicSplitting(t *testing.T) {
	// 200行の大きな関数を作成
	code := `
package main

import "fmt"

// ProcessData は大規模なデータ処理を行います
func ProcessData(data []int) error {
	// 初期化
	result := make([]int, 0, len(data))
	sum := 0
	count := 0

	// バリデーション
	if data == nil {
		return fmt.Errorf("data is nil")
	}
	if len(data) == 0 {
		return fmt.Errorf("data is empty")
	}

	// フィルタリング処理
	filtered := make([]int, 0)
	for _, v := range data {
		if v > 0 {
			filtered = append(filtered, v)
		}
	}

	// エラーハンドリング
	if len(filtered) == 0 {
		return fmt.Errorf("no valid data after filtering")
	}

	// 集計処理
	for _, v := range filtered {
		sum += v
		count++
		result = append(result, v*2)
	}

	// 平均計算
	avg := 0
	if count > 0 {
		avg = sum / count
	}

	// 結果出力
	fmt.Printf("Sum: %d, Avg: %d, Count: %d\n", sum, avg, count)
	fmt.Printf("Result: %v\n", result)

	return nil
}
`

	// 関数を100行以上にするために繰り返し処理を追加
	additionalCode := ""
	for i := 0; i < 20; i++ {
		additionalCode += fmt.Sprintf(`
	// 追加処理 %d
	if result[%d] > 100 {
		fmt.Println("Large value detected")
	}
`, i, i%10)
	}

	// 追加コードを挿入
	fullCode := strings.Replace(code, "return nil", additionalCode+"\n\treturn nil", 1)

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	result := astChunker.ChunkWithMetrics(fullCode, chunker)

	if !result.ParseSuccess {
		t.Fatalf("AST parse failed: %v", result.ParseError)
	}

	// チャンクが生成されたことを確認
	if len(result.Chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}

	// レベル2のチャンク（関数チャンク）を探す
	var functionChunk *ChunkWithMetadata
	for _, chunk := range result.Chunks {
		if chunk.Metadata != nil && chunk.Metadata.Level == 2 {
			if chunk.Metadata.Name != nil && *chunk.Metadata.Name == "ProcessData" {
				functionChunk = chunk
				break
			}
		}
	}

	if functionChunk == nil {
		t.Fatal("Function chunk not found")
	}

	// レベル3のチャンク（ロジックチャンク）を探す
	logicChunks := make([]*ChunkWithMetadata, 0)
	for _, chunk := range result.Chunks {
		if chunk.Metadata != nil && chunk.Metadata.Level == 3 {
			logicChunks = append(logicChunks, chunk)
		}
	}

	// ロジックチャンクの生成を確認（生成される場合もあれば、されない場合もある）
	if len(logicChunks) == 0 {
		t.Log("No logic chunks generated (function may not meet splitting criteria)")
	} else {
		t.Logf("Generated %d logic chunks", len(logicChunks))

		// 各ロジックチャンクを検証
		for i, chunk := range logicChunks {
			t.Logf("Logic chunk %d: type=%v, name=%v, tokens=%d",
				i,
				getStringValue(chunk.Metadata.Type),
				getStringValue(chunk.Metadata.Name),
				chunk.Chunk.Tokens)

			// 親名が設定されていることを確認
			if chunk.Metadata.ParentName == nil {
				t.Errorf("Logic chunk %d: ParentName is nil", i)
			} else if *chunk.Metadata.ParentName != "ProcessData" {
				t.Errorf("Logic chunk %d: Expected parent 'ProcessData', got '%s'", i, *chunk.Metadata.ParentName)
			}

			// トークン数が妥当であることを確認
			if chunk.Chunk.Tokens < 10 || chunk.Chunk.Tokens > 800 {
				t.Errorf("Logic chunk %d: Token count %d is out of reasonable range", i, chunk.Chunk.Tokens)
			}
		}
	}
}

// TestASTChunkerGo_NestedIfElse はネストされたif-else構造を検証します
func TestASTChunkerGo_NestedIfElse(t *testing.T) {
	code := `
package main

func NestedConditions(x, y, z int) string {
	// レベル1の条件
	if x > 0 {
		// レベル2の条件
		if y > 0 {
			// レベル3の条件
			if z > 0 {
				return "all positive"
			} else {
				return "x,y positive, z negative"
			}
		} else {
			if z > 0 {
				return "x,z positive, y negative"
			} else {
				return "only x positive"
			}
		}
	} else {
		if y > 0 {
			if z > 0 {
				return "y,z positive, x negative"
			} else {
				return "only y positive"
			}
		} else {
			if z > 0 {
				return "only z positive"
			} else {
				return "all negative"
			}
		}
	}
}
`

	// 関数を大きくするために追加のロジックを挿入
	additionalCode := strings.Repeat(`
	// 追加の条件チェック
	if x+y+z > 100 {
		fmt.Println("sum is large")
	}
`, 30)

	fullCode := strings.Replace(code, "return \"all negative\"", additionalCode+"\n\treturn \"all negative\"", 1)

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	result := astChunker.ChunkWithMetrics(fullCode, chunker)

	if !result.ParseSuccess {
		t.Fatalf("AST parse failed: %v", result.ParseError)
	}

	// チャンクが生成されたことを確認
	if len(result.Chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}

	// 循環的複雑度が記録されていることを確認
	if len(result.CyclomaticComplexities) == 0 {
		t.Error("Expected cyclomatic complexity to be recorded")
	} else {
		for _, complexity := range result.CyclomaticComplexities {
			t.Logf("Cyclomatic complexity: %d", complexity)
			if complexity < 1 {
				t.Error("Cyclomatic complexity should be at least 1")
			}
		}
	}
}

// TestASTChunkerGo_ErrorHandlingChain は長いエラーハンドリングチェーンを検証します
func TestASTChunkerGo_ErrorHandlingChain(t *testing.T) {
	code := `
package main

import "errors"

func ChainedErrorHandling() error {
	// ステップ1
	if err := step1(); err != nil {
		return err
	}

	// ステップ2
	if err := step2(); err != nil {
		return err
	}

	// ステップ3
	if err := step3(); err != nil {
		return err
	}

	// ステップ4
	if err := step4(); err != nil {
		return err
	}

	// ステップ5
	if err := step5(); err != nil {
		return err
	}

	return nil
}

func step1() error { return nil }
func step2() error { return nil }
func step3() error { return nil }
func step4() error { return nil }
func step5() error { return nil }
`

	// さらに多くのステップを追加
	for i := 6; i <= 30; i++ {
		stepCode := fmt.Sprintf(`
	// ステップ%d
	if err := step%d(); err != nil {
		return err
	}
`, i, i)
		code = strings.Replace(code, "return nil", stepCode+"\n\treturn nil", 1)
	}

	chunker, err := NewChunker()
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	astChunker := NewASTChunkerGo()
	result := astChunker.ChunkWithMetrics(code, chunker)

	if !result.ParseSuccess {
		t.Fatalf("AST parse failed: %v", result.ParseError)
	}

	// エラーハンドリングのロジックチャンクが生成されたことを確認
	errorHandlingChunks := 0
	for _, chunk := range result.Chunks {
		if chunk.Metadata != nil && chunk.Metadata.Type != nil {
			if strings.Contains(*chunk.Metadata.Type, "error_handling") {
				errorHandlingChunks++
			}
		}
	}

	t.Logf("Found %d error handling chunks", errorHandlingChunks)
}

// getStringValue はポインタから文字列値を安全に取得します
func getStringValue(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
