package adaptor

import (
	"net"
	"testing"
)

func TestUpdateNetworkConfigHotSwap(t *testing.T) {
	defer func() { _, _ = UpdateNetworkConfig(NetworkConfig{Mode: ProxyModeSystem}) }()
	if _, err := UpdateNetworkConfig(NetworkConfig{Mode: ProxyModeDirect}); err != nil {
		t.Fatal(err)
	}
	first := HTTPClient()
	status, err := UpdateNetworkConfig(NetworkConfig{Mode: ProxyModeManual, ManualURL: "http://127.0.0.1:7890", NoProxy: "localhost"})
	if err != nil {
		t.Fatal(err)
	}
	if first == HTTPClient() {
		t.Fatal("expected client to be hot-swapped")
	}
	if status.EffectiveSource != "manual" || status.EffectiveProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestFakeIPRange(t *testing.T) {
	for _, tc := range []struct {
		ip   string
		fake bool
	}{
		{"198.18.0.1", true},
		{"198.19.255.254", true},
		{"198.20.0.1", false},
		{"127.0.0.1", false},
	} {
		if got := isFakeIP(netParseIP(t, tc.ip)); got != tc.fake {
			t.Fatalf("isFakeIP(%s)=%v", tc.ip, got)
		}
	}
}

func netParseIP(t *testing.T, value string) net.IP {
	t.Helper()
	ip := net.ParseIP(value)
	if ip == nil {
		t.Fatalf("invalid test IP %s", value)
	}
	return ip
}
