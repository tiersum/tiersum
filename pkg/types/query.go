package types

// LLMFilterResult represents LLM filter result
type LLMFilterResult struct {
	ID          string  `json:"id"`          // Item ID or path
	Relevance   float64 `json:"relevance"`   // Relevance score 0-1
	Explanation string  `json:"explanation"` // Relevance explanation (optional)
}

// ProgressiveQueryRequest represents the new progressive query request (based on two-level tags)
type ProgressiveQueryRequest struct {
	Question   string `json:"question" binding:"required"`                   // User query
	MaxResults int    `json:"max_results" binding:"omitempty,min=1,max=100"` // Max documents to consider, default 100
}

// ProgressiveQueryResponse represents the new progressive query response
type ProgressiveQueryResponse struct {
	Question string `json:"question"`
	// Answer is GitHub-Flavored Markdown from the LLM (prompt requires full reply as renderable Markdown; citations [^N^]).
	Answer  string                 `json:"answer,omitempty"`
	Steps   []ProgressiveQueryStep `json:"steps"`
	Results []QueryItem            `json:"results"`
	// TraceID is the OpenTelemetry trace id (hex) when the incoming request context carries a recording span (same trace as HTTP middleware when sampled).
	TraceID string `json:"trace_id,omitempty"`
}

// ProgressiveQueryStep represents query step result
type ProgressiveQueryStep struct {
	Step     string      `json:"step"`        // Step name: L1_tags, L2_tags, documents, chapters, source
	Input    interface{} `json:"input"`       // Input data
	Output   interface{} `json:"output"`      // Output data
	Duration int64       `json:"duration_ms"` // Duration in milliseconds
}

// QueryItem represents query result item (extended version)
type QueryItem struct {
	ID         string         `json:"id"`                    // Item ID (document ID)
	Title      string         `json:"title"`                 // Title
	Content    string         `json:"content"`               // Content (summary or original text)
	Tier       SummaryTier    `json:"tier"`                  // Tier level
	Path       string         `json:"path"`                  // Path, e.g.: doc_id/chapter_title
	Relevance  float64        `json:"relevance"`             // LLM evaluated relevance 0-1
	IsSource   bool           `json:"is_source"`             // Whether it is already original text (true=cannot go deeper, false=can go deeper)
	ChildCount int            `json:"child_count,omitempty"` // Number of child items
	Status     DocumentStatus `json:"status,omitempty"`      // Document tier: hot, cold, warming
	// ContentSource explains where Content came from (for UI/debug): hot path uses DB chapter summaries;
	// cold path uses cold index (bm25, vector, or hybrid after merge).
	ContentSource string `json:"content_source,omitempty"`
}
