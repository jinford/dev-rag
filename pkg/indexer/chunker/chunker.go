package chunker

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

// Chunker はテキストを小さな単位に分割します
type Chunker struct {
	encoder *tiktoken.Tiktoken

	// チャンクサイズ設定
	targetTokens int // 目標トークン数（デフォルト: 800）
	maxTokens    int // 最大トークン数（デフォルト: 1200）
	minTokens    int // 最小トークン数（デフォルト: 100）
	overlap      int // オーバーラップトークン数（デフォルト: 200）
}

// Chunk はチャンクを表します
type Chunk struct {
	Content   string
	StartLine int
	EndLine   int
	Tokens    int
}

// NewChunker は新しいChunkerを作成します
func NewChunker() (*Chunker, error) {
	// cl100k_baseエンコーダを使用（OpenAIのtext-embedding-3-smallと互換）
	encoder, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoder: %w", err)
	}

	return &Chunker{
		encoder:      encoder,
		targetTokens: 800,
		maxTokens:    1200,
		minTokens:    100,
		overlap:      200,
	}, nil
}

// Chunk はテキストをチャンク化します
func (c *Chunker) Chunk(content, contentType string) ([]*Chunk, error) {
	// コンテンツタイプから種別を判定
	if contentType == "text/markdown" {
		return c.chunkMarkdown(content)
	}
	if isSourceCodeType(contentType) {
		return c.chunkSourceCode(content)
	}
	return c.chunkPlainText(content)
}

// chunkMarkdown はMarkdownを見出し単位でチャンク化します
func (c *Chunker) chunkMarkdown(content string) ([]*Chunk, error) {
	lines := strings.Split(content, "\n")
	var chunks []*Chunk

	// 見出しで分割
	var currentChunk []string
	var currentStartLine int = 1
	var currentLine int = 1

	for i, line := range lines {
		currentLine = i + 1

		// 見出し行を検出（# で始まる行）
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			// 現在のチャンクがある場合、保存
			if len(currentChunk) > 0 {
				chunk := c.createChunk(currentChunk, currentStartLine, currentLine-1)
				if chunk != nil {
					chunks = append(chunks, chunk)
				}
			}

			// 新しいチャンクを開始
			currentChunk = []string{line}
			currentStartLine = currentLine
		} else {
			currentChunk = append(currentChunk, line)

			// トークン数をチェック
			chunkText := strings.Join(currentChunk, "\n")
			tokens := c.countTokens(chunkText)

			// 最大トークン数を超えた場合、分割
			if tokens > c.maxTokens {
				// 最後の数行を次のチャンクに持ち越す
				overlapLines := c.calculateOverlapLines(currentChunk)
				splitPoint := len(currentChunk) - overlapLines

				if splitPoint > 0 {
					chunk := c.createChunk(currentChunk[:splitPoint], currentStartLine, currentStartLine+splitPoint-1)
					if chunk != nil {
						chunks = append(chunks, chunk)
					}

					// オーバーラップ分を次のチャンクの開始に
					currentChunk = currentChunk[splitPoint:]
					currentStartLine = currentStartLine + splitPoint
				}
			}
		}
	}

	// 最後のチャンクを保存
	if len(currentChunk) > 0 {
		chunk := c.createChunk(currentChunk, currentStartLine, currentLine)
		if chunk != nil {
			chunks = append(chunks, chunk)
		}
	}

	return chunks, nil
}

