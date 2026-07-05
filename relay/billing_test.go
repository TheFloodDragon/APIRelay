package relay

import (
	"testing"

	"github.com/apirelay/apirelay/common/config"
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

func setupBillingSessionTestDB(t *testing.T) {
	t.Helper()
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	model.DB.Exec("DELETE FROM tokens")
	model.DB.Exec("DELETE FROM logs")
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
