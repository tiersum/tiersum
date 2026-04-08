// Package memory provides in-memory indexing for cold documents
// Uses bleve for BM25 text search and hnsw for vector similarity search
// Supports Chinese text segmentation using gojieba
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/registry"
	"github.com/coder/hnsw"
	"github.com/yanyiwu/gojieba"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

const (
	// VectorDimension is the dimension of the embedding vectors (MiniLM-L6-v2)
	VectorDimension = 384
	// DefaultHNSWM is the M parameter for HNSW (max connections per layer)
	DefaultHNSWM = 16
	// DefaultHNSWEfConstruction is the efConstruction parameter for HNSW
	DefaultHNSWEfConstruction = 200
	// DefaultHNSWEfSearch is the efSearch parameter for HNSW
	DefaultHNSWEfSearch = 100
	// BM25TopK is the number of results to return from BM25 search
	BM25TopK = 20
	// VectorTopK is the number of results to return from vector search
	VectorTopK = 20
)

// DocumentIndex represents a document in the index
type DocumentIndex struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Embedding []float32 `json:"embedding"`
}

// SearchResult is an alias for storage.SearchResult
type SearchResult = storage.SearchResult

// Snippet is an alias for storage.Snippet
type Snippet = storage.Snippet

// FragmentConfig configures the snippet extraction behavior
const (
	// ContextWindowSize is the number of characters before and after the keyword
	ContextWindowSize = 200
	// MaxSnippetLength is the maximum length of a single snippet
	MaxSnippetLength = 500
	// MaxSnippetsPerDoc is the maximum number of snippets to return per document
	MaxSnippetsPerDoc = 3
	// MergeDistance is the distance threshold for merging adjacent snippets
	MergeDistance = 50
)

// hnswConfig stores HNSW configuration parameters
type hnswConfig struct {
	M        int
	EfSearch int
}

// Index provides in-memory BM25 + Vector hybrid search for cold documents
type Index struct {
	mu           sync.RWMutex
	bleveIndex   bleve.Index
	hnswIndex    *hnsw.Graph[string]
	documents    map[string]*DocumentIndex
	logger       *zap.Logger
	hnswConfig   hnswConfig
}

// GojiebaTokenizer implements bleve's Tokenizer interface using gojieba
type GojiebaTokenizer struct {
	jieba *gojieba.Jieba
}

// NewGojiebaTokenizer creates a new gojieba tokenizer
func NewGojiebaTokenizer() *GojiebaTokenizer {
	return &GojiebaTokenizer{
		jieba: gojieba.NewJieba(),
	}
}

// Tokenize splits input text into tokens using gojieba
func (t *GojiebaTokenizer) Tokenize(input []byte) analysis.TokenStream {
	// Use full mode for better search coverage
	words := t.jieba.CutForSearch(string(input), true)
	
	result := make(analysis.TokenStream, 0, len(words))
	position := 1
	start := 0
	
	for _, word := range words {
		if word == " " {
			start += len(word)
			continue
		}
		
		token := &analysis.Token{
			Term:     []byte(word),
			Position: position,
			Start:    start,
			End:      start + len(word),
			Type:     analysis.Ideographic,
		}
		
		result = append(result, token)
		position++
		start += len(word)
	}
	
	return result
}

// Close releases resources used by the tokenizer
func (t *GojiebaTokenizer) Close() {
	if t.jieba != nil {
		t.jieba.Free()
	}
}

// NewIndex creates a new in-memory index with Chinese text segmentation support
func NewIndex(logger *zap.Logger) (*Index, error) {
	// Create bleve index mapping with Chinese tokenizer
	mapping, err := createIndexMapping()
	if err != nil {
		return nil, fmt.Errorf("failed to create index mapping: %w", err)
	}
	
	// Create in-memory bleve index
	bleveIdx, err := bleve.NewMemOnly(mapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create bleve index: %w", err)
	}

	// Create HNSW index
	hnswIdx := hnsw.NewGraph[string]()
	hnswIdx.Distance = hnsw.CosineDistance
	hnswIdx.M = DefaultHNSWM
	hnswIdx.EfSearch = DefaultHNSWEfSearch

	return &Index{
		bleveIndex: bleveIdx,
		hnswIndex:  hnswIdx,
		documents:  make(map[string]*DocumentIndex),
		logger:     logger,
		hnswConfig: hnswConfig{
			M:        DefaultHNSWM,
			EfSearch: DefaultHNSWEfSearch,
		},
	}, nil
}