// chunkSourceCode はソースコードを関数/クラス境界を考慮してチャンク化します
func (c *Chunker) chunkSourceCode(content string) ([]*Chunk, error) {
	lines := strings.Split(content, "\n")
	var chunks []*Chunk

	// 関数/クラスの開始を検出する正規表現
	functionPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^\s*(func|function|def|class|interface|struct|enum|type)\s+\w+`), // Go, Python, JS, etc.
		regexp.MustCompile(`^\s*(public|private|protected|static)?\s*(async)?\s*\w+\s*\(`),   // Java, C#, etc.
	}

	var currentChunk []string
	var currentStartLine int = 1
	var currentLine int = 1

	for i, line := range lines {
		currentLine = i + 1

		// 関数/クラスの開始を検出
		isFunctionStart := false
		for _, pattern := range functionPatterns {
			if pattern.MatchString(line) {
				isFunctionStart = true
				break
			}
		}

		if isFunctionStart && len(currentChunk) > 0 {
			// 現在のチャンクがある場合、保存
			chunk := c.createChunk(currentChunk, currentStartLine, currentLine-1)
			if chunk != nil {
				chunks = append(chunks, chunk)
			}

			// 新しいチャンクを開始
			currentChunk = []string{line}
			currentStartLine = currentLine
		} else {
			currentChunk = append(currentChunk, line)

			// トークン数をチェック
			chunkText := strings.Join(currentChunk, "\n")
			tokens := c.countTokens(chunkText)

			// 最大トークン数を超えた場合、分割
			if tokens > c.maxTokens {
				// 最後の数行を次のチャンクに持ち越す
				overlapLines := c.calculateOverlapLines(currentChunk)
				splitPoint := len(currentChunk) - overlapLines

				if splitPoint > 0 {
					chunk := c.createChunk(currentChunk[:splitPoint], currentStartLine, currentStartLine+splitPoint-1)
					if chunk != nil {
						chunks = append(chunks, chunk)
					}

					// オーバーラップ分を次のチャンクの開始に
					currentChunk = currentChunk[splitPoint:]
					currentStartLine = currentStartLine + splitPoint
				}
			}
		}
	}

	// 最後のチャンクを保存
	if len(currentChunk) > 0 {
		chunk := c.createChunk(currentChunk, currentStartLine, currentLine)
		if chunk != nil {
			chunks = append(chunks, chunk)
		}
	}

	return chunks, nil
}

// chunkPlainText はプレーンテキストを行ベースでチャンク化します
func (c *Chunker) chunkPlainText(content string) ([]*Chunk, error) {
	lines := strings.Split(content, "\n")
	var chunks []*Chunk

	var currentChunk []string
	var currentStartLine int = 1

	for i, line := range lines {
		currentChunk = append(currentChunk, line)

		// トークン数をチェック
		chunkText := strings.Join(currentChunk, "\n")
		tokens := c.countTokens(chunkText)

		// 目標トークン数を超えた場合、チャンクを保存
		if tokens >= c.targetTokens {
			chunk := c.createChunk(currentChunk, currentStartLine, i+1)
			if chunk != nil {
				chunks = append(chunks, chunk)
			}

			// オーバーラップ分を次のチャンクの開始に
			overlapLines := c.calculateOverlapLines(currentChunk)
			if overlapLines > 0 && overlapLines < len(currentChunk) {
				currentChunk = currentChunk[len(currentChunk)-overlapLines:]
				currentStartLine = i + 2 - overlapLines
			} else {
				currentChunk = []string{}
				currentStartLine = i + 2
			}
		}
	}

	// 最後のチャンクを保存
	if len(currentChunk) > 0 {
		chunk := c.createChunk(currentChunk, currentStartLine, len(lines))
		if chunk != nil {
			chunks = append(chunks, chunk)
		}
	}

	return chunks, nil
}

// createChunk はチャンクを作成します
func (c *Chunker) createChunk(lines []string, startLine, endLine int) *Chunk {
	content := strings.Join(lines, "\n")
	tokens := c.countTokens(content)

	// 最小トークン数未満の場合はスキップ
	if tokens < c.minTokens {
		return nil
	}

	return &Chunk{
		Content:   content,
		StartLine: startLine,
		EndLine:   endLine,
		Tokens:    tokens,
	}
}

// countTokens はテキストのトークン数をカウントします
func (c *Chunker) countTokens(text string) int {
	tokens := c.encoder.Encode(text, nil, nil)
	return len(tokens)
}

// calculateOverlapLines はオーバーラップする行数を計算します
func (c *Chunker) calculateOverlapLines(lines []string) int {
	// 後ろから順にトークン数をカウントし、オーバーラップトークン数に達するまでの行数を返す
	var totalTokens int
	for i := len(lines) - 1; i >= 0; i-- {
		lineTokens := c.countTokens(lines[i])
		totalTokens += lineTokens

		if totalTokens >= c.overlap {
			return len(lines) - i
		}
	}
	return len(lines)
}

// sourceCodeTypes はプログラミング言語のMIMEタイプのセット
var sourceCodeTypes = map[string]bool{
	"text/x-go":          true,
	"text/javascript":    true,
	"text/x-typescript":  true,
	"text/x-python":      true,
	"text/x-java":        true,
	"text/x-c":           true,
	"text/x-c++":         true,
	"text/x-csharp":      true,
	"text/x-ruby":        true,
	"text/x-php":         true,
	"text/x-rust":        true,
	"text/x-swift":       true,
	"text/x-kotlin":      true,
	"text/x-scala":       true,
	"text/x-shellscript": true,
}

// isSourceCodeType はコンテンツタイプがソースコードかどうかを判定します
func isSourceCodeType(contentType string) bool {
	return sourceCodeTypes[contentType]
}
