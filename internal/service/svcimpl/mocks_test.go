package svcimpl

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// MockDocumentRepository is a mock implementation of storage.IDocumentRepository
type MockDocumentRepository struct {
	mu          sync.RWMutex
	docs        map[string]*types.Document
	queryCounts map[string]int
	err         error
}

func NewMockDocumentRepository() *MockDocumentRepository {
	return &MockDocumentRepository{
		docs:        make(map[string]*types.Document),
		queryCounts: make(map[string]int),
	}
}

func (m *MockDocumentRepository) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MockDocumentRepository) Create(ctx context.Context, doc *types.Document) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docs[doc.ID] = doc
	return nil
}

func (m *MockDocumentRepository) GetByID(ctx context.Context, id string) (*types.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.docs[id], nil
}

func (m *MockDocumentRepository) GetRecent(ctx context.Context, limit int) ([]*types.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*types.Document
	for _, doc := range m.docs {
		result = append(result, doc)
	}
	return result, nil
}

func (m *MockDocumentRepository) ListByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []types.Document
	tagSet := make(map[string]bool)
	for _, t := range tags {
		tagSet[t] = true
	}
	for _, doc := range m.docs {
		for _, tag := range doc.Tags {
			if tagSet[tag] {
				result = append(result, *doc)
				break
			}
		}
	}
	return result, nil
}

func (m *MockDocumentRepository) ListMetaByTagsAndStatuses(ctx context.Context, tags []string, statuses []types.DocumentStatus, limit int) ([]types.Document, error) {
	docs, err := m.ListByTags(ctx, tags, limit)
	if err != nil {
		return nil, err
	}
	allowed := make(map[types.DocumentStatus]struct{}, len(statuses))
	for _, s := range statuses {
		allowed[s] = struct{}{}
	}
	var out []types.Document
	for _, d := range docs {
		if len(allowed) > 0 {
			if _, ok := allowed[d.Status]; !ok {
				continue
			}
		}
		copy := d
		copy.Content = ""
		out = append(out, copy)
	}
	return out, nil
}

func (m *MockDocumentRepository) ListByStatus(ctx context.Context, status types.DocumentStatus, limit int) ([]types.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []types.Document
	for _, doc := range m.docs {
		if doc.Status == status {
			result = append(result, *doc)
		}
	}
	return result, nil
}

func (m *MockDocumentRepository) IncrementQueryCount(ctx context.Context, docID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryCounts[docID]++
	return nil
}

func (m *MockDocumentRepository) UpdateStatus(ctx context.Context, docID string, status types.DocumentStatus) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if doc, ok := m.docs[docID]; ok {
		doc.Status = status
	}
	return nil
}

func (m *MockDocumentRepository) UpdateHotScore(ctx context.Context, docID string, score float64) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if doc, ok := m.docs[docID]; ok {
		doc.HotScore = score
	}
	return nil
}

func (m *MockDocumentRepository) ListAll(ctx context.Context, limit int) ([]types.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []types.Document
	for _, doc := range m.docs {
		result = append(result, *doc)
	}
	return result, nil
}

// MockSummaryRepository is a mock implementation of storage.ISummaryRepository
type MockSummaryRepository struct {
	mu       sync.RWMutex
	summaries map[string]*types.Summary
	err      error
}

func NewMockSummaryRepository() *MockSummaryRepository {
	return &MockSummaryRepository{
		summaries: make(map[string]*types.Summary),
	}
}

func (m *MockSummaryRepository) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MockSummaryRepository) Create(ctx context.Context, summary *types.Summary) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := summary.ID
	if key == "" {
		key = summary.Path
	}
	m.summaries[key] = summary
	return nil
}