// createIndexMapping creates a custom index mapping with Chinese tokenizer
func createIndexMapping() (mapping.IndexMapping, error) {
	// Register gojieba tokenizer in bleve registry
	// This must be done before creating the index mapping
	registry.RegisterTokenizer("gojieba", func(config map[string]interface{}, cache *registry.Cache) (analysis.Tokenizer, error) {
		return NewGojiebaTokenizer(), nil
	})
	
	// Register custom analyzer that uses gojieba tokenizer
	registry.RegisterAnalyzer("jieba_analyzer", func(config map[string]interface{}, cache *registry.Cache) (analysis.Analyzer, error) {
		tokenizer, err := cache.TokenizerNamed("gojieba")
		if err != nil {
			return nil, err
		}
		
		// Import and use the lowercase filter directly
		// Note: lowercase filter is registered as "to_lower" in bleve registry
		lowercaseFilter, err := cache.TokenFilterNamed("to_lower")
		if err != nil {
			// If not found, create analyzer without token filter
			return &analysis.DefaultAnalyzer{
				Tokenizer: tokenizer,
			}, nil
		}
		
		return &analysis.DefaultAnalyzer{
			Tokenizer: tokenizer,
			TokenFilters: []analysis.TokenFilter{
				// Add lowercase filter for English text
				lowercaseFilter,
			},
		}, nil
	})
	
	// Create a new index mapping
	indexMapping := bleve.NewIndexMapping()
	
	// Create document mapping for DocumentIndex
	docMapping := bleve.NewDocumentMapping()
	
	// Configure title field with Chinese analyzer
	titleFieldMapping := bleve.NewTextFieldMapping()
	titleFieldMapping.Analyzer = "jieba_analyzer"
	titleFieldMapping.Store = true
	titleFieldMapping.Index = true
	docMapping.AddFieldMappingsAt("title", titleFieldMapping)
	
	// Configure content field with Chinese analyzer
	contentFieldMapping := bleve.NewTextFieldMapping()
	contentFieldMapping.Analyzer = "jieba_analyzer"
	contentFieldMapping.Store = true
	contentFieldMapping.Index = true
	docMapping.AddFieldMappingsAt("content", contentFieldMapping)
	
	// Configure ID field (no analysis needed)
	idFieldMapping := bleve.NewTextFieldMapping()
	idFieldMapping.Store = true
	idFieldMapping.Index = false
	docMapping.AddFieldMappingsAt("id", idFieldMapping)
	
	// Add document mapping to index mapping
	indexMapping.AddDocumentMapping("DocumentIndex", docMapping)
	
	// Set default analyzer for any other fields
	indexMapping.DefaultAnalyzer = "jieba_analyzer"
	
	return indexMapping, nil
}

// AddDocument adds a document to the index
// For cold documents without pre-computed embeddings, embedding can be nil
func (idx *Index) AddDocument(doc *types.Document, embedding []float32) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	docIdx := &DocumentIndex{
		ID:        doc.ID,
		Title:     doc.Title,
		Content:   doc.Content,
		Embedding: embedding,
	}

	// Index in bleve (BM25)
	if err := idx.bleveIndex.Index(doc.ID, docIdx); err != nil {
		return fmt.Errorf("failed to index document in bleve: %w", err)
	}

	// Index in HNSW (vector) if embedding provided
	if len(embedding) == VectorDimension {
		// Add to HNSW - the ID is the document ID
		// Note: coder/hnsw uses []float32 directly, no conversion needed
		idx.hnswIndex.Add(hnsw.MakeNode(doc.ID, embedding))
	}

	// Store document
	idx.documents[doc.ID] = docIdx

	return nil
}

// Search performs hybrid BM25 + Vector search
// Returns merged and deduplicated results sorted by relevance with keyword-based snippets
func (idx *Index) Search(ctx context.Context, queryText string, queryEmbedding []float32, topK int) ([]SearchResult, error) {
	// Delegate to HybridSearch which now includes snippet extraction
	return idx.HybridSearch(queryText, queryEmbedding, topK)
}

