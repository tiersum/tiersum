package types

// ColdSearchHit is one ranked cold chapter match from hybrid cold search (API read model).
type ColdSearchHit struct {
	DocumentID string  `json:"document_id"`
	Path       string  `json:"path,omitempty"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	Source     string  `json:"source,omitempty"`
}
