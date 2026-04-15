package common

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/internal/service/svcimpl/stubs"
	"github.com/tiersum/tiersum/pkg/types"
)

func TestSummarizerCore_FilterDocuments(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()

	core := NewSummarizerCore(provider, stubs.TestLogger())

	docs := []types.Document{
		{ID: "doc1", Title: "Go Programming", Content: "Learn Go"},
		{ID: "doc2", Title: "Python Guide", Content: "Learn Python"},
	}

	// Set up valid LLM response
	provider.SetResponse(`[
		{"id": "doc1", "relevance": 0.95},
		{"id": "doc2", "relevance": 0.7}
	]`)

	results, err := core.FilterDocuments(ctx, "go programming", docs)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "doc1", results[0].ID)
	assert.Equal(t, 0.95, results[0].Relevance)
}

func TestSummarizerCore_FilterDocuments_Empty(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()

	core := NewSummarizerCore(provider, stubs.TestLogger())

	results, err := core.FilterDocuments(ctx, "query", nil)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestSummarizerCore_FilterDocuments_LLMError(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()

	core := NewSummarizerCore(provider, stubs.TestLogger())

	docs := []types.Document{
		{ID: "doc1", Title: "Go Programming"},
		{ID: "doc2", Title: "Python Guide"},
	}

	// Set error on provider - should fallback
	provider.SetError(errors.New("llm error"))

	results, err := core.FilterDocuments(ctx, "query", docs)
	require.NoError(t, err) // Should not error, uses fallback
	assert.Len(t, results, 2)
	// Fallback gives equal relevance
	assert.Equal(t, 0.5, results[0].Relevance)
	assert.Equal(t, 0.5, results[1].Relevance)
}

func TestSummarizerCore_FilterChapters(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()

	core := NewSummarizerCore(provider, stubs.TestLogger())

	chapters := []types.Chapter{
		{DocumentID: "doc1", Path: "doc1/intro", Title: "intro", Summary: "Introduction to Go", Content: "Introduction to Go"},
		{DocumentID: "doc1", Path: "doc1/chapter1", Title: "chapter1", Summary: "Go basics", Content: "Go basics"},
	}

	// Set up valid LLM response
	provider.SetResponse(`[
		{"id": "doc1/chapter1", "relevance": 0.9},
		{"id": "doc1/intro", "relevance": 0.6}
	]`)

	results, err := core.FilterChapters(ctx, "go basics", chapters)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSummarizerCore_FilterChapters_Empty(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()

	core := NewSummarizerCore(provider, stubs.TestLogger())

	results, err := core.FilterChapters(ctx, "query", nil)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestSummarizerCore_FilterTopicsByQuery(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()

	core := NewSummarizerCore(provider, stubs.TestLogger())

	groups := []types.Topic{
		{ID: "group1", Name: "Programming Languages", Description: "Programming languages"},
		{ID: "group2", Name: "Databases", Description: "Database systems"},
	}

	// Set up valid LLM response
	provider.SetResponse(`[
		{"id": "group1", "relevance": 0.95}
	]`)

	results, err := core.FilterTopicsByQuery(ctx, "programming", groups)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "group1", results[0].ID)
}

func TestSummarizerCore_FilterTopicsByQuery_Empty(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()

	core := NewSummarizerCore(provider, stubs.TestLogger())

	results, err := core.FilterTopicsByQuery(ctx, "query", nil)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestSummarizerCore_FilterTopicsByQuery_LLMError(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()

	core := NewSummarizerCore(provider, stubs.TestLogger())

	groups := []types.Topic{
		{ID: "group1", Name: "Languages"},
	}

	// Set error - should fallback
	provider.SetError(errors.New("llm error"))

	results, err := core.FilterTopicsByQuery(ctx, "query", groups)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, 0.5, results[0].Relevance)
}

func TestSummarizerCore_FilterTagsByQuery(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()

	core := NewSummarizerCore(provider, stubs.TestLogger())

	tags := []types.Tag{
		{ID: "tag1", Name: "golang", DocumentCount: 10},
		{ID: "tag2", Name: "python", DocumentCount: 5},
	}

	// Set up valid LLM response
	provider.SetResponse(`[
		{"tag": "golang", "relevance": 0.9},
		{"tag": "python", "relevance": 0.7}
	]`)

	results, err := core.FilterTagsByQuery(ctx, "programming", tags)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "golang", results[0].Tag)
}

func TestSummarizerCore_FilterTagsByQuery_Empty(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()

	core := NewSummarizerCore(provider, stubs.TestLogger())

	results, err := core.FilterTagsByQuery(ctx, "query", nil)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestSummarizerCore_extractChapters(t *testing.T) {
	provider := stubs.NewMockLLMProvider()
	core := NewSummarizerCore(provider, stubs.TestLogger())

	tests := []struct {
		name       string
		content    string
		expected   int
		firstTitle string
	}{
		{
			name:       "no headings",
			content:    "Just some plain text without any headings",
			expected:   1,
			firstTitle: "Content",
		},
		{
			name: "single heading",
			content: `# Main Title

This is the content`,
			expected:   1,
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
			expected:   3,
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
			expected:   3,
			firstTitle: "Level 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chapters := core.extractChapters(tt.content)
			assert.Len(t, chapters, tt.expected)
			if tt.expected > 0 {
				assert.Equal(t, tt.firstTitle, chapters[0].Title)
			}
		})
	}
}

func TestSummarizerCore_parseAnalysisResponse(t *testing.T) {
	provider := stubs.NewMockLLMProvider()
	core := NewSummarizerCore(provider, stubs.TestLogger())

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
			result, err := core.parseAnalysisResponse(tt.response)
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

func TestSummarizerCore_parseFilterResults(t *testing.T) {
	provider := stubs.NewMockLLMProvider()
	core := NewSummarizerCore(provider, stubs.TestLogger())

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
			results := core.parseFilterResults(tt.response)
			assert.Len(t, results, tt.expected)
		})
	}
}

