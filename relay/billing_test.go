package relay

import (
	"testing"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/model"
)

func TestCalcQuota(t *testing.T) {
	// 1M input tokens @ $1/1M + 1M output @ $2/1M = $3 = 3_000_000 微美元
	if got := CalcQuota(1_000_000, 1_000_000, 1.0, 2.0); got != 3_000_000 {
		t.Errorf("CalcQuota = %d, want 3000000", got)
	}
	// 价格为 0 -> 不计费
	if got := CalcQuota(1000, 1000, 0, 0); got != 0 {
		t.Errorf("zero price = %d, want 0", got)
	}
	// 向上取整：极小用量也至少 1 微美元
	if got := CalcQuota(1, 0, 1.0, 0); got != 1 {
		t.Errorf("tiny usage = %d, want 1 (round up)", got)
	}
}

func TestResolvePrice_ChannelOverride(t *testing.T) {
	ch := &model.Channel{
		ModelConfigs: `[{"name":"gpt-4o","enabled":true,"input":2.5,"output":10}]`,
	}
	in, out := ResolvePrice(ch, "gpt-4o")
	if in != 2.5 || out != 10 {
		t.Errorf("channel price = (%v,%v), want (2.5,10)", in, out)
	}
}

func TestResolvePrice_NoPrice(t *testing.T) {
	ch := &model.Channel{ModelConfigs: `[{"name":"gpt-4o","enabled":true}]`}
	in, out := ResolvePrice(ch, "gpt-4o")
	if in != 0 || out != 0 {
		t.Errorf("no price = (%v,%v), want (0,0)", in, out)
	}
}

func TestEstimateTokens(t *testing.T) {
	ir := &dto.UnifiedRequest{
		System: "you are helpful",
		Messages: []dto.UnifiedMessage{
			{Role: dto.RoleUser, Content: "hello world this is a test"},
		},
	}
	est := EstimateTokens(ir)
	if est < 1 {
		t.Errorf("estimate should be >= 1, got %d", est)
	}
	// 空请求至少返回正数
	if EstimateTokens(&dto.UnifiedRequest{}) < 1 {
		t.Error("empty estimate should be >= 1")
	}
	if EstimateTokens(nil) != 0 {
		t.Error("nil estimate should be 0")
	}
}

func TestEstimateCompletionTokens(t *testing.T) {
	mt := 500
	if got := estimateCompletionTokens(&dto.UnifiedRequest{MaxTokens: &mt}); got != 500 {
		t.Errorf("with max_tokens = %d, want 500", got)
	}
	if got := estimateCompletionTokens(&dto.UnifiedRequest{}); got != 1024 {
		t.Errorf("default = %d, want 1024", got)
	}
}

func TestEstimateTokens_IncludesImages(t *testing.T) {
	ir := &dto.UnifiedRequest{Messages: []dto.UnifiedMessage{{Role: dto.RoleUser, Parts: []dto.UnifiedContentPart{{Type: "image_url", ImageURL: "data:image/png;base64,abc"}}}}}
	if got := EstimateTokens(ir); got < estimatedImageTokens {
		t.Fatalf("image token estimate = %d, want at least %d", got, estimatedImageTokens)
	}
}

func TestEstimateQuotaForCandidates_UsesChannelPriceCeiling(t *testing.T) {
	mt := 100
	ir := &dto.UnifiedRequest{Model: "gpt-test", MaxTokens: &mt, Messages: []dto.UnifiedMessage{{Role: dto.RoleUser, Content: "hello"}}}
	candidates := []model.ChannelCandidate{{Channel: &model.Channel{ModelConfigs: `[{"name":"gpt-test","enabled":true,"input":2,"output":10}]`}}}
	withoutSafety := CalcQuota(EstimateTokens(ir), estimateCompletionTokens(ir), 2, 10)
	want := applyReserveSafety(withoutSafety)
	if got := EstimateQuotaForCandidates(ir, candidates); got != want {
		t.Fatalf("estimate quota = %d, want %d", got, want)
	}
}

func setupBillingSessionTestDB(t *testing.T) {
	t.Helper()
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	model.DB.Exec("DELETE FROM tokens")
	model.DB.Exec("DELETE FROM logs")
}