// SearchWithBleve performs a bleve search and returns results with keyword-based snippets
func (idx *Index) SearchWithBleve(queryText string, topK int) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Create query
	q := bleve.NewQueryStringQuery(queryText)
	
	// Create search request
	searchRequest := bleve.NewSearchRequest(q)
	searchRequest.Size = topK
	searchRequest.Fields = []string{"*"}

	// Execute search
	searchResult, err := idx.bleveIndex.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, hit := range searchResult.Hits {
		doc, ok := idx.documents[hit.ID]
		if !ok {
			continue
		}

		// Extract keyword-based snippets
		snippets := idx.ExtractSnippets(doc.Content, queryText)
		content := idx.FormatSnippets(snippets, doc.Content)

		results = append(results, SearchResult{
			DocumentID: doc.ID,
			Title:      doc.Title,
			Content:    content,
			Score:      hit.Score,
			Source:     "bm25",
			Snippets:   snippets,
		})
	}

	return results, nil
}

// SearchWithVector performs vector similarity search
// For vector search, we don't have query text, so we use the first chunk of content as snippet
func (idx *Index) SearchWithVector(queryEmbedding []float32, topK int, queryText string) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(queryEmbedding) != VectorDimension {
		return nil, fmt.Errorf("invalid embedding dimension: expected %d, got %d", VectorDimension, len(queryEmbedding))
	}

	// Search HNSW - coder/hnsw uses []float32 directly
	neighbors := idx.hnswIndex.Search(queryEmbedding, topK)

	var results []SearchResult
	for _, neighbor := range neighbors {
		doc, ok := idx.documents[neighbor.Key]
		if !ok {
			continue
		}

		// Calculate cosine similarity from the query and neighbor vectors
		similarity := float64(ComputeSimilarity(queryEmbedding, neighbor.Value))
		if similarity < 0 {
			similarity = 0
		}

		// If query text exists, extract keyword-based snippets; otherwise return first MaxSnippetLength characters
		var snippets []Snippet
		var content string
		if queryText != "" {
			snippets = idx.ExtractSnippets(doc.Content, queryText)
			content = idx.FormatSnippets(snippets, doc.Content)
		} else {
			end := MaxSnippetLength
			if len(doc.Content) < end {
				end = len(doc.Content)
			}
			content = doc.Content[:end]
			if len(doc.Content) > end {
				content += "..."
			}
			snippets = []Snippet{{
				Text:     content,
				StartPos: 0,
				EndPos:   end,
				Keyword:  "",
			}}
		}

		results = append(results, SearchResult{
			DocumentID: doc.ID,
			Title:      doc.Title,
			Content:    content,
			Score:      similarity,
			Source:     "vector",
			Snippets:   snippets,
		})
	}

	return results, nil
}

// HybridSearch performs hybrid search combining BM25 and vector results
func (idx *Index) HybridSearch(queryText string, queryEmbedding []float32, topK int) ([]SearchResult, error) {
	// Get BM25 results (with keyword-based snippets)
	bm25Results, err := idx.SearchWithBleve(queryText, topK)
	if err != nil {
		idx.logger.Warn("BM25 search failed", zap.Error(err))
		bm25Results = []SearchResult{}
	}

	// Get vector results (also pass queryText for snippet extraction)
	var vectorResults []SearchResult
	if len(queryEmbedding) == VectorDimension {
		vectorResults, err = idx.SearchWithVector(queryEmbedding, topK, queryText)
		if err != nil {
			idx.logger.Warn("Vector search failed", zap.Error(err))
			vectorResults = []SearchResult{}
		}
	}

	// Merge and deduplicate results
	return idx.mergeHybridResults(bm25Results, vectorResults, topK), nil
}

