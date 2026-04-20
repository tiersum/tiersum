// Package coldindex implements the cold-document retrieval index: chapter splitting,
// Bleve BM25 text search, and HNSW vector search (with optional Chinese segmentation via gojieba).
// The index is held in process memory; persistence of documents is the database layer (storage.IColdIndex).
package coldindex

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
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
	"github.com/tiersum/tiersum/internal/storage/coldindex/coldvec"
	"github.com/tiersum/tiersum/pkg/types"
)

const (
	// coldIndexBleveStorageBackend is how TierSum holds the cold Bleve index (see bleve.NewMemOnly).
	coldIndexBleveStorageBackend = "memory"
	// coldIndexBleveTextAnalyzer labels the default title/content analyzer (see createIndexMapping).
	coldIndexBleveTextAnalyzer = "jieba_analyzer (gojieba cut-for-search + to_lower)"

	// VectorDimension is the dimension of cold-document embedding vectors (see types.ColdEmbeddingVectorDimension).
	VectorDimension = types.ColdEmbeddingVectorDimension
	// DefaultHNSWM is the M parameter for HNSW (max connections per layer)
	DefaultHNSWM = 16
	// DefaultHNSWEfConstruction is the efConstruction parameter for HNSW
	DefaultHNSWEfConstruction = 200
	// DefaultHNSWEfSearch is the efSearch parameter for HNSW
	DefaultHNSWEfSearch = 100

	defaultBranchRecallMultiplier = 2
	defaultBranchRecallFloor      = 20
	defaultBranchRecallCeiling    = 200
)