func TestInterruptedStreamUsagePrefersActualAndEstimatesMissingValues(t *testing.T) {
	ir := &dto.UnifiedRequest{Messages: []dto.UnifiedMessage{{Role: dto.RoleUser, Content: "hello"}}}

	actual := interruptedStreamUsage(ir, &dto.Usage{PromptTokens: 11, CompletionTokens: 7, TotalTokens: 18}, 100)
	if actual.PromptTokens != 11 || actual.CompletionTokens != 7 || actual.TotalTokens != 18 {
		t.Fatalf("actual usage must win, got %+v", actual)
	}

	estimated := interruptedStreamUsage(ir, nil, 9)
	if estimated.PromptTokens != EstimateTokens(ir) {
		t.Fatalf("estimated prompt = %d, want %d", estimated.PromptTokens, EstimateTokens(ir))
	}
	if estimated.CompletionTokens != 3 {
		t.Fatalf("estimated completion = %d, want ceil(9/4)=3", estimated.CompletionTokens)
	}
	if estimated.TotalTokens != estimated.PromptTokens+estimated.CompletionTokens {
		t.Fatalf("estimated total is inconsistent: %+v", estimated)
	}
}

func TestStreamChunkCompletionCharsRawProtocols(t *testing.T) {
	tests := []struct {
		name  string
		ep    constant.EndpointType
		raw   string
		chars int
	}{
		{"openai", constant.EndpointOpenAI, `data: {"choices":[{"delta":{"content":"你好","tool_calls":[{"function":{"name":"f","arguments":"{}"}}]}}]}`, 5},
		{"anthropic", constant.EndpointAnthropic, `data: {"delta":{"text":"你好","partial_json":"{}"},"content_block":{"name":"f"}}`, 5},
		{"responses", constant.EndpointResponses, `data: {"delta":"你好","item":{"name":"f","arguments":"{}"}}`, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := streamChunkCompletionChars(tt.ep, &dto.UnifiedStreamChunk{Raw: tt.raw, IsRaw: true})
			if got != tt.chars {
				t.Fatalf("completion chars = %d, want %d", got, tt.chars)
			}
		})
	}
}

func TestInterruptedStreamSettlementPreventsRefund(t *testing.T) {
	setupBillingSessionTestDB(t)
	tok := &model.Token{Name: "billing-interrupted", Quota: 1000, Status: model.TokenStatusEnabled}
	if err := model.CreateToken(tok, "k-billing-interrupted"); err != nil {
		t.Fatalf("create token: %v", err)
	}
	billing := NewBillingSession(tok.Id)
	if err := billing.Reserve(400); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	info := &RelayInfo{TokenId: tok.Id, OriginModel: "gpt-test", IsStream: true}
	ir := &dto.UnifiedRequest{Messages: []dto.UnifiedMessage{{Role: dto.RoleUser, Content: "hello"}}}
	NewRelayer(&config.RelayConfig{}).logInterruptedStream(info, ir, nil, 12, billing, 502, "unexpected EOF")

	if !info.Settled {
		t.Fatal("interrupted stream should mark relay as settled")
	}
	if billing.Refund() {
		t.Fatal("request defer must not refund a settled interrupted stream")
	}
}

func TestBillingSessionRefundOnlyOnce(t *testing.T) {
	setupBillingSessionTestDB(t)
	tok := &model.Token{Name: "billing-refund", Quota: 1000, Unlimited: false, Status: model.TokenStatusEnabled}
	if err := model.CreateToken(tok, "k-billing-refund"); err != nil {
		t.Fatalf("create token: %v", err)
	}

	billing := NewBillingSession(tok.Id)
	if err := billing.Reserve(400); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if !billing.Refund() {
		t.Fatal("first refund should happen")
	}
	if billing.Refund() {
		t.Fatal("second refund should be ignored")
	}

	var r model.Token
	model.DB.First(&r, tok.Id)
	if r.UsedQuota != 0 {
		t.Fatalf("used quota after duplicate refund = %d, want 0", r.UsedQuota)
	}
}

func TestBillingSessionSettleOnlyOnce(t *testing.T) {
	setupBillingSessionTestDB(t)
	tok := &model.Token{Name: "billing-settle", Quota: 1000, Unlimited: false, Status: model.TokenStatusEnabled}
	if err := model.CreateToken(tok, "k-billing-settle"); err != nil {
		t.Fatalf("create token: %v", err)
	}

	billing := NewBillingSession(tok.Id)
	if err := billing.Reserve(400); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := billing.Settle(100); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if err := billing.Settle(900); err != nil {
		t.Fatalf("duplicate settle should be ignored: %v", err)
	}
	if billing.Refund() {
		t.Fatal("refund after settle should be ignored")
	}

	var r model.Token
	model.DB.First(&r, tok.Id)
	if r.UsedQuota != 100 {
		t.Fatalf("used quota after duplicate settle = %d, want 100", r.UsedQuota)
	}
}
