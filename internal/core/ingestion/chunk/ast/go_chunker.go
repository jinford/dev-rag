package ast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// Chunk はチャンクを表します
type Chunk struct {
	Content   string
	StartLine int
	EndLine   int
	Tokens    int
}

// ChunkMetadata はチャンクの構造メタデータを表します
type ChunkMetadata struct {
	Type                 *string
	Name                 *string
	ParentName           *string
	Signature            *string
	DocComment           *string
	Imports              []string
	Calls                []string
	StandardImports      []string
	ExternalImports      []string
	InternalCalls        []string
	ExternalCalls        []string
	TypeDependencies     []string
	LinesOfCode          *int
	CommentRatio         *float64
	CyclomaticComplexity *int
	Level                int
	ImportanceScore      *float64
}

// ChunkWithMetadata はチャンクとメタデータをセットで保持します
type ChunkWithMetadata struct {
	Chunk    *Chunk
	Metadata *ChunkMetadata
}

// TokenCounter はトークン数をカウントするインターフェース
type TokenCounter interface {
	CountTokens(text string) int
	TrimToTokenLimit(text string, maxTokens int) string
}

// ASTChunkerGo はGo言語のAST解析によるチャンク化を行います
type ASTChunkerGo struct {
	fset *token.FileSet
}

// ASTChunkResult はAST解析の結果とメトリクスを保持します
type ASTChunkResult struct {
	Chunks                   []*ChunkWithMetadata
	ParseSuccess             bool
	ParseError               error // AST解析エラー（失敗時のみ）
	HighCommentRatioExcluded int
	CyclomaticComplexities   []int
}

// NewASTChunkerGo は新しいASTChunkerGoを作成します
func NewASTChunkerGo() *ASTChunkerGo {
	return &ASTChunkerGo{
		fset: token.NewFileSet(),
	}
}

// Chunk はGo言語のソースコードをAST解析してチャンク化します
func (ac *ASTChunkerGo) Chunk(content string, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) ([]*ChunkWithMetadata, error) {
	result := ac.ChunkWithMetrics(content, chunkCounter)
	if !result.ParseSuccess {
		return nil, fmt.Errorf("failed to parse Go source")
	}
	return result.Chunks, nil
}

// ChunkWithMetrics はGo言語のソースコードをAST解析してチャンク化し、メトリクスも返します
func (ac *ASTChunkerGo) ChunkWithMetrics(content string, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) *ASTChunkResult {
	result := &ASTChunkResult{
		Chunks:                   make([]*ChunkWithMetadata, 0),
		ParseSuccess:             false,
		ParseError:               nil,
		HighCommentRatioExcluded: 0,
		CyclomaticComplexities:   make([]int, 0),
	}

	// ASTを解析
	file, err := parser.ParseFile(ac.fset, "", content, parser.ParseComments)
	if err != nil {
		// AST解析失敗
		result.ParseError = err
		return result
	}

	// AST解析成功
	result.ParseSuccess = true

	lines := strings.Split(content, "\n")

	// パッケージレベルのコメントを抽出
	if file.Doc != nil {
		pkgChunk := ac.extractPackageDoc(file, lines, chunkCounter)
		if pkgChunk != nil {
			pkgChunk.Metadata.Level = 2 // レベル2: 関数/クラス単位
			result.Chunks = append(result.Chunks, pkgChunk)
		}
	}

	// インポートリストを抽出（詳細版）
	importInfo := ac.extractImportsDetailed(file)

	// トップレベルの宣言を処理
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			// 関数・メソッドを抽出（ロジック分割を含む）
			chunks, excluded := ac.extractFunctionWithLogicSplittingDetailed(d, file, lines, importInfo, chunkCounter)
			for _, chunk := range chunks {
				// レベル設定（関数チャンクはレベル2、ロジックチャンクはレベル3）
				if chunk.Metadata.Level == 0 {
					chunk.Metadata.Level = 2 // デフォルトはレベル2
				}
				result.Chunks = append(result.Chunks, chunk)
				// 循環的複雑度を記録（関数チャンクのみ）
				if chunk.Metadata != nil && chunk.Metadata.CyclomaticComplexity != nil && chunk.Metadata.Level == 2 {
					result.CyclomaticComplexities = append(result.CyclomaticComplexities, *chunk.Metadata.CyclomaticComplexity)
				}
			}
			if excluded {
				result.HighCommentRatioExcluded++
			}
		case *ast.GenDecl:
			// 型定義、変数、定数を抽出（詳細版）
			declChunks, excludedCount := ac.extractGenDeclWithMetricsDetailed(d, file, lines, importInfo, chunkCounter)
			for _, chunk := range declChunks {
				chunk.Metadata.Level = 2 // レベル2: 関数/クラス単位
			}
			result.Chunks = append(result.Chunks, declChunks...)
			result.HighCommentRatioExcluded += excludedCount
		}
	}

	return result
}

