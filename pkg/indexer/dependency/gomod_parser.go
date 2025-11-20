package dependency

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// GoModParser はgo.modファイルを解析します
type GoModParser struct{}

// NewGoModParser は新しいGoModParserを作成します
func NewGoModParser() *GoModParser {
	return &GoModParser{}
}

// ParseFile はgo.modファイルを解析してパッケージとバージョンのマップを返します
func (p *GoModParser) ParseFile(filePath string) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open go.mod: %w", err)
	}
	defer file.Close()

	return p.Parse(file)
}

// Parse はgo.modの内容を解析します
func (p *GoModParser) Parse(file *os.File) (map[string]string, error) {
	versions := make(map[string]string)
	scanner := bufio.NewScanner(file)
	inRequireBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// コメント行をスキップ
		if strings.HasPrefix(line, "//") {
			continue
		}

		// requireブロックの開始
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}

		// requireブロックの終了
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}

		// require行の解析
		if strings.HasPrefix(line, "require ") || inRequireBlock {
			pkg, version := p.parseRequireLine(line)
			if pkg != "" && version != "" {
				versions[pkg] = version
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading go.mod: %w", err)
	}

	return versions, nil
}

// parseRequireLine は1行のrequire文を解析します
func (p *GoModParser) parseRequireLine(line string) (string, string) {
	// "require " プレフィックスを除去
	line = strings.TrimPrefix(line, "require ")
	line = strings.TrimSpace(line)

	// 空行をスキップ
	if line == "" || line == "(" || line == ")" {
		return "", ""
	}

	// インラインコメントを除去
	if idx := strings.Index(line, "//"); idx != -1 {
		line = line[:idx]
		line = strings.TrimSpace(line)
	}

	// パッケージとバージョンを分割
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		pkg := parts[0]
		version := parts[1]
		return pkg, version
	}

	return "", ""
}

// ParseContent は文字列からgo.modを解析します（テスト用）
func (p *GoModParser) ParseContent(content string) (map[string]string, error) {
	versions := make(map[string]string)
	inRequireBlock := false

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// コメント行をスキップ
		if strings.HasPrefix(line, "//") {
			continue
		}

		// requireブロックの開始
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}

		// requireブロックの終了
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}

		// require行の解析
		if strings.HasPrefix(line, "require ") || inRequireBlock {
			pkg, version := p.parseRequireLine(line)
			if pkg != "" && version != "" {
				versions[pkg] = version
			}
		}
	}

	return versions, nil
}

// GetVersion は指定されたパッケージのバージョンを取得します
func GetVersion(versions map[string]string, pkg string) string {
	if version, ok := versions[pkg]; ok {
		return version
	}
	return ""
}

// IsStandardLibrary は標準ライブラリかどうか判定します
func IsStandardLibrary(pkg string) bool {
	// 標準ライブラリはドメインを含まない、またはgolang.org/x/で始まる
	if !strings.Contains(pkg, ".") {
		return true
	}
	if strings.HasPrefix(pkg, "golang.org/x/") {
		return true
	}
	return false
}

// GetModulePath はgo.modからモジュールパスを抽出します
func (p *GoModParser) GetModulePath(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}
