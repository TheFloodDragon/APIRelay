//go:build windows

package adaptor

import "testing"

func TestParseWindowsProxyServer(t *testing.T) {
	httpProxy, httpsProxy := parseWindowsProxyServer("http=127.0.0.1:7890;https=127.0.0.1:7891")
	if httpProxy != "http://127.0.0.1:7890" || httpsProxy != "http://127.0.0.1:7891" {
		t.Fatalf("unexpected protocol proxies: http=%q https=%q", httpProxy, httpsProxy)
	}

	httpProxy, httpsProxy = parseWindowsProxyServer("socks=127.0.0.1:1080")
	if httpProxy != "socks5://127.0.0.1:1080" || httpsProxy != "socks5://127.0.0.1:1080" {
		t.Fatalf("unexpected socks proxies: http=%q https=%q", httpProxy, httpsProxy)
	}
}
