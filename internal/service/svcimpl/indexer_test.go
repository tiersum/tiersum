package svcimpl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/pkg/types"
)

func TestIndexerSvc_Index(t *testing.T) {
	ctx := context.Background()
	summaryRepo := NewMockSummaryRepository()
	summarizer := NewMockSummarizer()

	indexer := NewIndexerSvc(summarizer, summaryRepo, testLogger())

	doc := &types.Document{
		ID:    "doc1",
		Title: "Test Document",
		Tags:  []string{"test", "document"},
	}

	analysis := &types.DocumentAnalysisResult{
		Summary: "This is a test document summary",
		Tags:    []string{"test", "document"},
		Chapters: []types.ChapterInfo{
			{
				Title:   "Introduction",
				Summary: "Intro summary",
				Content: "Introduction content here",
			},
			{
				Title:   "Chapter 1",
				Summary: "Chapter 1 summary",
				Content: "Chapter 1 content here",
			},
		},
	}

	err := indexer.Index(ctx, doc, analysis)
	require.NoError(t, err)

	// Verify document-level summary was created
	docSum, err := summaryRepo.GetByPath(ctx, "doc1")
	require.NoError(t, err)
	require.NotNil(t, docSum)
	assert.Equal(t, types.TierDocument, docSum.Tier)
	assert.Equal(t, analysis.Summary, docSum.Content)

	// Verify chapter summaries were created
	chapters, err := summaryRepo.GetByDocument(ctx, "doc1")
	require.NoError(t, err)
	// 1 document + 2 chapters + 2 source rows
	assert.Len(t, chapters, 5)
}

func TestIndexerSvc_Index_NoChapters(t *testing.T) {
	ctx := context.Background()
	summaryRepo := NewMockSummaryRepository()
	summarizer := NewMockSummarizer()

	indexer := NewIndexerSvc(summarizer, summaryRepo, testLogger())

	doc := &types.Document{
		ID:    "doc2",
		Title: "Simple Document",
	}

	analysis := &types.DocumentAnalysisResult{
		Summary:  "Simple summary",
		Tags:     []string{"simple"},
		Chapters: []types.ChapterInfo{},
	}

	err := indexer.Index(ctx, doc, analysis)
	require.NoError(t, err)

	// Should only have document-level summary
	chapters, err := summaryRepo.GetByDocument(ctx, "doc2")
	require.NoError(t, err)
	assert.Len(t, chapters, 1)
}

func TestIndexerSvc_Index_Reindex(t *testing.T) {
	ctx := context.Background()
	summaryRepo := NewMockSummaryRepository()
	summarizer := NewMockSummarizer()

	indexer := NewIndexerSvc(summarizer, summaryRepo, testLogger())

	doc := &types.Document{
		ID:    "doc3",
		Title: "Reindex Document",
	}

	// First index
	analysis1 := &types.DocumentAnalysisResult{
		Summary: "Original summary",
		Tags:    []string{"original"},
		Chapters: []types.ChapterInfo{
			{Title: "Chapter 1", Summary: "Original chapter", Content: "Original content"},
		},
	}
	err := indexer.Index(ctx, doc, analysis1)
	require.NoError(t, err)

	// Re-index with new analysis
	analysis2 := &types.DocumentAnalysisResult{
		Summary: "Updated summary",
		Tags:    []string{"updated"},
		Chapters: []types.ChapterInfo{
			{Title: "Chapter 1", Summary: "Updated chapter", Content: "Updated content"},
			{Title: "Chapter 2", Summary: "New chapter", Content: "New content"},
		},
	}
	err = indexer.Index(ctx, doc, analysis2)
	require.NoError(t, err)

	// Verify new summaries were created
	chapters, err := summaryRepo.GetByDocument(ctx, "doc3")
	require.NoError(t, err)
	// Should have 1 document + 2 chapters + 2 sources = 5
	assert.Len(t, chapters, 5)

	// Verify document summary was updated
	docSum, err := summaryRepo.GetByPath(ctx, "doc3")
	require.NoError(t, err)
	assert.Equal(t, "Updated summary", docSum.Content)
}

func TestIndexerSvc_Index_CreateError(t *testing.T) {
	ctx := context.Background()
	summaryRepo := NewMockSummaryRepository()
	summarizer := NewMockSummarizer()

	indexer := NewIndexerSvc(summarizer, summaryRepo, testLogger())

	// Set error on summary repo
	summaryRepo.SetError(errors.New("database error"))

	doc := &types.Document{
		ID:    "doc4",
		Title: "Error Document",
	}

	analysis := &types.DocumentAnalysisResult{
		Summary: "Summary",
		Chapters: []types.ChapterInfo{
			{Title: "Chapter 1", Summary: "Chapter summary", Content: "Content"},
		},
	}

	err := indexer.Index(ctx, doc, analysis)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Simple Title",
			expected: "Simple Title",
		},
		{
			input:    "Title/With/Slashes",
			expected: "Title-With-Slashes",
		},
		{
			input:    "Title\\With\\Backslashes",
			expected: "Title-With-Backslashes",
		},
		{
			input:    "  Trimmed Title  ",
			expected: "Trimmed Title",
		},
		{
			input:    "Very Long Title " + makeString(200),
			expected: ("Very Long Title " + makeString(200))[:100],
		},
		{
			input:    "Mixed/Slash\\And Backslash",
			expected: "Mixed-Slash-And Backslash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(20, len(tt.input))], func(t *testing.T) {
			result := sanitizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizePath_LengthLimit(t *testing.T) {
	// Test exact 100 character limit
	input := makeString(100)
	result := sanitizePath(input)
	assert.Equal(t, 100, len(result))

	// Test over 100 characters
	input = makeString(150)
	result = sanitizePath(input)
	assert.Equal(t, 100, len(result))
}
