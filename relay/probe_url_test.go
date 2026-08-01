package relay

import (
	"context"
	"strings"
	"testing"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/adaptor"
)

func TestModelsURLCandidatesHandlesBaseURLShapes(t *testing.T) {
	cases := []struct {
		name string
		base string
		typ  int
		want []string
	}{
		{
			name: "根域名补 /v1/models",
			base: "https://api.openai.com",
			typ:  constant.ChannelTypeOpenAI,
			want: []string{"https://api.openai.com/v1/models", "https://api.openai.com/models"},
		},
		{
			name: "已带 /v1 只补 /models",
			base: "https://api.openai.com/v1",
			typ:  constant.ChannelTypeOpenAI,
			want: []string{"https://api.openai.com/v1/models"},
		},
		{
			name: "尾随斜杠归一",
			base: "https://api.openai.com/v1/",
			typ:  constant.ChannelTypeOpenAI,
			want: []string{"https://api.openai.com/v1/models"},
		},
		{
			// 中转站常见形态：旧实现会拼成 /anthropic/v1/models 而 404。
			name: "剥离 /anthropic 兼容后缀",
			base: "https://relay.example.com/anthropic",
			typ:  constant.ChannelTypeAnthropic,
			want: []string{
				"https://relay.example.com/anthropic/v1/models",
				"https://relay.example.com/anthropic/models",
				"https://relay.example.com/v1/models",
			},
		},
		{
			name: "剥离 /api/claudecode 兼容后缀",
			base: "https://relay.example.com/api/claudecode",
			typ:  constant.ChannelTypeAnthropic,
			want: []string{
				"https://relay.example.com/api/claudecode/v1/models",
				"https://relay.example.com/api/claudecode/models",
				"https://relay.example.com/v1/models",
			},
		},
		{
			// 智谱式非 /v1 版本段：不应插入 /v1。
			name: "识别 /paas/v4 版本段",
			base: "https://open.bigmodel.cn/api/paas/v4",
			typ:  constant.ChannelTypeOpenAI,
			want: []string{"https://open.bigmodel.cn/api/paas/v4/models"},
		},
		{
			name: "识别 /v1beta 版本段",
			base: "https://generativelanguage.googleapis.com/v1beta",
			typ:  constant.ChannelTypeOpenAI,
			want: []string{"https://generativelanguage.googleapis.com/v1beta/models"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch := &model.Channel{BaseURL: tc.base, Type: tc.typ}
			got := modelsURLCandidates(ch)
			if len(got) != len(tc.want) {
				t.Fatalf("candidates = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("candidate[%d] = %q, want %q (full: %v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

// base_url 留空时回退到各协议的官方地址。
func TestModelsURLCandidatesFallsBackToOfficialBase(t *testing.T) {
	openai := modelsURLCandidates(&model.Channel{Type: constant.ChannelTypeOpenAI})
	if !strings.HasPrefix(openai[0], "https://api.openai.com/") {
		t.Fatalf("openai fallback = %v", openai)
	}
	anthropic := modelsURLCandidates(&model.Channel{Type: constant.ChannelTypeAnthropic})
	if !strings.HasPrefix(anthropic[0], "https://api.anthropic.com/") {
		t.Fatalf("anthropic fallback = %v", anthropic)
	}
}

// 候选列表必须去重，避免同一地址被重复请求。
func TestModelsURLCandidatesAreDeduplicated(t *testing.T) {
	ch := &model.Channel{BaseURL: "https://relay.example.com/v1/anthropic", Type: constant.ChannelTypeAnthropic}
	got := modelsURLCandidates(ch)
	seen := map[string]struct{}{}
	for _, item := range got {
		if _, dup := seen[item]; dup {
			t.Fatalf("duplicate candidate %q in %v", item, got)
		}
		seen[item] = struct{}{}
	}
}

// modelsURL 应返回候选列表的首项，保持与旧行为一致。
func TestModelsURLReturnsFirstCandidate(t *testing.T) {
	ch := &model.Channel{BaseURL: "https://api.openai.com/v1", Type: constant.ChannelTypeOpenAI}
	if got := modelsURL(ch); got != "https://api.openai.com/v1/models" {
		t.Fatalf("modelsURL = %q", got)
	}
}

// 只允许 http/https：file:// 与 gopher:// 可用于读本地文件或打内网协议。
func TestValidateProbeTargetRejectsUnsupportedSchemes(t *testing.T) {
	restore := adaptor.SetProbeGuardDisabledForTest(false)
	defer restore()

	for _, raw := range []string{
		"file:///etc/passwd",
		"gopher://internal.example:70/",
		"ftp://internal.example/",
		"redis://127.0.0.1:6379",
	} {
		if err := validateProbeTarget(context.Background(), raw); err == nil {
			t.Errorf("scheme in %q should be rejected", raw)
		}
	}
}

func TestValidateProbeTargetRejectsMissingHost(t *testing.T) {
	restore := adaptor.SetProbeGuardDisabledForTest(false)
	defer restore()

	if err := validateProbeTarget(context.Background(), "https://"); err == nil {
		t.Fatal("URL without host must be rejected")
	}
}

func TestValidateProbeTargetRejectsInternalHost(t *testing.T) {
	restore := adaptor.SetProbeGuardDisabledForTest(false)
	defer restore()

	for _, raw := range []string{
		"http://127.0.0.1:8080/v1",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.5/v1",
		"http://[::1]:3000/v1",
	} {
		if err := validateProbeTarget(context.Background(), raw); err == nil {
			t.Errorf("internal target %q should be rejected", raw)
		}
	}
}

// ValidateChannelProbeTarget 是 controller 用的入口：留空 base_url 视为使用官方地址。
func TestValidateChannelProbeTargetAllowsEmptyBaseURL(t *testing.T) {
	restore := adaptor.SetProbeGuardDisabledForTest(false)
	defer restore()

	if err := ValidateChannelProbeTarget(context.Background(), &model.Channel{}); err != nil {
		t.Fatalf("empty base_url should be allowed: %v", err)
	}
}

func TestValidateChannelProbeTargetRejectsInternalBaseURL(t *testing.T) {
	restore := adaptor.SetProbeGuardDisabledForTest(false)
	defer restore()

	ch := &model.Channel{BaseURL: "http://169.254.169.254"}
	if err := ValidateChannelProbeTarget(context.Background(), ch); err == nil {
		t.Fatal("cloud metadata base_url must be rejected")
	}
}

func TestValidateChannelProbeTargetRejectsNilChannel(t *testing.T) {
	if err := ValidateChannelProbeTarget(context.Background(), nil); err == nil {
		t.Fatal("nil channel must be rejected")
	}
}
