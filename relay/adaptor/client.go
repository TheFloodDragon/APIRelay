package adaptor

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/http/httpproxy"
)

const (
	ProxyModeSystem = "system"
	ProxyModeManual = "manual"
	ProxyModeDirect = "direct"

	upstreamDialTimeout           = 30 * time.Second
	upstreamTLSHandshakeTimeout   = 10 * time.Second
	upstreamResponseHeaderTimeout = 120 * time.Second
	upstreamIdleConnTimeout       = 90 * time.Second
)

// NetworkConfig 控制所有上游请求使用的代理策略。
type NetworkConfig struct {
	Mode      string `json:"mode"`
	ManualURL string `json:"manual_url"`
	NoProxy   string `json:"no_proxy"`
}

// NetworkStatus 描述当前实际生效的网络策略。
type NetworkStatus struct {
	Mode              string `json:"mode"`
	EffectiveSource   string `json:"effective_source"`
	EffectiveProxyURL string `json:"effective_proxy_url,omitempty"`
}

type clientSnapshot struct {
	client    *http.Client
	transport *http.Transport
	status    NetworkStatus
}

var (
	clientState atomic.Pointer[clientSnapshot]
	clientMu    sync.Mutex
)

// HTTPClient 返回共享客户端；配置更新后，新请求自动使用新客户端，进行中的请求不受影响。
func HTTPClient() *http.Client {
	if current := clientState.Load(); current != nil {
		return current.client
	}
	clientMu.Lock()
	defer clientMu.Unlock()
	if current := clientState.Load(); current != nil {
		return current.client
	}
	next, err := buildClient(NetworkConfig{Mode: ProxyModeSystem})
	if err != nil {
		next, _ = buildClient(NetworkConfig{Mode: ProxyModeDirect})
	}
	clientState.Store(next)
	return next.client
}

// UpdateNetworkConfig 校验并热切换共享上游客户端。
func UpdateNetworkConfig(cfg NetworkConfig) (NetworkStatus, error) {
	next, err := buildClient(cfg)
	if err != nil {
		return NetworkStatus{}, err
	}
	clientMu.Lock()
	old := clientState.Swap(next)
	clientMu.Unlock()
	if old != nil {
		old.transport.CloseIdleConnections()
	}
	return next.status, nil
}

// CurrentNetworkStatus 返回当前客户端的生效来源。
func CurrentNetworkStatus() NetworkStatus {
	HTTPClient()
	return clientState.Load().status
}

func normalizeNetworkConfig(cfg NetworkConfig) (NetworkConfig, error) {
	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	if cfg.Mode == "" {
		cfg.Mode = ProxyModeSystem
	}
	switch cfg.Mode {
	case ProxyModeSystem, ProxyModeDirect:
	case ProxyModeManual:
		if strings.TrimSpace(cfg.ManualURL) == "" {
			return cfg, errors.New("手动代理模式必须填写代理 URL")
		}
		parsed, err := normalizeProxyURL(cfg.ManualURL)
		if err != nil {
			return cfg, err
		}
		cfg.ManualURL = parsed.String()
	default:
		return cfg, fmt.Errorf("不支持的代理模式 %q", cfg.Mode)
	}
	return cfg, nil
}

func buildClient(cfg NetworkConfig) (*clientSnapshot, error) {
	cfg, err := normalizeNetworkConfig(cfg)
	if err != nil {
		return nil, err
	}
	proxyFunc, status, err := resolveProxy(cfg)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: upstreamDialTimeout, KeepAlive: 30 * time.Second}
	dialContext := dialer.DialContext
	if cfg.Mode == ProxyModeDirect {
		dialContext = fakeIPGuardDialer(dialer)
	}
	transport := &http.Transport{
		Proxy:                 proxyFunc,
		DialContext:           dialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       upstreamIdleConnTimeout,
		TLSHandshakeTimeout:   upstreamTLSHandshakeTimeout,
		ResponseHeaderTimeout: upstreamResponseHeaderTimeout,
		ExpectContinueTimeout: time.Second,
	}
	return &clientSnapshot{
		client:    &http.Client{Timeout: 0, Transport: transport},
		transport: transport,
		status:    status,
	}, nil
}