// mergeHybridResults merges BM25 and vector results with deduplication
func (idx *Index) mergeHybridResults(bm25Results, vectorResults []SearchResult, topK int) []SearchResult {
	resultMap := make(map[string]*SearchResult)

	// Add BM25 results with normalized scores
	maxBM25Score := 0.0
	for _, r := range bm25Results {
		if r.Score > maxBM25Score {
			maxBM25Score = r.Score
		}
	}

	for _, r := range bm25Results {
		normalizedScore := r.Score
		if maxBM25Score > 0 {
			normalizedScore = r.Score / maxBM25Score
		}
		resultMap[r.DocumentID] = &SearchResult{
			DocumentID: r.DocumentID,
			Title:      r.Title,
			Content:    r.Content,
			Score:      normalizedScore * 0.5, // Weight: 50% for BM25
			Source:     "bm25",
		}
	}

	// Merge vector results
	for _, r := range vectorResults {
		if existing, ok := resultMap[r.DocumentID]; ok {
			// Document exists in both, combine scores
			existing.Score = existing.Score + r.Score*0.5 // Add 50% vector weight
			existing.Source = "hybrid"
		} else {
			resultMap[r.DocumentID] = &SearchResult{
				DocumentID: r.DocumentID,
				Title:      r.Title,
				Content:    r.Content,
				Score:      r.Score * 0.5, // Weight: 50% for vector
				Source:     "vector",
			}
		}
	}

	// Convert map to slice and sort by score
	results := make([]SearchResult, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, *r)
	}

	// Sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Return topK results
	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

// RemoveDocument removes a document from the index
func (idx *Index) RemoveDocument(docID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Delete from bleve
	if err := idx.bleveIndex.Delete(docID); err != nil {
		return fmt.Errorf("failed to delete from bleve: %w", err)
	}

	// Delete from HNSW - coder/hnsw Delete returns bool
	if !idx.hnswIndex.Delete(docID) {
		idx.logger.Warn("failed to delete from HNSW", zap.String("doc_id", docID))
	}

	// Delete from documents map
	delete(idx.documents, docID)

	return nil
}

// GetDocumentCount returns the number of documents in the index
func (idx *Index) GetDocumentCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.documents)
}

// Close closes the index
func (idx *Index) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.bleveIndex.Close()
}

// truncateContent truncates content to max length
func (idx *Index) truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// ExtractSnippets extracts relevant snippets from content based on query keywords
// 1. Keyword positioning: locate query term positions in original text
// 2. Context window: extract fixed chars before/after keyword (e.g., 200 chars) as snippet
// 3. Deduplication & merging: merge overlapping snippets from multiple keyword hits
func (idx *Index) ExtractSnippets(content string, query string) []Snippet {
	// Extract query keywords
	keywords := extractKeywords(query, 10)
	if len(keywords) == 0 {
		// No keywords found, return first MaxSnippetLength characters as default snippet
		end := MaxSnippetLength
		if len(content) < end {
			end = len(content)
		}
		return []Snippet{{
			Text:     content[:end],
			StartPos: 0,
			EndPos:   end,
			Keyword:  "",
		}}
	}

	// Find all keyword positions and create snippets
	var snippets []Snippet
	contentLower := strings.ToLower(content)

	for _, keyword := range keywords {
		keywordLower := strings.ToLower(keyword)
		// Find all occurrence positions
		start := 0
		for {
			pos := strings.Index(contentLower[start:], keywordLower)
			if pos == -1 {
				break
			}
			actualPos := start + pos
			
			// Calculate snippet boundaries
			snippetStart := actualPos - ContextWindowSize
			if snippetStart < 0 {
				snippetStart = 0
			}
			snippetEnd := actualPos + len(keyword) + ContextWindowSize
			if snippetEnd > len(content) {
				snippetEnd = len(content)
			}
			
			snippets = append(snippets, Snippet{
				Text:     content[snippetStart:snippetEnd],
				StartPos: snippetStart,
				EndPos:   snippetEnd,
				Keyword:  keyword,
			})
			
			// Continue to find next position
			start = actualPos + 1
			
			// Limit match count per keyword
			if len(snippets) >= MaxSnippetsPerDoc*2 {
				break
			}
		}
	}

	if len(snippets) == 0 {
		// No keyword hits, return first MaxSnippetLength characters
		end := MaxSnippetLength
		if len(content) < end {
			end = len(content)
		}
		return []Snippet{{
			Text:     content[:end],
			StartPos: 0,
			EndPos:   end,
			Keyword:  "",
		}}
	}

	// Sort by position
	for i := 0; i < len(snippets); i++ {
		for j := i + 1; j < len(snippets); j++ {
			if snippets[j].StartPos < snippets[i].StartPos {
				snippets[i], snippets[j] = snippets[j], snippets[i]
			}
		}
	}

	// Merge overlapping or adjacent snippets
	merged := mergeSnippets(snippets, content)

	// Limit return count
	if len(merged) > MaxSnippetsPerDoc {
		merged = merged[:MaxSnippetsPerDoc]
	}

	return merged
}

