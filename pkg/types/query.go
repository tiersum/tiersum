package types

// LLMFilterResult represents LLM filter result
type LLMFilterResult struct {
	ID          string  `json:"id"`          // Item ID or path
	Relevance   float64 `json:"relevance"`   // Relevance score 0-1
	Explanation string  `json:"explanation"` // Relevance explanation (optional)
}

// ProgressiveQueryRequest is the body for POST /query/progressive (question + optional max_results cap).
type ProgressiveQueryRequest struct {
	Question   string `json:"question" binding:"required"`                   // User query
	MaxResults int    `json:"max_results" binding:"omitempty,min=1,max=100"` // Max documents to consider, default 15
}

// ProgressiveQueryResponse is the JSON body returned by progressive query (answer, trace steps, merged hits).
type ProgressiveQueryResponse struct {
	Question string `json:"question"`
	// Answer is GitHub-Flavored Markdown from the LLM (prompt requires full reply as renderable Markdown; citations [^N^]).
	// Deprecated: kept for backward compatibility; use AnswerFromReferences and AnswerFromKnowledge.
	Answer string `json:"answer,omitempty"`
	// AnswerFromReferences is the evidence-based answer derived solely from the provided reference excerpts.
	AnswerFromReferences string `json:"answer_from_references,omitempty"`
	// AnswerFromKnowledge is the concise supplementary answer from the LLM's own knowledge (≤200 chars).
	AnswerFromKnowledge string                 `json:"answer_from_knowledge,omitempty"`
	Steps               []ProgressiveQueryStep `json:"steps"`
	Results             []QueryItem            `json:"results"`
	// TraceID is the OpenTelemetry trace id (hex) when the incoming request context carries a recording span (same trace as HTTP middleware when sampled).
	TraceID string `json:"trace_id,omitempty"`
}

// ProgressiveQueryStep represents query step result
type ProgressiveQueryStep struct {
	Step     string      `json:"step"`        // Step name: tags, documents, chapters, cold_docs (see progressive query implementation)
	Input    interface{} `json:"input"`       // Input data
	Output   interface{} `json:"output"`      // Output data
	Duration int64       `json:"duration_ms"` // Duration in milliseconds
}

// QueryItem represents query result item (extended version)
type QueryItem struct {
	ID         string         `json:"id"`                    // Item ID (document ID)
	Title      string         `json:"title"`                 // Title
	Content    string         `json:"content"`               // Content (summary or original text)
	Path       string         `json:"path"`                  // Path, e.g.: doc_id/chapter_title
	Relevance  float64        `json:"relevance"`             // LLM evaluated relevance 0-1
	ChildCount int            `json:"child_count,omitempty"` // Number of child items
	Status     DocumentStatus `json:"status,omitempty"`      // Document status: hot, cold, warming
	// ContentSource is a coarse hint for UI/debug. Hot-path items currently use the label "chapter_summary" even when
	// Content fell back to chapter body because Summary was empty. Cold-path items expose the index branch (bm25, vector, hybrid).
	ContentSource string `json:"content_source,omitempty"`
}
