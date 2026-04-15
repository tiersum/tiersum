package document

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/internal/service/svcimpl/stubs"
)

func TestDocumentAnalyzer_AnalyzeDocument(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()
	analyzer := NewDocumentAnalyzer(provider, stubs.TestLogger())

	provider.SetResponse(`{
		"summary": "This is a test document about Go programming",
		"tags": ["golang", "programming", "tutorial"],
		"chapters": [
			{"title": "Introduction", "summary": "Intro to Go", "content": "Go is a programming language"},
			{"title": "Getting Started", "summary": "How to start", "content": "Install Go and write your first program"}
		]
	}`)

	result, err := analyzer.AnalyzeDocument(ctx, "Go Programming Guide", "# Go Programming\n\nGo is great!")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "This is a test document about Go programming", result.Summary)
	assert.Equal(t, []string{"golang", "programming", "tutorial"}, result.Tags)
	assert.Len(t, result.Chapters, 2)
}

func TestDocumentAnalyzer_AnalyzeDocument_LLMError(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()
	analyzer := NewDocumentAnalyzer(provider, stubs.TestLogger())
	provider.SetError(errors.New("llm service unavailable"))

	_, err := analyzer.AnalyzeDocument(ctx, "Test Title", "Test content")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm service unavailable")
}

func TestDocumentAnalyzer_AnalyzeDocument_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()
	analyzer := NewDocumentAnalyzer(provider, stubs.TestLogger())
	provider.SetResponse("not valid json")

	result, err := analyzer.AnalyzeDocument(ctx, "Go Guide", "Some content about Go programming language")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Summary, "Some content")
	assert.Contains(t, result.Tags, "guide")
}

func TestDocumentAnalyzer_AnalyzeDocument_TagLimit(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()
	analyzer := NewDocumentAnalyzer(provider, stubs.TestLogger())
	provider.SetResponse(`{
		"summary": "Test summary",
		"tags": ["tag1", "tag2", "tag3", "tag4", "tag5", "tag6", "tag7", "tag8", "tag9", "tag10", "tag11", "tag12"],
		"chapters": []
	}`)

	result, err := analyzer.AnalyzeDocument(ctx, "Test", "Content")
	require.NoError(t, err)
	assert.Len(t, result.Tags, 10)
}

func TestDocumentAnalyzer_AnalyzeDocument_TagNormalization(t *testing.T) {
	ctx := context.Background()
	provider := stubs.NewMockLLMProvider()
	analyzer := NewDocumentAnalyzer(provider, stubs.TestLogger())
	provider.SetResponse(`{
		"summary": "Test summary",
		"tags": ["  GoLang  ", "PYTHON", "JavaScript"],
		"chapters": []
	}`)

	result, err := analyzer.AnalyzeDocument(ctx, "Test", "Content")
	require.NoError(t, err)
	assert.Equal(t, []string{"golang", "python", "javascript"}, result.Tags)
}
