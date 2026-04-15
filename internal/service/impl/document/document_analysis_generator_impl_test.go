package document

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAnalysis_NilProviderMarkdownChapters(t *testing.T) {
	a := NewDocumentAnalysisGenerator(nil, nil)
	md := "# One\nbody1\n\n## Two\nbody2\n"
	res, err := a.GenerateAnalysis(context.Background(), "T", md)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.GreaterOrEqual(t, len(res.Chapters), 2, "expected heading-split chapters")
	require.NotEmpty(t, res.Summary)
}

func TestGenerateAnalysis_NilProviderEmptyContent(t *testing.T) {
	a := NewDocumentAnalysisGenerator(nil, nil)
	res, err := a.GenerateAnalysis(context.Background(), "OnlyTitle", "")
	require.NoError(t, err)
	require.Len(t, res.Chapters, 1)
}