// extractPackageDoc はパッケージレベルのコメントを抽出します
func (ac *ASTChunkerGo) extractPackageDoc(file *ast.File, lines []string, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) *ChunkWithMetadata {
	if file.Doc == nil {
		return nil
	}

	startPos := ac.fset.Position(file.Doc.Pos())
	endPos := ac.fset.Position(file.Doc.End())

	content := ac.extractContent(lines, startPos.Line, endPos.Line)
	tokens := chunkCounter.CountTokens(content)

	// トークンサイズ検証
	// パッケージドキュメントは最小トークン数10に緩和
	minTokensForAST := 10
	if tokens < minTokensForAST || tokens > 1600 {
		return nil
	}

	docComment := file.Doc.Text()

	return &ChunkWithMetadata{
		Chunk: &Chunk{
			Content:   content,
			StartLine: startPos.Line,
			EndLine:   endPos.Line,
			Tokens:    tokens,
		},
		Metadata: &ChunkMetadata{
			Type:       stringPtr("package"),
			Name:       stringPtr(file.Name.Name),
			DocComment: &docComment,
		},
	}
}

// ImportInfo はインポート情報の詳細を保持します
type ImportInfo struct {
	All      []string // 全インポート
	Standard []string // 標準ライブラリ
	External []string // 外部依存
}

// extractImports はインポート情報を抽出します（後方互換性のため）
func (ac *ASTChunkerGo) extractImports(file *ast.File) []string {
	var imports []string
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, path)
	}
	return imports
}

// extractImportsDetailed はインポート情報を詳細に抽出します
func (ac *ASTChunkerGo) extractImportsDetailed(file *ast.File) *ImportInfo {
	info := &ImportInfo{
		All:      []string{},
		Standard: []string{},
		External: []string{},
	}

	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		info.All = append(info.All, path)

		// 標準ライブラリの判定
		// 1. ドットを含まない（例: "fmt", "net/http"）
		// 2. golang.orgで始まる
		// 3. 既知の標準ライブラリパス
		if ac.isStandardLibrary(path) {
			info.Standard = append(info.Standard, path)
		} else {
			info.External = append(info.External, path)
		}
	}

	return info
}

// isStandardLibrary は標準ライブラリかどうかを判定します
func (ac *ASTChunkerGo) isStandardLibrary(path string) bool {
	// ドットを含まない、またはgolang.orgで始まる場合は標準ライブラリ
	if !strings.Contains(path, ".") || strings.HasPrefix(path, "golang.org/x/") {
		return true
	}

	// 既知の標準ライブラリパターン
	stdPrefixes := []string{
		"archive/", "bufio", "builtin", "bytes", "compress/",
		"container/", "context", "crypto/", "database/", "debug/",
		"embed", "encoding/", "errors", "expvar", "flag", "fmt",
		"go/", "hash/", "html/", "image/", "index/", "io", "log",
		"math", "mime", "net", "os", "path", "plugin", "reflect",
		"regexp", "runtime", "sort", "strconv", "strings", "sync",
		"syscall", "testing", "text/", "time", "unicode", "unsafe",
	}

	for _, prefix := range stdPrefixes {
		if strings.HasPrefix(path, prefix) || path == strings.TrimSuffix(prefix, "/") {
			return true
		}
	}

	return false
}

// extractFunction は関数・メソッドを抽出します（後方互換性のため）
func (ac *ASTChunkerGo) extractFunction(fn *ast.FuncDecl, file *ast.File, lines []string, imports []string, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) *ChunkWithMetadata {
	chunk, _ := ac.extractFunctionWithMetrics(fn, file, lines, imports, chunkCounter)
	return chunk
}

