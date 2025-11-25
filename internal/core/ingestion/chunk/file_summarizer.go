package chunk

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// FileSummarizer はファイル全体のサマリーを生成します（ルールベース版）
type FileSummarizer struct {
	maxTokens int // サマリーの最大トークン数（デフォルト: 400）
}

// NewFileSummarizer は新しいFileSummarizerを作成します
func NewFileSummarizer() *FileSummarizer {
	return &FileSummarizer{
		maxTokens: 400,
	}
}

// FileSummary はファイルサマリーの情報を保持します
type FileSummary struct {
	Language       string
	MainComponents []ComponentInfo
	Dependencies   []string
	TopComment     string
}

// ComponentInfo はファイル内のコンポーネント（関数、クラスなど）の情報を保持します
type ComponentInfo struct {
	Type string // "Function", "Method", "Struct", "Interface", etc.
	Name string
}

// GenerateSummary はファイル全体のサマリーを生成します
func (fs *FileSummarizer) GenerateSummary(content, language string, chunker *DefaultChunker) (string, error) {
	// Go言語の場合はAST解析
	if language == "go" {
		return fs.generateGoSummary(content, chunker)
	}

	// その他の言語の場合は簡易サマリー
	return fs.generateSimpleSummary(content, language, chunker)
}

// generateGoSummary はGo言語のサマリーを生成します
func (fs *FileSummarizer) generateGoSummary(content string, chunker *DefaultChunker) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		// AST解析失敗時は簡易サマリーにフォールバック
		return fs.generateSimpleSummary(content, "go", chunker)
	}

	summary := FileSummary{
		Language:       "Go",
		MainComponents: []ComponentInfo{},
		Dependencies:   []string{},
	}

	// トップレベルコメントの抽出
	if file.Doc != nil {
		summary.TopComment = strings.TrimSpace(file.Doc.Text())
	}

	// インポート情報の抽出
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		summary.Dependencies = append(summary.Dependencies, path)
	}

	// 主要コンポーネントの抽出
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			// 関数・メソッド
			if d.Recv != nil && len(d.Recv.List) > 0 {
				// メソッド
				summary.MainComponents = append(summary.MainComponents, ComponentInfo{
					Type: "Method",
					Name: d.Name.Name,
				})
			} else {
				// 関数
				summary.MainComponents = append(summary.MainComponents, ComponentInfo{
					Type: "Function",
					Name: d.Name.Name,
				})
			}
		case *ast.GenDecl:
			// 型定義、変数、定数
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					var typeKind string
					switch s.Type.(type) {
					case *ast.StructType:
						typeKind = "Struct"
					case *ast.InterfaceType:
						typeKind = "Interface"
					default:
						typeKind = "Type"
					}
					summary.MainComponents = append(summary.MainComponents, ComponentInfo{
						Type: typeKind,
						Name: s.Name.Name,
					})
				}
			}
		}
	}

	// サマリーテキストの構築
	return fs.buildSummaryText(summary, chunker)
}

// generateSimpleSummary は簡易サマリーを生成します（AST解析が使えない場合）
func (fs *FileSummarizer) generateSimpleSummary(content, language string, chunker *DefaultChunker) (string, error) {
	summary := FileSummary{
		Language:       language,
		MainComponents: []ComponentInfo{},
		Dependencies:   []string{},
	}

	lines := strings.Split(content, "\n")

	// 先頭のコメントブロックを取得（最大10行）
	commentLines := []string{}
	for i, line := range lines {
		if i >= 10 {
			break
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			commentLines = append(commentLines, strings.TrimLeft(trimmed, "/*"))
		} else if trimmed == "" {
			if len(commentLines) > 0 {
				break
			}
		} else {
			break
		}
	}
	if len(commentLines) > 0 {
		summary.TopComment = strings.Join(commentLines, "\n")
	}

	// 簡易的な関数検出（正規表現の代わりにキーワード検索）
	funcKeywords := []string{"func ", "function ", "def ", "class "}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, keyword := range funcKeywords {
			if strings.HasPrefix(trimmed, keyword) {
				// 関数名を抽出（簡易版）
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					name := parts[1]
					// 括弧などを除去
					name = strings.TrimSuffix(name, "(")
					summary.MainComponents = append(summary.MainComponents, ComponentInfo{
						Type: "Function",
						Name: name,
					})
				}
				break
			}
		}
	}

	// サマリーテキストの構築
	return fs.buildSummaryText(summary, chunker)
}

