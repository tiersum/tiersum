package svcimpl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tiersum/tiersum/internal/telemetry"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"

	"go.opentelemetry.io/otel"
)

func TestQuerySvc_ProgressiveQuery_EmptyTags(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	// No tags in the system
	req := types.ProgressiveQueryRequest{
		Question:   "go programming",
		MaxResults: 10,
	}

	resp, err := svc.ProgressiveQuery(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, req.Question, resp.Question)
	assert.NotNil(t, resp.Results)
}

func TestQuerySvc_ProgressiveQuery_WithHotDocs(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	// Set up test data
	tagRepo.Create(ctx, &types.Tag{ID: "tag1", Name: "golang", GroupID: "group1"})
	docRepo.Create(ctx, &types.Document{
		ID:     "doc1",
		Title:  "Go Programming",
		Tags:   []string{"golang"},
		Status: types.DocStatusHot,
	})
	summaryRepo.Create(ctx, &types.Summary{
		ID:         "sum1",
		DocumentID: "doc1",
		Tier:       types.TierChapter,
		Path:       "doc1/chapter1",
		Content:    "Go is a programming language",
	})

	// Set up mock filter results
	summarizer.SetTagFilterResults([]types.TagFilterResult{
		{Tag: "golang", Relevance: 0.9},
	})
	summarizer.SetFilterResults([]types.LLMFilterResult{
		{ID: "doc1", Relevance: 0.95},
		{ID: "doc1/chapter1", Relevance: 0.9},
	})

	req := types.ProgressiveQueryRequest{
		Question:   "go programming",
		MaxResults: 10,
	}

	resp, err := svc.ProgressiveQuery(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Results)
	assert.True(t, len(resp.Steps) > 0)
}

func TestQuerySvc_filterL2Tags_DirectFilter(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	// Add less than threshold tags (direct filter)
	for i := 0; i < 5; i++ {
		tagRepo.Create(ctx, &types.Tag{ID: "tag" + string(rune('0'+i)), Name: "tag" + string(rune('0'+i))})
	}

	summarizer.SetTagFilterResults([]types.TagFilterResult{
		{Tag: "tag0", Relevance: 0.9},
		{Tag: "tag1", Relevance: 0.8},
	})

	tags, err := svc.filterL2Tags(ctx, "test query")
	require.NoError(t, err)
	assert.Len(t, tags, 2)
}

func TestQuerySvc_filterL2Tags_TwoLevelFilter(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	// Add groups
	groupRepo.Create(ctx, &types.TagGroup{
		ID:   "group1",
		Name: "Programming",
		Tags: []string{"golang", "python"},
	})

	// Add more than threshold tags (two-level filter)
	for i := 0; i < 250; i++ {
		tagRepo.Create(ctx, &types.Tag{
			ID:      "tag" + string(rune('0'+i%10)),
			Name:    "tag" + string(rune('0'+i%10)),
			GroupID: "group1",
		})
	}

	summarizer.SetTagFilterResults([]types.TagFilterResult{
		{Tag: "golang", Relevance: 0.9},
	})

	tags, err := svc.filterL2Tags(ctx, "programming")
	require.NoError(t, err)
	assert.NotNil(t, tags)
}

func TestQuerySvc_queryAndFilterDocuments(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	// Create test documents
	docRepo.Create(ctx, &types.Document{
		ID:      "hot1",
		Title:   "Hot Doc 1",
		Tags:    []string{"golang"},
		Status:  types.DocStatusHot,
		Content: "This is about Go programming",
	})
	docRepo.Create(ctx, &types.Document{
		ID:      "cold1",
		Title:   "Cold Doc 1",
		Tags:    []string{"golang"},
		Status:  types.DocStatusCold,
		Content: "This is also about Go",
	})

	summarizer.SetFilterResults([]types.LLMFilterResult{
		{ID: "hot1", Relevance: 0.9},
	})

	docs, err := svc.queryAndFilterDocuments(ctx, "go programming", []string{"golang"}, 10)
	require.NoError(t, err)
	assert.NotNil(t, docs)
}

