package llm

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestStripThinkingFromModelOutput(t *testing.T) {
	raw := `<think>
step

</think>
Hello **world**.`
	assert.Equal(t, "Hello **world**.", StripThinkingFromModelOutput(raw))

	r2 := `Intro
<think>
x

</think>
Done.`
	assert.Equal(t, "Intro\n\nDone.", StripThinkingFromModelOutput(r2))
}

func TestOpenAIThinkOffChatTemplate(t *testing.T) {
	viper.Set("llm.disable_think", true)
	t.Cleanup(func() { viper.Set("llm.disable_think", nil) })

	k := openAIThinkOffChatTemplate("https://api.deepseek.com/v1", "deepseek-chat")
	assert.NotNil(t, k)
	assert.Equal(t, false, k["enable_thinking"])

	assert.Nil(t, openAIThinkOffChatTemplate("https://api.openai.com/v1", "gpt-4o-mini"))
}