// extractFunctionWithMetrics は関数・メソッドを抽出し、除外されたかどうかを返します
// 大きな関数の場合はロジック単位に分割します
func (ac *ASTChunkerGo) extractFunctionWithMetrics(fn *ast.FuncDecl, file *ast.File, lines []string, imports []string, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) (*ChunkWithMetadata, bool) {
	startPos := ac.fset.Position(fn.Pos())
	endPos := ac.fset.Position(fn.End())

	content := ac.extractContent(lines, startPos.Line, endPos.Line)
	tokens := chunkCounter.CountTokens(content)

	// トークンサイズ検証
	// AST解析の場合、意味のある単位（関数）であれば最小トークン数は10に緩和
	// これにより小さな関数もチャンクとして抽出される
	minTokensForAST := 10
	if tokens < minTokensForAST || tokens > 1600 {
		return nil, false
	}

	// メタデータ抽出
	funcName := fn.Name.Name
	funcType := "function"
	var parentName *string
	var signature string

	// メソッドかどうか判定
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		funcType = "method"
		recv := fn.Recv.List[0]
		parentName = stringPtr(ac.extractTypeName(recv.Type))
	}

	// シグネチャを構築
	signature = ac.buildFunctionSignature(fn)

	// DocCommentを抽出
	var docComment *string
	if fn.Doc != nil {
		doc := fn.Doc.Text()
		docComment = &doc
	}

	// 関数内の呼び出しを抽出
	calls := ac.extractFunctionCalls(fn)

	// 品質メトリクス計測
	loc := ac.calculateLinesOfCode(content)
	commentRatio := ac.calculateCommentRatio(content)
	complexity := ac.calculateCyclomaticComplexity(fn)

	// コメント比率95%以上の場合は除外
	if commentRatio > 0.95 {
		return nil, true // 除外された
	}

	return &ChunkWithMetadata{
		Chunk: &Chunk{
			Content:   content,
			StartLine: startPos.Line,
			EndLine:   endPos.Line,
			Tokens:    tokens,
		},
		Metadata: &ChunkMetadata{
			Type:                 &funcType,
			Name:                 &funcName,
			ParentName:           parentName,
			Signature:            &signature,
			DocComment:           docComment,
			Imports:              imports,
			Calls:                calls,
			LinesOfCode:          &loc,
			CommentRatio:         &commentRatio,
			CyclomaticComplexity: &complexity,
		},
	}, false // 除外されていない
}

// extractFunctionWithMetricsDetailed は関数・メソッドを抽出し、詳細な依存関係情報を含めます
func (ac *ASTChunkerGo) extractFunctionWithMetricsDetailed(fn *ast.FuncDecl, file *ast.File, lines []string, importInfo *ImportInfo, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) (*ChunkWithMetadata, bool) {
	startPos := ac.fset.Position(fn.Pos())
	endPos := ac.fset.Position(fn.End())

	content := ac.extractContent(lines, startPos.Line, endPos.Line)
	tokens := chunkCounter.CountTokens(content)

	// トークンサイズ検証
	minTokensForAST := 10
	if tokens < minTokensForAST || tokens > 1600 {
		return nil, false
	}

	// メタデータ抽出
	funcName := fn.Name.Name
	funcType := "function"
	var parentName *string
	var signature string

	// メソッドかどうか判定
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		funcType = "method"
		recv := fn.Recv.List[0]
		parentName = stringPtr(ac.extractTypeName(recv.Type))
	}

	// シグネチャを構築
	signature = ac.buildFunctionSignature(fn)

	// DocCommentを抽出
	var docComment *string
	if fn.Doc != nil {
		doc := fn.Doc.Text()
		docComment = &doc
	}

	// 関数内の呼び出しを抽出
	calls := ac.extractFunctionCalls(fn)

	// 型依存を抽出
	typeDeps := ac.extractTypeDependencies(fn)

	// 品質メトリクス計測
	loc := ac.calculateLinesOfCode(content)
	commentRatio := ac.calculateCommentRatio(content)
	complexity := ac.calculateCyclomaticComplexity(fn)

	// コメント比率95%以上の場合は除外
	if commentRatio > 0.95 {
		return nil, true // 除外された
	}

	return &ChunkWithMetadata{
		Chunk: &Chunk{
			Content:   content,
			StartLine: startPos.Line,
			EndLine:   endPos.Line,
			Tokens:    tokens,
		},
		Metadata: &ChunkMetadata{
			Type:                 &funcType,
			Name:                 &funcName,
			ParentName:           parentName,
			Signature:            &signature,
			DocComment:           docComment,
			Imports:              importInfo.All,
			Calls:                calls,
			LinesOfCode:          &loc,
			CommentRatio:         &commentRatio,
			CyclomaticComplexity: &complexity,
			// 詳細な依存関係情報
			StandardImports:  importInfo.Standard,
			ExternalImports:  importInfo.External,
			TypeDependencies: typeDeps,
		},
	}, false // 除外されていない
}

