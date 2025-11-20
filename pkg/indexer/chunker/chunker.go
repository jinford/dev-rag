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
		maxTokens:    1600,
		minTokens:    100,
		overlap:      200,
	}, nil
}

// Chunk はテキストをチャンク化します
func (c *Chunker) Chunk(content, contentType string) ([]*Chunk, error) {
	// メタデータ付きチャンク化を実行
	chunksWithMeta, err := c.ChunkWithMetadata(content, contentType)
	if err != nil {
		return nil, err
	}

	// Chunkのみを抽出
	chunks := make([]*Chunk, len(chunksWithMeta))
	for i, cwm := range chunksWithMeta {
		chunks[i] = cwm.Chunk
	}
	return chunks, nil
}

// ChunkWithMetadata はテキストをチャンク化し、メタデータも返します
func (c *Chunker) ChunkWithMetadata(content, contentType string) ([]*ChunkWithMetadata, error) {
	return c.ChunkWithMetadataAndMetrics(content, contentType, nil, nil)
}

// ChunkWithMetadataAndMetrics はテキストをチャンク化し、メタデータとメトリクスを記録します
func (c *Chunker) ChunkWithMetadataAndMetrics(content, contentType string, metricsCollector MetricsCollector, logger Logger) ([]*ChunkWithMetadata, error) {
	// Go言語の場合はAST解析を使用
	if contentType == "text/x-go" {
		return c.chunkGoSourceCodeWithMetrics(content, metricsCollector, logger)
	}

	// その他の場合は既存の方法でチャンク化（メタデータなし）
	var chunks []*Chunk
	var err error

	if contentType == "text/markdown" {
		chunks, err = c.chunkMarkdown(content)
	} else if isSourceCodeType(contentType) {
		chunks, err = c.chunkSourceCode(content)
	} else {
		chunks, err = c.chunkPlainText(content)
	}

	if err != nil {
		return nil, err
	}

	// Chunkをメタデータなしで返す
	chunksWithMeta := make([]*ChunkWithMetadata, len(chunks))
	for i, chunk := range chunks {
		chunksWithMeta[i] = &ChunkWithMetadata{
			Chunk:    chunk,
			Metadata: nil, // メタデータなし
		}
	}
	return chunksWithMeta, nil
}

// chunkGoSourceCode はGo言語のソースコードをAST解析してチャンク化します
func (c *Chunker) chunkGoSourceCode(content string) ([]*ChunkWithMetadata, error) {
	return c.chunkGoSourceCodeWithMetrics(content, nil, nil)
}

// chunkGoSourceCodeWithMetrics はGo言語のソースコードをAST解析してチャンク化し、メトリクスも記録します
func (c *Chunker) chunkGoSourceCodeWithMetrics(content string, metricsCollector MetricsCollector, logger Logger) ([]*ChunkWithMetadata, error) {
	astChunker := NewASTChunkerGo()
	result := astChunker.ChunkWithMetrics(content, c)

	// メトリクスを記録
	if metricsCollector != nil {
		metricsCollector.RecordASTParseAttempt()
		if result.ParseSuccess {
			metricsCollector.RecordASTParseSuccess()
		} else {
			metricsCollector.RecordASTParseFailure()
			// AST解析失敗時の詳細ログ
			if logger != nil && result.ParseError != nil {
				logger.Warn("AST parse failed, falling back to regex-based chunking", "error", result.ParseError)
			}
		}

		// コメント比率95%超過で除外されたチャンク数を記録
		for i := 0; i < result.HighCommentRatioExcluded; i++ {
			metricsCollector.RecordHighCommentRatioExcluded()
		}

		// 循環的複雑度を記録
		for _, complexity := range result.CyclomaticComplexities {
			metricsCollector.RecordCyclomaticComplexity(complexity)
		}

		// メタデータ抽出の成功数を記録
		for range result.Chunks {
			metricsCollector.RecordMetadataExtractAttempt()
			metricsCollector.RecordMetadataExtractSuccess()
		}
	}

	if !result.ParseSuccess {
		// AST解析に失敗した場合は正規表現ベースにフォールバック
		fallbackChunks, fallbackErr := c.chunkSourceCode(content)
		if fallbackErr != nil {
			return nil, fallbackErr
		}
		// メタデータなしで返す
		chunksWithMeta := make([]*ChunkWithMetadata, len(fallbackChunks))
		for i, chunk := range fallbackChunks {
			chunksWithMeta[i] = &ChunkWithMetadata{
				Chunk:    chunk,
				Metadata: nil,
			}
		}
		return chunksWithMeta, nil
	}
	return result.Chunks, nil
}

// MetricsCollector はメトリクスを収集するインターフェース
type MetricsCollector interface {
	RecordASTParseAttempt()
	RecordASTParseSuccess()
	RecordASTParseFailure()
	RecordMetadataExtractAttempt()
	RecordMetadataExtractSuccess()
	RecordMetadataExtractFailure()
	RecordHighCommentRatioExcluded()
	RecordCyclomaticComplexity(complexity int)
}

// Logger はログ出力のインターフェース
type Logger interface {
	Warn(msg string, args ...any)
}

