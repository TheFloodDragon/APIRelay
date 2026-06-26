package relay

import (
	"testing"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"
)

func TestResolveAPIType_ModelExplicitOverride(t *testing.T) {
	ch := &model.Channel{
		Type:         constant.ChannelTypeOpenAI,
		ModelConfigs: `[{"name":"claude-3","enabled":true,"protocol":"anthropic"}]`,
	}
	if got := ResolveAPIType(ch, "claude-3"); got != constant.APITypeAnthropic {
		t.Errorf("explicit override: got %v, want anthropic", got)
	}
}

func TestResolveAPIType_ChannelRule(t *testing.T) {
	ch := &model.Channel{
		Type:          constant.ChannelTypeOpenAI,
		ModelConfigs:  `[{"name":"claude-3-opus","enabled":true}]`,
		ProtocolRules: `[{"pattern":"^claude","protocol":"anthropic"}]`,
	}
	if got := ResolveAPIType(ch, "claude-3-opus"); got != constant.APITypeAnthropic {
		t.Errorf("channel rule: got %v, want anthropic", got)
	}
	// 不匹配规则的模型走供应商默认
	ch2 := &model.Channel{
		Type:          constant.ChannelTypeOpenAI,
		ModelConfigs:  `[{"name":"gpt-4o","enabled":true}]`,
		ProtocolRules: `[{"pattern":"^claude","protocol":"anthropic"}]`,
	}
	if got := ResolveAPIType(ch2, "gpt-4o"); got != constant.APITypeOpenAI {
		t.Errorf("non-match: got %v, want openai", got)
	}
}

func TestResolveAPIType_ChannelDefault(t *testing.T) {
	ch := &model.Channel{
		Type:         constant.ChannelTypeResponses,
		ModelConfigs: `[{"name":"gpt-4o","enabled":true}]`,
	}
	if got := ResolveAPIType(ch, "gpt-4o"); got != constant.APITypeResponses {
		t.Errorf("channel default: got %v, want responses", got)
	}
}

func TestResolveAPIType_Priority(t *testing.T) {
	// 模型显式协议应压过供应商规则
	ch := &model.Channel{
		Type:          constant.ChannelTypeOpenAI,
		ModelConfigs:  `[{"name":"claude-3","enabled":true,"protocol":"responses"}]`,
		ProtocolRules: `[{"pattern":"^claude","protocol":"anthropic"}]`,
	}
	if got := ResolveAPIType(ch, "claude-3"); got != constant.APITypeResponses {
		t.Errorf("priority: explicit should win, got %v want responses", got)
	}
}

func TestResolveAPIType_InvalidRegexIgnored(t *testing.T) {
	ch := &model.Channel{
		Type:          constant.ChannelTypeOpenAI,
		ModelConfigs:  `[{"name":"gpt-4o","enabled":true}]`,
		ProtocolRules: `[{"pattern":"[invalid(","protocol":"anthropic"}]`,
	}
	// 无效正则被忽略，回退供应商默认
	if got := ResolveAPIType(ch, "gpt-4o"); got != constant.APITypeOpenAI {
		t.Errorf("invalid regex: got %v, want openai (fallback)", got)
	}
}