// extractFunctionWithLogicSplitting は関数を抽出し、必要に応じてロジック単位に分割します
// レベル3ロジック単位チャンキング
func (ac *ASTChunkerGo) extractFunctionWithLogicSplitting(fn *ast.FuncDecl, file *ast.File, lines []string, imports []string, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) ([]*ChunkWithMetadata, bool) {
	// まず通常の関数チャンクを生成
	funcChunk, excluded := ac.extractFunctionWithMetrics(fn, file, lines, imports, chunkCounter)
	if funcChunk == nil {
		return nil, excluded
	}

	chunks := []*ChunkWithMetadata{funcChunk}

	// ロジック分割が必要かチェック
	logicChunker := NewLogicChunker(ac.fset)
	config := DefaultSplitConfig()

	complexity := 0
	if funcChunk.Metadata.CyclomaticComplexity != nil {
		complexity = *funcChunk.Metadata.CyclomaticComplexity
	}

	if !logicChunker.ShouldSplit(fn, complexity, config) {
		// 分割不要の場合は関数チャンクのみを返す
		return chunks, excluded
	}

	// ロジック単位に分割
	logicBlocks := logicChunker.SplitIntoLogicBlocks(fn, lines, config)
	if len(logicBlocks) == 0 {
		// ブロックが見つからない場合は関数チャンクのみを返す
		return chunks, excluded
	}

	// 孫チャンクを生成
	logicChunks := logicChunker.GenerateLogicChunks(fn, funcChunk.Metadata, lines, logicBlocks, chunkCounter, config)

	// 孫チャンクを追加
	chunks = append(chunks, logicChunks...)

	return chunks, excluded
}

// extractFunctionWithLogicSplittingDetailed は関数を抽出し、必要に応じてロジック単位に分割します（詳細版）
// 詳細な依存関係情報を含む
func (ac *ASTChunkerGo) extractFunctionWithLogicSplittingDetailed(fn *ast.FuncDecl, file *ast.File, lines []string, importInfo *ImportInfo, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) ([]*ChunkWithMetadata, bool) {
	// まず詳細な関数チャンクを生成
	funcChunk, excluded := ac.extractFunctionWithMetricsDetailed(fn, file, lines, importInfo, chunkCounter)
	if funcChunk == nil {
		return nil, excluded
	}

	chunks := []*ChunkWithMetadata{funcChunk}

	// ロジック分割が必要かチェック
	logicChunker := NewLogicChunker(ac.fset)
	config := DefaultSplitConfig()

	complexity := 0
	if funcChunk.Metadata.CyclomaticComplexity != nil {
		complexity = *funcChunk.Metadata.CyclomaticComplexity
	}

	if !logicChunker.ShouldSplit(fn, complexity, config) {
		// 分割不要の場合は関数チャンクのみを返す
		return chunks, excluded
	}

	// ロジック単位に分割
	logicBlocks := logicChunker.SplitIntoLogicBlocks(fn, lines, config)
	if len(logicBlocks) == 0 {
		// ブロックが見つからない場合は関数チャンクのみを返す
		return chunks, excluded
	}

	// 孫チャンクを生成
	logicChunks := logicChunker.GenerateLogicChunks(fn, funcChunk.Metadata, lines, logicBlocks, chunkCounter, config)

	// 孫チャンクを追加
	chunks = append(chunks, logicChunks...)

	return chunks, excluded
}

// extractGenDecl は型定義、変数、定数を抽出します（後方互換性のため）
func (ac *ASTChunkerGo) extractGenDecl(decl *ast.GenDecl, file *ast.File, lines []string, imports []string, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) []*ChunkWithMetadata {
	chunks, _ := ac.extractGenDeclWithMetrics(decl, file, lines, imports, chunkCounter)
	return chunks
}

// extractGenDeclWithMetrics は型定義、変数、定数を抽出し、除外数を返します
func (ac *ASTChunkerGo) extractGenDeclWithMetrics(decl *ast.GenDecl, file *ast.File, lines []string, imports []string, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) ([]*ChunkWithMetadata, int) {
	var chunks []*ChunkWithMetadata
	excludedCount := 0

	// 各specを処理
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			// 型定義を処理
			chunk, excluded := ac.extractTypeSpecWithMetrics(s, decl, lines, imports, chunkCounter)
			if chunk != nil {
				chunks = append(chunks, chunk)
			}
			if excluded {
				excludedCount++
			}
		case *ast.ValueSpec:
			// 変数・定数を処理
			chunk, excluded := ac.extractValueSpecWithMetrics(s, decl, lines, imports, chunkCounter)
			if chunk != nil {
				chunks = append(chunks, chunk)
			}
			if excluded {
				excludedCount++
			}
		}
	}

	return chunks, excludedCount
}

