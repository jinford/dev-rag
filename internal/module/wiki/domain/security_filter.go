package domain

// SecurityFilter は秘匿情報のフィルタリングを行うインターフェース
type SecurityFilter interface {
	// ContainsSensitiveInfo はコンテンツに秘匿情報が含まれているかチェックする
	ContainsSensitiveInfo(content string) bool

	// MaskSensitiveInfo はコンテンツ内の秘匿情報をマスクする
	MaskSensitiveInfo(content string) string

	// FilterFiles はファイルリストから秘匿情報を含む可能性のあるファイルをフィルタリングする
	FilterFiles(files []string) []string

	// ShouldExclude はファイルパスが除外対象かどうかを判定する
	ShouldExclude(path string) bool
}
