package models

// SearchResult はベクトル検索の結果を表します
type SearchResult struct {
	FilePath    string  `json:"filePath"`
	StartLine   int     `json:"startLine"`
	EndLine     int     `json:"endLine"`
	Content     string  `json:"content"`
	Score       float64 `json:"score"`
	PrevContent *string `json:"prevContent,omitempty"`
	NextContent *string `json:"nextContent,omitempty"`
}
