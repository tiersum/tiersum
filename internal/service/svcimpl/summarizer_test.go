package svcimpl

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/pkg/types"
)

func TestSummarizerSvc_AnalyzeDocument(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	// Set up valid LLM response
	provider.SetResponse(`{
		"summary": "This is a test document about Go programming",
		"tags": ["golang", "programming", "tutorial"],
		"chapters": [
			{
				"title": "Introduction",
				"summary": "Intro to Go",
				"content": "Go is a programming language"
			},
			{
				"title": "Getting Started",
				"summary": "How to start",
				"content": "Install Go and write your first program"
			}
		]
	}`)

	result, err := summarizer.AnalyzeDocument(ctx, "Go Programming Guide", "# Go Programming\n\nGo is great!")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "This is a test document about Go programming", result.Summary)
	assert.Equal(t, []string{"golang", "programming", "tutorial"}, result.Tags)
	assert.Len(t, result.Chapters, 2)
}

func TestSummarizerSvc_AnalyzeDocument_LLMError(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	// Set error on provider
	provider.SetError(errors.New("llm service unavailable"))

	_, err := summarizer.AnalyzeDocument(ctx, "Test Title", "Test content")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm service unavailable")
}

func TestSummarizerSvc_AnalyzeDocument_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	// Set invalid JSON response - should fallback
	provider.SetResponse("not valid json")

	result, err := summarizer.AnalyzeDocument(ctx, "Go Guide", "Some content about Go programming language")
	require.NoError(t, err) // Should not error, uses fallback
	assert.NotNil(t, result)
	// Fallback creates summary from content
	assert.Contains(t, result.Summary, "Some content")
	// Fallback creates tags from title
	assert.Contains(t, result.Tags, "guide")
}

func TestSummarizerSvc_AnalyzeDocument_TagLimit(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	// Set up response with more than 10 tags
	provider.SetResponse(`{
		"summary": "Test summary",
		"tags": ["tag1", "tag2", "tag3", "tag4", "tag5", "tag6", "tag7", "tag8", "tag9", "tag10", "tag11", "tag12"],
		"chapters": []
	}`)

	result, err := summarizer.AnalyzeDocument(ctx, "Test", "Content")
	require.NoError(t, err)
	assert.Len(t, result.Tags, 10) // Should be limited to 10
}

func TestSummarizerSvc_AnalyzeDocument_TagNormalization(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	// Set up response with mixed case tags and extra spaces
	provider.SetResponse(`{
		"summary": "Test summary",
		"tags": ["  GoLang  ", "PYTHON", "JavaScript"],
		"chapters": []
	}`)

	result, err := summarizer.AnalyzeDocument(ctx, "Test", "Content")
	require.NoError(t, err)
	assert.Equal(t, []string{"golang", "python", "javascript"}, result.Tags)
}

func TestSummarizerSvc_FilterDocuments(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	docs := []types.Document{
		{ID: "doc1", Title: "Go Programming", Content: "Learn Go"},
		{ID: "doc2", Title: "Python Guide", Content: "Learn Python"},
	}

	// Set up valid LLM response
	provider.SetResponse(`[
		{"id": "doc1", "relevance": 0.95},
		{"id": "doc2", "relevance": 0.7}
	]`)

	results, err := summarizer.FilterDocuments(ctx, "go programming", docs)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "doc1", results[0].ID)
	assert.Equal(t, 0.95, results[0].Relevance)
}