func TestQuerySvc_filterColdDocuments(t *testing.T) {
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	docs := []types.Document{
		{
			ID:      "doc1",
			Title:   "Go Programming Guide",
			Content: "Learn Go programming language",
			Tags:    []string{"golang", "programming"},
		},
		{
			ID:      "doc2",
			Title:   "Python Tutorial",
			Content: "Learn Python programming",
			Tags:    []string{"python"},
		},
	}

	filtered := svc.filterColdDocuments("golang", docs)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "doc1", filtered[0].ID)
}

func TestQuerySvc_matchesColdDocument(t *testing.T) {
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	tests := []struct {
		name     string
		doc      types.Document
		keywords []string
		expected bool
	}{
		{
			name:     "match in title",
			doc:      types.Document{Title: "Go Programming", Content: "Some content"},
			keywords: []string{"programming"},
			expected: true,
		},
		{
			name:     "match in content",
			doc:      types.Document{Title: "Some Title", Content: "Go programming guide"},
			keywords: []string{"programming"},
			expected: true,
		},
		{
			name:     "match in tags",
			doc:      types.Document{Title: "Title", Content: "Content", Tags: []string{"golang"}},
			keywords: []string{"golang"},
			expected: true,
		},
		{
			name:     "no match",
			doc:      types.Document{Title: "Title", Content: "Content", Tags: []string{"python"}},
			keywords: []string{"golang"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.matchesColdDocument(tt.doc, tt.keywords)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQuerySvc_queryAndFilterChapters(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	// Create test chapters
	summaryRepo.Create(ctx, &types.Summary{
		ID:         "ch1",
		DocumentID: "doc1",
		Tier:       types.TierChapter,
		Path:       "doc1/chapter1",
		Content:    "Introduction to Go",
	})

	docs := []types.Document{
		{ID: "doc1", Status: types.DocStatusHot},
	}

	summarizer.SetFilterResults([]types.LLMFilterResult{
		{ID: "doc1/chapter1", Relevance: 0.9},
	})

	chapters, err := svc.queryAndFilterChapters(ctx, "go programming", docs)
	require.NoError(t, err)
	assert.NotNil(t, chapters)
}

func TestQuerySvc_createColdDocumentChapter(t *testing.T) {
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	tests := []struct {
		name  string
		doc   types.Document
		query string
	}{
		{
			name: "with keyword match",
			doc: types.Document{
				ID:      "doc1",
				Content: "This is a long content about Go programming. " + makeString(1000),
			},
			query: "go programming",
		},
		{
			name: "without keyword match",
			doc: types.Document{
				ID:      "doc2",
				Content: "This is content about Python. " + makeString(1000),
			},
			query: "java programming",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chapter := svc.createColdDocumentChapter(tt.doc, tt.query)
			require.NotNil(t, chapter)
			assert.Equal(t, types.TierChapter, chapter.Tier)
			assert.NotEmpty(t, chapter.Content)
		})
	}
}

func TestQuerySvc_mergeHotAndColdResults(t *testing.T) {
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	hotResults := []types.QueryItem{
		{ID: "doc1", Title: "Doc 1", Relevance: 0.9},
		{ID: "doc2", Title: "Doc 2", Relevance: 0.8},
	}

	coldResults := []types.QueryItem{
		{ID: "doc2", Title: "Doc 2 Cold", Relevance: 0.85}, // Duplicate
		{ID: "doc3", Title: "Doc 3", Relevance: 0.7},
	}

	merged := svc.mergeHotAndColdResults(hotResults, coldResults, 10)

	// Should have 3 unique documents
	assert.Len(t, merged, 3)

	// Should be sorted by relevance
	assert.Equal(t, "doc1", merged[0].ID)
	assert.Equal(t, 0.9, merged[0].Relevance)
}

func TestQuerySvc_queryColdPath(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	// Set up mock search results
	coldIndex.SetSearchResults([]storage.ColdIndexHit{
		{
			DocumentID: "cold1",
			Path:       "cold1/Intro",
			Title:      "Cold Document",
			Content:    "This is a cold document",
			Score:      0.85,
		},
	})

	req := types.ProgressiveQueryRequest{
		Question:   "test query",
		MaxResults: 10,
	}

	results, step, err := svc.queryColdPath(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Equal(t, "cold_docs", step.Step)
	require.Len(t, results, 1)
	assert.Equal(t, types.DocStatusCold, results[0].Status)
	assert.Equal(t, "cold1/Intro", results[0].Path)
	assert.Equal(t, "Intro", results[0].Title)
}

func TestQuerySvc_queryColdPath_NilIndex(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		nil, // nil index
		nil,
		testLogger(),
	)

	req := types.ProgressiveQueryRequest{
		Question:   "test query",
		MaxResults: 10,
	}

	results, step, err := svc.queryColdPath(ctx, req)
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.Equal(t, "cold_docs", step.Step)
	assert.Equal(t, 0, step.Output)
}

func TestQuerySvc_filterL1Groups(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	// Create groups
	groupRepo.Create(ctx, &types.TagGroup{
		ID:   "group1",
		Name: "Programming",
	})
	groupRepo.Create(ctx, &types.TagGroup{
		ID:   "group2",
		Name: "Databases",
	})

	groups, err := svc.filterL1Groups(ctx, "programming languages")
	require.NoError(t, err)
	assert.NotNil(t, groups)
}

func TestQuerySvc_getL2TagsFromGroups(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	// Create tags in groups
	tagRepo.Create(ctx, &types.Tag{ID: "tag1", Name: "golang", GroupID: "group1"})
	tagRepo.Create(ctx, &types.Tag{ID: "tag2", Name: "python", GroupID: "group1"})
	tagRepo.Create(ctx, &types.Tag{ID: "tag3", Name: "postgres", GroupID: "group2"})

	tags, err := svc.getL2TagsFromGroups(ctx, []string{"group1"})
	require.NoError(t, err)
	assert.Len(t, tags, 2)
}

func TestQuerySvc_extractTagNames(t *testing.T) {
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	tags := []types.Tag{
		{ID: "1", Name: "golang"},
		{ID: "2", Name: "python"},
	}

	names := svc.extractTagNames(tags)
	assert.Equal(t, []string{"golang", "python"}, names)
}

func TestQuerySvc_extractRelevantTags(t *testing.T) {
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	results := []types.TagFilterResult{
		{Tag: "golang", Relevance: 0.9},
		{Tag: "python", Relevance: 0.4}, // Below threshold
		{Tag: "java", Relevance: 0.8},
	}

	names := svc.extractRelevantTags(results)
	assert.Equal(t, []string{"golang", "java"}, names)
}

func TestQuerySvc_buildResults(t *testing.T) {
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	chapters := []types.Summary{
		{
			ID:         "ch1",
			DocumentID: "doc1",
			Tier:       types.TierChapter,
			Path:       "doc1/intro",
			Content:    "Introduction",
		},
	}

	statusMap := map[string]types.DocumentStatus{"doc1": types.DocStatusHot}
	results := svc.buildResults(chapters, statusMap)
	assert.Len(t, results, 1)
	assert.Equal(t, "doc1", results[0].ID)
	assert.Equal(t, "intro", results[0].Title)
	assert.Equal(t, types.TierChapter, results[0].Tier)
	assert.Equal(t, types.DocStatusHot, results[0].Status)
}

func TestExtractTitleFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"doc1/intro", "intro"},
		{"doc1", "doc1"},
		{"", ""},
		{"doc1/chapter1/section1", "section1"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractTitleFromPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"doc1/intro", []string{"doc1", "intro"}},
		{"doc1", []string{"doc1"}},
		{"", nil},
		{"/doc1/intro/", []string{"doc1", "intro"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := splitPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQuerySvc_filterHotDocuments_Error(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	docs := []types.Document{
		{ID: "doc1", Title: "Doc 1", Status: types.DocStatusHot},
	}

	// Set error on summarizer
	summarizer.SetError(errors.New("llm error"))

	_, err := svc.filterHotDocuments(ctx, "query", docs)
	assert.Error(t, err)
}

func TestQuerySvc_filterL2TagsDirect_Error(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		nil,
		testLogger(),
	)

	tags := []types.Tag{
		{ID: "tag1", Name: "golang"},
	}

	// Set error on summarizer
	summarizer.SetError(errors.New("llm error"))

	// Should fallback to returning all tags
	result, err := svc.filterL2TagsDirect(ctx, "query", tags)
	require.NoError(t, err)
	assert.Equal(t, []string{"golang"}, result)
}

func TestBuildProgressiveAnswerPrompt_Numbering(t *testing.T) {
	items := []types.QueryItem{
		{ID: "d1", Title: "T1", Path: "d1/c1", Tier: types.TierChapter, Relevance: 0.9, Content: "alpha"},
	}
	p := buildProgressiveAnswerPrompt("What is X?", items)
	assert.Contains(t, p, "### Reference [^1^]")
	assert.Contains(t, p, "What is X?")
	assert.Contains(t, p, "alpha")
}

func TestQuerySvc_ProgressiveQuery_AnswerFromLLM(t *testing.T) {
	ctx := context.Background()
	docRepo := NewMockDocumentRepository()
	summaryRepo := NewMockSummaryRepository()
	tagRepo := NewMockTagRepository()
	groupRepo := NewMockTagGroupRepository()
	summarizer := NewMockSummarizer()
	coldIndex := NewMockColdIndex()
	llm := NewMockLLMProvider()
	llm.SetResponse("Synthetic answer with [^1^].")

	tagRepo.Create(ctx, &types.Tag{ID: "tag1", Name: "golang", GroupID: "group1"})
	docRepo.Create(ctx, &types.Document{
		ID:     "doc1",
		Title:  "Go Programming",
		Tags:   []string{"golang"},
		Status: types.DocStatusHot,
	})
	summaryRepo.Create(ctx, &types.Summary{
		ID:         "sum1",
		DocumentID: "doc1",
		Tier:       types.TierChapter,
		Path:       "doc1/chapter1",
		Content:    "Go is a programming language",
	})

	summarizer.SetTagFilterResults([]types.TagFilterResult{
		{Tag: "golang", Relevance: 0.9},
	})
	summarizer.SetFilterResults([]types.LLMFilterResult{
		{ID: "doc1", Relevance: 0.95},
		{ID: "doc1/chapter1", Relevance: 0.9},
	})

	svc := NewQuerySvc(
		docRepo,
		summaryRepo,
		tagRepo,
		groupRepo,
		summarizer,
		coldIndex,
		llm,
		testLogger(),
	)

	req := types.ProgressiveQueryRequest{
		Question:   "go programming",
		MaxResults: 10,
	}

	resp, err := svc.ProgressiveQuery(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "Synthetic answer with [^1^].", resp.Answer)
}

func progressiveQueryTestFixture(ctx context.Context) (
	docRepo *MockDocumentRepository,
	summaryRepo *MockSummaryRepository,
	tagRepo *MockTagRepository,
	groupRepo *MockTagGroupRepository,
	summarizer *MockSummarizer,
	coldIndex *MockColdIndex,
	wrapped client.ILLMProvider,
) {
	docRepo = NewMockDocumentRepository()
	summaryRepo = NewMockSummaryRepository()
	tagRepo = NewMockTagRepository()
	groupRepo = NewMockTagGroupRepository()
	summarizer = NewMockSummarizer()
	coldIndex = NewMockColdIndex()
	inner := NewMockLLMProvider()
	inner.SetResponse("Synthetic answer with [^1^].")
	wrapped = NewOTelContextLLM(inner)

	tagRepo.Create(ctx, &types.Tag{ID: "tag1", Name: "golang", GroupID: "group1"})
	docRepo.Create(ctx, &types.Document{
		ID: "doc1", Title: "Go Programming", Tags: []string{"golang"}, Status: types.DocStatusHot,
	})
	summaryRepo.Create(ctx, &types.Summary{
		ID: "sum1", DocumentID: "doc1", Tier: types.TierChapter, Path: "doc1/chapter1", Content: "Go",
	})
	summarizer.SetTagFilterResults([]types.TagFilterResult{{Tag: "golang", Relevance: 0.9}})
	summarizer.SetFilterResults([]types.LLMFilterResult{
		{ID: "doc1", Relevance: 0.95}, {ID: "doc1/chapter1", Relevance: 0.9},
	})
	return
}

func TestQuerySvc_ProgressiveQuery_TraceIDEmptyWithoutRecordingSpan(t *testing.T) {
	ctx := context.Background()
	docRepo, summaryRepo, tagRepo, groupRepo, summarizer, coldIndex, wrapped := progressiveQueryTestFixture(ctx)
	svc := NewQuerySvc(docRepo, summaryRepo, tagRepo, groupRepo, summarizer, coldIndex, wrapped, testLogger())
	resp, err := svc.ProgressiveQuery(ctx, types.ProgressiveQueryRequest{Question: "go programming", MaxResults: 10})
	require.NoError(t, err)
	assert.Empty(t, resp.TraceID)
}

func TestQuerySvc_ProgressiveQuery_TraceIDEmptyWhenServerDisallows(t *testing.T) {
	prev := viper.GetBool("query.allow_progressive_debug")
	viper.Set("query.allow_progressive_debug", false)
	t.Cleanup(func() { viper.Set("query.allow_progressive_debug", prev) })

	ctx := context.Background()
	docRepo, summaryRepo, tagRepo, groupRepo, summarizer, coldIndex, wrapped := progressiveQueryTestFixture(ctx)
	svc := NewQuerySvc(docRepo, summaryRepo, tagRepo, groupRepo, summarizer, coldIndex, wrapped, testLogger())
	resp, err := svc.ProgressiveQuery(ctx, types.ProgressiveQueryRequest{
		Question: "go programming", MaxResults: 10,
	})
	require.NoError(t, err)
	assert.Empty(t, resp.TraceID)
}

func recordingTraceContext(t *testing.T) context.Context {
	t.Helper()
	tr := otel.Tracer("github.com/tiersum/tiersum/query_test")
	ctx, sp := tr.Start(context.Background(), "test_parent")
	t.Cleanup(func() { sp.End() })
	return ctx
}

func TestQuerySvc_ProgressiveQuery_OtelSpansPersistedWhenTraceRecording(t *testing.T) {
	prevAllow := viper.GetBool("query.allow_progressive_debug")
	prevEn := viper.GetBool("telemetry.enabled")
	prevPersist := viper.GetBool("telemetry.persist_to_db")
	prevRatio := viper.GetFloat64("telemetry.sample_ratio")
	viper.Set("query.allow_progressive_debug", true)
	viper.Set("telemetry.enabled", true)
	viper.Set("telemetry.persist_to_db", true)
	viper.Set("telemetry.sample_ratio", 1.0)
	t.Cleanup(func() {
		viper.Set("query.allow_progressive_debug", prevAllow)
		viper.Set("telemetry.enabled", prevEn)
		viper.Set("telemetry.persist_to_db", prevPersist)
		viper.Set("telemetry.sample_ratio", prevRatio)
	})

	mockTrace := NewMockOtelSpanRepository()
	shutdown := telemetry.InitGlobalTracer(mockTrace, zap.NewNop())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	})

	ctx := recordingTraceContext(t)
	docRepo, summaryRepo, tagRepo, groupRepo, summarizer, coldIndex, wrapped := progressiveQueryTestFixture(ctx)
	svc := NewQuerySvc(docRepo, summaryRepo, tagRepo, groupRepo, summarizer, coldIndex, wrapped, testLogger())
	resp, err := svc.ProgressiveQuery(ctx, types.ProgressiveQueryRequest{
		Question: "go programming", MaxResults: 10,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.TraceID)
	flushCtx, flushCancel := context.WithTimeout(context.Background(), 3*time.Second)
	require.NoError(t, telemetry.FlushSpans(flushCtx))
	flushCancel()
	require.NotEmpty(t, mockTrace.Rows)
	for _, r := range mockTrace.Rows {
		assert.Equal(t, resp.TraceID, r.TraceID)
	}
	var names []string
	for _, r := range mockTrace.Rows {
		names = append(names, r.Name)
	}
	assert.Contains(t, names, "progressive_query")
	assert.Contains(t, names, "llm.Generate")
}
