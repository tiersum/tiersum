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
	// CJK: ~1 budget unit per rune (vector-length alignment), not runes/4.
	assert.Equal(t, 4, EstimateTokens("中文测试"))
	mixed := "ab中文" // 2 Latin + 2 Han -> 2 + (2+3)/4 = 2+1 = 3
	assert.Equal(t, 3, EstimateTokens(mixed))
}

func TestParseNumberedOutlineHeading_rejectsRemainderStartingWithDigit(t *testing.T) {
	_, _, ok := parseNumberedOutlineHeading("1. 2. nested list style")
	assert.False(t, ok)
	_, _, ok2 := parseNumberedOutlineHeading("1. Introduction")
	assert.True(t, ok2)
}

func TestParseNumberedOutlineHeading_rejectsRemainderStartingWithFullwidthDigit(t *testing.T) {
	_, _, ok := parseNumberedOutlineHeading("1. ２全角数字开头")
	assert.False(t, ok)
	_, _, ok2 := parseNumberedOutlineHeading("1. 第二章概述")
	assert.True(t, ok2)
}

func TestSplitMarkdown_setextHeadings(t *testing.T) {
	md := "Chapter One\n=========\n\nIntro body.\n\nSection Two\n------------\n\nMore text.\n"
	segs := SplitMarkdown("doc1", "Book", md, 512)
	require.NotEmpty(t, segs)
	var blob strings.Builder
	for _, s := range segs {
		blob.WriteString(s.Path)
		blob.WriteByte('\n')
		blob.WriteString(s.Text)
		blob.WriteByte('\n')
	}
	out := blob.String()
	assert.Contains(t, out, "Chapter One")
	assert.Contains(t, out, "Section Two")
	assert.Contains(t, out, "Intro body")
	assert.Contains(t, out, "More text")
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

func TestSplitMarkdown_tildeFenceHeadingIgnored(t *testing.T) {
	md := "~~~\n# not a heading\n~~~\n\n# Real\n\nok\n"
	segs := SplitMarkdown("d", "T", md, 512)
	var real int
	for _, s := range segs {
		if strings.Contains(s.Path, "Real") {
			real++
		}
	}
	assert.GreaterOrEqual(t, real, 1)
}

func TestSplitMarkdown_indentedCodeIgnoresHashLine(t *testing.T) {
	md := "\n    # not a heading\n    more code\n\n# Real\n\nok\n"
	segs := SplitMarkdown("d", "T", md, 512)
	var real int
	for _, s := range segs {
		if strings.Contains(s.Path, "Real") {
			real++
		}
	}
	assert.GreaterOrEqual(t, real, 1)
}

func TestSplitMarkdown_shortOrderedTripleStaysBody(t *testing.T) {
	md := "# Doc\n\n1. aa\n2. bb\n3. cc\n\nTail.\n"
	segs := SplitMarkdown("d", "T", md, 512)
	for _, s := range segs {
		assert.NotContains(t, s.Path, "/aa", "short triple should not become outline chapters: %#v", segs)
		assert.NotContains(t, s.Path, "/bb")
		assert.NotContains(t, s.Path, "/cc")
	}
	var doc bool
	for _, s := range segs {
		if strings.Contains(s.Path, "Doc") && strings.Contains(s.Text, "Tail.") {
			doc = true
		}
	}
	assert.True(t, doc, "list lines should stay under # Doc, got %#v", segs)
}

func TestSplitMarkdown_setextNoBlankBetweenTitleAndUnderline(t *testing.T) {
	md := "My Title\n\n---\n\nbody\n"
	segs := SplitMarkdown("d", "T", md, 512)
	for _, s := range segs {
		assert.NotContains(t, s.Path, "My Title", "blank between title and --- should not form Setext: %#v", segs)
	}
}

func TestSplitMarkdown_setextImmediateUnderlineStillWorks(t *testing.T) {
	md := "Chapter One\n=========\n\nIntro.\n"
	segs := SplitMarkdown("d", "T", md, 512)
	found := false
	for _, s := range segs {
		if strings.Contains(s.Path, "Chapter One") && strings.Contains(s.Text, "Intro.") {
			found = true
		}
	}
	assert.True(t, found, "immediate === underline should keep Setext, got %#v", segs)
}

func TestSplitMarkdown_blockquoteATXNotChapterPath(t *testing.T) {
	md := "> # Inside Quote\n\nQuoted line.\n\n# Real Section\n\nok\n"
	segs := SplitMarkdown("d", "T", md, 512)
	for _, s := range segs {
		assert.NotContains(t, s.Path, "Inside Quote", "blockquote ATX must not define chapter path: %s", s.Path)
	}
	var real bool
	for _, s := range segs {
		if strings.Contains(s.Path, "Real Section") {
			real = true
		}
	}
	assert.True(t, real)
}
