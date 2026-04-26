package document

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAnalysis_NilProviderMarkdownChapters(t *testing.T) {
	a := NewDocumentAnalysisGenerator(nil, "dummy template %s %s", nil)
	md := "# One\nbody1\n\n## Two\nbody2\n"
	res, err := a.GenerateAnalysis(context.Background(), "T", md)
	require.Error(t, err)
	require.Nil(t, res)
}

func TestGenerateAnalysis_NilProviderEmptyContent(t *testing.T) {
	a := NewDocumentAnalysisGenerator(nil, "dummy template %s %s", nil)
	res, err := a.GenerateAnalysis(context.Background(), "OnlyTitle", "")
	require.Error(t, err)
	require.Nil(t, res)
}

func TestParseAnalysisJSON_UnwrapsFencedJSON(t *testing.T) {
	raw := "```json\n{\"summary\":\"S\",\"tags\":[\"a\"],\"chapters\":[{\"title\":\"T\",\"summary\":\"CS\",\"content\":\"C\"}]}\n```"
	res, err := parseAnalysisJSON(raw)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "S", res.Summary)
	require.Len(t, res.Chapters, 1)
	require.Equal(t, "T", res.Chapters[0].Title)
}

func TestParseAnalysisJSON_ExtractsJSONObjectFromProse(t *testing.T) {
	raw := "Here is the JSON you requested:\n\n{\"summary\":\"S\",\"tags\":[],\"chapters\":[{\"title\":\"T\",\"summary\":\"CS\",\"content\":\"C\"}]}\n\nThanks!"
	res, err := parseAnalysisJSON(raw)
	require.NoError(t, err)
	require.Equal(t, "S", res.Summary)
	require.Len(t, res.Chapters, 1)
}

func TestParseAnalysisJSON_IgnoresBracesInsideStrings(t *testing.T) {
	raw := "{\"summary\":\"S\",\"tags\":[],\"chapters\":[{\"title\":\"T\",\"summary\":\"CS\",\"content\":\"code { not a brace } inside string\"}]}\nTrailing text"
	res, err := parseAnalysisJSON(raw)
	require.NoError(t, err)
	require.Len(t, res.Chapters, 1)
	require.Contains(t, res.Chapters[0].Content, "{ not a brace }")
}
