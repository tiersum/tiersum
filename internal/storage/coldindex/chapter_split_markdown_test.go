package coldindex

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEstimateTokens(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
	assert.Equal(t, 1, EstimateTokens("abcd"))
	assert.Equal(t, 2, EstimateTokens("abcdefgh"))
}

func TestSplitMarkdown_orderedListWithBoldNotExtraChapters(t *testing.T) {
	md := "## 10. 总结\n\nPrinciples:\n\n1. **顺序写**：a\n2. **稀疏索引**：b\n5. **页缓存依赖**：c\n\nFooter.\n"
	segs := SplitMarkdown("d", "T", md, 512)
	require.NotEmpty(t, segs)
	for _, s := range segs {
		assert.NotContains(t, s.Path, "页缓存依赖")
		assert.NotContains(t, s.Path, "顺序写")
	}
	var found10 bool
	for _, s := range segs {
		if strings.Contains(s.Path, "10. 总结") && strings.Contains(s.Text, "页缓存依赖") {
			found10 = true
		}
	}
	assert.True(t, found10, "list items should stay inside ## 10 body, got %#v", segs)
}

func TestSplitMarkdown_noHeadings(t *testing.T) {
	docID := "d1"
	segs := SplitMarkdown(docID, "My Doc", "Hello world.\nSecond line.", 512)
	require.Len(t, segs, 1)
	assert.True(t, strings.HasPrefix(segs[0].Path, docID+"/"))
	assert.Contains(t, segs[0].Text, "Hello world")
}

func TestSplitMarkdown_singleHeading_small(t *testing.T) {
	md := "# Intro\n\nShort body.\n"
	segs := SplitMarkdown("id-1", "T", md, 512)
	require.NotEmpty(t, segs)
	found := false
	for _, s := range segs {
		if strings.Contains(s.Path, "Intro") && strings.Contains(s.Text, "Short body") {
			found = true
		}
	}
	assert.True(t, found)
}

func TestSplitMarkdown_nestedMergePreservesChildHeadings(t *testing.T) {
	md := "# A\n\n## B\n\nx.\n\n## C\n\ny.\n"
	segs := SplitMarkdown("doc", "Book", md, 512)
	require.NotEmpty(t, segs)
	merged := false
	for _, s := range segs {
		if strings.HasSuffix(s.Path, "/A") && strings.Contains(s.Text, "# A") && strings.Contains(s.Text, "## B") &&
			strings.Contains(s.Text, "## C") && strings.Contains(s.Text, "x.") && strings.Contains(s.Text, "y.") {
			merged = true
		}
	}
	assert.True(t, merged, "expected merged chapter at #A with child ## headings in body, got %#v", segs)
}

func TestSplitMarkdown_oversizedLeaf_parts(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Big\n\n")
	for i := 0; i < 3000; i++ {
		b.WriteString("word ")
	}
	md := b.String()
	segs := SplitMarkdown("x", "T", md, 64)
	require.GreaterOrEqual(t, len(segs), 1)
	hasNumberedSplit := false
	for _, s := range segs {
		if strings.Contains(s.Path, "/1") || strings.Contains(s.Path, "__root__/") {
			hasNumberedSplit = true
		}
	}
	assert.True(t, hasNumberedSplit || len(segs) >= 2)
}

func TestSplitMarkdown_codeFenceHeadingIgnored(t *testing.T) {
	md := "```\n# not a heading\n```\n\n# Real\n\nok\n"
	segs := SplitMarkdown("d", "T", md, 512)
	var titles int
	for _, s := range segs {
		if strings.Contains(s.Path, "Real") {
			titles++
		}
	}
	assert.GreaterOrEqual(t, titles, 1)
}
