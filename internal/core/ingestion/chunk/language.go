package chunk

// Language はサポートされるプログラミング言語を表します
type Language string

const (
	// プログラミング言語
	LanguageGo         Language = "go"
	LanguageJava       Language = "java"
	LanguagePython     Language = "python"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
	LanguageRust       Language = "rust"
	LanguageC          Language = "c"
	LanguageCPP        Language = "cpp"
	LanguageCSharp     Language = "csharp"
	LanguageRuby       Language = "ruby"
	LanguagePHP        Language = "php"
	LanguageSwift      Language = "swift"
	LanguageKotlin     Language = "kotlin"
	LanguageScala      Language = "scala"
	LanguageShell      Language = "shell"

	// マークアップ言語
	LanguageMarkdown Language = "markdown"
	LanguageHTML     Language = "html"
	LanguageXML      Language = "xml"

	// その他
	LanguagePlainText Language = "plaintext"
	LanguageUnknown   Language = "unknown"
)

// ContentType はMIMEタイプのような言語識別子を表します
type ContentType string

const (
	ContentTypeGo         ContentType = "text/x-go"
	ContentTypeJava       ContentType = "text/x-java"
	ContentTypePython     ContentType = "text/x-python"
	ContentTypeJavaScript ContentType = "text/javascript"
	ContentTypeTypeScript ContentType = "text/x-typescript"
	ContentTypeRust       ContentType = "text/x-rust"
	ContentTypeC          ContentType = "text/x-c"
	ContentTypeCPP        ContentType = "text/x-c++"
	ContentTypeCSharp     ContentType = "text/x-csharp"
	ContentTypeRuby       ContentType = "text/x-ruby"
	ContentTypePHP        ContentType = "text/x-php"
	ContentTypeSwift      ContentType = "text/x-swift"
	ContentTypeKotlin     ContentType = "text/x-kotlin"
	ContentTypeScala      ContentType = "text/x-scala"
	ContentTypeShell      ContentType = "text/x-shellscript"
	ContentTypeMarkdown   ContentType = "text/markdown"
	ContentTypeHTML       ContentType = "text/html"
	ContentTypeXML        ContentType = "text/xml"
	ContentTypePlainText  ContentType = "text/plain"
)

// LanguageFromContentType はContentTypeからLanguageに変換します
func LanguageFromContentType(ct ContentType) Language {
	switch ct {
	case ContentTypeGo:
		return LanguageGo
	case ContentTypeJava:
		return LanguageJava
	case ContentTypePython:
		return LanguagePython
	case ContentTypeJavaScript:
		return LanguageJavaScript
	case ContentTypeTypeScript:
		return LanguageTypeScript
	case ContentTypeRust:
		return LanguageRust
	case ContentTypeC:
		return LanguageC
	case ContentTypeCPP:
		return LanguageCPP
	case ContentTypeCSharp:
		return LanguageCSharp
	case ContentTypeRuby:
		return LanguageRuby
	case ContentTypePHP:
		return LanguagePHP
	case ContentTypeSwift:
		return LanguageSwift
	case ContentTypeKotlin:
		return LanguageKotlin
	case ContentTypeScala:
		return LanguageScala
	case ContentTypeShell:
		return LanguageShell
	case ContentTypeMarkdown:
		return LanguageMarkdown
	case ContentTypeHTML:
		return LanguageHTML
	case ContentTypeXML:
		return LanguageXML
	case ContentTypePlainText:
		return LanguagePlainText
	default:
		return LanguageUnknown
	}
}

// ContentTypeFromLanguage はLanguageからContentTypeに変換します
func ContentTypeFromLanguage(lang Language) ContentType {
	switch lang {
	case LanguageGo:
		return ContentTypeGo
	case LanguageJava:
		return ContentTypeJava
	case LanguagePython:
		return ContentTypePython
	case LanguageJavaScript:
		return ContentTypeJavaScript
	case LanguageTypeScript:
		return ContentTypeTypeScript
	case LanguageRust:
		return ContentTypeRust
	case LanguageC:
		return ContentTypeC
	case LanguageCPP:
		return ContentTypeCPP
	case LanguageCSharp:
		return ContentTypeCSharp
	case LanguageRuby:
		return ContentTypeRuby
	case LanguagePHP:
		return ContentTypePHP
	case LanguageSwift:
		return ContentTypeSwift
	case LanguageKotlin:
		return ContentTypeKotlin
	case LanguageScala:
		return ContentTypeScala
	case LanguageShell:
		return ContentTypeShell
	case LanguageMarkdown:
		return ContentTypeMarkdown
	case LanguageHTML:
		return ContentTypeHTML
	case LanguageXML:
		return ContentTypeXML
	case LanguagePlainText:
		return ContentTypePlainText
	default:
		return ContentTypePlainText
	}
}

// IsSourceCode は指定された言語がソースコードかどうかを判定します
func IsSourceCode(lang Language) bool {
	switch lang {
	case LanguageGo, LanguageJava, LanguagePython, LanguageJavaScript,
		LanguageTypeScript, LanguageRust, LanguageC, LanguageCPP,
		LanguageCSharp, LanguageRuby, LanguagePHP, LanguageSwift,
		LanguageKotlin, LanguageScala, LanguageShell:
		return true
	default:
		return false
	}
}

// SupportsASTChunking は指定された言語がAST解析によるチャンク化に対応しているかを判定します
func SupportsASTChunking(lang Language) bool {
	switch lang {
	case LanguageGo:
		// 現時点ではGoのみサポート
		// 将来的に他の言語も追加予定
		return true
	default:
		return false
	}
}

// FileExtensions は言語に対応するファイル拡張子のリストを返します
func FileExtensions(lang Language) []string {
	switch lang {
	case LanguageGo:
		return []string{".go"}
	case LanguageJava:
		return []string{".java"}
	case LanguagePython:
		return []string{".py"}
	case LanguageJavaScript:
		return []string{".js", ".jsx", ".mjs", ".cjs"}
	case LanguageTypeScript:
		return []string{".ts", ".tsx"}
	case LanguageRust:
		return []string{".rs"}
	case LanguageC:
		return []string{".c", ".h"}
	case LanguageCPP:
		return []string{".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx"}
	case LanguageCSharp:
		return []string{".cs"}
	case LanguageRuby:
		return []string{".rb"}
	case LanguagePHP:
		return []string{".php"}
	case LanguageSwift:
		return []string{".swift"}
	case LanguageKotlin:
		return []string{".kt", ".kts"}
	case LanguageScala:
		return []string{".scala"}
	case LanguageShell:
		return []string{".sh", ".bash", ".zsh"}
	case LanguageMarkdown:
		return []string{".md", ".markdown"}
	case LanguageHTML:
		return []string{".html", ".htm"}
	case LanguageXML:
		return []string{".xml"}
	default:
		return []string{}
	}
}
