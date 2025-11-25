package dependency

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// ImportInfo はインポート情報の詳細を表します
type ImportInfo struct {
	Path       string           // インポートパス
	Alias      string           // エイリアス（存在する場合）
	Type       ImportType       // 標準ライブラリか外部依存か
	Version    string           // バージョン情報（go.modから取得）
	IsUsed     bool             // 実際に使用されているか
	UsageCount int              // 使用回数
	UsedBy     []string         // 使用している関数・型のリスト
}

// ImportType はインポートの種別を表します
type ImportType int

const (
	ImportTypeUnknown ImportType = iota
	ImportTypeStandard            // 標準ライブラリ
	ImportTypeExternal            // 外部依存
	ImportTypeInternal            // 内部パッケージ
)

// FunctionCall は関数呼び出し情報を表します
type FunctionCall struct {
	Name         string       // 関数名
	Package      string       // パッケージ名
	Type         CallType     // 呼び出しタイプ
	ReceiverType string       // レシーバー型（メソッドの場合）
	Position     token.Pos    // ソースコード内の位置
	Arguments    []string     // 引数の型情報
}

// CallType は呼び出しの種別を表します
type CallType int

const (
	CallTypeUnknown CallType = iota
	CallTypeInternal         // 内部関数呼び出し
	CallTypeExternal         // 外部関数呼び出し
	CallTypeMethod           // メソッド呼び出し
	CallTypeBuiltin          // ビルトイン関数呼び出し
)

// TypeDependency は型依存情報を表します
type TypeDependency struct {
	TypeName     string   // 型名
	Package      string   // パッケージ名
	Kind         TypeKind // 型の種類
	UsedBy       []string // 使用している関数・型のリスト
	FieldTypes   []string // 構造体フィールドの型（構造体の場合）
	ParameterOf  []string // パラメータとして使用されている関数のリスト
	ReturnTypeOf []string // 戻り値として使用されている関数のリスト
}

// TypeKind は型の種類を表します
type TypeKind int

const (
	TypeKindUnknown TypeKind = iota
	TypeKindBasic            // 基本型
	TypeKindStruct           // 構造体
	TypeKindInterface        // インターフェース
	TypeKindPointer          // ポインタ
	TypeKindSlice            // スライス
	TypeKindMap              // マップ
	TypeKindChannel          // チャネル
)

// DependencyInfo は依存関係の詳細情報を表します
type DependencyInfo struct {
	Imports        map[string]*ImportInfo      // key: インポートパス
	FunctionCalls  []*FunctionCall             // 関数呼び出しリスト
	TypeDeps       map[string]*TypeDependency  // key: 型名
}

// Analyzer は依存関係解析を行います
type Analyzer struct {
	fset *token.FileSet
}

// NewAnalyzer は新しいAnalyzerを作成します
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		fset: token.NewFileSet(),
	}
}

// Analyze はGoソースコードから依存関係情報を抽出します
func (a *Analyzer) Analyze(content string, goModData map[string]string) (*DependencyInfo, error) {
	file, err := parser.ParseFile(a.fset, "", content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go source: %w", err)
	}

	info := &DependencyInfo{
		Imports:       make(map[string]*ImportInfo),
		FunctionCalls: make([]*FunctionCall, 0),
		TypeDeps:      make(map[string]*TypeDependency),
	}

	// インポート情報の抽出
	a.extractImports(file, info, goModData)

	// 関数呼び出しの抽出
	a.extractFunctionCalls(file, info)

	// 型依存の抽出
	a.extractTypeDependencies(file, info)

	// インポートの使用状況を更新
	a.updateImportUsage(info)

	return info, nil
}

// extractImports はインポート情報を抽出します
func (a *Analyzer) extractImports(file *ast.File, info *DependencyInfo, goModData map[string]string) {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)

		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}

		importInfo := &ImportInfo{
			Path:       path,
			Alias:      alias,
			Type:       a.classifyImport(path),
			Version:    goModData[path],
			IsUsed:     false,
			UsageCount: 0,
			UsedBy:     make([]string, 0),
		}

		info.Imports[path] = importInfo
	}
}