func (m *MockSummaryRepository) GetByDocument(ctx context.Context, docID string) ([]types.Summary, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []types.Summary
	for _, s := range m.summaries {
		if s.DocumentID == docID {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (m *MockSummaryRepository) GetByPath(ctx context.Context, path string) (*types.Summary, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.summaries {
		if s.Path == path {
			return s, nil
		}
	}
	return nil, nil
}

func (m *MockSummaryRepository) ListDocumentTierByDocumentIDs(ctx context.Context, documentIDs []string) ([]types.Summary, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []types.Summary
	want := make(map[string]struct{}, len(documentIDs))
	for _, id := range documentIDs {
		want[id] = struct{}{}
	}
	for _, s := range m.summaries {
		if s.Tier != types.TierDocument {
			continue
		}
		if _, ok := want[s.DocumentID]; ok {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (m *MockSummaryRepository) ListSourcesByPaths(ctx context.Context, chapterPaths []string) ([]types.Summary, error) {
	if m.err != nil {
		return nil, m.err
	}
	_ = chapterPaths
	return nil, nil
}

func (m *MockSummaryRepository) QueryByTierAndPrefix(ctx context.Context, tier types.SummaryTier, pathPrefix string) ([]types.Summary, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []types.Summary
	for _, s := range m.summaries {
		if s.Tier == tier && len(s.Path) >= len(pathPrefix) && s.Path[:len(pathPrefix)] == pathPrefix {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (m *MockSummaryRepository) DeleteByDocument(ctx context.Context, docID string) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.summaries {
		if s.DocumentID == docID {
			delete(m.summaries, id)
		}
	}
	return nil
}

// MockTagRepository is a mock implementation of storage.ITagRepository
type MockTagRepository struct {
	mu   sync.RWMutex
	tags map[string]*types.Tag
	err  error
}

func NewMockTagRepository() *MockTagRepository {
	return &MockTagRepository{
		tags: make(map[string]*types.Tag),
	}
}

func (m *MockTagRepository) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MockTagRepository) Create(ctx context.Context, tag *types.Tag) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tags[tag.Name] = tag
	return nil
}

func (m *MockTagRepository) GetByName(ctx context.Context, name string) (*types.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tags[name], nil
}

func (m *MockTagRepository) List(ctx context.Context) ([]types.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []types.Tag
	for _, tag := range m.tags {
		result = append(result, *tag)
	}
	return result, nil
}

func (m *MockTagRepository) ListByGroup(ctx context.Context, groupID string) ([]types.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []types.Tag
	for _, tag := range m.tags {
		if tag.GroupID == groupID {
			result = append(result, *tag)
		}
	}
	return result, nil
}

func (m *MockTagRepository) ListByGroupIDs(ctx context.Context, groupIDs []string, limit int) ([]types.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	want := make(map[string]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		want[id] = struct{}{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []types.Tag
	for _, tag := range m.tags {
		if _, ok := want[tag.GroupID]; ok {
			result = append(result, *tag)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockTagRepository) IncrementDocumentCount(ctx context.Context, tagName string) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if tag, ok := m.tags[tagName]; ok {
		tag.DocumentCount++
	}
	return nil
}

func (m *MockTagRepository) DeleteAll(ctx context.Context) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tags = make(map[string]*types.Tag)
	return nil
}

func (m *MockTagRepository) GetCount(ctx context.Context) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tags), nil
}

// MockTagGroupRepository is a mock implementation of storage.ITagGroupRepository
type MockTagGroupRepository struct {
	mu     sync.RWMutex
	groups map[string]*types.TagGroup
	err    error
}

func NewMockTagGroupRepository() *MockTagGroupRepository {
	return &MockTagGroupRepository{
		groups: make(map[string]*types.TagGroup),
	}
}

func (m *MockTagGroupRepository) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MockTagGroupRepository) Create(ctx context.Context, group *types.TagGroup) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := group.ID
	if key == "" {
		key = group.Name
	}
	m.groups[key] = group
	return nil
}

func (m *MockTagGroupRepository) GetByID(ctx context.Context, id string) (*types.TagGroup, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.groups[id], nil
}

func (m *MockTagGroupRepository) List(ctx context.Context) ([]types.TagGroup, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []types.TagGroup
	for _, g := range m.groups {
		result = append(result, *g)
	}
	return result, nil
}

func (m *MockTagGroupRepository) DeleteAll(ctx context.Context) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groups = make(map[string]*types.TagGroup)
	return nil
}

func (m *MockTagGroupRepository) GetCount(ctx context.Context) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.groups), nil
}

// MockInMemoryIndex is a mock implementation of storage.IInMemoryIndex
type MockInMemoryIndex struct {
	mu        sync.RWMutex
	docs      map[string]*types.Document
	searchResults []storage.SearchResult
	err       error
}

func NewMockInMemoryIndex() *MockInMemoryIndex {
	return &MockInMemoryIndex{
		docs: make(map[string]*types.Document),
	}
}

func (m *MockInMemoryIndex) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MockInMemoryIndex) SetSearchResults(results []storage.SearchResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.searchResults = results
}

func (m *MockInMemoryIndex) AddDocument(doc *types.Document, embedding []float32) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docs[doc.ID] = doc
	return nil
}

func (m *MockInMemoryIndex) RemoveDocument(docID string) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.docs, docID)
	return nil
}

func (m *MockInMemoryIndex) Search(ctx context.Context, queryText string, queryEmbedding []float32, topK int) ([]storage.SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.searchResults, nil
}

func (m *MockInMemoryIndex) SearchWithBleve(queryText string, topK int) ([]storage.SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.searchResults, nil
}

func (m *MockInMemoryIndex) SearchWithVector(queryEmbedding []float32, topK int, queryText string) ([]storage.SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.searchResults, nil
}

func (m *MockInMemoryIndex) HybridSearch(queryText string, queryEmbedding []float32, topK int) ([]storage.SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.searchResults, nil
}

func (m *MockInMemoryIndex) GetDocumentCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.docs)
}

func (m *MockInMemoryIndex) RebuildFromDocuments(ctx context.Context, docs []types.Document, getEmbedding func(doc *types.Document) []float32) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docs = make(map[string]*types.Document)
	for i := range docs {
		m.docs[docs[i].ID] = &docs[i]
	}
	return nil
}

func (m *MockInMemoryIndex) Close() error {
	return nil
}

