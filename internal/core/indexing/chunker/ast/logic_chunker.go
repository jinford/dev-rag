package ast

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// LogicChunker は大きな関数をロジック単位に分割します
type LogicChunker struct {
	fset *token.FileSet
}

// NewLogicChunker は新しいLogicChunkerを作成します
func NewLogicChunker(fset *token.FileSet) *LogicChunker {
	return &LogicChunker{
		fset: fset,
	}
}

// SplitConfig はロジック分割の設定を表します
type SplitConfig struct {
	// 分割閾値
	LineThreshold       int // 行数閾値（デフォルト: 100行）
	ComplexityThreshold int // 循環的複雑度閾値（デフォルト: 15）

	// チャンクサイズ制約
	MinTokens int // 最小トークン数（デフォルト: 50）
	MaxTokens int // 最大トークン数（デフォルト: 800）
}

// DefaultSplitConfig はデフォルトの分割設定を返します
func DefaultSplitConfig() *SplitConfig {
	return &SplitConfig{
		LineThreshold:       100,
		ComplexityThreshold: 15,
		MinTokens:           50,
		MaxTokens:           800,
	}
}

// ShouldSplit は関数が分割対象かどうかを判定します
func (lc *LogicChunker) ShouldSplit(fn *ast.FuncDecl, complexity int, config *SplitConfig) bool {
	if config == nil {
		config = DefaultSplitConfig()
	}

	startPos := lc.fset.Position(fn.Pos())
	endPos := lc.fset.Position(fn.End())
	lines := endPos.Line - startPos.Line + 1

	// 行数または循環的複雑度が閾値を超える場合に分割
	return lines >= config.LineThreshold || complexity >= config.ComplexityThreshold
}

// LogicBlock は関数内の論理ブロックを表します
type LogicBlock struct {
	Type      string    // ブロックの種類（initialization, loop, error_handling, など）
	StartPos  token.Pos // 開始位置
	EndPos    token.Pos // 終了位置
	StartLine int       // 開始行
	EndLine   int       // 終了行
	Depth     int       // ネストの深さ
	Comment   string    // ブロック前のコメント
}

// SplitIntoLogicBlocks は関数を論理ブロックに分割します
// 意味のあるまとまり（セクション）ごとにグループ化します
func (lc *LogicChunker) SplitIntoLogicBlocks(fn *ast.FuncDecl, lines []string, config *SplitConfig) []*LogicBlock {
	if config == nil {
		config = DefaultSplitConfig()
	}

	blocks := make([]*LogicBlock, 0)

	// 関数本体がない場合は空を返す
	if fn.Body == nil {
		return blocks
	}

	// ステートメントをグループ化する
	// 1. コメントで区切られたセクション
	// 2. 重要な構造（if、for、switch）
	// 3. 連続する代入文・初期化

	stmtGroups := lc.groupStatements(fn.Body.List, lines)

	// 各グループをブロックに変換
	for _, group := range stmtGroups {
		if len(group.Stmts) == 0 {
			continue
		}

		firstStmt := group.Stmts[0]
		lastStmt := group.Stmts[len(group.Stmts)-1]

		startPos := lc.fset.Position(firstStmt.Pos())
		endPos := lc.fset.Position(lastStmt.End())

		blocks = append(blocks, &LogicBlock{
			Type:      group.Type,
			StartPos:  firstStmt.Pos(),
			EndPos:    lastStmt.End(),
			StartLine: startPos.Line,
			EndLine:   endPos.Line,
			Depth:     0,
			Comment:   group.Comment,
		})
	}

	// ブロックが空の場合、関数全体を1つのブロックとして扱う
	if len(blocks) == 0 {
		startPos := lc.fset.Position(fn.Body.Pos())
		endPos := lc.fset.Position(fn.Body.End())
		blocks = append(blocks, &LogicBlock{
			Type:      "main_logic",
			StartPos:  fn.Body.Pos(),
			EndPos:    fn.Body.End(),
			StartLine: startPos.Line,
			EndLine:   endPos.Line,
			Depth:     0,
			Comment:   "",
		})
	}

	return blocks
}

// StmtGroup はステートメントのグループを表します
type StmtGroup struct {
	Type    string
	Stmts   []ast.Stmt
	Comment string
}