// ChapterIndex represents one cold document chapter in the index (Bleve + HNSW).
type ChapterIndex struct {
	DocumentID string    `json:"document_id"`
	Path       string    `json:"path"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Embedding  []float32 `json:"embedding"`
}

// scoredChapter is an internal ranked row before exposing storage.ColdIndexHit (no API leakage).
type scoredChapter struct {
	DocumentID string
	Path       string
	Title      string
	Content    string
	Score      float64
	source     string // merge bookkeeping only (bm25 / vector / hybrid)
}

// hnswConfig stores HNSW configuration parameters
type hnswConfig struct {
	M        int
	EfSearch int
}

// Index provides BM25 + vector hybrid search for cold document chapters (Bleve + HNSW).
// It composes an inverted (Bleve) index and a vector (HNSW) index over chapters from IColdChapterSplitter.
type Index struct {
	mu                     sync.RWMutex
	inverted               *invertedBleve
	vector                 *vectorHNSW
	documents              map[string]*ChapterIndex // key: chapter path (docID/...)
	docChapterPaths        map[string][]string       // docID -> chapter paths (same keys as DocumentIndex.Path)
	coldChapterMaxTokens   int
	branchRecallMultiplier int // per-branch fetch size = clamp(limit*multiplier, floor, ceiling)
	branchRecallFloor      int
	branchRecallCeiling    int
	coldSplitter           IColdChapterSplitter // swappable for tests or alternate strategies
	textEmbedder           IColdTextEmbedder    // optional; chapter body + query vectors (same model as ingest when set)
	logger                 *zap.Logger
	hnswConfig             hnswConfig
}

// SetTextEmbedder configures text-derived vectors for indexing and Search (optional).
// Call once during startup before concurrent use.
func (idx *Index) SetTextEmbedder(e IColdTextEmbedder) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.textEmbedder = e
}

// GojiebaTokenizer implements bleve's Tokenizer interface using gojieba
type GojiebaTokenizer struct {
	jieba *gojieba.Jieba
}

// findGojiebaDictPath searches for gojieba dictionary files in common locations.
// Priority: GOJIEA_DICT_PATH env > executable-relative > working-directory-relative.
func findGojiebaDictPath() string {
	// 1. Explicit env var (Docker, systemd, etc.)
	if envPath := os.Getenv("GOJIEA_DICT_PATH"); envPath != "" {
		if _, err := os.Stat(filepath.Join(envPath, "jieba.dict.utf8")); err == nil {
			return envPath
		}
	}

	// 2. Relative to executable (release tarball layout: bin alongside third_party/)
	if ex, err := os.Executable(); err == nil {
		execDir := filepath.Dir(ex)
		candidate := filepath.Join(execDir, "third_party", "gojieba", "dict")
		if _, err := os.Stat(filepath.Join(candidate, "jieba.dict.utf8")); err == nil {
			return candidate
		}
	}

	// 3. Current working directory
	if wd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(wd, "third_party", "gojieba", "dict")
		if _, err := os.Stat(filepath.Join(candidate, "jieba.dict.utf8")); err == nil {
			return candidate
		}
	}

	// 4. Fall back to gojieba default (uses compiled-in module cache path)
	return ""
}

// NewGojiebaTokenizer creates a new gojieba tokenizer
func NewGojiebaTokenizer() *GojiebaTokenizer {
	var jieba *gojieba.Jieba
	dictPath := findGojiebaDictPath()
	if dictPath != "" {
		jieba = gojieba.NewJieba(
			filepath.Join(dictPath, "jieba.dict.utf8"),
			filepath.Join(dictPath, "hmm_model.utf8"),
			filepath.Join(dictPath, "user.dict.utf8"),
			filepath.Join(dictPath, "idf.utf8"),
			filepath.Join(dictPath, "stop_words.utf8"),
		)
	} else {
		jieba = gojieba.NewJieba()
	}
	return &GojiebaTokenizer{
		jieba: jieba,
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

// NewIndex creates a new cold index with Chinese text segmentation support
func NewIndex(logger *zap.Logger) (*Index, error) {
	// Create bleve index mapping with Chinese tokenizer
	mapping, err := createIndexMapping()
	if err != nil {
		return nil, fmt.Errorf("failed to create index mapping: %w", err)
	}

	// Create Bleve inverted index
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
		inverted:               newInvertedBleve(bleveIdx),
		vector:                 newVectorHNSW(hnswIdx, VectorDimension),
		documents:              make(map[string]*ChapterIndex),
		docChapterPaths:        make(map[string][]string),
		coldChapterMaxTokens:   types.DefaultColdChapterMaxTokens,
		branchRecallMultiplier: defaultBranchRecallMultiplier,
		branchRecallFloor:      defaultBranchRecallFloor,
		branchRecallCeiling:    defaultBranchRecallCeiling,
		coldSplitter:           DefaultColdChapterSplitter(),
		logger:                 logger,
		hnswConfig: hnswConfig{
			M:        DefaultHNSWM,
			EfSearch: DefaultHNSWEfSearch,
		},
	}, nil
}

// SetColdChapterSplitter replaces the cold markdown chapter extractor (e.g. custom format or LLM-based chunking).
// Passing nil restores the default markdown splitter.
func (idx *Index) SetColdChapterSplitter(s IColdChapterSplitter) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if s == nil {
		idx.coldSplitter = DefaultColdChapterSplitter()
		return
	}
	idx.coldSplitter = s
}

// SetColdChapterMaxTokens sets the estimated token budget per cold chapter (splitter). Non-positive resets to default.
func (idx *Index) SetColdChapterMaxTokens(n int) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if n <= 0 {
		idx.coldChapterMaxTokens = types.DefaultColdChapterMaxTokens
		return
	}
	idx.coldChapterMaxTokens = n
}

// SetColdSearchRecall configures how many hits each sub-index (Bleve / HNSW) fetches before merge.
// Non-positive values use built-in defaults; ceiling is raised to at least floor.
func (idx *Index) SetColdSearchRecall(multiplier, floor, ceiling int) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if multiplier < 1 {
		multiplier = defaultBranchRecallMultiplier
	}
	if floor < 1 {
		floor = defaultBranchRecallFloor
	}
	if ceiling < 1 {
		ceiling = defaultBranchRecallCeiling
	}
	if ceiling < floor {
		ceiling = floor
	}
	idx.branchRecallMultiplier = multiplier
	idx.branchRecallFloor = floor
	idx.branchRecallCeiling = ceiling
}

// branchRecallSize is how many hits each sub-index fetches before mergeHybridResults trims to topK.
func (idx *Index) branchRecallSize(finalLimit int) int {
	idx.mu.RLock()
	mul := idx.branchRecallMultiplier
	floor := idx.branchRecallFloor
	ceil := idx.branchRecallCeiling
	idx.mu.RUnlock()
	if mul < 1 {
		mul = defaultBranchRecallMultiplier
	}
	if floor < 1 {
		floor = defaultBranchRecallFloor
	}
	if ceil < 1 {
		ceil = defaultBranchRecallCeiling
	}
	if ceil < floor {
		ceil = floor
	}
	if finalLimit < 1 {
		finalLimit = 1
	}
	recall := finalLimit * mul
	if recall < floor {
		recall = floor
	}
	if recall > ceil {
		recall = ceil
	}
	return recall
}

// createIndexMapping creates a custom index mapping with Chinese tokenizer
func createIndexMapping() (mapping.IndexMapping, error) {
	// Register gojieba tokenizer in bleve registry (handle duplicate registration)
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Tokenizer already registered, ignore
			}
		}()
		registry.RegisterTokenizer("gojieba", func(config map[string]interface{}, cache *registry.Cache) (analysis.Tokenizer, error) {
			return NewGojiebaTokenizer(), nil
		})
	}()

	// Register custom analyzer that uses gojieba tokenizer (handle duplicate registration)
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Analyzer already registered, ignore
			}
		}()
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
	}()

	// Create a new index mapping
	indexMapping := bleve.NewIndexMapping()

	// Create document mapping for ChapterIndex
	docMapping := bleve.NewDocumentMapping()

	// Configure title field with Chinese analyzer (index only, do not store —
	// full text lives in idx.documents shared with the vector branch).
	titleFieldMapping := bleve.NewTextFieldMapping()
	titleFieldMapping.Analyzer = "jieba_analyzer"
	titleFieldMapping.Store = false
	titleFieldMapping.Index = true
	docMapping.AddFieldMappingsAt("title", titleFieldMapping)

	// Configure content field with Chinese analyzer (index only, do not store).
	contentFieldMapping := bleve.NewTextFieldMapping()
	contentFieldMapping.Analyzer = "jieba_analyzer"
	contentFieldMapping.Store = false
	contentFieldMapping.Index = true
	docMapping.AddFieldMappingsAt("content", contentFieldMapping)

	// Path and document_id are indexed only (not stored) because full metadata is
	// always read from idx.documents after the Bleve search returns the hit key.
	pathFieldMapping := bleve.NewTextFieldMapping()
	pathFieldMapping.Store = false
	pathFieldMapping.Index = false
	docMapping.AddFieldMappingsAt("path", pathFieldMapping)

	docIDFieldMapping := bleve.NewTextFieldMapping()
	docIDFieldMapping.Store = false
	docIDFieldMapping.Index = false
	docMapping.AddFieldMappingsAt("document_id", docIDFieldMapping)

	// Add document mapping to index mapping
	indexMapping.AddDocumentMapping("ChapterIndex", docMapping)

	// Set default analyzer for any other fields
	indexMapping.DefaultAnalyzer = "jieba_analyzer"

	return indexMapping, nil
}

// AddDocument indexes a cold document (implementation splits body and assigns vectors internally).
func (idx *Index) AddDocument(ctx context.Context, doc *types.Document) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if err := idx.removeChaptersForDocLocked(doc.ID); err != nil {
		return err
	}

	maxTok := idx.coldChapterMaxTokens
	if maxTok <= 0 {
		maxTok = types.DefaultColdChapterMaxTokens
	}
	splitter := idx.coldSplitter
	if splitter == nil {
		splitter = DefaultColdChapterSplitter()
	}
	chapters := splitter.Split(doc.ID, doc.Title, doc.Content, maxTok)
	if len(chapters) == 0 {
		return nil
	}
	emb := idx.textEmbedder

	paths := make([]string, 0, len(chapters))
	for _, ch := range chapters {
		text := strings.TrimSpace(ch.Text)
		if text == "" {
			continue
		}
		var vec []float32
		if emb != nil {
			vec = FallbackColdTextEmbedding(ctx, idx.logger, emb, text)
		} else {
			vec = GenerateSimpleEmbedding(text)
		}
		docIdx := &ChapterIndex{
			DocumentID: doc.ID,
			Path:       ch.Path,
			Title:      doc.Title,
			Content:    text,
			Embedding:  vec,
		}
		if err := idx.inverted.indexChapter(ch.Path, docIdx); err != nil {
			return fmt.Errorf("failed to index chapter in bleve: %w", err)
		}
		idx.vector.add(ch.Path, vec)
		idx.documents[ch.Path] = docIdx
		paths = append(paths, ch.Path)
	}
	idx.docChapterPaths[doc.ID] = paths
	return nil
}

func (idx *Index) removeChaptersForDocLocked(docID string) error {
	keys := idx.docChapterPaths[docID]
	if keys == nil {
		if _, ok := idx.documents[docID]; ok {
			keys = []string{docID}
		}
	}
	for _, k := range keys {
		if err := idx.inverted.deleteChapter(k); err != nil {
			idx.logger.Warn("bleve delete chapter", zap.String("key", k), zap.Error(err))
		}
		idx.vector.delete(k)
		delete(idx.documents, k)
	}
	delete(idx.docChapterPaths, docID)
	return nil
}

func mergeResultKeyChapter(r scoredChapter) string {
	if r.Path != "" {
		return r.Path
	}
	return r.DocumentID
}

func scoredToColdHits(rs []scoredChapter) []storage.ColdIndexHit {
	out := make([]storage.ColdIndexHit, 0, len(rs))
	for i := range rs {
		out = append(out, storage.ColdIndexHit{
			DocumentID: rs[i].DocumentID,
			Path:       rs[i].Path,
			Title:      rs[i].Title,
			Content:    rs[i].Content,
			Score:      rs[i].Score,
			Source:     rs[i].source,
		})
	}
	return out
}

// Search ranks matches for the query string.
func (idx *Index) Search(ctx context.Context, queryText string, topK int) ([]storage.ColdIndexHit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	idx.mu.RLock()
	emb := idx.textEmbedder
	idx.mu.RUnlock()
	var queryEmbedding []float32
	if emb != nil {
		queryEmbedding = FallbackColdTextEmbedding(ctx, idx.logger, emb, queryText)
	}
	raw, err := idx.hybridSearch(queryText, queryEmbedding, topK)
	if err != nil {
		return nil, err
	}
	return scoredToColdHits(raw), nil
}

// searchWithBleve runs the text index over indexed cold chapters.
func (idx *Index) searchWithBleve(queryText string, topK int) ([]scoredChapter, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	searchResult, err := idx.inverted.search(queryText, topK)
	if err != nil {
		return nil, err
	}

	var results []scoredChapter
	for _, hit := range searchResult.Hits {
		doc, ok := idx.documents[hit.ID]
		if !ok {
			continue
		}

		results = append(results, scoredChapter{
			DocumentID: doc.DocumentID,
			Path:       doc.Path,
			Title:      doc.Title,
			Content:    doc.Content,
			Score:      hit.Score,
			source:     "bm25",
		})
	}

	return results, nil
}

// searchWithVector runs similarity search over chapter embeddings.
func (idx *Index) searchWithVector(queryEmbedding []float32, topK int) ([]scoredChapter, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(queryEmbedding) != VectorDimension {
		return nil, fmt.Errorf("invalid embedding dimension: expected %d, got %d", VectorDimension, len(queryEmbedding))
	}

	neighbors, err := idx.vector.search(queryEmbedding, topK)
	if err != nil {
		return nil, err
	}

	var results []scoredChapter
	for _, neighbor := range neighbors {
		doc, ok := idx.documents[neighbor.Key]
		if !ok {
			continue
		}

		similarity := float64(ComputeSimilarity(queryEmbedding, neighbor.Value))
		if similarity < 0 {
			similarity = 0
		}

		results = append(results, scoredChapter{
			DocumentID: doc.DocumentID,
			Path:       doc.Path,
			Title:      doc.Title,
			Content:    doc.Content,
			Score:      similarity,
			source:     "vector",
		})
	}

	return results, nil
}

// hybridSearch merges text-index and vector hits, deduplicated by chapter path.
func (idx *Index) hybridSearch(queryText string, queryEmbedding []float32, topK int) ([]scoredChapter, error) {
	recall := idx.branchRecallSize(topK)

	bm25Results, err := idx.searchWithBleve(queryText, recall)
	if err != nil {
		idx.logger.Warn("text index search failed", zap.Error(err))
		bm25Results = []scoredChapter{}
	}

	var vectorResults []scoredChapter
	if len(queryEmbedding) == VectorDimension {
		vectorResults, err = idx.searchWithVector(queryEmbedding, recall)
		if err != nil {
			idx.logger.Warn("vector index search failed", zap.Error(err))
			vectorResults = []scoredChapter{}
		}
	}

	return idx.mergeHybridResults(bm25Results, vectorResults, topK), nil
}

// mergeHybridResults merges text and vector scored chapters with deduplication.
func (idx *Index) mergeHybridResults(bm25Results, vectorResults []scoredChapter, topK int) []scoredChapter {
	resultMap := make(map[string]*scoredChapter)

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
		k := mergeResultKeyChapter(r)
		resultMap[k] = &scoredChapter{
			DocumentID: r.DocumentID,
			Path:       r.Path,
			Title:      r.Title,
			Content:    r.Content,
			Score:      normalizedScore * 0.5,
			source:     "bm25",
		}
	}

	for _, r := range vectorResults {
		k := mergeResultKeyChapter(r)
		if existing, ok := resultMap[k]; ok {
			existing.Score = existing.Score + r.Score*0.5
			existing.source = "hybrid"
		} else {
			resultMap[k] = &scoredChapter{
				DocumentID: r.DocumentID,
				Path:       r.Path,
				Title:      r.Title,
				Content:    r.Content,
				Score:      r.Score * 0.5,
				source:     "vector",
			}
		}
	}

	results := make([]scoredChapter, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, *r)
	}

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

// MarkdownChapters implements storage.IColdIndex: delegates to MarkdownChaptersFromSplit (same splitter and token budget as cold ingest).
func (idx *Index) MarkdownChapters(docID, title, markdown string) []types.Chapter {
	idx.mu.RLock()
	splitter := idx.coldSplitter
	maxTok := idx.coldChapterMaxTokens
	idx.mu.RUnlock()
	return MarkdownChaptersFromSplit(splitter, maxTok, docID, title, markdown)
}

// RemoveDocument removes all chapters for a cold document from the index.
func (idx *Index) RemoveDocument(docID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.removeChaptersForDocLocked(docID)
}

// ApproxEntries implements storage.IColdIndex.
func (idx *Index) ApproxEntries() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.documents)
}

// VectorStats implements storage.IColdIndex.
func (idx *Index) VectorStats() storage.ColdIndexVectorStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	st := storage.ColdIndexVectorStats{
		VectorDim:              VectorDimension,
		HNSWM:                  idx.hnswConfig.M,
		HNSWEfSearch:           idx.hnswConfig.EfSearch,
		TextEmbedderConfigured: idx.textEmbedder != nil,
	}
	if idx.vector != nil && idx.vector.g != nil {
		st.HNSWNodes = idx.vector.g.Len()
		st.VectorDim = idx.vector.dim
	}
	return st
}

// InvertedIndexStats implements storage.IColdIndex.
func (idx *Index) InvertedIndexStats() storage.ColdIndexInvertedStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	st := storage.ColdIndexInvertedStats{
		StorageBackend: coldIndexBleveStorageBackend,
		TextAnalyzer:   coldIndexBleveTextAnalyzer,
	}
	if idx.inverted != nil && idx.inverted.idx != nil {
		if n, err := idx.inverted.idx.DocCount(); err == nil {
			st.BleveDocCount = n
		}
	}
	return st
}

// Close closes the index
func (idx *Index) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.inverted.close()
}

// RebuildFromDocuments rebuilds the index from a list of documents
// This is called on startup to load all cold documents
func (idx *Index) RebuildFromDocuments(ctx context.Context, docs []types.Document) error {
	start := time.Now()
	idx.logger.Info("Rebuilding cold index", zap.Int("document_count", len(docs)))

	idx.mu.Lock()
	savedMaxTok := idx.coldChapterMaxTokens
	savedMul := idx.branchRecallMultiplier
	savedFloor := idx.branchRecallFloor
	savedCeil := idx.branchRecallCeiling
	savedSplitter := idx.coldSplitter
	savedEmb := idx.textEmbedder
	idx.mu.Unlock()

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
	idx.mu.Lock()
	if savedEmb != nil {
		idx.textEmbedder = savedEmb
	}
	if savedMaxTok > 0 {
		idx.coldChapterMaxTokens = savedMaxTok
	}
	if savedMul > 0 && savedFloor > 0 && savedCeil > 0 {
		ceil := savedCeil
		if ceil < savedFloor {
			ceil = savedFloor
		}
		idx.branchRecallMultiplier = savedMul
		idx.branchRecallFloor = savedFloor
		idx.branchRecallCeiling = ceil
	}
	if savedSplitter != nil {
		idx.coldSplitter = savedSplitter
	}
	idx.mu.Unlock()

	// Add documents
	successCount := 0
	for i := range docs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		doc := &docs[i]

		if err := idx.AddDocument(ctx, doc); err != nil {
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

// GenerateSimpleEmbedding generates a simple embedding for a document (hash projection via coldvec).
func GenerateSimpleEmbedding(content string) []float32 {
	return coldvec.SimpleHashEmbedding(content, types.ColdEmbeddingVectorDimension)
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

// MarshalJSON implements json.Marshaler for ChapterIndex
func (d *ChapterIndex) MarshalJSON() ([]byte, error) {
	type Alias ChapterIndex
	return json.Marshal(&struct {
		*Alias
		EmbeddingLen int `json:"embedding_len"`
	}{
		Alias:        (*Alias)(d),
		EmbeddingLen: len(d.Embedding),
	})
}
