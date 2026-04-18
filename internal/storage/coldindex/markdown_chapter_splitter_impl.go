package coldindex

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tiersum/tiersum/pkg/types"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// ColdChapter is one path-addressable markdown chapter (body slice) after splitting a cold document for indexing.
type ColdChapter struct {
	Path string
	Text string
}

// IColdChapterSplitter splits cold document markdown into chapters sized for cold vector indexing
// (embedder sequence-length budget via EstimateTokens), not for LLM prompts.
type IColdChapterSplitter interface {
	Split(docID, docTitle, markdown string, maxTokens int) []ColdChapter
}

// IColdTextEmbedder produces dense vectors for cold text; Index uses it internally when set via SetTextEmbedder.
// Successful Embed results must have length types.ColdEmbeddingVectorDimension.
type IColdTextEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Close() error
}

var (
	numberedHeadingMulti  = regexp.MustCompile(`^(\d+(?:\.\d+)+)\s+(\S.*)$`)
	numberedHeadingSingle = regexp.MustCompile(`^(\d+)\.\s+(\S.*)$`)
)

// parseNumberedOutlineHeading treats lines like "1. 概述", "2.1 小节" as headings for cold split.
// It rejects "1. **bold** list item" (ordered list under a section) so those stay in body text.
// Single-line "N." headings are also rejected when the title text starts with ASCII or fullwidth digits
// (nested ordered markers / mixed-numeral lists), to reduce false chapter boundaries for vector indexing.
func parseNumberedOutlineHeading(trimmed string) (title string, level int, ok bool) {
	if trimmed == "" {
		return "", 0, false
	}
	restIsListLike := func(s string) bool {
		s = strings.TrimSpace(s)
		return strings.HasPrefix(s, "*") || strings.HasPrefix(s, "_")
	}
	if m := numberedHeadingMulti.FindStringSubmatch(trimmed); m != nil {
		rest := strings.TrimSpace(m[2])
		if rest == "" || restIsListLike(rest) {
			return "", 0, false
		}
		segs := strings.Split(m[1], ".")
		if len(segs) < 2 {
			return "", 0, false
		}
		lev := 1 + len(segs)
		if lev > 6 {
			lev = 6
		}
		return trimmed, lev, true
	}
	if m := numberedHeadingSingle.FindStringSubmatch(trimmed); m != nil {
		rest := strings.TrimSpace(m[2])
		if rest == "" || restIsListLike(rest) {
			return "", 0, false
		}
		// Reject "1. 2. step" / year-like "1. 2024 …" where the remainder reads as a nested ordered marker or leading digit span.
		r0, w := utf8.DecodeRuneInString(rest)
		if w > 0 && r0 >= '0' && r0 <= '9' {
			return "", 0, false
		}
		// Reject remainder starting with fullwidth digits (e.g. "1. ２…") as list-like, not outline titles.
		if w > 0 && r0 >= '\uFF10' && r0 <= '\uFF19' {
			return "", 0, false
		}
		return trimmed, 2, true
	}
	return "", 0, false
}

// isCJKHeavyRune treats Han / Hiragana / Katakana / Hangul and common fullwidth CJK punctuation as
// ~1 budget unit per rune (closer to MiniLM BPE token counts than the Latin 4-runes heuristic).
func isCJKHeavyRune(r rune) bool {
	if unicode.Is(unicode.Han, r) {
		return true
	}
	switch {
	case r >= '\u3040' && r <= '\u309F':
		return true // Hiragana
	case r >= '\u30A0' && r <= '\u30FF':
		return true // Katakana
	case r >= '\uAC00' && r <= '\uD7AF':
		return true // Hangul syllables
	case r >= '\u3000' && r <= '\u303F':
		return true // CJK symbols and punctuation
	case r >= '\uFF00' && r <= '\uFFEF':
		return true // Halfwidth and fullwidth forms
	}
	return false
}

// EstimateTokens approximates embedder subword budget for cold chapter merge/split (no ONNX call).
// CJK-heavy runes use ~1 unit each so merged text stays near typical ~512 subword sequence caps;
// other scripts keep ~4 runes per unit. Goal: full chapters where possible while filling the vector budget.
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	var cjk, other int
	for _, r := range s {
		if isCJKHeavyRune(r) {
			cjk++
		} else {
			other++
		}
	}
	return cjk + (other+3)/4
}

