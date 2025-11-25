package summary

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// Hasher はsource_hash計算を行う
type Hasher struct{}

// NewHasher は新しいHasherを作成
func NewHasher() *Hasher {
	return &Hasher{}
}

// HashString は文字列のSHA256ハッシュを計算
func (h *Hasher) HashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// HashFileSource はファイル要約のsource_hashを計算
// 入力: ファイルのcontent_hash
func (h *Hasher) HashFileSource(fileContentHash string) string {
	return fileContentHash
}

// HashDirectorySource はディレクトリ要約のsource_hashを計算
// 入力: 配下のファイル要約のcontent_hashリスト + サブディレクトリ要約のcontent_hashリスト
func (h *Hasher) HashDirectorySource(fileSummaryHashes, subdirSummaryHashes []string) string {
	// ソートして結合
	all := make([]string, 0, len(fileSummaryHashes)+len(subdirSummaryHashes))
	all = append(all, fileSummaryHashes...)
	all = append(all, subdirSummaryHashes...)
	sort.Strings(all)
	combined := strings.Join(all, ":")
	return h.HashString(combined)
}

// HashArchitectureSource はアーキテクチャ要約のsource_hashを計算
// 入力: 全ディレクトリ要約のcontent_hashリスト
func (h *Hasher) HashArchitectureSource(dirSummaryHashes []string) string {
	sorted := make([]string, len(dirSummaryHashes))
	copy(sorted, dirSummaryHashes)
	sort.Strings(sorted)
	combined := strings.Join(sorted, ":")
	return h.HashString(combined)
}

// HashContent は要約内容のcontent_hashを計算
func (h *Hasher) HashContent(content string) string {
	return h.HashString(content)
}
