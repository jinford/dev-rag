package domain

// Detector はファイルのコンテンツタイプを検出するインターフェース
type Detector interface {
	// DetectContentType はファイルのパスと内容からMIMEタイプを判定します
	DetectContentType(path string, content []byte) string
}