// splitFirstLine returns the first line of rest (without trailing \n) and the remainder after the newline.
func splitFirstLine(rest string) (line, after string) {
	i := strings.IndexByte(rest, '\n')
	if i < 0 {
		return rest, ""
	}
	return rest[:i], rest[i+1:]
}

// parseSetextUnderline recognizes a CommonMark-style Setext underline (=== or ---) on its own line.
func parseSetextUnderline(trimmed string) (level int, ok bool) {
	s := strings.TrimSpace(trimmed)
	if len(s) < 3 {
		return 0, false
	}
	onlyRun := func(b byte) bool {
		saw := false
		for k := 0; k < len(s); k++ {
			c := s[k]
			if c == ' ' || c == '\t' {
				continue
			}
			if c != b {
				return false
			}
			saw = true
		}
		return saw
	}
	if onlyRun('=') {
		return 1, true
	}
	if onlyRun('-') {
		return 2, true
	}
	return 0, false
}

type splitNode struct {
	level      int
	title      string
	pathTitles []string
	localBody  strings.Builder
	children   []*splitNode
}

// SplitMarkdown splits document markdown into cold chapters using bottom-up merge.
// Sliding stride for oversized leaves comes from SetColdMarkdownSlidingStrideTokens / default (100 tokens).
func SplitMarkdown(docID, docTitle, markdown string, maxTokens int) []ColdChapter {
	mt := maxTokens
	if mt <= 0 {
		mt = types.DefaultColdChapterMaxTokens
	}
	stride := effectiveSlidingStrideTokens(mt)
	return splitMarkdownImpl(docID, docTitle, markdown, mt, stride)
}

func splitMarkdownImpl(docID, docTitle, markdown string, maxTokens, strideTokens int) []ColdChapter {
	root := parseSplitTree(markdown)
	raw := postOrderMergeSplit(root, maxTokens, strideTokens)
	if len(raw) == 0 {
		return []ColdChapter{{
			Path: docID + "/" + sanitizePathPart(docTitle),
			Text: strings.TrimSpace(markdown),
		}}
	}
	out := make([]ColdChapter, 0, len(raw))
	for _, s := range raw {
		rel := strings.Join(s.pathTitles, "/")
		if rel == "" {
			rel = sanitizePathPart(docTitle)
			if rel == "" {
				rel = "body"
			}
		}
		out = append(out, ColdChapter{
			Path: docID + "/" + rel,
			Text: strings.TrimSpace(s.text),
		})
	}
	return out
}

type rawSplitChapter struct {
	pathTitles []string
	text       string
}

func sanitizePathPart(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	if len(s) > 120 {
		s = s[:120]
	}
	if s == "" {
		return "body"
	}
	return s
}

// utf8Pos tracks a byte offset in a string together with the rune index from the string start.
type utf8Pos struct {
	b int
	r int
}

// advanceUtf8Pos advances p by up to deltaRunes runes in s (forward UTF-8 decode).
func advanceUtf8Pos(s string, p *utf8Pos, deltaRunes int) {
	for i := 0; i < deltaRunes && p.b < len(s); i++ {
		_, sz := utf8.DecodeRuneInString(s[p.b:])
		if sz == 0 {
			break
		}
		p.b += sz
		p.r++
	}
}

// buildMergedChapterBody joins subtree bodies for one merged cold chapter while re-inserting ATX
// heading lines from the tree (parent + each child). Heading lines are not stored in localBody,
// so without this the merged text would drop "### 2.1" / "### 2.2" even though the prose is kept.
func buildMergedChapterBody(n *splitNode, prefix string, nested [][]rawSplitChapter) string {
	var sb strings.Builder
	if n.level > 0 && strings.TrimSpace(n.title) != "" {
		sb.WriteString(strings.Repeat("#", n.level))
		sb.WriteString(" ")
		sb.WriteString(strings.TrimSpace(n.title))
		sb.WriteString("\n\n")
	}
	if strings.TrimSpace(prefix) != "" {
		sb.WriteString(prefix)
	}
	first := true
	for i := range nested {
		if i >= len(n.children) {
			break
		}
		chs := nested[i]
		child := n.children[i]
		for j, piece := range chs {
			if !first {
				sb.WriteString("\n\n")
			}
			first = false
			if j == 0 && child.level > 0 && strings.TrimSpace(child.title) != "" {
				sb.WriteString(strings.Repeat("#", child.level))
				sb.WriteString(" ")
				sb.WriteString(strings.TrimSpace(child.title))
				sb.WriteString("\n\n")
			}
			sb.WriteString(piece.text)
		}
	}
	return strings.TrimSpace(sb.String())
}

