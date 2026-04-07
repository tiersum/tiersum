package svcimpl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/pkg/types"
)

func TestDocumentSvc_shouldBeHot(t *testing.T) {
	quotaManager := NewMockQuotaManager()
	svc := &DocumentSvc{
		quotaManager: quotaManager,
		logger:       testLogger(),
	}

	tests := []struct {
		name               string
		content            string
		forceHot           bool
		hasPrebuiltSummary bool
		quotaAvailable     bool
		expected           bool
	}{
		{
			name:               "force hot always returns true",
			content:            "short",
			forceHot:           true,
			hasPrebuiltSummary: false,
			quotaAvailable:     false,
			expected:           true,
		},
		{
			name:               "prebuilt summary returns true",
			content:            "short",
			forceHot:           false,
			hasPrebuiltSummary: true,
			quotaAvailable:     false,
			expected:           true,
		},
		{
			name:               "long content with quota returns true",
			content:            makeString(5001),
			forceHot:           false,
			hasPrebuiltSummary: false,
			quotaAvailable:     true,
			expected:           true,
		},
		{
			name:               "long content without quota returns false",
			content:            makeString(5001),
			forceHot:           false,
			hasPrebuiltSummary: false,
			quotaAvailable:     false,
			expected:           false,
		},
		{
			name:               "short content with quota returns false",
			content:            "short content",
			forceHot:           false,
			hasPrebuiltSummary: false,
			quotaAvailable:     true,
			expected:           false,
		},
		{
			name:               "short content without quota returns false",
			content:            "short content",
			forceHot:           false,
			hasPrebuiltSummary: false,
			quotaAvailable:     false,
			expected:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quotaManager.SetAvailable(tt.quotaAvailable)
			result := svc.shouldBeHot(tt.content, tt.forceHot, tt.hasPrebuiltSummary)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDocumentSvc_Ingest_ColdDocument(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	indexer := NewMockIndexer()
	summarizer := NewMockSummarizer()
	tagRepo := NewMockTagRepository()
	memIndex := NewMockInMemoryIndex()
	quotaManager := NewMockQuotaManager()

	svc := NewDocumentSvc(
		docRepo,
		indexer,
		summarizer,
		tagRepo,
		memIndex,
		quotaManager,
		testLogger(),
	)

	// Short content should be cold
	req := types.CreateDocumentRequest{
		Title:   "Test Document",
		Content: "Short content",
		Format:  "markdown",
	}

	resp, err := svc.Ingest(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, req.Title, resp.Title)
	assert.Equal(t, types.DocStatusCold, resp.Status)
	assert.Empty(t, resp.Tags)

	// Verify document was saved
	savedDoc, err := docRepo.GetByID(ctx, resp.ID)
	require.NoError(t, err)
	assert.Equal(t, types.DocStatusCold, savedDoc.Status)
}

func TestDocumentSvc_Ingest_HotDocument_WithFullAnalysis(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	indexer := NewMockIndexer()
	summarizer := NewMockSummarizer()
	tagRepo := NewMockTagRepository()
	memIndex := NewMockInMemoryIndex()
	quotaManager := NewMockQuotaManager()

	svc := NewDocumentSvc(
		docRepo,
		indexer,
		summarizer,
		tagRepo,
		memIndex,
		quotaManager,
		testLogger(),
	)

	// Set up mock analysis result
	analysisResult := &types.DocumentAnalysisResult{
		Summary: "Test summary",
		Tags:    []string{"golang", "testing"},
		Chapters: []types.ChapterInfo{
			{Title: "Chapter 1", Summary: "Summary 1"},
		},
	}
	summarizer.SetAnalysisResult(analysisResult)

	// Long content should be hot
	req := types.CreateDocumentRequest{
		Title:   "Test Document",
		Content: makeString(5001),
		Format:  "markdown",
	}

	resp, err := svc.Ingest(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, types.DocStatusHot, resp.Status)
	assert.Equal(t, []string{"golang", "testing"}, resp.Tags)

	// Verify document was saved
	savedDoc, err := docRepo.GetByID(ctx, resp.ID)
	require.NoError(t, err)
	assert.Equal(t, types.DocStatusHot, savedDoc.Status)

	// Verify indexer was called
	indexed := indexer.GetIndexed(resp.ID)
	require.NotNil(t, indexed)
	assert.Equal(t, analysisResult.Tags, indexed.Tags)
}

func TestDocumentSvc_Ingest_WithPrebuiltData(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	indexer := NewMockIndexer()
	summarizer := NewMockSummarizer()
	tagRepo := NewMockTagRepository()
	memIndex := NewMockInMemoryIndex()
	quotaManager := NewMockQuotaManager()

	svc := NewDocumentSvc(
		docRepo,
		indexer,
		summarizer,
		tagRepo,
		memIndex,
		quotaManager,
		testLogger(),
	)

	// Short content with prebuilt summary should be hot
	req := types.CreateDocumentRequest{
		Title:    "Test Document",
		Content:  "Short content",
		Format:   "markdown",
		Tags:     []string{"prebuilt-tag"},
		Summary:  "Prebuilt summary",
		Chapters: []types.ChapterInfo{{Title: "Chapter 1"}},
	}

	resp, err := svc.Ingest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, types.DocStatusHot, resp.Status)
	assert.Contains(t, resp.Tags, "prebuilt-tag")

	// Verify indexer was called with prebuilt data
	indexed := indexer.GetIndexed(resp.ID)
	require.NotNil(t, indexed)
	assert.Equal(t, "Prebuilt summary", indexed.Summary)
}

func TestDocumentSvc_Ingest_ForceHot(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	indexer := NewMockIndexer()
	summarizer := NewMockSummarizer()
	tagRepo := NewMockTagRepository()
	memIndex := NewMockInMemoryIndex()
	quotaManager := NewMockQuotaManager()

	svc := NewDocumentSvc(
		docRepo,
		indexer,
		summarizer,
		tagRepo,
		memIndex,
		quotaManager,
		testLogger(),
	)

	// Short content with ForceHot should be hot
	req := types.CreateDocumentRequest{
		Title:    "Test Document",
		Content:  "Short",
		Format:   "markdown",
		ForceHot: true,
	}

	resp, err := svc.Ingest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, types.DocStatusHot, resp.Status)
}

func TestDocumentSvc_Ingest_DocRepoError(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	indexer := NewMockIndexer()
	summarizer := NewMockSummarizer()
	tagRepo := NewMockTagRepository()
	memIndex := NewMockInMemoryIndex()
	quotaManager := NewMockQuotaManager()

	svc := NewDocumentSvc(
		docRepo,
		indexer,
		summarizer,
		tagRepo,
		memIndex,
		quotaManager,
		testLogger(),
	)

	// Set error on doc repo
	docRepo.SetError(errors.New("database error"))

	req := types.CreateDocumentRequest{
		Title:   "Test Document",
		Content: "Short content",
		Format:  "markdown",
	}

	_, err := svc.Ingest(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

func TestDocumentSvc_Ingest_MergeTags(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	indexer := NewMockIndexer()
	summarizer := NewMockSummarizer()
	tagRepo := NewMockTagRepository()
	memIndex := NewMockInMemoryIndex()
	quotaManager := NewMockQuotaManager()

	svc := NewDocumentSvc(
		docRepo,
		indexer,
		summarizer,
		tagRepo,
		memIndex,
		quotaManager,
		testLogger(),
	)

	// Set up mock analysis result with different tags
	analysisResult := &types.DocumentAnalysisResult{
		Summary: "Test summary",
		Tags:    []string{"generated-tag"},
	}
	summarizer.SetAnalysisResult(analysisResult)

	// Request with prebuilt tags but no summary
	req := types.CreateDocumentRequest{
		Title:   "Test Document",
		Content: makeString(5001),
		Format:  "markdown",
		Tags:    []string{"prebuilt-tag"},
	}

	resp, err := svc.Ingest(ctx, req)
	require.NoError(t, err)

	// Tags should be merged
	assert.Contains(t, resp.Tags, "prebuilt-tag")
	assert.Contains(t, resp.Tags, "generated-tag")
}

func TestDocumentSvc_Get(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	indexer := NewMockIndexer()
	summarizer := NewMockSummarizer()
	tagRepo := NewMockTagRepository()
	memIndex := NewMockInMemoryIndex()
	quotaManager := NewMockQuotaManager()

	svc := NewDocumentSvc(
		docRepo,
		indexer,
		summarizer,
		tagRepo,
		memIndex,
		quotaManager,
		testLogger(),
	)

	// Create a document first
	doc := &types.Document{
		ID:        "doc-123",
		Title:     "Test Document",
		Content:   "Content",
		Format:    "markdown",
		Status:    types.DocStatusHot,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := docRepo.Create(ctx, doc)
	require.NoError(t, err)

	// Get the document
	result, err := svc.Get(ctx, "doc-123")
	require.NoError(t, err)
	assert.Equal(t, doc.ID, result.ID)
	assert.Equal(t, doc.Title, result.Title)

	// Get non-existent document
	result, err = svc.Get(ctx, "non-existent")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestDocumentSvc_GetRecent(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	indexer := NewMockIndexer()
	summarizer := NewMockSummarizer()
	tagRepo := NewMockTagRepository()
	memIndex := NewMockInMemoryIndex()
	quotaManager := NewMockQuotaManager()

	svc := NewDocumentSvc(
		docRepo,
		indexer,
		summarizer,
		tagRepo,
		memIndex,
		quotaManager,
		testLogger(),
	)

	// Create some documents
	for i := 0; i < 5; i++ {
		doc := &types.Document{
			ID:        "doc-" + string(rune('0'+i)),
			Title:     "Document " + string(rune('0'+i)),
			Content:   "Content",
			Format:    "markdown",
			Status:    types.DocStatusHot,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := docRepo.Create(ctx, doc)
		require.NoError(t, err)
	}

	// Get recent documents
	results, err := svc.GetRecent(ctx, 3)
	require.NoError(t, err)
	assert.Len(t, results, 5) // Mock returns all
}

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		maxLen  int
		want    string
	}{
		{
			name:    "content shorter than max",
			content: "short",
			maxLen:  100,
			want:    "short",
		},
		{
			name:    "content longer than max",
			content: "this is a very long content that needs to be truncated",
			maxLen:  10,
			want:    "this is a ...",
		},
		{
			name:    "content equal to max",
			content: "exactlyten",
			maxLen:  10,
			want:    "exactlyten",
		},
		{
			name:    "empty content",
			content: "",
			maxLen:  10,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateContent(tt.content, tt.maxLen)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Helper function to create a string of specified length
func makeString(length int) string {
	result := make([]byte, length)
	for i := range result {
		result[i] = 'a' + byte(i%26)
	}
	return string(result)
}
