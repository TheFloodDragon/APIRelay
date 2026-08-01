package adaptor

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestIsBlockedProbeIPRejectsInternalRanges(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // 环回
		"127.13.37.1",     // 整个 127/8
		"0.0.0.0",         // 本网络
		"10.1.2.3",        // 私有 A
		"172.16.0.1",      // 私有 B
		"172.31.255.254",  // 私有 B 边界
		"192.168.1.1",     // 私有 C
		"169.254.169.254", // 云实例元数据，最关键的一条
		"169.254.0.1",     // 链路本地
		"100.64.0.1",      // 运营商级 NAT
		"198.18.0.1",      // 基准测试段
		"224.0.0.1",       // 组播
		"255.255.255.255", // 广播
		"::1",             // IPv6 环回
		"::",              // IPv6 未指定
		"fc00::1",         // IPv6 唯一本地
		"fe80::1",         // IPv6 链路本地
		"ff02::1",         // IPv6 组播
	}
	for _, item := range blocked {
		ip := net.ParseIP(item)
		if ip == nil {
			t.Fatalf("test fixture %q is not a valid IP", item)
		}
		if !IsBlockedProbeIP(ip) {
			t.Errorf("IP %s should be blocked", item)
		}
	}
}

func TestIsBlockedProbeIPAllowsPublicAddresses(t *testing.T) {
	allowed := []string{
		"1.1.1.1",
		"8.8.8.8",
		"104.18.32.7",
		"172.15.255.255", // 恰在私有 B 段之外
		"172.32.0.1",     // 恰在私有 B 段之外
		"11.0.0.1",       // 恰在私有 A 段之外
		"2606:4700::1111",
	}
	for _, item := range allowed {
		ip := net.ParseIP(item)
		if ip == nil {
			t.Fatalf("test fixture %q is not a valid IP", item)
		}
		if IsBlockedProbeIP(ip) {
			t.Errorf("IP %s should be allowed", item)
		}
	}
}

// IPv4-mapped IPv6 是常见的绕过手法：::ffff:127.0.0.1 必须按 IPv4 规则判断。
func TestIsBlockedProbeIPHandlesIPv4MappedAddresses(t *testing.T) {
	cases := map[string]bool{
		"::ffff:127.0.0.1":       true,
		"::ffff:169.254.169.254": true,
		"::ffff:10.0.0.1":        true,
		"::ffff:8.8.8.8":         false,
	}
	for item, wantBlocked := range cases {
		ip := net.ParseIP(item)
		if ip == nil {
			t.Fatalf("test fixture %q is not a valid IP", item)
		}
		if got := IsBlockedProbeIP(ip); got != wantBlocked {
			t.Errorf("IsBlockedProbeIP(%s) = %v, want %v", item, got, wantBlocked)
		}
	}
}

func TestIsBlockedProbeIPRejectsNil(t *testing.T) {
	if !IsBlockedProbeIP(nil) {
		t.Fatal("nil IP must be treated as blocked")
	}
}

// 字面量 IP 主机名无需 DNS 即可判定。
func TestValidateProbeHostRejectsLiteralInternalIP(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "169.254.169.254", "[::1]"} {
		trimmed := host
		if trimmed == "[::1]" {
			trimmed = "::1" // Hostname() 会去掉方括号
		}
		err := ValidateProbeHost(context.Background(), trimmed)
		if err == nil {
			t.Errorf("host %s should be rejected", host)
			continue
		}
		var blocked *ErrBlockedProbeTarget
		if !errors.As(err, &blocked) {
			t.Errorf("host %s: error = %v, want ErrBlockedProbeTarget", host, err)
		}
	}
}

func TestValidateProbeHostAllowsLiteralPublicIP(t *testing.T) {
	if err := ValidateProbeHost(context.Background(), "1.1.1.1"); err != nil {
		t.Fatalf("public IP should be allowed: %v", err)
	}
}

func TestValidateProbeHostRejectsEmptyHost(t *testing.T) {
	if err := ValidateProbeHost(context.Background(), "   "); err == nil {
		t.Fatal("empty host must be rejected")
	}
}

// 测试开关必须能关闭守卫并正确恢复，否则包内其它测试会互相污染。
func TestSetProbeGuardDisabledForTestRoundTrips(t *testing.T) {
	if IsBlockedProbeIP(net.ParseIP("127.0.0.1")) != true {
		t.Fatal("guard should be enabled by default")
	}

	restore := SetProbeGuardDisabledForTest(true)
	if IsBlockedProbeIP(net.ParseIP("127.0.0.1")) {
		t.Fatal("guard should be disabled")
	}
	if err := ValidateProbeHost(context.Background(), "127.0.0.1"); err != nil {
		t.Fatalf("guard disabled but host still rejected: %v", err)
	}

	restore()
	if !IsBlockedProbeIP(net.ParseIP("127.0.0.1")) {
		t.Fatal("guard should be restored after calling restore()")
	}
}

// 守卫 dialer 必须在建连前拦截，消除 DNS rebinding 窗口。
func TestProbeGuardDialerBlocksInternalTarget(t *testing.T) {
	called := false
	base := func(context.Context, string, string) (net.Conn, error) {
		called = true
		return nil, nil
	}
	guarded := probeGuardDialer(base)

	if _, err := guarded(context.Background(), "tcp", "169.254.169.254:80"); err == nil {
		t.Fatal("expected metadata endpoint to be blocked")
	}
	if called {
		t.Fatal("underlying dialer must not be reached for a blocked target")
	}
}

func TestProbeGuardDialerAllowsPublicTarget(t *testing.T) {
	called := false
	base := func(context.Context, string, string) (net.Conn, error) {
		called = true
		return nil, nil
	}
	guarded := probeGuardDialer(base)

	if _, err := guarded(context.Background(), "tcp", "1.1.1.1:443"); err != nil {
		t.Fatalf("public target should pass: %v", err)
	}
	if !called {
		t.Fatal("underlying dialer should be reached for an allowed target")
	}
}

// ProbeHTTPClient 必须装上守卫，且调用方拿到可用的清理函数。
func TestProbeHTTPClientAppliesGuard(t *testing.T) {
	client, closeIdle, err := ProbeHTTPClient(5 * 1e9)
	if err != nil {
		t.Fatalf("build probe client: %v", err)
	}
	defer closeIdle()
	if client == nil {
		t.Fatal("client must not be nil")
	}
	if client.Timeout <= 0 {
		t.Fatal("probe client must carry a timeout")
	}

	// 直接向元数据端点发请求应在建连层被拒。
	_, reqErr := client.Get("http://169.254.169.254/latest/meta-data/")
	if reqErr == nil {
		t.Fatal("probe client must refuse the cloud metadata endpoint")
	}
}