func postOrderMergeSplit(n *splitNode, maxTokens, strideTokens int) []rawSplitChapter {
	if n == nil {
		return nil
	}

	nested := make([][]rawSplitChapter, 0, len(n.children))
	for _, c := range n.children {
		nested = append(nested, postOrderMergeSplit(c, maxTokens, strideTokens))
	}

	local := strings.TrimSpace(n.localBody.String())
	prefix := ""
	if local != "" {
		prefix = local + "\n\n"
	}

	if len(n.children) == 0 {
		if n.level == 0 {
			t := strings.TrimSpace(local)
			if t == "" {
				return nil
			}
			return splitOversizedRaw(n.pathTitles, t, maxTokens, strideTokens)
		}
		return splitOversizedRaw(n.pathTitles, local, maxTokens, strideTokens)
	}

	combined := buildMergedChapterBody(n, prefix, nested)
	if combined == "" {
		return nil
	}
	if EstimateTokens(combined) <= maxTokens && n.level > 0 {
		return []rawSplitChapter{{pathTitles: n.pathTitles, text: combined}}
	}
	var out []rawSplitChapter
	if strings.TrimSpace(local) != "" && n.level > 0 {
		out = append(out, splitOversizedRaw(n.pathTitles, local, maxTokens, strideTokens)...)
	}
	for _, chs := range nested {
		out = append(out, chs...)
	}
	if n.level == 0 && len(out) == 0 && strings.TrimSpace(local) != "" {
		out = append(out, splitOversizedRaw(nil, local, maxTokens, strideTokens)...)
	}
	if n.level == 0 && strings.TrimSpace(local) != "" && len(out) > 0 {
		out[0].text = strings.TrimSpace(local + "\n\n" + out[0].text)
	}
	return out
}

// splitOversizedRaw splits text that still exceeds maxTokens after tree merge (e.g. a huge leaf).
// It uses sliding windows: each window is up to maxTokens (estimated) runes wide; the next window
// starts strideTokens later (same token heuristic), so overlap is (maxTokens - strideTokens) when stride < maxTokens.
// Chapter paths are parent pathTitles + "1","2",…; with no parent, synthetic "__root__" + index is used.
func splitOversizedRaw(pathTitles []string, text string, maxTokens, strideTokens int) []rawSplitChapter {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if EstimateTokens(text) <= maxTokens {
		return []rawSplitChapter{{pathTitles: pathTitles, text: text}}
	}
	const runesPerToken = 4
	if maxTokens < 1 {
		maxTokens = 1
	}
	stride := clampSlidingStrideTokens(strideTokens, maxTokens)

	winRunes := maxTokens * runesPerToken
	stepRunes := stride * runesPerToken
	totalRunes := utf8.RuneCountInString(text)

	var chunks []string
	if totalRunes <= winRunes {
		chunks = []string{text}
	} else {
		nWin := 1 + (totalRunes-winRunes+stepRunes-1)/stepRunes
		if nWin < 1 {
			nWin = 1
		}
		chunks = make([]string, 0, nWin)
		var cur utf8Pos
		for cur.r < totalRunes {
			endR := cur.r + winRunes
			if endR > totalRunes {
				endR = totalRunes
			}
			nRunes := endR - cur.r
			startB := cur.b
			var endPos utf8Pos
			endPos.b = cur.b
			endPos.r = cur.r
			advanceUtf8Pos(text, &endPos, nRunes)
			chunks = append(chunks, text[startB:endPos.b])
			if endR >= totalRunes {
				break
			}
			advanceUtf8Pos(text, &cur, stepRunes)
		}
	}

	base := pathTitles
	out := make([]rawSplitChapter, 0, len(chunks))
	for i, ch := range chunks {
		var pt []string
		if len(chunks) == 1 {
			pt = append([]string(nil), base...)
		} else {
			idx := strconv.Itoa(i + 1)
			if len(base) == 0 {
				pt = []string{"__root__", idx}
			} else {
				pt = append(append([]string(nil), base...), idx)
			}
		}
		out = append(out, rawSplitChapter{pathTitles: pt, text: ch})
	}
	return out
}

// MarkdownSplitter is the default IColdChapterSplitter: heading tree + bottom-up token merge.
// If SlidingStrideTokens > 0, it overrides the package stride for this splitter only (tests).
type MarkdownSplitter struct {
	SlidingStrideTokens int
}

