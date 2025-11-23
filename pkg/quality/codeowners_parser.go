package quality

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CodeownersParser はCODEOWNERSファイルをパースするサービスです
type CodeownersParser struct{}

// NewCodeownersParser は新しいCodeownersParserを作成します
func NewCodeownersParser() *CodeownersParser {
	return &CodeownersParser{}
}

// ParseCodeowners はCODEOWNERSファイルをパースしてマップを返します
// Key: ファイルパス/パターン
// Value: オーナーのリスト
func (p *CodeownersParser) ParseCodeowners(repoPath string) (map[string][]string, error) {
	// CODEOWNERSファイルの場所を探す
	// GitHub, GitLab, Bitbucketでサポートされている場所:
	// - CODEOWNERS
	// - .github/CODEOWNERS
	// - .gitlab/CODEOWNERS
	// - docs/CODEOWNERS
	possiblePaths := []string{
		filepath.Join(repoPath, "CODEOWNERS"),
		filepath.Join(repoPath, ".github", "CODEOWNERS"),
		filepath.Join(repoPath, ".gitlab", "CODEOWNERS"),
		filepath.Join(repoPath, "docs", "CODEOWNERS"),
	}

	var codeownersPath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			codeownersPath = path
			break
		}
	}

	// CODEOWNERSファイルが見つからない場合は空のマップを返す
	if codeownersPath == "" {
		return make(map[string][]string), nil
	}

	// ファイルを開く
	file, err := os.Open(codeownersPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CODEOWNERS file: %w", err)
	}
	defer file.Close()

	// パース
	result := make(map[string][]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// コメント行または空行はスキップ
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// パターンとオーナーを分離
		// フォーマット: <pattern> <owner1> <owner2> ...
		parts := strings.Fields(line)
		if len(parts) < 2 {
			// オーナーが指定されていない行はスキップ
			continue
		}

		pattern := parts[0]
		owners := parts[1:]

		result[pattern] = owners
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan CODEOWNERS file: %w", err)
	}

	return result, nil
}

// GetOwnersForFile は指定されたファイルパスに対するオーナーを取得します
// CODEOWNERSのパターンマッチングを適用します
func (p *CodeownersParser) GetOwnersForFile(filePath string, codeownersLookup map[string][]string) []string {
	// 完全一致を優先
	if owners, ok := codeownersLookup[filePath]; ok {
		return owners
	}

	// パターンマッチング
	// 具体的なパターン(文字数が多い)を優先
	var bestMatch string
	var matchedOwners []string

	for pattern, owners := range codeownersLookup {
		if matchPattern(pattern, filePath) {
			// より具体的なパターンを選択(文字数が多い方を優先)
			if len(pattern) > len(bestMatch) {
				bestMatch = pattern
				matchedOwners = owners
			}
		}
	}

	return matchedOwners
}

// matchPattern はCODEOWNERSのパターンマッチングを行います
// 簡易実装: ワイルドカード (*) と ディレクトリマッチング (/) をサポート
func matchPattern(pattern, filePath string) bool {
	// パターンが / で終わる場合はディレクトリ
	if strings.HasSuffix(pattern, "/") {
		// 絶対パスの場合
		if strings.HasPrefix(pattern, "/") {
			dirPattern := strings.TrimPrefix(pattern, "/")
			dirPattern = strings.TrimSuffix(dirPattern, "/")
			return strings.HasPrefix(filePath, dirPattern+"/") || filePath == dirPattern
		}
		// 相対パスの場合
		dirPattern := strings.TrimSuffix(pattern, "/")
		return strings.HasPrefix(filePath, dirPattern+"/") || filePath == dirPattern
	}

	// パターンが / で始まる場合はリポジトリルートからの絶対パス
	if strings.HasPrefix(pattern, "/") {
		pattern = strings.TrimPrefix(pattern, "/")
		return matchWildcard(pattern, filePath)
	}

	// パターンに / が含まれていない場合は、任意のディレクトリのファイル名にマッチ
	if !strings.Contains(pattern, "/") {
		fileName := filepath.Base(filePath)
		return matchWildcard(pattern, fileName)
	}

	// それ以外の場合は、パスの末尾とマッチ
	return matchWildcard(pattern, filePath) || strings.HasSuffix(filePath, "/"+pattern)
}

// matchWildcard はワイルドカード (*) を含むパターンマッチングを行います
func matchWildcard(pattern, str string) bool {
	// 簡易実装: * を任意の文字列にマッチさせる
	if !strings.Contains(pattern, "*") {
		return pattern == str
	}

	parts := strings.Split(pattern, "*")
	if len(parts) == 0 {
		return true
	}

	// 最初の部分が一致するか確認
	if !strings.HasPrefix(str, parts[0]) {
		return false
	}
	str = strings.TrimPrefix(str, parts[0])

	// 最後の部分が一致するか確認
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		if !strings.HasSuffix(str, lastPart) {
			return false
		}
		str = strings.TrimSuffix(str, lastPart)
	}

	// 中間部分のチェック
	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(str, parts[i])
		if idx == -1 {
			return false
		}
		str = str[idx+len(parts[i]):]
	}

	return true
}
