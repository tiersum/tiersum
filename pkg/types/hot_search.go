package types

// HotSearchHit is one ranked hot/warming chapter match for progressive query (DB-backed chapters; keyword-ranked).
type HotSearchHit struct {
	DocumentID string         `json:"document_id"`
	Path       string         `json:"path,omitempty"`
	Title      string         `json:"title"`
	Content    string         `json:"content"`
	Score      float64        `json:"score"`
	Source     string         `json:"source,omitempty"`
	Status     DocumentStatus `json:"status,omitempty"`
	// QueryCount is used only server-side (e.g. promotion bookkeeping); not part of public JSON for this hit type.
	QueryCount int `json:"-"`
}