// Split implements IColdChapterSplitter.
func (m MarkdownSplitter) Split(docID, docTitle, markdown string, maxTokens int) []ColdChapter {
	mt := maxTokens
	if mt <= 0 {
		mt = types.DefaultColdChapterMaxTokens
	}
	stride := m.slidingStride(mt)
	return splitMarkdownImpl(docID, docTitle, markdown, mt, stride)
}

func (m MarkdownSplitter) slidingStride(maxTokens int) int {
	if m.SlidingStrideTokens > 0 {
		return clampSlidingStrideTokens(m.SlidingStrideTokens, maxTokens)
	}
	return effectiveSlidingStrideTokens(maxTokens)
}

// DefaultColdChapterSplitter returns the standard markdown-based splitter.
func DefaultColdChapterSplitter() IColdChapterSplitter {
	return MarkdownSplitter{}
}

var _ IColdChapterSplitter = MarkdownSplitter{}

// ============ V2: Goldmark-based heading extraction with numbered outline supplement ============

var coldChapterParser = goldmark.New()

type headingSpan struct {
	level int
	title string
	start int
	end   int
}

// goldmarkHeadingAdopt applies "prefer missed headings over false extractions": only *ast.Heading nodes
// that are direct children of the document (not inside blockquotes, lists, tables, etc.).
func goldmarkHeadingAdopt(h *ast.Heading) bool {
	p := h.Parent()
	if p == nil || p.Kind() != ast.KindDocument {
		return false
	}
	if h.Level < 1 || h.Level > 6 {
		return false
	}
	return true
}

func collectGoldmarkHeadingSpans(source []byte) []headingSpan {
	reader := text.NewReader(source)
	doc := coldChapterParser.Parser().Parse(reader)
	var spans []headingSpan
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok || !goldmarkHeadingAdopt(h) {
			return ast.WalkContinue, nil
		}
		lines := h.Lines()
		if lines.Len() == 0 {
			return ast.WalkContinue, nil
		}
		title := strings.TrimSpace(string(h.Text(source)))
		if title == "" {
			return ast.WalkContinue, nil
		}
		start := lines.At(0).Start
		end := lines.At(lines.Len() - 1).Stop
		if start < 0 || end > len(source) || start > end {
			return ast.WalkContinue, nil
		}
		spans = append(spans, headingSpan{
			level: h.Level,
			title: title,
			start: start,
			end:   end,
		})
		return ast.WalkContinue, nil
	})
	return spans
}

// collectOutlineHeadingSpans scans for numbered outline headings that goldmark does not treat as
// ast.Heading (e.g. "1. Introduction" is parsed as a list item). Only lines matching
// parseNumberedOutlineHeading and not already covered by goldmark spans are collected.
func collectOutlineHeadingSpans(source string) []headingSpan {
	lines := strings.Split(source, "\n")
	var spans []headingSpan
	var offset int

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if title, level, ok := parseNumberedOutlineHeading(trimmed); ok {
			// Skip if this line falls inside an existing goldmark heading span.
			// We approximate by checking if the line offset overlaps any span.
			lineStart := offset
			lineEnd := offset + len(line)
			if i < len(lines)-1 {
				lineEnd++ // include \n
			}

			// Context-aware rejection: skip short ordered list triples.
			if partOfShortOrderedListTriple(lines, i) {
				offset = lineEnd
				continue
			}

			spans = append(spans, headingSpan{
				level: level,
				title: title,
				start: lineStart,
				end:   lineEnd,
			})
		}
		offset += len(line) + 1 // +1 for \n
	}
	return spans
}

// partOfShortOrderedListTriple rejects a line when it is part of 3 consecutive short ordered-list
// lines ("1. aa", "2. bb", "3. cc"), a common false-positive pattern for numbered outlines.
func partOfShortOrderedListTriple(lines []string, idx int) bool {
	if idx < 0 || idx >= len(lines) {
		return false
	}
	// Look for a window of 3 consecutive lines around idx that are all short ordered items.
	for start := idx - 2; start <= idx; start++ {
		if start < 0 || start+2 >= len(lines) {
			continue
		}
		if shortOrderedListTriple(lines, start) {
			return true
		}
	}
	return false
}

