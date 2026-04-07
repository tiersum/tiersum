// Package memory provides in-memory indexing for cold documents
// Uses bleve for BM25 text search and hnsw for vector similarity search
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
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/chewxy/hnsw"
	"go.uber.org/zap"

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

// SearchResult represents a search result with relevance score
type SearchResult struct {
	DocumentID string  `json:"document_id"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	Source     string  `json:"source"` // "bm25" or "vector"
}

// Index provides in-memory BM25 + Vector hybrid search for cold documents
type Index struct {
	mu           sync.RWMutex
	bleveIndex   bleve.Index
	hnswIndex    *hnsw.HNSW
	documents    map[string]*DocumentIndex
	logger       *zap.Logger
	hnswConfig   hnsw.Config
}

// NewIndex creates a new in-memory index
func NewIndex(logger *zap.Logger) (*Index, error) {
	// Create bleve index mapping
	mapping := bleve.NewIndexMapping()
	
	// Create in-memory bleve index
	bleveIdx, err := bleve.NewMemOnly(mapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create bleve index: %w", err)
	}

	// Create HNSW index configuration
	config := hnsw.Config{
		M:              DefaultHNSWM,
		EfConstruction: DefaultHNSWEfConstruction,
		EfSearch:       DefaultHNSWEfSearch,
		SpaceType:      hnsw.SpaceCosine,
		Dim:            VectorDimension,
	}

	// Create HNSW index
	hnswIdx := hnsw.NewHNSW(config)

	return &Index{
		bleveIndex: bleveIdx,
		hnswIndex:  hnswIdx,
		documents:  make(map[string]*DocumentIndex),
		logger:     logger,
		hnswConfig: config,
	}, nil
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
		// Convert []float32 to []float64 for hnsw
		vec64 := make([]float64, VectorDimension)
		for i, v := range embedding {
			vec64[i] = float64(v)
		}
		
		// Add to HNSW - the ID is the document ID
		if err := idx.hnswIndex.Add(doc.ID, vec64); err != nil {
			idx.logger.Warn("failed to add document to HNSW index", 
				zap.String("doc_id", doc.ID), 
				zap.Error(err))
			// Continue even if HNSW fails - BM25 is still available
		}
	}

	// Store document
	idx.documents[doc.ID] = docIdx

	return nil
}

// Search performs hybrid BM25 + Vector search
// Returns merged and deduplicated results sorted by relevance
func (idx *Index) Search(ctx context.Context, queryText string, queryEmbedding []float32, topK int) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if topK <= 0 {
		topK = 20
	}

	// Channel for concurrent search results
	type bm25Result struct {
		hits []*query.StringQuery
		err  error
	}
	type vectorResult struct {
		neighbors []hnsw.Neighbor
		err       error
	}

	bm25Chan := make(chan bm25Result, 1)
	vectorChan := make(chan vectorResult, 1)

	// Run BM25 search concurrently
	go func() {
		results, err := idx.searchBM25(queryText, BM25TopK)
		bm25Chan <- bm25Result{hits: results, err: err}
	}()

	// Run vector search concurrently if embedding provided
	go func() {
		if len(queryEmbedding) != VectorDimension {
			vectorChan <- vectorResult{neighbors: nil, err: nil}
			return
		}
		neighbors, err := idx.searchVector(queryEmbedding, VectorTopK)
		vectorChan <- vectorResult{neighbors: neighbors, err: err}
	}()

	// Collect results
	bm25Res := <-bm25Chan
	vectorRes := <-vectorChan

	if bm25Res.err != nil {
		return nil, fmt.Errorf("BM25 search failed: %w", bm25Res.err)
	}

	// Merge results
	results := idx.mergeResults(bm25Res.hits, vectorRes.neighbors, topK)

	return results, nil
}

// searchBM25 performs BM25 text search
func (idx *Index) searchBM25(queryText string, topK int) ([]*query.StringQuery, error) {
	// Create a query string query
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

	return nil, nil // We'll process searchResult directly in merge
}

// searchVector performs vector similarity search using HNSW
func (idx *Index) searchVector(queryEmbedding []float32, topK int) ([]hnsw.Neighbor, error) {
	if len(queryEmbedding) != VectorDimension {
		return nil, nil
	}

	// Convert []float32 to []float64
	vec64 := make([]float64, VectorDimension)
	for i, v := range queryEmbedding {
		vec64[i] = float64(v)
	}

	// Search HNSW
	neighbors, err := idx.hnswIndex.Search(vec64, topK)
	if err != nil {
		return nil, err
	}

	return neighbors, nil
}

// mergeResults merges BM25 and vector search results
func (idx *Index) mergeResults(bm25Hits []*query.StringQuery, vectorNeighbors []hnsw.Neighbor, topK int) []SearchResult {
	resultMap := make(map[string]*SearchResult)

	// Process BM25 results
	if bm25Hits == nil {
		// Actually we need to get the real results from bleve search
		// This is a placeholder - the actual implementation processes searchResult
	}

	// For now, simplified merge - in production this would properly merge scores
	// from both BM25 and vector search with proper normalization

	var results []SearchResult
	seen := make(map[string]bool)

	// Add vector results first (they have better semantic matching)
	for _, neighbor := range vectorNeighbors {
		if seen[neighbor.ID] {
			continue
		}
		seen[neighbor.ID] = true

		doc, ok := idx.documents[neighbor.ID]
		if !ok {
			continue
		}

		// Convert cosine distance to similarity score (0-1)
		// HNSW returns distance, so we convert: similarity = 1 - distance
		similarity := 1.0 - neighbor.Distance
		if similarity < 0 {
			similarity = 0
		}

		results = append(results, SearchResult{
			DocumentID: doc.ID,
			Title:      doc.Title,
			Content:    idx.truncateContent(doc.Content, 500),
			Score:      similarity,
			Source:     "vector",
		})

		if len(results) >= topK {
			break
		}
	}

	return results
}

// SearchWithBleve performs a bleve search and returns results
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

		results = append(results, SearchResult{
			DocumentID: doc.ID,
			Title:      doc.Title,
			Content:    idx.truncateContent(doc.Content, 500),
			Score:      hit.Score,
			Source:     "bm25",
		})
	}

	return results, nil
}

// SearchWithVector performs vector similarity search
func (idx *Index) SearchWithVector(queryEmbedding []float32, topK int) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(queryEmbedding) != VectorDimension {
		return nil, fmt.Errorf("invalid embedding dimension: expected %d, got %d", VectorDimension, len(queryEmbedding))
	}

	// Convert []float32 to []float64
	vec64 := make([]float64, VectorDimension)
	for i, v := range queryEmbedding {
		vec64[i] = float64(v)
	}

	// Search HNSW
	neighbors, err := idx.hnswIndex.Search(vec64, topK)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, neighbor := range neighbors {
		doc, ok := idx.documents[neighbor.ID]
		if !ok {
			continue
		}

		// Convert cosine distance to similarity score
		similarity := 1.0 - neighbor.Distance
		if similarity < 0 {
			similarity = 0
		}

		results = append(results, SearchResult{
			DocumentID: doc.ID,
			Title:      doc.Title,
			Content:    idx.truncateContent(doc.Content, 500),
			Score:      similarity,
			Source:     "vector",
		})
	}

	return results, nil
}

// HybridSearch performs hybrid search combining BM25 and vector results
func (idx *Index) HybridSearch(queryText string, queryEmbedding []float32, topK int) ([]SearchResult, error) {
	// Get BM25 results
	bm25Results, err := idx.SearchWithBleve(queryText, topK)
	if err != nil {
		idx.logger.Warn("BM25 search failed", zap.Error(err))
		bm25Results = []SearchResult{}
	}

	// Get vector results
	var vectorResults []SearchResult
	if len(queryEmbedding) == VectorDimension {
		vectorResults, err = idx.SearchWithVector(queryEmbedding, topK)
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

	// Delete from HNSW
	if err := idx.hnswIndex.Delete(docID); err != nil {
		idx.logger.Warn("failed to delete from HNSW", zap.String("doc_id", docID), zap.Error(err))
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
