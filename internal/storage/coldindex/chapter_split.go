package coldindex

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tiersum/tiersum/pkg/types"
)

// ColdChapter is one path-addressable markdown chapter (body slice) after splitting a cold document for indexing.
type ColdChapter struct {
	Path string
	Text string
}

// IColdChapterSplitter splits cold document markdown into token-budgeted chapters for the cold index.
type IColdChapterSplitter interface {
	Split(docID, docTitle, markdown string, maxTokens int) []ColdChapter
}

// IColdTextEmbedder produces dense vectors for cold text; Index uses it internally when set via SetTextEmbedder.
// Successful Embed results must have length types.ColdEmbeddingVectorDimension.
type IColdTextEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Close() error
}

var headingLine = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// EstimateTokens approximates token count (no external tokenizer).
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return (utf8.RuneCountInString(s) + 3) / 4
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

func parseSplitTree(markdown string) *splitNode {
	root := &splitNode{level: 0, title: ""}
	var stack []*splitNode
	stack = append(stack, root)

	lines := strings.Split(markdown, "\n")
	inFence := false
	var bodyBuf strings.Builder

	flushBodyToCurrent := func() {
		if bodyBuf.Len() == 0 {
			return
		}
		cur := stack[len(stack)-1]
		cur.localBody.WriteString(bodyBuf.String())
		bodyBuf.Reset()
	}

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") {
			inFence = !inFence
			bodyBuf.WriteString(line)
			bodyBuf.WriteByte('\n')
			continue
		}

		if !inFence {
			if m := headingLine.FindStringSubmatch(line); m != nil {
				flushBodyToCurrent()
				level := len(m[1])
				title := strings.TrimSpace(m[2])
				for len(stack) > 1 && stack[len(stack)-1].level >= level {
					stack = stack[:len(stack)-1]
				}
				parent := stack[len(stack)-1]
				pathTitles := make([]string, 0, len(parent.pathTitles)+1)
				pathTitles = append(pathTitles, parent.pathTitles...)
				pathTitles = append(pathTitles, title)
				child := &splitNode{level: level, title: title, pathTitles: pathTitles}
				parent.children = append(parent.children, child)
				stack = append(stack, child)
				continue
			}
		}
		bodyBuf.WriteString(line)
		bodyBuf.WriteByte('\n')
	}
	flushBodyToCurrent()
	return root
}

func postOrderMergeSplit(n *splitNode, maxTokens, strideTokens int) []rawSplitChapter {
	if n == nil {
		return nil
	}

	var nested [][]rawSplitChapter
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

	var merged strings.Builder
	merged.WriteString(prefix)
	first := true
	for _, chs := range nested {
		for _, piece := range chs {
			if !first {
				merged.WriteString("\n\n")
			}
			first = false
			merged.WriteString(piece.text)
		}
	}
	combined := strings.TrimSpace(merged.String())
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
	runes := []rune(text)

	var chunks []string
	for start := 0; start < len(runes); {
		end := start + winRunes
		if end > len(runes) {
			end = len(runes)
		}
		if end <= start {
			break
		}
		chunks = append(chunks, string(runes[start:end]))
		if end >= len(runes) {
			break
		}
		start += stepRunes
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