// mergeSnippets merges overlapping or adjacent snippets
func mergeSnippets(snippets []Snippet, originalContent string) []Snippet {
	if len(snippets) <= 1 {
		return snippets
	}

	var merged []Snippet
	current := snippets[0]

	for i := 1; i < len(snippets); i++ {
		next := snippets[i]
		
		// Check if overlapping or adjacent (distance less than MergeDistance)
		if next.StartPos <= current.EndPos+MergeDistance {
			// Merge snippets - extend end position
			if next.EndPos > current.EndPos {
				current.EndPos = next.EndPos
			}
			// Merge keyword lists
			if current.Keyword != "" && next.Keyword != "" {
				current.Keyword = current.Keyword + "," + next.Keyword
			} else if next.Keyword != "" {
				current.Keyword = next.Keyword
			}
		} else {
			// Save current snippet, re-extract full text from original content
			current.Text = originalContent[current.StartPos:current.EndPos]
			merged = append(merged, current)
			current = next
		}
	}
	// Add last snippet, re-extract full text from original content
	current.Text = originalContent[current.StartPos:current.EndPos]
	merged = append(merged, current)

	return merged
}

// FormatSnippets formats snippets into a single display string
func (idx *Index) FormatSnippets(snippets []Snippet, originalContent string) string {
	if len(snippets) == 0 {
		return ""
	}

	if len(snippets) == 1 {
		text := snippets[0].Text
		if snippets[0].StartPos > 0 {
			text = "..." + text
		}
		if snippets[0].EndPos < len(originalContent) {
			text = text + "..."
		}
		return text
	}

	// Multiple snippets, join with separator
	var parts []string
	for i, snip := range snippets {
		text := snip.Text
		
		// Add ellipsis
		if snip.StartPos > 0 && i == 0 {
			text = "..." + text
		}
		if snip.EndPos < len(originalContent) && i == len(snippets)-1 {
			text = text + "..."
		}
		
		parts = append(parts, text)
	}

	return strings.Join(parts, "\n...\n")
}

// ExtractSnippet extracts relevant snippet from content based on query keywords
// 1. Keyword positioning: locate query term positions in original text
// 2. Context window: extract fixed chars before/after keyword (e.g., 200 chars)
// 3. Deduplication & merging: merge overlapping snippets from multiple keyword hits
func (idx *Index) ExtractSnippet(content string, query string, windowSize int) string {
	if windowSize <= 0 {
		windowSize = 200
	}

	// Extract keywords from query
	keywords := extractKeywords(query, 10)
	if len(keywords) == 0 {
		// No keywords found, return beginning of content
		return idx.truncateContent(content, windowSize*2)
	}

	// Find all keyword positions in content
	type hit struct {
		start int
		end   int
	}
	var hits []hit

	contentLower := strings.ToLower(content)
	for _, keyword := range keywords {
		keywordLower := strings.ToLower(keyword)
		pos := 0
		for {
			idx := strings.Index(contentLower[pos:], keywordLower)
			if idx == -1 {
				break
			}
			actualPos := pos + idx
			hits = append(hits, hit{
				start: actualPos,
				end:   actualPos + len(keyword),
			})
			pos = actualPos + 1
		}
	}

	if len(hits) == 0 {
		// No keyword hits, return beginning of content
		return idx.truncateContent(content, windowSize*2)
	}

	// Sort hits by position
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].start < hits[i].start {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}

	// Merge overlapping/adjacent hits
	var merged []hit
	current := hits[0]
	for i := 1; i < len(hits); i++ {
		// If next hit overlaps or is adjacent (within window), merge them
		if hits[i].start <= current.end+windowSize {
			if hits[i].end > current.end {
				current.end = hits[i].end
			}
		} else {
			merged = append(merged, current)
			current = hits[i]
		}
	}
	merged = append(merged, current)

	// Extract snippets with context windows
	var snippets []string
	for _, h := range merged {
		// Calculate snippet boundaries
		start := h.start - windowSize
		if start < 0 {
			start = 0
		}
		end := h.end + windowSize
		if end > len(content) {
			end = len(content)
		}

		snippet := content[start:end]

		// Add ellipsis if truncated
		if start > 0 {
			snippet = "..." + snippet
		}
		if end < len(content) {
			snippet = snippet + "..."
		}

		snippets = append(snippets, snippet)
	}

	// Join snippets with separator
	result := strings.Join(snippets, "\n---\n")

	// Limit total length
	maxTotalLen := windowSize * 4
	if len(result) > maxTotalLen {
		result = result[:maxTotalLen] + "..."
	}

	return result
}