func TestSummarizerSvc_FilterDocuments_Empty(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	results, err := summarizer.FilterDocuments(ctx, "query", nil)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestSummarizerSvc_FilterDocuments_LLMError(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	docs := []types.Document{
		{ID: "doc1", Title: "Go Programming"},
		{ID: "doc2", Title: "Python Guide"},
	}

	// Set error on provider - should fallback
	provider.SetError(errors.New("llm error"))

	results, err := summarizer.FilterDocuments(ctx, "query", docs)
	require.NoError(t, err) // Should not error, uses fallback
	assert.Len(t, results, 2)
	// Fallback gives equal relevance
	assert.Equal(t, 0.5, results[0].Relevance)
	assert.Equal(t, 0.5, results[1].Relevance)
}

func TestSummarizerSvc_FilterChapters(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	chapters := []types.Summary{
		{Path: "doc1/intro", Content: "Introduction to Go"},
		{Path: "doc1/chapter1", Content: "Go basics"},
	}

	// Set up valid LLM response
	provider.SetResponse(`[
		{"id": "doc1/chapter1", "relevance": 0.9},
		{"id": "doc1/intro", "relevance": 0.6}
	]`)

	results, err := summarizer.FilterChapters(ctx, "go basics", chapters)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSummarizerSvc_FilterChapters_Empty(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	results, err := summarizer.FilterChapters(ctx, "query", nil)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestSummarizerSvc_FilterL1GroupsByQuery(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	groups := []types.TagGroup{
		{ID: "group1", Name: "Programming Languages", Description: "Programming languages"},
		{ID: "group2", Name: "Databases", Description: "Database systems"},
	}

	// Set up valid LLM response
	provider.SetResponse(`[
		{"id": "group1", "relevance": 0.95}
	]`)

	results, err := summarizer.FilterL1GroupsByQuery(ctx, "programming", groups)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "group1", results[0].ID)
}

func TestSummarizerSvc_FilterL1GroupsByQuery_Empty(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	results, err := summarizer.FilterL1GroupsByQuery(ctx, "query", nil)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestSummarizerSvc_FilterL1GroupsByQuery_LLMError(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	groups := []types.TagGroup{
		{ID: "group1", Name: "Languages"},
	}

	// Set error - should fallback
	provider.SetError(errors.New("llm error"))

	results, err := summarizer.FilterL1GroupsByQuery(ctx, "query", groups)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, 0.5, results[0].Relevance)
}

func TestSummarizerSvc_FilterL2TagsByQuery(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	tags := []types.Tag{
		{ID: "tag1", Name: "golang", DocumentCount: 10},
		{ID: "tag2", Name: "python", DocumentCount: 5},
	}

	// Set up valid LLM response
	provider.SetResponse(`[
		{"tag": "golang", "relevance": 0.9},
		{"tag": "python", "relevance": 0.7}
	]`)

	results, err := summarizer.FilterL2TagsByQuery(ctx, "programming", tags)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "golang", results[0].Tag)
}

func TestSummarizerSvc_FilterL2TagsByQuery_Empty(t *testing.T) {
	ctx := context.Background()
	provider := NewMockLLMProvider()

	summarizer := NewSummarizerSvc(provider, testLogger())

	results, err := summarizer.FilterL2TagsByQuery(ctx, "query", nil)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestSummarizerSvc_extractChapters(t *testing.T) {
	provider := NewMockLLMProvider()
	summarizer := NewSummarizerSvc(provider, testLogger())

	tests := []struct {
		name     string
		content  string
		expected int
		firstTitle string
	}{
		{
			name:     "no headings",
			content:  "Just some plain text without any headings",
			expected: 1,
			firstTitle: "Content",
		},
		{
			name: "single heading",
			content: `# Main Title

This is the content`,
			expected: 1,
			firstTitle: "Main Title",
		},
		{
			name: "multiple headings",
			content: `# Title

Content here

## Chapter 1

Chapter 1 content

## Chapter 2

Chapter 2 content`,
			expected: 3,
			firstTitle: "Title",
		},
		{
			name: "mixed levels",
			content: `# Level 1

Content

## Level 2

More content

### Level 3

Even more`,
			expected: 3,
			firstTitle: "Level 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chapters := summarizer.extractChapters(tt.content)
			assert.Len(t, chapters, tt.expected)
			if tt.expected > 0 {
				assert.Equal(t, tt.firstTitle, chapters[0].Title)
			}
		})
	}
}

func TestSummarizerSvc_parseAnalysisResponse(t *testing.T) {
	provider := NewMockLLMProvider()
	summarizer := NewSummarizerSvc(provider, testLogger())

	tests := []struct {
		name     string
		response string
		wantErr  bool
		check    func(t *testing.T, result *types.DocumentAnalysisResult)
	}{
		{
			name:     "valid JSON",
			response: `{"summary": "Test", "tags": ["tag1"], "chapters": [{"title": "Ch1", "summary": "Sum", "content": "Content"}]}`,
			wantErr:  false,
			check: func(t *testing.T, result *types.DocumentAnalysisResult) {
				assert.Equal(t, "Test", result.Summary)
				assert.Len(t, result.Tags, 1)
				assert.Len(t, result.Chapters, 1)
			},
		},
		{
			name:     "JSON with markdown",
			response: "```json\n{\"summary\": \"Test\", \"tags\": [], \"chapters\": []}\n```",
			wantErr:  false,
		},
		{
			name:     "invalid JSON",
			response: "not json",
			wantErr:  true,
		},
		{
			name:     "no JSON object",
			response: "some text without braces",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := summarizer.parseAnalysisResponse(tt.response)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.check != nil {
					tt.check(t, result)
				}
			}
		})
	}
}