// extractGenDeclWithMetricsDetailed は型定義、変数、定数を抽出し、除外数を返します（詳細版）
func (ac *ASTChunkerGo) extractGenDeclWithMetricsDetailed(decl *ast.GenDecl, file *ast.File, lines []string, importInfo *ImportInfo, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) ([]*ChunkWithMetadata, int) {
	var chunks []*ChunkWithMetadata
	excludedCount := 0

	// 各specを処理
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			// 型定義を処理
			chunk, excluded := ac.extractTypeSpecWithMetricsDetailed(s, decl, lines, importInfo, chunkCounter)
			if chunk != nil {
				chunks = append(chunks, chunk)
			}
			if excluded {
				excludedCount++
			}
		case *ast.ValueSpec:
			// 変数・定数を処理
			chunk, excluded := ac.extractValueSpecWithMetricsDetailed(s, decl, lines, importInfo, chunkCounter)
			if chunk != nil {
				chunks = append(chunks, chunk)
			}
			if excluded {
				excludedCount++
			}
		}
	}

	return chunks, excludedCount
}

// extractTypeSpec は型定義を抽出します（後方互換性のため）
func (ac *ASTChunkerGo) extractTypeSpec(spec *ast.TypeSpec, decl *ast.GenDecl, lines []string, imports []string, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) *ChunkWithMetadata {
	chunk, _ := ac.extractTypeSpecWithMetrics(spec, decl, lines, imports, chunkCounter)
	return chunk
}

// extractTypeSpecWithMetrics は型定義を抽出し、除外されたかどうかを返します
func (ac *ASTChunkerGo) extractTypeSpecWithMetrics(spec *ast.TypeSpec, decl *ast.GenDecl, lines []string, imports []string, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) (*ChunkWithMetadata, bool) {
	startPos := ac.fset.Position(decl.Pos())
	endPos := ac.fset.Position(decl.End())

	content := ac.extractContent(lines, startPos.Line, endPos.Line)
	tokens := chunkCounter.CountTokens(content)

	// トークンサイズ検証
	// 型定義は最小トークン数5に緩和（小さなstructも含める）
	minTokensForAST := 5
	if tokens < minTokensForAST || tokens > 1600 {
		return nil, false
	}

	typeName := spec.Name.Name
	var typeKind string

	switch spec.Type.(type) {
	case *ast.StructType:
		typeKind = "struct"
	case *ast.InterfaceType:
		typeKind = "interface"
	default:
		typeKind = "type"
	}

	// DocCommentを抽出
	var docComment *string
	if decl.Doc != nil {
		doc := decl.Doc.Text()
		docComment = &doc
	}

	// 品質メトリクス計測
	loc := ac.calculateLinesOfCode(content)
	commentRatio := ac.calculateCommentRatio(content)

	// コメント比率95%以上の場合は除外
	if commentRatio > 0.95 {
		return nil, true // 除外された
	}

	return &ChunkWithMetadata{
		Chunk: &Chunk{
			Content:   content,
			StartLine: startPos.Line,
			EndLine:   endPos.Line,
			Tokens:    tokens,
		},
		Metadata: &ChunkMetadata{
			Type:         &typeKind,
			Name:         &typeName,
			DocComment:   docComment,
			Imports:      imports,
			LinesOfCode:  &loc,
			CommentRatio: &commentRatio,
		},
	}, false // 除外されていない
}

