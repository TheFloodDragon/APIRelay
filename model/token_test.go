package model

import (
	"testing"

	"github.com/apirelay/apirelay/common/config"
)

func setupQuotaTestDB(t *testing.T) {
	t.Helper()
	if err := InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	DB.Exec("DELETE FROM tokens")
}

func TestPreConsumeQuota(t *testing.T) {
	setupQuotaTestDB(t)

	// 限额令牌：额度 1000，已用 0
	tok := &Token{Name: "limited", Quota: 1000, Unlimited: false, Status: TokenStatusEnabled}
	if err := CreateToken(tok, "k-limited"); err != nil {
		t.Fatalf("create token: %v", err)
	}

	// 扣 600 应成功
	if err := PreConsumeQuota(tok.Id, 600); err != nil {
		t.Fatalf("preconsume 600: %v", err)
	}
	// 再扣 600 应失败（600+600 > 1000）
	if err := PreConsumeQuota(tok.Id, 600); err != ErrQuotaInsufficient {
		t.Fatalf("preconsume over limit: got %v, want ErrQuotaInsufficient", err)
	}
	// 扣 400 应成功（600+400 = 1000）
	if err := PreConsumeQuota(tok.Id, 400); err != nil {
		t.Fatalf("preconsume 400: %v", err)
	}

	var reload Token
	DB.First(&reload, tok.Id)
	if reload.UsedQuota != 1000 {
		t.Errorf("used quota = %d, want 1000", reload.UsedQuota)
	}
}

func TestPreConsumeQuota_Unlimited(t *testing.T) {
	setupQuotaTestDB(t)
	tok := &Token{Name: "unlimited", Unlimited: true, Status: TokenStatusEnabled}
	if err := CreateToken(tok, "k-unlimited"); err != nil {
		t.Fatalf("create: %v", err)
	}
	// 不限额令牌即使无 quota 也应成功，并累加统计
	if err := PreConsumeQuota(tok.Id, 99999); err != nil {
		t.Fatalf("unlimited preconsume: %v", err)
	}
	var reload Token
	DB.First(&reload, tok.Id)
	if reload.UsedQuota != 99999 {
		t.Errorf("used = %d, want 99999", reload.UsedQuota)
	}
}

func TestSettleQuota(t *testing.T) {
	setupQuotaTestDB(t)
	tok := &Token{Name: "settle", Quota: 10000, Unlimited: false, Status: TokenStatusEnabled}
	CreateToken(tok, "k-settle")

	// 预扣 1000
	if err := PreConsumeQuota(tok.Id, 1000); err != nil {
		t.Fatal(err)
	}
	// 实际只用了 300 -> 退还 700
	SettleQuota(tok.Id, 1000, 300)
	var r1 Token
	DB.First(&r1, tok.Id)
	if r1.UsedQuota != 300 {
		t.Errorf("after settle down: used = %d, want 300", r1.UsedQuota)
	}

	// 预扣 300，实际用了 800 -> 补扣 500
	PreConsumeQuota(tok.Id, 300) // used = 600
	SettleQuota(tok.Id, 300, 800)
	var r2 Token
	DB.First(&r2, tok.Id)
	// 600 - 300(reserved) + 800(actual) = 1100... 用 diff 逻辑：used=600，diff=500 -> 1100
	if r2.UsedQuota != 1100 {
		t.Errorf("after settle up: used = %d, want 1100", r2.UsedQuota)
	}
}

func TestRefundQuota(t *testing.T) {
	setupQuotaTestDB(t)
	tok := &Token{Name: "refund", Quota: 5000, Unlimited: false, Status: TokenStatusEnabled}
	CreateToken(tok, "k-refund")
	PreConsumeQuota(tok.Id, 2000)
	RefundQuota(tok.Id, 2000)
	var r Token
	DB.First(&r, tok.Id)
	if r.UsedQuota != 0 {
		t.Errorf("after refund: used = %d, want 0", r.UsedQuota)
	}
}

func TestRefundQuota_DoesNotGoNegative(t *testing.T) {
	setupQuotaTestDB(t)
	tok := &Token{Name: "refund-floor", Quota: 5000, Unlimited: false, Status: TokenStatusEnabled}
	CreateToken(tok, "k-refund-floor")
	PreConsumeQuota(tok.Id, 100)
	RefundQuota(tok.Id, 1000)
	var r Token
	DB.First(&r, tok.Id)
	if r.UsedQuota != 0 {
		t.Errorf("after over-refund: used = %d, want 0", r.UsedQuota)
	}
}

func TestSettleQuota_DoesNotExceedLimitedQuota(t *testing.T) {
	setupQuotaTestDB(t)
	tok := &Token{Name: "settle-limit", Quota: 1000, Unlimited: false, Status: TokenStatusEnabled}
	CreateToken(tok, "k-settle-limit")
	if err := PreConsumeQuota(tok.Id, 900); err != nil {
		t.Fatal(err)
	}
	if err := SettleQuota(tok.Id, 900, 1200); err != ErrQuotaInsufficient {
		t.Fatalf("settle over quota error = %v, want ErrQuotaInsufficient", err)
	}
	var r Token
	DB.First(&r, tok.Id)
	if r.UsedQuota != 900 {
		t.Errorf("used after failed settle = %d, want 900", r.UsedQuota)
	}
}

func TestSettleQuota_UnlimitedCanExceedQuota(t *testing.T) {
	setupQuotaTestDB(t)
	tok := &Token{Name: "settle-unlimited", Quota: 0, Unlimited: true, Status: TokenStatusEnabled}
	CreateToken(tok, "k-settle-unlimited")
	if err := SettleQuota(tok.Id, 0, 1200); err != nil {
		t.Fatalf("unlimited settle: %v", err)
	}
	var r Token
	DB.First(&r, tok.Id)
	if r.UsedQuota != 1200 {
		t.Errorf("used after unlimited settle = %d, want 1200", r.UsedQuota)
	}
}
