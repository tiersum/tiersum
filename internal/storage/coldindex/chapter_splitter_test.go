package coldindex

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/pkg/types"
)

func TestMarkdownSplitter_Split(t *testing.T) {
	var s MarkdownSplitter
	chapters := s.Split("d", "T", "# H\n\nbody", 512)
	require.Len(t, chapters, 1)
	assert.Contains(t, chapters[0].Path, "d/")
	assert.True(t, strings.Contains(chapters[0].Text, "body"))
}

func TestMarkdownSplitter_respectsMaxTokens(t *testing.T) {
	var s MarkdownSplitter
	chapters := s.Split("id", "Title", "plain", 100)
	require.NotEmpty(t, chapters)
}

func TestDefaultColdChapterSplitter(t *testing.T) {
	sp := DefaultColdChapterSplitter()
	require.NotNil(t, sp)
	chapters := sp.Split("x", "Y", "# A\n\nb", types.DefaultColdChapterMaxTokens)
	require.NotEmpty(t, chapters)
}