// extractTypeSpecWithMetricsDetailed は型定義を抽出し、詳細な依存関係情報を含めます
func (ac *ASTChunkerGo) extractTypeSpecWithMetricsDetailed(spec *ast.TypeSpec, decl *ast.GenDecl, lines []string, importInfo *ImportInfo, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) (*ChunkWithMetadata, bool) {
	startPos := ac.fset.Position(decl.Pos())
	endPos := ac.fset.Position(decl.End())

	content := ac.extractContent(lines, startPos.Line, endPos.Line)
	tokens := chunkCounter.CountTokens(content)

	// トークンサイズ検証
	minTokensForAST := 5
	if tokens < minTokensForAST || tokens > 1600 {
		return nil, false
	}

	typeName := spec.Name.Name
	var typeKind string

	switch spec.Type.(type) {
	case *ast.StructType:
		typeKind = "struct"
	case *ast.InterfaceType:
		typeKind = "interface"
	default:
		typeKind = "type"
	}

	// DocCommentを抽出
	var docComment *string
	if decl.Doc != nil {
		doc := decl.Doc.Text()
		docComment = &doc
	}

	// 品質メトリクス計測
	loc := ac.calculateLinesOfCode(content)
	commentRatio := ac.calculateCommentRatio(content)

	// コメント比率95%以上の場合は除外
	if commentRatio > 0.95 {
		return nil, true // 除外された
	}

	return &ChunkWithMetadata{
		Chunk: &Chunk{
			Content:   content,
			StartLine: startPos.Line,
			EndLine:   endPos.Line,
			Tokens:    tokens,
		},
		Metadata: &ChunkMetadata{
			Type:         &typeKind,
			Name:         &typeName,
			DocComment:   docComment,
			Imports:      importInfo.All,
			LinesOfCode:  &loc,
			CommentRatio: &commentRatio,
			// 詳細な依存関係情報
			StandardImports: importInfo.Standard,
			ExternalImports: importInfo.External,
		},
	}, false // 除外されていない
}

// extractValueSpec は変数・定数を抽出します（後方互換性のため）
func (ac *ASTChunkerGo) extractValueSpec(spec *ast.ValueSpec, decl *ast.GenDecl, lines []string, imports []string, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) *ChunkWithMetadata {
	chunk, _ := ac.extractValueSpecWithMetrics(spec, decl, lines, imports, chunkCounter)
	return chunk
}

// extractValueSpecWithMetrics は変数・定数を抽出し、除外されたかどうかを返します
func (ac *ASTChunkerGo) extractValueSpecWithMetrics(spec *ast.ValueSpec, decl *ast.GenDecl, lines []string, imports []string, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) (*ChunkWithMetadata, bool) {
	startPos := ac.fset.Position(decl.Pos())
	endPos := ac.fset.Position(decl.End())

	content := ac.extractContent(lines, startPos.Line, endPos.Line)
	tokens := chunkCounter.CountTokens(content)

	// トークンサイズ検証
	// 変数・定数は最小トークン数10に緩和
	minTokensForAST := 10
	if tokens < minTokensForAST || tokens > 1600 {
		return nil, false
	}

	// 名前を抽出（複数の変数が同時に宣言されている場合は最初の名前を使用）
	var name string
	if len(spec.Names) > 0 {
		name = spec.Names[0].Name
	} else {
		return nil, false
	}

	var typeKind string
	if decl.Tok == token.CONST {
		typeKind = "const"
	} else {
		typeKind = "var"
	}

	// DocCommentを抽出
	var docComment *string
	if decl.Doc != nil {
		doc := decl.Doc.Text()
		docComment = &doc
	}

	// 品質メトリクス計測
	loc := ac.calculateLinesOfCode(content)
	commentRatio := ac.calculateCommentRatio(content)

	// コメント比率95%以上の場合は除外
	if commentRatio > 0.95 {
		return nil, true // 除外された
	}

	return &ChunkWithMetadata{
		Chunk: &Chunk{
			Content:   content,
			StartLine: startPos.Line,
			EndLine:   endPos.Line,
			Tokens:    tokens,
		},
		Metadata: &ChunkMetadata{
			Type:         &typeKind,
			Name:         &name,
			DocComment:   docComment,
			Imports:      imports,
			LinesOfCode:  &loc,
			CommentRatio: &commentRatio,
		},
	}, false // 除外されていない
}