// shortOrderedListTriple returns true if lines[start:start+3] are all short ordered list items.
func shortOrderedListTriple(lines []string, start int) bool {
	if start < 0 || start+2 >= len(lines) {
		return false
	}
	var nums []int
	for i := 0; i < 3; i++ {
		trim := strings.TrimSpace(lines[start+i])
		n, rest, ok := extractSingleOrderedPrefix(trim)
		if !ok {
			return false
		}
		if len(rest) > 20 {
			return false
		}
		nums = append(nums, n)
	}
	if len(nums) != 3 {
		return false
	}
	return nums[1] == nums[0]+1 && nums[2] == nums[1]+1
}

// extractSingleOrderedPrefix extracts "N. " from the start of a line.
func extractSingleOrderedPrefix(trim string) (n int, rest string, ok bool) {
	if m := numberedHeadingSingle.FindStringSubmatch(trim); m != nil {
		num, _ := strconv.Atoi(m[1])
		return num, strings.TrimSpace(m[2]), true
	}
	return 0, "", false
}

// mergeAndSortSpans merges goldmark and outline spans, removing overlaps, and sorts by start position.
func mergeAndSortSpans(goldmarkSpans, outlineSpans []headingSpan) []headingSpan {
	// Build a set of byte ranges already covered by goldmark.
	type interval struct{ start, end int }
	var covered []interval
	for _, s := range goldmarkSpans {
		covered = append(covered, interval{s.start, s.end})
	}

	var merged []headingSpan
	merged = append(merged, goldmarkSpans...)

	for _, o := range outlineSpans {
		// Skip outline spans that overlap any goldmark span.
		overlaps := false
		for _, c := range covered {
			if o.start < c.end && o.end > c.start {
				overlaps = true
				break
			}
		}
		if !overlaps {
			merged = append(merged, o)
		}
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].start < merged[j].start
	})
	return merged
}

// parseSplitTree builds a heading tree for cold chapter splitting using goldmark CommonMark AST.
// Only headings that are direct children of the document are adopted (no blockquote/list/table headings).
// Setext and ATX headings are both represented as ast.Heading.
func parseSplitTree(markdown string) *splitNode {
	source := normalizeEOL(markdown)
	source = stripYAMLFrontmatter(source)
	sourceBytes := []byte(source)
	spans := collectGoldmarkHeadingSpans(sourceBytes)
	allSpans := spans

	root := &splitNode{level: 0, title: ""}
	stack := []*splitNode{root}
	prevEnd := 0

	flushBody := func(a, b int) {
		if a >= b || a < 0 || b > len(sourceBytes) {
			return
		}
		cur := stack[len(stack)-1]
		cur.localBody.Write(sourceBytes[a:b])
	}

	for _, h := range allSpans {
		if h.start > prevEnd {
			flushBody(prevEnd, h.start)
		}
		for len(stack) > 1 && stack[len(stack)-1].level >= h.level {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		pathTitles := make([]string, 0, len(parent.pathTitles)+1)
		pathTitles = append(pathTitles, parent.pathTitles...)
		pathTitles = append(pathTitles, h.title)
		child := &splitNode{level: h.level, title: h.title, pathTitles: pathTitles}
		parent.children = append(parent.children, child)
		stack = append(stack, child)
		prevEnd = h.end
	}
	if prevEnd < len(sourceBytes) {
		flushBody(prevEnd, len(sourceBytes))
	}
	return root
}

func normalizeEOL(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// stripYAMLFrontmatter removes YAML frontmatter blocks (delimited by --- on their own lines)
// to prevent goldmark from treating the closing --- as a Setext underline.
// It only strips blocks where the inner lines look like frontmatter (contain ':').
func stripYAMLFrontmatter(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	var pending []string
	inFrontmatter := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if !inFrontmatter {
				// Potential start
				inFrontmatter = true
				pending = []string{line}
				continue
			}
			// Potential end - check if content looks like frontmatter
			looksLikeFrontmatter := false
			for _, p := range pending[1:] {
				if strings.Contains(p, ":") {
					looksLikeFrontmatter = true
					break
				}
			}
			if looksLikeFrontmatter {
				// Skip the frontmatter block
				inFrontmatter = false
				pending = nil
				continue
			}
			// Not frontmatter, flush pending
			result = append(result, pending...)
			inFrontmatter = false
			pending = nil
		}
		if inFrontmatter {
			pending = append(pending, line)
		} else {
			result = append(result, line)
		}
	}
	if inFrontmatter {
		result = append(result, pending...)
	}
	return strings.Join(result, "\n")
}