// classifyImport はインポートを分類します
func (a *Analyzer) classifyImport(path string) ImportType {
	// 標準ライブラリの判定（ドット区切りが1つ以下、または特定のプレフィックス）
	if !strings.Contains(path, ".") || strings.HasPrefix(path, "golang.org/x/") {
		return ImportTypeStandard
	}

	// 内部パッケージの判定（プロジェクト内のパス）
	// ここではヒューリスティックとして、特定のパターンで判定
	// より正確な判定には、プロジェクトのモジュールパスが必要
	if strings.Contains(path, "/internal/") || strings.Contains(path, "/pkg/") {
		return ImportTypeInternal
	}

	return ImportTypeExternal
}

// extractFunctionCalls は関数呼び出しを抽出します
func (a *Analyzer) extractFunctionCalls(file *ast.File, info *DependencyInfo) {
	// 各関数宣言を走査
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			a.extractCallsFromFunc(fn, info)
		}
	}
}

// extractCallsFromFunc は関数内の呼び出しを抽出します
func (a *Analyzer) extractCallsFromFunc(fn *ast.FuncDecl, info *DependencyInfo) {
	caller := fn.Name.Name

	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		funcCall := a.analyzeFunctionCall(call, caller)
		if funcCall != nil {
			info.FunctionCalls = append(info.FunctionCalls, funcCall)
		}

		return true
	})
}

// analyzeFunctionCall は個別の関数呼び出しを解析します
func (a *Analyzer) analyzeFunctionCall(call *ast.CallExpr, caller string) *FunctionCall {
	funcCall := &FunctionCall{
		Position:  call.Pos(),
		Arguments: make([]string, 0),
	}

	switch fun := call.Fun.(type) {
	case *ast.Ident:
		// 単純な関数呼び出し: func()
		funcCall.Name = fun.Name
		funcCall.Type = a.classifyCall(fun.Name, "")

	case *ast.SelectorExpr:
		// パッケージ修飾または メソッド呼び出し: pkg.Func() or obj.Method()
		funcCall.Name = fun.Sel.Name

		switch x := fun.X.(type) {
		case *ast.Ident:
			// pkg.Func() の場合
			funcCall.Package = x.Name
			funcCall.Type = CallTypeExternal

		default:
			// obj.Method() の場合
			funcCall.Type = CallTypeMethod
			funcCall.ReceiverType = a.extractTypeName(fun.X)
		}

	default:
		// その他の複雑な呼び出し
		funcCall.Type = CallTypeUnknown
	}

	// 引数の型情報を抽出（簡易版）
	for _, arg := range call.Args {
		argType := a.extractTypeName(arg)
		if argType != "" {
			funcCall.Arguments = append(funcCall.Arguments, argType)
		}
	}

	return funcCall
}

// classifyCall は呼び出しを分類します
func (a *Analyzer) classifyCall(name, pkg string) CallType {
	// ビルトイン関数のチェック
	builtins := map[string]bool{
		"append": true, "cap": true, "close": true, "complex": true,
		"copy": true, "delete": true, "imag": true, "len": true,
		"make": true, "new": true, "panic": true, "print": true,
		"println": true, "real": true, "recover": true,
	}

	if builtins[name] {
		return CallTypeBuiltin
	}

	if pkg == "" {
		return CallTypeInternal
	}

	return CallTypeExternal
}

// extractTypeDependencies は型依存を抽出します
func (a *Analyzer) extractTypeDependencies(file *ast.File, info *DependencyInfo) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					a.extractTypeSpec(typeSpec, info)
				}
			}

		case *ast.FuncDecl:
			a.extractFuncTypeDeps(d, info)
		}
	}
}

// extractTypeSpec は型定義から依存を抽出します
func (a *Analyzer) extractTypeSpec(typeSpec *ast.TypeSpec, info *DependencyInfo) {
	typeName := typeSpec.Name.Name
	typeDep := &TypeDependency{
		TypeName:     typeName,
		Package:      "",
		UsedBy:       make([]string, 0),
		FieldTypes:   make([]string, 0),
		ParameterOf:  make([]string, 0),
		ReturnTypeOf: make([]string, 0),
	}

	switch t := typeSpec.Type.(type) {
	case *ast.StructType:
		typeDep.Kind = TypeKindStruct
		// 構造体フィールドの型を抽出
		if t.Fields != nil {
			for _, field := range t.Fields.List {
				fieldType := a.extractTypeName(field.Type)
				if fieldType != "" && fieldType != typeName {
					typeDep.FieldTypes = append(typeDep.FieldTypes, fieldType)
				}
			}
		}

	case *ast.InterfaceType:
		typeDep.Kind = TypeKindInterface

	default:
		typeDep.Kind = TypeKindBasic
	}

	info.TypeDeps[typeName] = typeDep
}