// RebuildFromDocuments rebuilds the index from a list of documents
// This is called on startup to load all cold documents
func (idx *Index) RebuildFromDocuments(ctx context.Context, docs []types.Document, getEmbedding func(doc *types.Document) []float32) error {
	start := time.Now()
	idx.logger.Info("Rebuilding memory index", zap.Int("document_count", len(docs)))

	// Clear existing index
	if err := idx.Close(); err != nil {
		idx.logger.Warn("Failed to close existing index", zap.Error(err))
	}

	// Create new index
	newIdx, err := NewIndex(idx.logger)
	if err != nil {
		return fmt.Errorf("failed to create new index: %w", err)
	}

	*idx = *newIdx

	// Add documents
	successCount := 0
	for i := range docs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		doc := &docs[i]
		embedding := getEmbedding(doc)
		
		if err := idx.AddDocument(doc, embedding); err != nil {
			idx.logger.Warn("Failed to add document to index",
				zap.String("doc_id", doc.ID),
				zap.Error(err))
			continue
		}
		successCount++

		// Log progress every 1000 documents
		if (i+1)%1000 == 0 {
			idx.logger.Info("Index rebuild progress",
				zap.Int("processed", i+1),
				zap.Int("total", len(docs)))
		}
	}

	idx.logger.Info("Index rebuild completed",
		zap.Int("success", successCount),
		zap.Int("failed", len(docs)-successCount),
		zap.Duration("duration", time.Since(start)))

	return nil
}

// GenerateSimpleEmbedding generates a simple embedding for a document
// This is a fallback when no external embedding service is available
// Uses a simple bag-of-words approach with random projection
func GenerateSimpleEmbedding(content string) []float32 {
	// Simple hash-based embedding for fallback
	// In production, use a proper embedding model like MiniLM
	embedding := make([]float32, VectorDimension)
	
	// Simple character n-gram hashing
	for i := 0; i < len(content)-3 && i < 10000; i++ {
		// Use 4-gram hash
		hash := 0
		for j := 0; j < 4 && i+j < len(content); j++ {
			hash = hash*31 + int(content[i+j])
		}
		idx := int(math.Abs(float64(hash))) % VectorDimension
		embedding[idx] += 1.0
	}

	// Normalize
	norm := float32(0)
	for _, v := range embedding {
		norm += v * v
	}
	if norm > 0 {
		norm = float32(math.Sqrt(float64(norm)))
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding
}

// ComputeSimilarity computes cosine similarity between two vectors
func ComputeSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// MarshalJSON implements json.Marshaler for DocumentIndex
func (d *DocumentIndex) MarshalJSON() ([]byte, error) {
	type Alias DocumentIndex
	return json.Marshal(&struct {
		*Alias
		EmbeddingLen int `json:"embedding_len"`
	}{
		Alias:        (*Alias)(d),
		EmbeddingLen: len(d.Embedding),
	})
}

// extractKeywords extracts keywords from text for BM25 fallback
func extractKeywords(text string, maxKeywords int) []string {
	// Simple keyword extraction - split by non-alphanumeric
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	})

	// Count frequency
	freq := make(map[string]int)
	for _, w := range words {
		w = strings.ToLower(w)
		if len(w) > 3 {
			freq[w]++
		}
	}

	// Get top keywords
	type wordFreq struct {
		word string
		freq int
	}
	var wf []wordFreq
	for w, f := range freq {
		wf = append(wf, wordFreq{w, f})
	}

	// Sort by frequency
	for i := 0; i < len(wf); i++ {
		for j := i + 1; j < len(wf); j++ {
			if wf[j].freq > wf[i].freq {
				wf[i], wf[j] = wf[j], wf[i]
			}
		}
	}

	// Take top keywords
	var result []string
	for i := 0; i < len(wf) && i < maxKeywords; i++ {
		result = append(result, wf[i].word)
	}

	return result
}