// extractValueSpecWithMetricsDetailed は変数・定数を抽出し、詳細な依存関係情報を含めます
func (ac *ASTChunkerGo) extractValueSpecWithMetricsDetailed(spec *ast.ValueSpec, decl *ast.GenDecl, lines []string, importInfo *ImportInfo, chunkCounter interface {
	CountTokens(string) int
	TrimToTokenLimit(string, int) string
}) (*ChunkWithMetadata, bool) {
	startPos := ac.fset.Position(decl.Pos())
	endPos := ac.fset.Position(decl.End())

	content := ac.extractContent(lines, startPos.Line, endPos.Line)
	tokens := chunkCounter.CountTokens(content)

	// トークンサイズ検証
	minTokensForAST := 10
	if tokens < minTokensForAST || tokens > 1600 {
		return nil, false
	}

	// 名前を抽出（複数の変数が同時に宣言されている場合は最初の名前を使用）
	var name string
	if len(spec.Names) > 0 {
		name = spec.Names[0].Name
	} else {
		return nil, false
	}

	var typeKind string
	if decl.Tok == token.CONST {
		typeKind = "const"
	} else {
		typeKind = "var"
	}

	// DocCommentを抽出
	var docComment *string
	if decl.Doc != nil {
		doc := decl.Doc.Text()
		docComment = &doc
	}

	// 品質メトリクス計測
	loc := ac.calculateLinesOfCode(content)
	commentRatio := ac.calculateCommentRatio(content)

	// コメント比率95%以上の場合は除外
	if commentRatio > 0.95 {
		return nil, true // 除外された
	}

	return &ChunkWithMetadata{
		Chunk: &Chunk{
			Content:   content,
			StartLine: startPos.Line,
			EndLine:   endPos.Line,
			Tokens:    tokens,
		},
		Metadata: &ChunkMetadata{
			Type:         &typeKind,
			Name:         &name,
			DocComment:   docComment,
			Imports:      importInfo.All,
			LinesOfCode:  &loc,
			CommentRatio: &commentRatio,
			// 詳細な依存関係情報
			StandardImports: importInfo.Standard,
			ExternalImports: importInfo.External,
		},
	}, false // 除外されていない
}

// extractTypeName は型名を抽出します
func (ac *ASTChunkerGo) extractTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + ac.extractTypeName(t.X)
	default:
		return "unknown"
	}
}

// buildFunctionSignature は関数のシグネチャを構築します
func (ac *ASTChunkerGo) buildFunctionSignature(fn *ast.FuncDecl) string {
	var parts []string

	// 関数名
	parts = append(parts, fn.Name.Name)

	// パラメータ
	params := ac.formatFieldList(fn.Type.Params)
	parts = append(parts, fmt.Sprintf("(%s)", params))

	// 戻り値
	if fn.Type.Results != nil {
		results := ac.formatFieldList(fn.Type.Results)
		if results != "" {
			parts = append(parts, results)
		}
	}

	return strings.Join(parts, " ")
}

// formatFieldList はフィールドリストをフォーマットします
func (ac *ASTChunkerGo) formatFieldList(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}

	var parts []string
	for _, field := range fields.List {
		typeName := ac.formatExpr(field.Type)
		if len(field.Names) > 0 {
			for _, name := range field.Names {
				parts = append(parts, fmt.Sprintf("%s %s", name.Name, typeName))
			}
		} else {
			parts = append(parts, typeName)
		}
	}

	return strings.Join(parts, ", ")
}

// formatExpr は式を文字列にフォーマットします
func (ac *ASTChunkerGo) formatExpr(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + ac.formatExpr(t.X)
	case *ast.ArrayType:
		return "[]" + ac.formatExpr(t.Elt)
	case *ast.SelectorExpr:
		return ac.formatExpr(t.X) + "." + t.Sel.Name
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", ac.formatExpr(t.Key), ac.formatExpr(t.Value))
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return "unknown"
	}
}

// extractFunctionCalls は関数内の呼び出しを抽出します（簡易版）
func (ac *ASTChunkerGo) extractFunctionCalls(fn *ast.FuncDecl) []string {
	calls := make(map[string]bool)

	ast.Inspect(fn, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				calls[fun.Name] = true
			case *ast.SelectorExpr:
				calls[fun.Sel.Name] = true
			}
		}
		return true
	})

	result := make([]string, 0, len(calls))
	for call := range calls {
		result = append(result, call)
	}
	return result
}

// calculateLinesOfCode はコメント・空行を除外した行数を計算します
func (ac *ASTChunkerGo) calculateLinesOfCode(content string) int {
	lines := strings.Split(content, "\n")
	loc := 0

	inBlockComment := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 空行をスキップ
		if trimmed == "" {
			continue
		}

		// ブロックコメントの開始
		if strings.HasPrefix(trimmed, "/*") {
			inBlockComment = true
		}

		// ブロックコメント内
		if inBlockComment {
			if strings.Contains(trimmed, "*/") {
				inBlockComment = false
			}
			continue
		}

		// 行コメントをスキップ
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		loc++
	}

	return loc
}

