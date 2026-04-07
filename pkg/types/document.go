package types

import (
	"regexp"
	"strings"
	"time"
)

// DocumentStatus represents the hot/cold tier status of a document
type DocumentStatus string

const (
	// DocStatusHot indicates a frequently accessed document with full LLM analysis
	DocStatusHot DocumentStatus = "hot"
	// DocStatusCold indicates a rarely accessed document with minimal processing
	DocStatusCold DocumentStatus = "cold"
	// DocStatusWarming indicates a document being promoted from cold to hot
	DocStatusWarming DocumentStatus = "warming"
)

// Document represents a document in the system
type Document struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	Format      string         `json:"format"`
	Tags        []string       `json:"tags,omitempty"`
	Status      DocumentStatus `json:"status"`
	HotScore    float64        `json:"hot_score"`
	QueryCount  int            `json:"query_count"`
	LastQueryAt *time.Time     `json:"last_query_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// SummaryTier represents the level of summarization
// Hierarchy: Document > Chapter > Source
// Note: Topic and Paragraph tiers are removed in the new architecture
type SummaryTier string

const (
	TierDocument SummaryTier = "document" // Document level summary
	TierChapter  SummaryTier = "chapter"  // Chapter/section level summary
	TierSource   SummaryTier = "source"   // Original source text
)

// Summary represents a summary at a specific tier
type Summary struct {
	ID         string      `json:"id"`
	DocumentID string      `json:"document_id"`
	Tier       SummaryTier `json:"tier"`
	Path       string      `json:"path"`    // Format: doc_id or doc_id/chapter_title
	Content    string      `json:"content"` // Summary content or source content
	IsSource   bool        `json:"is_source"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// ChapterInfo represents a chapter/section in a document
type ChapterInfo struct {
	Title       string `json:"title"`       // Chapter title
	Level       int    `json:"level"`       // Header level (1=#, 2=##, 3=###)
	Summary     string `json:"summary"`     // Chapter summary
	Content     string `json:"content"`     // Chapter original content
	StartOffset int    `json:"start_offset"` // Start position in document
	EndOffset   int    `json:"end_offset"`   // End position in document
}

// DocumentAnalysisResult holds LLM analysis results for a document
type DocumentAnalysisResult struct {
	Summary  string        `json:"summary"`   // Document-level summary
	Tags     []string      `json:"tags"`      // Generated tags (max 10)
	Chapters []ChapterInfo `json:"chapters"`  // List of chapters with summaries
}

// TagGroup represents a group of tags (Level 1 categorization)
type TagGroup struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`        // Group name/category
	Description string   `json:"description"` // Group description
	Tags        []string `json:"tags"`        // Tags in this group
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Tag represents a global tag with its metadata
type Tag struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`        // Tag name
	GroupID   string    `json:"group_id"`  // Which group it belongs to (Level 1)
	DocumentCount int     `json:"document_count"` // Number of documents with this tag
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateDocumentRequest represents a request to create a document
type CreateDocumentRequest struct {
	Title   string   `json:"title" binding:"required"`
	Content string   `json:"content" binding:"required"`
	Format  string   `json:"format" binding:"required,oneof=markdown md"`
	Tags    []string `json:"tags,omitempty"`
	// ForceHot forces full LLM analysis regardless of heuristics
	ForceHot bool `json:"force_hot,omitempty"`
	// Summary is pre-generated document summary (from external agent)
	Summary string `json:"summary,omitempty"`
	// Chapters are pre-generated chapter summaries (from external agent)
	Chapters []ChapterInfo `json:"chapters,omitempty"`
	// Embedding is pre-computed vector embedding (from external agent)
	Embedding []float32 `json:"embedding,omitempty"`
}

// ExtractKeywords extracts keywords from content using simple regex patterns
// Returns lowercase words with length > 4, limited to maxKeywords
func ExtractKeywords(content string, maxKeywords int) []string {
	// Regex to match words with length > 4 (letters only)
	re := regexp.MustCompile(`[a-zA-Z]{5,}`)
	matches := re.FindAllString(content, -1)

	// Use map to deduplicate and count frequency
	wordFreq := make(map[string]int)
	for _, word := range matches {
		word = strings.ToLower(word)
		wordFreq[word]++
	}

	// Convert to slice and sort by frequency (simple approach)
	type wordCount struct {
		word  string
		count int
	}
	counts := make([]wordCount, 0, len(wordFreq))
	for word, count := range wordFreq {
		counts = append(counts, wordCount{word, count})
	}

	// Sort by frequency (higher first) - simple bubble sort for small lists
	for i := 0; i < len(counts); i++ {
		for j := i + 1; j < len(counts); j++ {
			if counts[j].count > counts[i].count {
				counts[i], counts[j] = counts[j], counts[i]
			}
		}
	}

	// Take top keywords
	result := make([]string, 0, maxKeywords)
	for i := 0; i < len(counts) && i < maxKeywords; i++ {
		result = append(result, counts[i].word)
	}

	return result
}

// CreateDocumentResponse represents the response from creating a document
type CreateDocumentResponse struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Format       string         `json:"format"`
	Tags         []string       `json:"tags"`
	Summary      string         `json:"summary"`
	ChapterCount int            `json:"chapter_count"`
	Status       DocumentStatus `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
}

// QueryRequest represents a query request
type QueryRequest struct {
	Question string      `form:"question" binding:"required"`
	Depth    SummaryTier `form:"depth" binding:"omitempty,oneof=document chapter source"`
	Tags     []string    `form:"tags,omitempty"` // Optional tag filter
}

// QueryResponse represents a query response
type QueryResponse struct {
	Question string        `json:"question"`
	Depth    SummaryTier   `json:"depth"`
	Results  []QueryResult `json:"results"`
}

// QueryResult represents a single query result
type QueryResult struct {
	DocumentID    string      `json:"document_id"`
	DocumentTitle string      `json:"document_title"`
	Tier          SummaryTier `json:"tier"`
	Path          string      `json:"path"`
	Content       string      `json:"content"`
	Relevance     float64     `json:"relevance"`
}

// TagFilterResult represents a tag filter result from LLM
type TagFilterResult struct {
	Tag       string  `json:"tag"`
	Relevance float64 `json:"relevance"`
}
