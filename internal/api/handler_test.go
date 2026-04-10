package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/pkg/types"
)

// Mock implementations for API testing
type mockDocService struct {
	docs map[string]*types.Document
	err  error
}

func newMockDocService() *mockDocService {
	return &mockDocService{
		docs: make(map[string]*types.Document),
	}
}

func (m *mockDocService) Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	doc := &types.Document{
		ID:     "test-id",
		Title:  req.Title,
		Status: types.DocStatusHot,
	}
	m.docs[doc.ID] = doc
	return &types.CreateDocumentResponse{
		ID:     doc.ID,
		Title:  doc.Title,
		Status: doc.Status,
	}, nil
}

func (m *mockDocService) Get(ctx context.Context, id string) (*types.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.docs[id], nil
}

type mockQueryService struct {
	results []types.QueryResult
	err     error
}

func (m *mockQueryService) Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func (m *mockQueryService) ProgressiveQuery(ctx context.Context, req types.ProgressiveQueryRequest) (*types.ProgressiveQueryResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &types.ProgressiveQueryResponse{
		Question: req.Question,
		Results: []types.QueryItem{
			{ID: "doc1", Title: "Test Doc", Relevance: 0.9},
		},
	}, nil
}

type mockTagGroupService struct {
	groups []types.TagGroup
	err    error
}

func (m *mockTagGroupService) ClusterTags(ctx context.Context) error {
	return m.err
}

func (m *mockTagGroupService) ShouldRefresh(ctx context.Context) (bool, error) {
	return false, nil
}

func (m *mockTagGroupService) GetL1Groups(ctx context.Context) ([]types.TagGroup, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.groups, nil
}

func (m *mockTagGroupService) GetL2TagsByGroup(ctx context.Context, groupID string) ([]types.Tag, error) {
	return nil, nil
}

func (m *mockTagGroupService) FilterL2TagsByQuery(ctx context.Context, query string, tags []types.Tag) ([]types.TagFilterResult, error) {
	return nil, nil
}

type mockTagRepo struct {
	tags []types.Tag
	err  error
}

func (m *mockTagRepo) Create(ctx context.Context, tag *types.Tag) error {
	return m.err
}

func (m *mockTagRepo) GetByName(ctx context.Context, name string) (*types.Tag, error) {
	return nil, nil
}

func (m *mockTagRepo) List(ctx context.Context) ([]types.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tags, nil
}

func (m *mockTagRepo) ListByGroup(ctx context.Context, groupID string) ([]types.Tag, error) {
	return nil, nil
}

func (m *mockTagRepo) ListByGroupIDs(ctx context.Context, groupIDs []string, limit int) ([]types.Tag, error) {
	return nil, nil
}

func (m *mockTagRepo) IncrementDocumentCount(ctx context.Context, tagName string) error {
	return nil
}

func (m *mockTagRepo) DeleteAll(ctx context.Context) error {
	return nil
}

func (m *mockTagRepo) GetCount(ctx context.Context) (int, error) {
	return len(m.tags), nil
}

type mockSummaryRepo struct {
	summaries []types.Summary
	err       error
}

func (m *mockSummaryRepo) Create(ctx context.Context, summary *types.Summary) error {
	return m.err
}

func (m *mockSummaryRepo) GetByDocument(ctx context.Context, docID string) ([]types.Summary, error) {
	return nil, nil
}

func (m *mockSummaryRepo) GetByPath(ctx context.Context, path string) (*types.Summary, error) {
	return nil, nil
}

func (m *mockSummaryRepo) QueryByTierAndPrefix(ctx context.Context, tier types.SummaryTier, pathPrefix string) ([]types.Summary, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.summaries, nil
}

func (m *mockSummaryRepo) ListDocumentTierByDocumentIDs(ctx context.Context, documentIDs []string) ([]types.Summary, error) {
	return nil, nil
}

func (m *mockSummaryRepo) ListSourcesByPaths(ctx context.Context, chapterPaths []string) ([]types.Summary, error) {
	return nil, nil
}

func (m *mockSummaryRepo) DeleteByDocument(ctx context.Context, docID string) error {
	return nil
}

func setupTestHandler() (*Handler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := &Handler{
		DocService:      newMockDocService(),
		QueryService:    &mockQueryService{},
		TagGroupService: &mockTagGroupService{},
		TagRepo:         &mockTagRepo{},
		SummaryRepo:     &mockSummaryRepo{},
		DocRepo:         nil,
		MemIndex:        nil,
		Quota:           nil,
		Logger:          zap.NewNop(),
	}

	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return handler, router
}

func TestCreateDocument(t *testing.T) {
	_, router := setupTestHandler()

	req := types.CreateDocumentRequest{
		Title:   "Test Document",
		Content: "# Test\nThis is a test.",
		Format:  "markdown",
	}

	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/documents", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp types.CreateDocumentResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Test Document", resp.Title)
	assert.NotEmpty(t, resp.ID)
}

func TestProgressiveQuery(t *testing.T) {
	_, router := setupTestHandler()

	req := types.ProgressiveQueryRequest{
		Question:   "test query",
		MaxResults: 10,
	}

	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/query/progressive", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp types.ProgressiveQueryResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "test query", resp.Question)
	assert.NotEmpty(t, resp.Results)
}

func TestGetDocument(t *testing.T) {
	handler, router := setupTestHandler()

	// Create a document first
	docService := handler.DocService.(*mockDocService)
	docService.docs["doc1"] = &types.Document{
		ID:    "doc1",
		Title: "Test Doc",
	}

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/api/v1/documents/doc1", nil)

	router.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListTagGroups(t *testing.T) {
	_, router := setupTestHandler()

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/api/v1/tags/groups", nil)

	router.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateDocument_InvalidJSON(t *testing.T) {
	_, router := setupTestHandler()

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/documents", bytes.NewBufferString("invalid json"))
	httpReq.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProgressiveQuery_InvalidJSON(t *testing.T) {
	_, router := setupTestHandler()

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/query/progressive", bytes.NewBufferString("invalid json"))
	httpReq.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