// MockIndexer is a mock implementation of service.IIndexer
type MockIndexer struct {
	mu      sync.RWMutex
	indexed map[string]*types.DocumentAnalysisResult
	err     error
}

func NewMockIndexer() *MockIndexer {
	return &MockIndexer{
		indexed: make(map[string]*types.DocumentAnalysisResult),
	}
}

func (m *MockIndexer) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MockIndexer) Index(ctx context.Context, doc *types.Document, analysis *types.DocumentAnalysisResult) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.indexed[doc.ID] = analysis
	return nil
}

func (m *MockIndexer) GetIndexed(docID string) *types.DocumentAnalysisResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.indexed[docID]
}

// MockSummarizer is a mock implementation of service.ISummarizer
type MockSummarizer struct {
	mu              sync.RWMutex
	analysisResult  *types.DocumentAnalysisResult
	filterResults   []types.LLMFilterResult
	tagFilterResults []types.TagFilterResult
	err             error
}

func NewMockSummarizer() *MockSummarizer {
	return &MockSummarizer{}
}

func (m *MockSummarizer) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MockSummarizer) SetAnalysisResult(result *types.DocumentAnalysisResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.analysisResult = result
}

func (m *MockSummarizer) SetFilterResults(results []types.LLMFilterResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filterResults = results
}

func (m *MockSummarizer) SetTagFilterResults(results []types.TagFilterResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tagFilterResults = results
}

func (m *MockSummarizer) AnalyzeDocument(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.analysisResult != nil {
		return m.analysisResult, nil
	}
	return &types.DocumentAnalysisResult{
		Summary:  "Mock summary for " + title,
		Tags:     []string{"mock", "test"},
		Chapters: []types.ChapterInfo{},
	}, nil
}

func (m *MockSummarizer) FilterDocuments(ctx context.Context, query string, docs []types.Document) ([]types.LLMFilterResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.filterResults != nil {
		return m.filterResults, nil
	}
	// Return all docs with high relevance
	results := make([]types.LLMFilterResult, len(docs))
	for i, doc := range docs {
		results[i] = types.LLMFilterResult{
			ID:        doc.ID,
			Relevance: 0.9,
		}
	}
	return results, nil
}

func (m *MockSummarizer) FilterChapters(ctx context.Context, query string, chapters []types.Summary) ([]types.LLMFilterResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.filterResults != nil {
		return m.filterResults, nil
	}
	results := make([]types.LLMFilterResult, len(chapters))
	for i, ch := range chapters {
		results[i] = types.LLMFilterResult{
			ID:        ch.Path,
			Relevance: 0.85,
		}
	}
	return results, nil
}

// FilterL2TagsByQuery implements the extended interface for tag filtering
func (m *MockSummarizer) FilterL2TagsByQuery(ctx context.Context, query string, tags []types.Tag) ([]types.TagFilterResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.tagFilterResults != nil {
		return m.tagFilterResults, nil
	}
	results := make([]types.TagFilterResult, 0)
	for _, tag := range tags {
		results = append(results, types.TagFilterResult{
			Tag:       tag.Name,
			Relevance: 0.8,
		})
	}
	return results, nil
}

// FilterL1GroupsByQuery implements the extended interface for group filtering
func (m *MockSummarizer) FilterL1GroupsByQuery(ctx context.Context, query string, groups []types.TagGroup) ([]types.LLMFilterResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	results := make([]types.LLMFilterResult, 0)
	for _, g := range groups {
		results = append(results, types.LLMFilterResult{
			ID:        g.ID,
			Relevance: 0.85,
		})
	}
	return results, nil
}

// MockLLMProvider is a mock implementation of client.ILLMProvider
type MockLLMProvider struct {
	mu       sync.RWMutex
	response string
	err      error
}

func NewMockLLMProvider() *MockLLMProvider {
	return &MockLLMProvider{
		response: `{"summary": "mock summary", "tags": ["test"], "chapters": []}`,
	}
}

func (m *MockLLMProvider) SetResponse(response string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.response = response
}

func (m *MockLLMProvider) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MockLLMProvider) Generate(ctx context.Context, prompt string, maxTokens int) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.response, nil
}

// MockQuotaManager is a mock implementation of quota management
type MockQuotaManager struct {
	mu         sync.RWMutex
	available  bool
	shouldFail bool
}

func NewMockQuotaManager() *MockQuotaManager {
	return &MockQuotaManager{
		available: true,
	}
}

func (m *MockQuotaManager) SetAvailable(available bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.available = available
}

func (m *MockQuotaManager) CheckAndConsume() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.shouldFail {
		return false
	}
	return m.available
}

func (m *MockQuotaManager) GetRemaining() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.available {
		return 100
	}
	return 0
}

// testLogger returns a no-op logger for testing
func testLogger() *zap.Logger {
	return zap.NewNop()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// makeString creates a string of specified length with repeating characters
func makeString(length int) string {
	result := make([]byte, length)
	for i := range result {
		result[i] = 'a' + byte(i%26)
	}
	return string(result)
}
