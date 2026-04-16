package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// stubProgramAuth satisfies service.IProgramAuth for handler tests (always initialized; permissive API key).
type stubProgramAuth struct{}

func (stubProgramAuth) IsSystemInitialized(ctx context.Context) (bool, error) {
	return true, nil
}

func (stubProgramAuth) ValidateAPIKey(ctx context.Context, bearerToken string) (*service.APIKeyPrincipal, error) {
	return &service.APIKeyPrincipal{KeyID: "test", Scope: types.AuthScopeAdmin, Name: "test"}, nil
}

func (stubProgramAuth) APIKeyMeetsScope(principal *service.APIKeyPrincipal, requiredScope string) bool {
	return true
}

func (stubProgramAuth) RecordAPIKeyUse(ctx context.Context, keyID, method, path, clientIP string) error {
	return nil
}

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

func (m *mockDocService) CreateDocument(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error) {
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

func (m *mockDocService) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.docs[id], nil
}

func (m *mockDocService) ListDocuments(ctx context.Context, limit int) ([]types.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	if limit <= 0 {
		limit = 200
	}
	out := make([]types.Document, 0, len(m.docs))
	for _, d := range m.docs {
		if d != nil {
			out = append(out, *d)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (m *mockDocService) ListHotDocumentsWithSummariesByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error) {
	_ = ctx
	_ = tags
	_ = limit
	if m.err != nil {
		return nil, m.err
	}
	return nil, nil
}

type mockQueryService struct {
	err error
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

type mockTopicService struct {
	topics []types.Topic
	err    error
}

func (m *mockTopicService) RegroupTags(ctx context.Context) error {
	return m.err
}

func (m *mockTopicService) ShouldRefresh(ctx context.Context) (bool, error) {
	return false, nil
}

func (m *mockTopicService) ListTopics(ctx context.Context) ([]types.Topic, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.topics, nil
}

type mockReadServices struct {
	err error
}

func (m *mockReadServices) ListTags(ctx context.Context, topicIDs []string, byTopicLimit int, listAllCap int) ([]types.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []types.Tag{}, nil
}

func (m *mockReadServices) ListChaptersByDocumentID(ctx context.Context, documentID string) ([]types.Chapter, error) {
	return nil, m.err
}

func (m *mockReadServices) ExtractChaptersFromMarkdown(ctx context.Context, doc *types.Document) ([]types.Chapter, error) {
	if m.err != nil {
		return nil, m.err
	}
	if doc == nil || strings.TrimSpace(doc.Content) == "" {
		return nil, nil
	}
	return []types.Chapter{{
		DocumentID: doc.ID,
		Path:       doc.ID + "/body",
		Title:      doc.Title,
		Summary:    doc.Content,
		Content:    doc.Content,
	}}, nil
}

func (m *mockReadServices) ListChaptersByDocumentIDs(ctx context.Context, docIDs []string) (map[string][]types.Chapter, error) {
	if m.err != nil {
		return nil, m.err
	}
	return map[string][]types.Chapter{}, nil
}

func (m *mockReadServices) SearchColdChapterHits(ctx context.Context, query string, limit int) ([]types.ColdSearchHit, error) {
	return nil, m.err
}

func (m *mockReadServices) SearchHotChapters(ctx context.Context, query string, limit int) ([]types.HotSearchHit, error) {
	_ = ctx
	_ = query
	_ = limit
	return nil, m.err
}

func (m *mockReadServices) ApproxColdIndexEntries() int {
	return 0
}

func (m *mockReadServices) ColdIndexVectorStats() storage.ColdIndexVectorStats {
	return storage.ColdIndexVectorStats{}
}

func (m *mockReadServices) ColdIndexInvertedStats() storage.ColdIndexInvertedStats {
	return storage.ColdIndexInvertedStats{}
}

type mockTraceService struct {
	err error
}

func (m *mockTraceService) ListTraceSummaries(ctx context.Context, limit, offset int) ([]types.OtelTraceSummary, error) {
	_ = ctx
	_ = limit
	_ = offset
	return nil, m.err
}

func (m *mockTraceService) ListSpansByTraceID(ctx context.Context, traceID string) ([]types.OtelSpanDTO, error) {
	_ = ctx
	_ = traceID
	return nil, m.err
}

func setupTestHandler() (*Handler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := &Handler{
		DocService:      newMockDocService(),
		QueryService:    &mockQueryService{},
		TopicService:    &mockTopicService{},
		TagsService:     &mockReadServices{},
		ChaptersService: &mockReadServices{},
		ObsService:      &mockReadServices{},
		TraceService:    &mockTraceService{},
		Quota:           nil,
		Logger:          zap.NewNop(),
		ServerVersion:   "test",
	}

	api := router.Group("/api/v1")
	api.Use(ProgramAuthMiddleware(stubProgramAuth{}))
	handler.RegisterRoutes(api, nil)

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

func TestCreateDocument_IngestValidationError(t *testing.T) {
	h, router := setupTestHandler()
	m := newMockDocService()
	m.err = fmt.Errorf("%w: content too large", service.ErrIngestValidation)
	h.DocService = m

	req := types.CreateDocumentRequest{Title: "t", Content: "x", Format: "markdown"}
	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/documents", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetMonitoringSnapshot(t *testing.T) {
	_, router := setupTestHandler()

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/api/v1/monitoring", nil)
	router.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Contains(t, body, "server")
	srv, ok := body["server"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test", srv["version"])
	goRt, ok := body["go"].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, goRt["version"])
	assert.NotEmpty(t, goRt["goos"])
	assert.NotEmpty(t, goRt["goarch"])
	assert.Contains(t, body, "documents")
	assert.Contains(t, body, "quota")
	assert.Contains(t, body, "cold_index")
	ci, ok := body["cold_index"].(map[string]any)
	require.True(t, ok)
	vec, ok := ci["vector"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, vec, "hnsw_nodes")
	assert.Contains(t, vec, "vector_dim")
	assert.Contains(t, vec, "text_embedder_configured")
	inv, ok := ci["inverted"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, inv, "bleve_doc_count")
	assert.Contains(t, inv, "storage_backend")
	assert.Contains(t, inv, "text_analyzer")
	tel, ok := body["telemetry"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, tel, "http_tracing_active")
	assert.Contains(t, tel, "progressive_debug_allowed")
	pm, ok := body["prometheus_metrics_path"].(string)
	require.True(t, ok)
	assert.Equal(t, "/metrics", pm)
}

func TestCreateDocument_InvalidIngestMode(t *testing.T) {
	_, router := setupTestHandler()

	req := types.CreateDocumentRequest{
		Title:      "Bad",
		Content:    "c",
		Format:     "markdown",
		IngestMode: "warm",
	}
	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/documents", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, httpReq)
	assert.Equal(t, http.StatusBadRequest, w.Code)
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

func TestListTopics(t *testing.T) {
	_, router := setupTestHandler()

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/api/v1/topics", nil)

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