// buildSummaryText はFileSummaryからサマリーテキストを構築します
func (fs *FileSummarizer) buildSummaryText(summary FileSummary, chunker *DefaultChunker) (string, error) {
	var builder strings.Builder

	// Language情報
	builder.WriteString(fmt.Sprintf("Language: %s\n\n", summary.Language))

	// Main Components
	if len(summary.MainComponents) > 0 {
		builder.WriteString("Main Components:\n")

		// コンポーネントを種類でグループ化
		componentsByType := make(map[string][]string)
		for _, comp := range summary.MainComponents {
			componentsByType[comp.Type] = append(componentsByType[comp.Type], comp.Name)
		}

		// 種類ごとにソートして出力
		types := make([]string, 0, len(componentsByType))
		for t := range componentsByType {
			types = append(types, t)
		}
		sort.Strings(types)

		for _, t := range types {
			names := componentsByType[t]
			for _, name := range names {
				builder.WriteString(fmt.Sprintf("- %s: %s\n", t, name))

				// トークン数をチェック
				if chunker.countTokens(builder.String()) > fs.maxTokens {
					// 最大トークン数を超えたら、最後の行を削除して終了
					lines := strings.Split(builder.String(), "\n")
					if len(lines) > 1 {
						builder.Reset()
						builder.WriteString(strings.Join(lines[:len(lines)-1], "\n"))
						builder.WriteString("\n... (truncated)\n")
					}
					goto BuildDependencies
				}
			}
		}
		builder.WriteString("\n")
	}

BuildDependencies:
	// Dependencies
	if len(summary.Dependencies) > 0 {
		builder.WriteString("Dependencies:\n")

		// 標準ライブラリと外部依存を分離
		stdDeps := []string{}
		extDeps := []string{}

		for _, dep := range summary.Dependencies {
			// Go言語の場合: ドットがない、または "golang.org" で始まるものは標準ライブラリ
			if !strings.Contains(dep, ".") || strings.HasPrefix(dep, "golang.org") {
				stdDeps = append(stdDeps, dep)
			} else {
				extDeps = append(extDeps, dep)
			}
		}

		// 外部依存を優先的に表示（重要度が高いため）
		sort.Strings(extDeps)
		for _, dep := range extDeps {
			line := fmt.Sprintf("- %s\n", dep)
			builder.WriteString(line)

			// トークン数をチェック
			if chunker.countTokens(builder.String()) > fs.maxTokens {
				// 最大トークン数を超えたら、最後の行を削除して終了
				lines := strings.Split(builder.String(), "\n")
				if len(lines) > 1 {
					builder.Reset()
					builder.WriteString(strings.Join(lines[:len(lines)-1], "\n"))
					builder.WriteString("\n... (truncated)\n")
				}
				goto BuildTopComment
			}
		}

		// 標準ライブラリは省略されやすい（トークン制限内であれば表示）
		sort.Strings(stdDeps)
		for _, dep := range stdDeps {
			line := fmt.Sprintf("- %s\n", dep)
			builder.WriteString(line)

			// トークン数をチェック
			if chunker.countTokens(builder.String()) > fs.maxTokens {
				// 最大トークン数を超えたら、最後の行を削除して終了
				lines := strings.Split(builder.String(), "\n")
				if len(lines) > 1 {
					builder.Reset()
					builder.WriteString(strings.Join(lines[:len(lines)-1], "\n"))
					builder.WriteString("\n... (truncated)\n")
				}
				goto BuildTopComment
			}
		}
		builder.WriteString("\n")
	}

BuildTopComment:
	// Top-level Comment
	if summary.TopComment != "" {
		currentTokens := chunker.countTokens(builder.String())
		remainingTokens := fs.maxTokens - currentTokens

		// 残りトークン数でコメントをトリミング
		if remainingTokens > 50 { // 最低50トークンは確保
			trimmedComment := chunker.TrimToTokenLimit(summary.TopComment, remainingTokens-10)
			if trimmedComment != "" {
				builder.WriteString("Description:\n")
				builder.WriteString(trimmedComment)
				builder.WriteString("\n")
			}
		}
	}

	result := builder.String()

	// 最終的なトークン数チェック（念のため）
	if chunker.countTokens(result) > fs.maxTokens {
		result = chunker.TrimToTokenLimit(result, fs.maxTokens)
	}

	return result, nil
}