// calculateCommentRatio はコメント行の割合を計算します
func (ac *ASTChunkerGo) calculateCommentRatio(content string) float64 {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return 0.0
	}

	commentLines := 0
	inBlockComment := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 空行はカウントしない
		if trimmed == "" {
			continue
		}

		// ブロックコメントの開始
		if strings.HasPrefix(trimmed, "/*") {
			inBlockComment = true
			commentLines++
			if strings.Contains(trimmed, "*/") {
				inBlockComment = false
			}
			continue
		}

		// ブロックコメント内
		if inBlockComment {
			commentLines++
			if strings.Contains(trimmed, "*/") {
				inBlockComment = false
			}
			continue
		}

		// 行コメント
		if strings.HasPrefix(trimmed, "//") {
			commentLines++
			continue
		}
	}

	// 空行を除いた総行数
	totalLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			totalLines++
		}
	}

	if totalLines == 0 {
		return 0.0
	}

	return float64(commentLines) / float64(totalLines)
}

// calculateCyclomaticComplexity はMcCabe複雑度を計算します
func (ac *ASTChunkerGo) calculateCyclomaticComplexity(fn *ast.FuncDecl) int {
	complexity := 1 // ベースライン

	ast.Inspect(fn, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt:
			complexity++
		case *ast.ForStmt:
			complexity++
		case *ast.RangeStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		case *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			// && や || は分岐点としてカウント
			if expr, ok := n.(*ast.BinaryExpr); ok {
				if expr.Op == token.LAND || expr.Op == token.LOR {
					complexity++
				}
			}
		}
		return true
	})

	return complexity
}

// extractContent は指定行範囲のコンテンツを抽出します
func (ac *ASTChunkerGo) extractContent(lines []string, startLine, endLine int) string {
	if startLine < 1 || endLine > len(lines) || startLine > endLine {
		return ""
	}

	// 1-indexedから0-indexedに変換
	start := startLine - 1
	end := endLine

	return strings.Join(lines[start:end], "\n")
}

// stringPtr は文字列のポインタを返します
func stringPtr(s string) *string {
	return &s
}

// extractTypeDependencies は型依存を抽出します
func (ac *ASTChunkerGo) extractTypeDependencies(fn *ast.FuncDecl) []string {
	typeDeps := make(map[string]bool)

	// 関数シグネチャの型を抽出
	if fn.Type.Params != nil {
		for _, param := range fn.Type.Params.List {
			typeStr := ac.extractTypeString(param.Type)
			if typeStr != "" && !isBuiltinType(typeStr) {
				typeDeps[typeStr] = true
			}
		}
	}

	// 戻り値の型を抽出
	if fn.Type.Results != nil {
		for _, result := range fn.Type.Results.List {
			typeStr := ac.extractTypeString(result.Type)
			if typeStr != "" && !isBuiltinType(typeStr) {
				typeDeps[typeStr] = true
			}
		}
	}

	// 関数本体内の型参照を抽出
	if fn.Body != nil {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CompositeLit:
				// 構造体リテラル
				typeStr := ac.extractTypeString(node.Type)
				if typeStr != "" && !isBuiltinType(typeStr) {
					typeDeps[typeStr] = true
				}
			case *ast.CallExpr:
				// 型変換
				if ident, ok := node.Fun.(*ast.Ident); ok {
					if !isBuiltinType(ident.Name) {
						typeDeps[ident.Name] = true
					}
				}
			}
			return true
		})
	}

	// マップをスライスに変換
	result := make([]string, 0, len(typeDeps))
	for typeName := range typeDeps {
		result = append(result, typeName)
	}
	return result
}

// extractTypeString はast.Exprから型名を文字列として抽出します
func (ac *ASTChunkerGo) extractTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		// pkg.Type 形式
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
	case *ast.StarExpr:
		// ポインタ型
		return "*" + ac.extractTypeString(t.X)
	case *ast.ArrayType:
		// 配列/スライス型
		return "[]" + ac.extractTypeString(t.Elt)
	case *ast.MapType:
		// マップ型
		return "map[" + ac.extractTypeString(t.Key) + "]" + ac.extractTypeString(t.Value)
	}
	return ""
}

// isBuiltinType は組み込み型かどうかを判定します
func isBuiltinType(typeName string) bool {
	builtins := []string{
		"bool", "byte", "rune", "string",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128",
		"error",
	}
	for _, b := range builtins {
		if typeName == b {
			return true
		}
	}
	return false
}
