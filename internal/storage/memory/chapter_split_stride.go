package memory

// defaultColdMarkdownSlidingStrideTokens is the default step (in estimated tokens) between
// consecutive sliding-window starts when splitting an oversized leaf (same 4-runes/token heuristic as maxTokens).
const defaultColdMarkdownSlidingStrideTokens = 100

// coldMarkdownSlidingStrideTokens holds the configured stride; 0 means use defaultColdMarkdownSlidingStrideTokens.
var coldMarkdownSlidingStrideTokens int

// SetColdMarkdownSlidingStrideTokens sets cold_index.markdown.sliding_stride_tokens from config.
// Values < 1 reset to built-in default behavior (stride 100 tokens).
func SetColdMarkdownSlidingStrideTokens(n int) {
	if n < 1 {
		coldMarkdownSlidingStrideTokens = 0
		return
	}
	coldMarkdownSlidingStrideTokens = n
}

func effectiveSlidingStrideTokens(maxTokens int) int {
	s := coldMarkdownSlidingStrideTokens
	if s < 1 {
		s = defaultColdMarkdownSlidingStrideTokens
	}
	return clampSlidingStrideTokens(s, maxTokens)
}

func clampSlidingStrideTokens(stride, maxTokens int) int {
	if maxTokens < 1 {
		maxTokens = 1
	}
	if stride < 1 {
		stride = 1
	}
	if stride > maxTokens {
		return maxTokens
	}
	return stride
}
