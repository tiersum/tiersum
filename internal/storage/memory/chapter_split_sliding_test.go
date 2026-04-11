package memory

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitMarkdown_oversizedPlain_slidingPathsUseRoot(t *testing.T) {
	const tokenBudget = 512
	const runesPerToken = 4
	strideTokens := defaultColdMarkdownSlidingStrideTokens
	stepRunes := strideTokens * runesPerToken
	winRunes := tokenBudget * runesPerToken
	overlapRunes := winRunes - stepRunes

	// Long plain body (no headings) → splitOversizedRaw(nil, …, 512)
	var sb strings.Builder
	for sb.Len() < winRunes+2*stepRunes+100 {
		sb.WriteString("abcdefghij") // 10 runes per iter
	}
	body := sb.String()
	rn := []rune(body)
	require.Greater(t, len(rn), winRunes+stepRunes)

	chapters := SplitMarkdown("doc1", "Book", body, tokenBudget)
	require.GreaterOrEqual(t, len(chapters), 2, "expected sliding windows for oversized leaf")

	assert.Equal(t, "doc1/__root__/1", chapters[0].Path)
	assert.Equal(t, "doc1/__root__/2", chapters[1].Path)

	r0 := []rune(chapters[0].Text)
	r1 := []rune(chapters[1].Text)
	require.GreaterOrEqual(t, len(r0), overlapRunes)
	require.GreaterOrEqual(t, len(r1), overlapRunes)
	assert.Equal(t,
		string(r0[len(r0)-overlapRunes:]),
		string(r1[:overlapRunes]),
		"consecutive windows should share overlap runes",
	)
}

func TestSplitMarkdown_oversizedUnderHeading_slidingPathsParentPlusIndex(t *testing.T) {
	const tokenBudget = 512
	var sb strings.Builder
	for sb.Len() < 6000 {
		sb.WriteString("word ")
	}
	md := "# Section A\n\n" + sb.String()

	chapters := SplitMarkdown("d2", "T", md, tokenBudget)
	require.GreaterOrEqual(t, len(chapters), 2)

	assert.Equal(t, "d2/Section A/1", chapters[0].Path)
	assert.Equal(t, "d2/Section A/2", chapters[1].Path)
}

func TestSplitMarkdown_oversized_overlapRuneCountMatchesSpec(t *testing.T) {
	const tokenBudget = 512
	const runesPerToken = 4
	strideTokens := defaultColdMarkdownSlidingStrideTokens
	stepRunes := strideTokens * runesPerToken
	winRunes := tokenBudget * runesPerToken
	overlapRunes := winRunes - stepRunes

	var parts []string
	for i := 0; i < 120; i++ {
		parts = append(parts, strings.Repeat("x", 63)+string(rune('A'+(i%26))))
	}
	body := strings.Join(parts, "")
	require.GreaterOrEqual(t, utf8.RuneCountInString(body), winRunes+stepRunes+50)

	chapters := SplitMarkdown("d3", "T", body, tokenBudget)
	require.GreaterOrEqual(t, len(chapters), 2)

	r0 := []rune(chapters[0].Text)
	r1 := []rune(chapters[1].Text)
	assert.Equal(t, winRunes, len(r0))
	assert.Equal(t, string(r0[len(r0)-overlapRunes:]), string(r1[:overlapRunes]))
}

func TestMarkdownSplitter_slidingStrideOverride(t *testing.T) {
	t.Cleanup(func() { SetColdMarkdownSlidingStrideTokens(0) })
	SetColdMarkdownSlidingStrideTokens(0)

	const tokenBudget = 512
	const runesPerToken = 4
	stride412 := 412
	stepRunes := stride412 * runesPerToken
	winRunes := tokenBudget * runesPerToken
	overlapRunes := winRunes - stepRunes // 100 tokens × 4 runes/token

	var sb strings.Builder
	for sb.Len() < winRunes+2*stepRunes+100 {
		sb.WriteString("abcdefghij")
	}
	body := sb.String()

	var sp MarkdownSplitter
	sp.SlidingStrideTokens = stride412
	chapters := sp.Split("ov", "T", body, tokenBudget)
	require.GreaterOrEqual(t, len(chapters), 2)
	r0 := []rune(chapters[0].Text)
	r1 := []rune(chapters[1].Text)
	assert.Equal(t, string(r0[len(r0)-overlapRunes:]), string(r1[:overlapRunes]))
}
