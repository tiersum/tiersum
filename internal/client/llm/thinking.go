package llm

import (
	"context"
	"regexp"
	"strings"

	"github.com/spf13/viper"

	"github.com/tiersum/tiersum/internal/client"
)

func init() {
	// Default: strip CoT / thinking from replies and send disable flags when the provider supports it.
	viper.SetDefault("llm.disable_think", true)
}

func disableThinkEnabled() bool {
	return viper.GetBool("llm.disable_think")
}

var thinkStripRes = []*regexp.Regexp{
	// DeepSeek / common "redacted" CoT wrappers
	regexp.MustCompile(`(?is)<\s*think\s*>.*?</\s*think\s*>`),
	regexp.MustCompile(`(?is)<\s*thinking\s*>.*?</\s*thinking\s*>`),
	regexp.MustCompile(`(?is)<\s*redacted_thinking\s*>.*?</\s*redacted_thinking\s*>`),
	regexp.MustCompile(`(?is)<\s*redacted_reasoning\s*>.*?</\s*redacted_reasoning\s*>`),
}

// StripThinkingFromModelOutput removes common chain-of-thought wrappers from model text.
func StripThinkingFromModelOutput(s string) string {
	if s == "" {
		return s
	}
	out := s
	for _, re := range thinkStripRes {
		out = re.ReplaceAllString(out, "")
	}
	return strings.TrimSpace(out)
}

// thinkStripProvider wraps an LLM provider to post-process answers when llm.disable_think is true.
type thinkStripProvider struct {
	inner client.ILLMProvider
}

// NewThinkStripProvider returns a provider that optionally strips thinking blocks from outputs.
func NewThinkStripProvider(inner client.ILLMProvider) client.ILLMProvider {
	if inner == nil {
		return nil
	}
	return &thinkStripProvider{inner: inner}
}

func (p *thinkStripProvider) Generate(ctx context.Context, prompt string, maxTokens int) (string, error) {
	s, err := p.inner.Generate(ctx, prompt, maxTokens)
	if err != nil || !disableThinkEnabled() {
		return s, err
	}
	return StripThinkingFromModelOutput(s), nil
}

var _ client.ILLMProvider = (*thinkStripProvider)(nil)