func resolveProxy(cfg NetworkConfig) (func(*http.Request) (*url.URL, error), NetworkStatus, error) {
	status := NetworkStatus{Mode: cfg.Mode}
	if cfg.Mode == ProxyModeDirect {
		status.EffectiveSource = "direct"
		return nil, status, nil
	}

	var httpProxy, httpsProxy, noProxy, source string
	if cfg.Mode == ProxyModeManual {
		httpProxy, httpsProxy, noProxy, source = cfg.ManualURL, cfg.ManualURL, cfg.NoProxy, "manual"
	} else {
		settings, err := loadSystemProxySettings()
		if err != nil {
			return nil, status, fmt.Errorf("读取系统代理失败: %w", err)
		}
		httpProxy, httpsProxy, noProxy, source = settings.HTTPProxy, settings.HTTPSProxy, settings.NoProxy, settings.Source
		if strings.TrimSpace(cfg.NoProxy) != "" {
			noProxy = cfg.NoProxy
		}
	}
	status.EffectiveSource = source
	status.EffectiveProxyURL = maskProxyURL(firstNonEmpty(httpsProxy, httpProxy))
	proxyCfg := &httpproxy.Config{HTTPProxy: httpProxy, HTTPSProxy: httpsProxy, NoProxy: noProxy}
	urlProxyFunc := proxyCfg.ProxyFunc()
	return func(req *http.Request) (*url.URL, error) {
		return urlProxyFunc(req.URL)
	}, status, nil
}

func normalizeProxyURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("代理 URL 无效: %q", raw)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("不支持的代理协议 %q", u.Scheme)
	}
	return u, nil
}

func maskProxyURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := normalizeProxyURL(raw)
	if err != nil {
		return raw
	}
	if u.User != nil {
		username := u.User.Username()
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(username, "***")
		}
	}
	return u.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func fakeIPGuardDialer(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("DNS 解析 %s 失败: %w", host, err)
		}
		for _, item := range ips {
			if isFakeIP(item.IP) {
				return nil, fmt.Errorf("DNS 将 %s 解析为 Fake-IP %s（198.18.0.0/15）；直连模式无法连接该地址，请切换为系统/手动代理，或让代理软件为 APIRelay 提供真实 DNS", host, item.IP)
			}
		}
		return dialer.DialContext(ctx, network, address)
	}
}

func isFakeIP(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && v4[0] == 198 && (v4[1] == 18 || v4[1] == 19)
}

// NetworkTestResult 是网络诊断的结构化结果。
type NetworkTestResult struct {
	Success     bool     `json:"success"`
	Target      string   `json:"target"`
	Stage       string   `json:"stage"`
	DNSResults  []string `json:"dns_results"`
	ProxySource string   `json:"proxy_source"`
	ProxyURL    string   `json:"proxy_url,omitempty"`
	StatusCode  int      `json:"status_code,omitempty"`
	LatencyMS   int64    `json:"latency_ms"`
	Error       string   `json:"error,omitempty"`
}

// TestNetwork 使用候选配置执行 DNS/TCP/TLS/HTTP 诊断，不修改当前生效配置。
func TestNetwork(ctx context.Context, cfg NetworkConfig, target string) NetworkTestResult {
	result := NetworkTestResult{Target: target, Stage: "config"}
	snapshot, err := buildClient(cfg)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer snapshot.transport.CloseIdleConnections()
	result.ProxySource = snapshot.status.EffectiveSource
	result.ProxyURL = snapshot.status.EffectiveProxyURL

	u, err := url.Parse(strings.TrimSpace(target))
	if err != nil || u.Scheme == "" || u.Hostname() == "" {
		result.Error = "测试地址必须是有效的 http/https URL"
		return result
	}
	lookupCtx, cancelLookup := context.WithTimeout(ctx, 10*time.Second)
	ips, lookupErr := net.DefaultResolver.LookupIPAddr(lookupCtx, u.Hostname())
	cancelLookup()
	if lookupErr != nil {
		result.Stage = "dns"
		result.Error = lookupErr.Error()
		return result
	}
	for _, ip := range ips {
		result.DNSResults = append(result.DNSResults, ip.String())
	}

	stage := "dns"
	trace := &httptrace.ClientTrace{
		ConnectStart: func(_, _ string) { stage = "tcp" },
		ConnectDone: func(_, _ string, err error) {
			if err == nil {
				stage = "tcp_connected"
			}
		},
		TLSHandshakeStart: func() { stage = "tls" },
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			if err == nil {
				stage = "tls_connected"
			}
		},
		GotFirstResponseByte: func() { stage = "http" },
	}
	requestCtx, cancel := context.WithTimeout(httptrace.WithClientTrace(ctx, trace), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if snapshot.transport.Proxy != nil {
		if proxyURL, proxyErr := snapshot.transport.Proxy(req); proxyErr == nil && proxyURL != nil {
			result.ProxyURL = maskProxyURL(proxyURL.String())
		}
	}
	start := time.Now()
	resp, err := snapshot.client.Do(req)
	result.LatencyMS = time.Since(start).Milliseconds()
	result.Stage = stage
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	result.Stage = "http"
	result.Success = true
	return result
}