// groupStatements はステートメントを意味のあるグループに分割します
func (lc *LogicChunker) groupStatements(stmts []ast.Stmt, lines []string) []*StmtGroup {
	groups := make([]*StmtGroup, 0)
	currentGroup := &StmtGroup{Type: "initialization", Stmts: make([]ast.Stmt, 0)}
	lastComment := ""

	for i, stmt := range stmts {
		// コメントをチェック
		if i > 0 {
			prevEnd := lc.fset.Position(stmts[i-1].End()).Line
			currStart := lc.fset.Position(stmt.Pos()).Line
			comment := lc.extractCommentBetween(lines, prevEnd, currStart)
			if comment != "" {
				// コメントが見つかった場合、新しいグループを開始
				if len(currentGroup.Stmts) > 0 {
					groups = append(groups, currentGroup)
				}
				lastComment = comment
				currentGroup = &StmtGroup{Type: "unknown", Stmts: make([]ast.Stmt, 0), Comment: lastComment}
			}
		}

		// ステートメントのタイプを判定
		stmtType := lc.getStatementType(stmt)

		// 重要な構造は独立したグループとする
		if lc.isSignificantStatement(stmt) {
			// 現在のグループを確定
			if len(currentGroup.Stmts) > 0 {
				groups = append(groups, currentGroup)
			}

			// 重要なステートメントを単独グループとして追加
			groups = append(groups, &StmtGroup{
				Type:    stmtType,
				Stmts:   []ast.Stmt{stmt},
				Comment: lastComment,
			})
			lastComment = ""
			currentGroup = &StmtGroup{Type: "unknown", Stmts: make([]ast.Stmt, 0)}
			continue
		}

		// グループタイプが変わった場合
		if currentGroup.Type == "unknown" {
			currentGroup.Type = stmtType
		}

		// 同じタイプのステートメントを同じグループに追加
		currentGroup.Stmts = append(currentGroup.Stmts, stmt)
	}

	// 最後のグループを追加
	if len(currentGroup.Stmts) > 0 {
		groups = append(groups, currentGroup)
	}

	return groups
}

// isSignificantStatement は重要な構造かどうかを判定します
func (lc *LogicChunker) isSignificantStatement(stmt ast.Stmt) bool {
	switch stmt.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return true
	default:
		return false
	}
}

// getStatementType はステートメントのタイプを取得します
func (lc *LogicChunker) getStatementType(stmt ast.Stmt) string {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		if lc.isErrorHandling(s) {
			return "error_handling"
		}
		return "conditional"
	case *ast.ForStmt, *ast.RangeStmt:
		return "loop"
	case *ast.SwitchStmt, *ast.TypeSwitchStmt:
		return "switch"
	case *ast.SelectStmt:
		return "channel_select"
	case *ast.DeferStmt:
		return "defer"
	case *ast.ReturnStmt:
		return "return"
	case *ast.AssignStmt:
		if lc.isInitialization(s) {
			return "initialization"
		}
		return "assignment"
	default:
		return "statement"
	}
}

// identifyLogicBlock はステートメントから論理ブロックを識別します
func (lc *LogicChunker) identifyLogicBlock(stmt ast.Stmt, depth int, comment string) *LogicBlock {
	startPos := lc.fset.Position(stmt.Pos())
	endPos := lc.fset.Position(stmt.End())

	block := &LogicBlock{
		StartPos:  stmt.Pos(),
		EndPos:    stmt.End(),
		StartLine: startPos.Line,
		EndLine:   endPos.Line,
		Depth:     depth,
		Comment:   comment,
	}

	switch s := stmt.(type) {
	case *ast.IfStmt:
		// if文：条件分岐
		block.Type = "conditional"
		// エラーハンドリングパターンを検出
		if lc.isErrorHandling(s) {
			block.Type = "error_handling"
		}

	case *ast.ForStmt, *ast.RangeStmt:
		// ループ処理
		block.Type = "loop"

	case *ast.SwitchStmt, *ast.TypeSwitchStmt:
		// switch文
		block.Type = "switch"

	case *ast.SelectStmt:
		// select文（チャネル操作）
		block.Type = "channel_select"

	case *ast.DeferStmt:
		// defer文
		block.Type = "defer"

	case *ast.ReturnStmt:
		// return文
		block.Type = "return"

	case *ast.AssignStmt:
		// 代入文：初期化パターンを検出
		if lc.isInitialization(s) {
			block.Type = "initialization"
		} else {
			block.Type = "assignment"
		}

	case *ast.BlockStmt:
		// ブロック文
		block.Type = "block"

	default:
		// その他のステートメント
		block.Type = "statement"
	}

	return block
}