func TestSummarizerSvc_parseFilterResults(t *testing.T) {
	provider := NewMockLLMProvider()
	summarizer := NewSummarizerSvc(provider, testLogger())

	tests := []struct {
		name     string
		response string
		expected int
	}{
		{
			name:     "valid array",
			response: `[{"id": "doc1", "relevance": 0.9}, {"id": "doc2", "relevance": 0.7}]`,
			expected: 2,
		},
		{
			name:     "empty array",
			response: `[]`,
			expected: 0,
		},
		{
			name:     "invalid JSON",
			response: "not json",
			expected: 0,
		},
		{
			name:     "no array",
			response: "{}",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := summarizer.parseFilterResults(tt.response)
			assert.Len(t, results, tt.expected)
		})
	}
}

func TestSummarizerSvc_fallbackAnalysis(t *testing.T) {
	provider := NewMockLLMProvider()
	summarizer := NewSummarizerSvc(provider, testLogger())

	chapters := []types.ChapterInfo{
		{Title: "Chapter 1", Content: "Content"},
	}

	result := summarizer.fallbackAnalysis("Go Programming Guide", makeString(500), chapters)

	assert.NotNil(t, result)
	assert.Contains(t, result.Tags, "programming")
	assert.Contains(t, result.Tags, "guide")
	assert.Len(t, result.Chapters, 1)
	// Summary should be truncated
	assert.LessOrEqual(t, len(result.Summary), 210) // 200 + "..."
	// Chapter summaries must not be empty (filled from content when missing)
	assert.Equal(t, "Content", strings.TrimSpace(result.Chapters[0].Summary))
}

func TestSummarizerSvc_fallbackFilterDocuments(t *testing.T) {
	provider := NewMockLLMProvider()
	summarizer := NewSummarizerSvc(provider, testLogger())

	docs := []types.Document{
		{ID: "doc1", Title: "Doc 1"},
		{ID: "doc2", Title: "Doc 2"},
	}

	results := summarizer.fallbackFilterDocuments(docs)

	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, 0.5, r.Relevance)
	}
}

func TestSummarizerSvc_fallbackFilterChapters(t *testing.T) {
	provider := NewMockLLMProvider()
	summarizer := NewSummarizerSvc(provider, testLogger())

	chapters := []types.Summary{
		{Path: "doc1/ch1"},
		{Path: "doc1/ch2"},
	}

	results := summarizer.fallbackFilterChapters(chapters)

	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, 0.5, r.Relevance)
	}
}

func TestSummarizerSvc_fallbackFilterGroups(t *testing.T) {
	provider := NewMockLLMProvider()
	summarizer := NewSummarizerSvc(provider, testLogger())

	groups := []types.TagGroup{
		{ID: "group1", Name: "Languages"},
		{ID: "group2", Name: "Databases"},
	}

	results := summarizer.fallbackFilterGroups(groups)

	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, 0.5, r.Relevance)
	}
}

func TestSummarizerSvc_parseTagFilterResults(t *testing.T) {
	provider := NewMockLLMProvider()
	summarizer := NewSummarizerSvc(provider, testLogger())

	tests := []struct {
		name     string
		response string
		expected int
		firstTag string
	}{
		{
			name:     "valid array sorted",
			response: `[{"tag": "python", "relevance": 0.7}, {"tag": "golang", "relevance": 0.9}]`,
			expected: 2,
			firstTag: "golang", // Should be sorted by relevance
		},
		{
			name:     "invalid JSON",
			response: "not json",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := summarizer.parseTagFilterResults(tt.response)
			assert.Len(t, results, tt.expected)
			if tt.expected > 0 {
				assert.Equal(t, tt.firstTag, results[0].Tag)
			}
		})
	}
}

func TestSummarizerSvc_fallbackTagFilter(t *testing.T) {
	provider := NewMockLLMProvider()
	summarizer := NewSummarizerSvc(provider, testLogger())

	tags := []types.Tag{
		{ID: "tag1", Name: "golang"},
		{ID: "tag2", Name: "python"},
	}

	results := summarizer.fallbackTagFilter(tags)

	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, 0.5, r.Relevance)
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input   string
		maxLen  int
		expected string
	}{
		{
			input:   "short",
			maxLen:  100,
			expected: "short",
		},
		{
			input:   makeString(200),
			maxLen:  100,
			expected: makeString(100) + "...",
		},
		{
			input:   "exact",
			maxLen:  5,
			expected: "exact",
		},
		{
			input:   "",
			maxLen:  10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(10, len(tt.input))], func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}