func TestSummarizerCore_fallbackAnalysis(t *testing.T) {
	provider := stubs.NewMockLLMProvider()
	core := NewSummarizerCore(provider, stubs.TestLogger())

	chapters := []types.ChapterInfo{
		{Title: "Chapter 1", Content: "Content"},
	}

	result := core.fallbackAnalysis("Go Programming Guide", stubs.MakeString(500), chapters)

	assert.NotNil(t, result)
	assert.Contains(t, result.Tags, "programming")
	assert.Contains(t, result.Tags, "guide")
	assert.Len(t, result.Chapters, 1)
	// Summary should be truncated
	assert.LessOrEqual(t, len(result.Summary), 210) // 200 + "..."
	// Chapter summaries must not be empty (filled from content when missing)
	assert.Equal(t, "Content", strings.TrimSpace(result.Chapters[0].Summary))
}

func TestSummarizerCore_fallbackFilterDocuments(t *testing.T) {
	provider := stubs.NewMockLLMProvider()
	core := NewSummarizerCore(provider, stubs.TestLogger())

	docs := []types.Document{
		{ID: "doc1", Title: "Doc 1"},
		{ID: "doc2", Title: "Doc 2"},
	}

	results := core.fallbackFilterDocuments(docs)

	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, 0.5, r.Relevance)
	}
}

func TestSummarizerCore_fallbackFilterChapters(t *testing.T) {
	provider := stubs.NewMockLLMProvider()
	core := NewSummarizerCore(provider, stubs.TestLogger())

	chapters := []types.Chapter{
		{DocumentID: "doc1", Path: "doc1/ch1", Title: "ch1"},
		{DocumentID: "doc1", Path: "doc1/ch2", Title: "ch2"},
	}

	results := core.fallbackFilterChapters(chapters)

	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, 0.5, r.Relevance)
	}
}

func TestSummarizerCore_fallbackFilterTopics(t *testing.T) {
	provider := stubs.NewMockLLMProvider()
	core := NewSummarizerCore(provider, stubs.TestLogger())

	groups := []types.Topic{
		{ID: "group1", Name: "Languages"},
		{ID: "group2", Name: "Databases"},
	}

	results := core.fallbackFilterTopics(groups)

	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, 0.5, r.Relevance)
	}
}

func TestSummarizerCore_parseTagFilterResults(t *testing.T) {
	provider := stubs.NewMockLLMProvider()
	core := NewSummarizerCore(provider, stubs.TestLogger())

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
			results := core.parseTagFilterResults(tt.response)
			assert.Len(t, results, tt.expected)
			if tt.expected > 0 {
				assert.Equal(t, tt.firstTag, results[0].Tag)
			}
		})
	}
}

func TestSummarizerCore_fallbackTagFilter(t *testing.T) {
	provider := stubs.NewMockLLMProvider()
	core := NewSummarizerCore(provider, stubs.TestLogger())

	tags := []types.Tag{
		{ID: "tag1", Name: "golang"},
		{ID: "tag2", Name: "python"},
	}

	results := core.fallbackTagFilter(tags)

	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, 0.5, r.Relevance)
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{
			input:    "short",
			maxLen:   100,
			expected: "short",
		},
		{
			input:    stubs.MakeString(200),
			maxLen:   100,
			expected: stubs.MakeString(100) + "...",
		},
		{
			input:    "exact",
			maxLen:   5,
			expected: "exact",
		},
		{
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input[:stubs.Min(10, len(tt.input))], func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}