// isErrorHandling はif文がエラーハンドリングかどうかを判定します
func (lc *LogicChunker) isErrorHandling(ifStmt *ast.IfStmt) bool {
	// パターン1: if err != nil
	if binExpr, ok := ifStmt.Cond.(*ast.BinaryExpr); ok {
		if binExpr.Op == token.NEQ {
			if ident, ok := binExpr.X.(*ast.Ident); ok {
				if ident.Name == "err" {
					return true
				}
			}
		}
	}

	// パターン2: if err := ... ; err != nil
	if ifStmt.Init != nil {
		if assignStmt, ok := ifStmt.Init.(*ast.AssignStmt); ok {
			for _, lhs := range assignStmt.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					if ident.Name == "err" {
						return true
					}
				}
			}
		}
	}

	return false
}

// isInitialization は代入文が初期化パターンかどうかを判定します
func (lc *LogicChunker) isInitialization(assignStmt *ast.AssignStmt) bool {
	// := による短縮変数宣言は初期化と判定
	return assignStmt.Tok == token.DEFINE
}

// extractCommentBetween は指定行範囲のコメントを抽出します
func (lc *LogicChunker) extractCommentBetween(lines []string, startLine, endLine int) string {
	if startLine < 1 || endLine > len(lines) || startLine >= endLine {
		return ""
	}

	var comments []string
	for i := startLine; i < endLine && i <= len(lines); i++ {
		line := strings.TrimSpace(lines[i-1])
		if strings.HasPrefix(line, "//") {
			comments = append(comments, line)
		}
	}

	return strings.Join(comments, "\n")
}

// GenerateLogicChunks は論理ブロックから孫チャンクを生成します
func (lc *LogicChunker) GenerateLogicChunks(
	fn *ast.FuncDecl,
	parentMetadata *ChunkMetadata,
	lines []string,
	blocks []*LogicBlock,
	chunkCounter TokenCounter,
	config *SplitConfig,
) []*ChunkWithMetadata {
	if config == nil {
		config = DefaultSplitConfig()
	}

	logicChunks := make([]*ChunkWithMetadata, 0)

	for _, block := range blocks {
		// ブロックのコンテンツを抽出
		content := lc.extractContent(lines, block.StartLine, block.EndLine)
		if content == "" {
			continue
		}

		// トークン数をチェック
		tokens := chunkCounter.CountTokens(content)

		// トークンサイズ検証
		if tokens < config.MinTokens || tokens > config.MaxTokens {
			// トークン数が範囲外の場合はスキップ
			continue
		}

		// 孫チャンクのメタデータを構築
		logicType := fmt.Sprintf("logic_%s", block.Type)
		logicName := fmt.Sprintf("%s_%s_%d", *parentMetadata.Name, block.Type, block.StartLine)

		chunkMeta := &ChunkMetadata{
			Type:       &logicType,
			Name:       &logicName,
			ParentName: parentMetadata.Name,  // 親関数名を継承
			Signature:  parentMetadata.Signature, // 親関数のシグネチャを継承
			Level:      3, // レベル3: ロジック単位
		}

		// DocCommentがあれば追加
		if block.Comment != "" {
			chunkMeta.DocComment = &block.Comment
		}

		logicChunks = append(logicChunks, &ChunkWithMetadata{
			Chunk: &Chunk{
				Content:   content,
				StartLine: block.StartLine,
				EndLine:   block.EndLine,
				Tokens:    tokens,
			},
			Metadata: chunkMeta,
		})
	}

	return logicChunks
}

// extractContent は指定行範囲のコンテンツを抽出します
func (lc *LogicChunker) extractContent(lines []string, startLine, endLine int) string {
	if startLine < 1 || endLine > len(lines) || startLine > endLine {
		return ""
	}

	// 1-indexedから0-indexedに変換
	start := startLine - 1
	end := endLine

	return strings.Join(lines[start:end], "\n")
}