// extractFuncTypeDeps は関数のシグネチャから型依存を抽出します
func (a *Analyzer) extractFuncTypeDeps(fn *ast.FuncDecl, info *DependencyInfo) {
	funcName := fn.Name.Name

	// パラメータの型を抽出
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			typeName := a.extractTypeName(field.Type)
			if typeName != "" {
				a.recordTypeUsage(info, typeName, funcName, "param")
			}
		}
	}

	// 戻り値の型を抽出
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			typeName := a.extractTypeName(field.Type)
			if typeName != "" {
				a.recordTypeUsage(info, typeName, funcName, "return")
			}
		}
	}

	// レシーバーの型を抽出（メソッドの場合）
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recv := fn.Recv.List[0]
		typeName := a.extractTypeName(recv.Type)
		if typeName != "" {
			a.recordTypeUsage(info, typeName, funcName, "receiver")
		}
	}
}

// recordTypeUsage は型の使用を記録します
func (a *Analyzer) recordTypeUsage(info *DependencyInfo, typeName, funcName, usageType string) {
	if typeDep, ok := info.TypeDeps[typeName]; ok {
		switch usageType {
		case "param":
			typeDep.ParameterOf = append(typeDep.ParameterOf, funcName)
		case "return":
			typeDep.ReturnTypeOf = append(typeDep.ReturnTypeOf, funcName)
		case "receiver":
			typeDep.UsedBy = append(typeDep.UsedBy, funcName)
		}
	} else {
		// 型が未定義の場合は新規作成
		typeDep := &TypeDependency{
			TypeName:     typeName,
			Package:      "",
			Kind:         TypeKindUnknown,
			UsedBy:       make([]string, 0),
			FieldTypes:   make([]string, 0),
			ParameterOf:  make([]string, 0),
			ReturnTypeOf: make([]string, 0),
		}

		switch usageType {
		case "param":
			typeDep.ParameterOf = append(typeDep.ParameterOf, funcName)
		case "return":
			typeDep.ReturnTypeOf = append(typeDep.ReturnTypeOf, funcName)
		case "receiver":
			typeDep.UsedBy = append(typeDep.UsedBy, funcName)
		}

		info.TypeDeps[typeName] = typeDep
	}
}

// extractTypeName は式から型名を抽出します
func (a *Analyzer) extractTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name

	case *ast.StarExpr:
		return "*" + a.extractTypeName(t.X)

	case *ast.ArrayType:
		return "[]" + a.extractTypeName(t.Elt)

	case *ast.SelectorExpr:
		pkg := a.extractTypeName(t.X)
		return pkg + "." + t.Sel.Name

	case *ast.MapType:
		key := a.extractTypeName(t.Key)
		val := a.extractTypeName(t.Value)
		return fmt.Sprintf("map[%s]%s", key, val)

	case *ast.InterfaceType:
		return "interface{}"

	case *ast.ChanType:
		return "chan " + a.extractTypeName(t.Value)

	default:
		return ""
	}
}

// updateImportUsage はインポートの使用状況を更新します
func (a *Analyzer) updateImportUsage(info *DependencyInfo) {
	// 関数呼び出しからインポートの使用を判定
	for _, call := range info.FunctionCalls {
		if call.Package != "" {
			// パッケージ名からインポートパスを探す
			for path, importInfo := range info.Imports {
				pkgName := a.getPackageNameFromPath(path)
				if pkgName == call.Package || importInfo.Alias == call.Package {
					importInfo.IsUsed = true
					importInfo.UsageCount++
					// 重複を避けてUsedByに追加
					if !contains(importInfo.UsedBy, call.Name) {
						importInfo.UsedBy = append(importInfo.UsedBy, call.Name)
					}
				}
			}
		}
	}

	// 型依存からインポートの使用を判定
	for _, typeDep := range info.TypeDeps {
		if typeDep.Package != "" {
			for path, importInfo := range info.Imports {
				pkgName := a.getPackageNameFromPath(path)
				if pkgName == typeDep.Package || importInfo.Alias == typeDep.Package {
					importInfo.IsUsed = true
					importInfo.UsageCount++
				}
			}
		}
	}
}

// getPackageNameFromPath はインポートパスからパッケージ名を取得します
func (a *Analyzer) getPackageNameFromPath(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

// contains はスライスに要素が含まれているかチェックします
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