// chunkMarkdown はMarkdownを見出し単位でチャンク化します
func (c *Chunker) chunkMarkdown(content string) ([]*Chunk, error) {
	lines := strings.Split(content, "\n")
	var chunks []*Chunk

	// 見出しで分割
	var currentChunk []string
	var currentStartLine int = 1
	var currentLine int = 1
	inCodeBlock := false
	inTable := false

	for i, line := range lines {
		currentLine = i + 1
		trimmedLine := strings.TrimSpace(line)

		// コードブロックの開始/終了を検出
		if strings.HasPrefix(trimmedLine, "```") {
			inCodeBlock = !inCodeBlock
		}

		// テーブル行を検出（| で始まるか、| を含む行）
		if strings.HasPrefix(trimmedLine, "|") || (strings.Contains(line, "|") && !inCodeBlock) {
			if !inTable {
				inTable = true
			}
		} else if inTable && trimmedLine == "" {
			// テーブルの終了（空行）
			inTable = false
		} else if inTable && !strings.Contains(line, "|") {
			// | を含まない行が来たらテーブル終了
			inTable = false
		}

		// 見出し行を検出（# で始まる行）
		// ただし、コードブロック内やテーブル内の場合は見出しとして扱わない
		isHeading := strings.HasPrefix(trimmedLine, "#") && !inCodeBlock && !inTable

		if isHeading {
			// 現在のチャンクがある場合、保存
			if len(currentChunk) > 0 {
				// 文末不完全検知を実行
				finalChunk := c.extendIncompleteChunk(currentChunk, lines, currentLine-1)
				chunk := c.createChunk(finalChunk, currentStartLine, currentStartLine+len(finalChunk)-1)
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
				// コードブロックやテーブルの途中で分割しないようにする
				if !inCodeBlock && !inTable {
					// 最後の数行を次のチャンクに持ち越す
					overlapLines := c.calculateOverlapLines(currentChunk)
					splitPoint := len(currentChunk) - overlapLines

					if splitPoint > 0 {
						// 分割点が構造要素の途中でないことを確認
						splitChunk := currentChunk[:splitPoint]
						chunk := c.createChunk(splitChunk, currentStartLine, currentStartLine+splitPoint-1)
						if chunk != nil {
							chunks = append(chunks, chunk)
						}

						// オーバーラップ分を次のチャンクの開始に
						currentChunk = currentChunk[splitPoint:]
						currentStartLine = currentStartLine + splitPoint
					}
				}
				// コードブロックやテーブルの途中の場合は、それらが終了するまで待つ
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

// extendIncompleteChunk は文末が不完全な場合に次の段落を含めてチャンクを拡張します
func (c *Chunker) extendIncompleteChunk(currentChunk []string, allLines []string, currentLineIndex int) []string {
	if len(currentChunk) == 0 {
		return currentChunk
	}

	// 最後の非空行を取得
	lastNonEmptyLine := ""
	for i := len(currentChunk) - 1; i >= 0; i-- {
		if strings.TrimSpace(currentChunk[i]) != "" {
			lastNonEmptyLine = strings.TrimSpace(currentChunk[i])
			break
		}
	}

	if lastNonEmptyLine == "" {
		return currentChunk
	}

	// 文末不完全パターンを検出
	isIncomplete := false

	// パターン1: 「:」「、」「,」で終わる
	if strings.HasSuffix(lastNonEmptyLine, ":") ||
		strings.HasSuffix(lastNonEmptyLine, "、") ||
		strings.HasSuffix(lastNonEmptyLine, ",") {
		isIncomplete = true
	}

	// パターン2: 指示語で終わる（「以下の」「次の」など）
	indicativePatterns := []string{
		"以下の", "次の", "以下に", "次に",
		"following", "next", "below",
	}
	for _, pattern := range indicativePatterns {
		if strings.HasSuffix(lastNonEmptyLine, pattern) ||
			strings.Contains(lastNonEmptyLine, pattern+":") ||
			strings.Contains(lastNonEmptyLine, pattern+"、") {
			isIncomplete = true
			break
		}
	}

	// 不完全でない場合はそのまま返す
	if !isIncomplete {
		return currentChunk
	}

	// 次の段落を探して追加（トークン制限を考慮）
	extendedChunk := make([]string, len(currentChunk))
	copy(extendedChunk, currentChunk)

	// 次の行から開始
	startIndex := currentLineIndex
	foundParagraph := false

	for i := startIndex; i < len(allLines); i++ {
		line := allLines[i]
		trimmedLine := strings.TrimSpace(line)

		// 空行または見出しに到達したら終了
		if trimmedLine == "" {
			if foundParagraph {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmedLine, "#") {
			break
		}

		// 段落の行を追加
		extendedChunk = append(extendedChunk, line)
		foundParagraph = true

		// トークン数をチェック
		extendedText := strings.Join(extendedChunk, "\n")
		tokens := c.countTokens(extendedText)

		// 最大トークン数を超えた場合は拡張を中止
		if tokens > c.maxTokens {
			// 追加した行を削除して返す
			return extendedChunk[:len(extendedChunk)-1]
		}
	}

	return extendedChunk
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

// TrimToTokenLimit はテキストを指定されたトークン数に収まるようトリミングします
func (c *Chunker) TrimToTokenLimit(text string, maxTokens int) string {
	// 現在のトークン数をチェック
	tokens := c.encoder.Encode(text, nil, nil)
	if len(tokens) <= maxTokens {
		return text
	}

	// 指定トークン数でトリミング
	trimmedTokens := tokens[:maxTokens]
	decoded := c.encoder.Decode(trimmedTokens)
	return decoded
}
