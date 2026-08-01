package adaptor

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
)

// SSRF 防护：管理端可以填任意 base_url，而探测/测试请求是由服务器自己发出的。
// 若不加限制，管理 API 就成了一个内网探测代理：既能扫内网端口，
// 也能读取云厂商的实例元数据端点（AWS/GCP/Azure 都在 169.254.169.254）。
//
// 只对「管理端触发的探测与连通性测试」生效，不影响正常转发 ——
// 用户完全可能合法地把上游模型服务部署在内网或 localhost。

// blockedCIDRs 是探测请求禁止访问的地址段。
var blockedCIDRs = buildBlockedCIDRs([]string{
	// IPv4 特殊用途
	"0.0.0.0/8",          // 本网络
	"10.0.0.0/8",         // 私有
	"100.64.0.0/10",      // 运营商级 NAT
	"127.0.0.0/8",        // 环回
	"169.254.0.0/16",     // 链路本地，含 169.254.169.254 云元数据
	"172.16.0.0/12",      // 私有
	"192.0.0.0/24",       // IETF 协议分配
	"192.0.2.0/24",       // TEST-NET-1
	"192.168.0.0/16",     // 私有
	"198.18.0.0/15",      // 基准测试 / 常被代理软件用作 Fake-IP
	"198.51.100.0/24",    // TEST-NET-2
	"203.0.113.0/24",     // TEST-NET-3
	"224.0.0.0/4",        // 组播
	"240.0.0.0/4",        // 保留
	"255.255.255.255/32", // 广播

	// IPv6 特殊用途
	"::/128",        // 未指定
	"::1/128",       // 环回
	"64:ff9b::/96",  // NAT64
	"100::/64",      // 丢弃前缀
	"2001:db8::/32", // 文档示例
	"fc00::/7",      // 唯一本地地址
	"fe80::/10",     // 链路本地
	"ff00::/8",      // 组播
})

func buildBlockedCIDRs(raw []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(raw))
	for _, item := range raw {
		if _, network, err := net.ParseCIDR(item); err == nil {
			out = append(out, network)
		}
	}
	return out
}

// probeGuardDisabled 仅供测试关闭守卫。
//
// 测试用 httptest 起本地服务（127.0.0.1），必然被守卫拦截。
// 用一个内部开关而不是环境变量，避免生产环境被误配置绕过。
var probeGuardDisabled atomic.Bool

// SetProbeGuardDisabledForTest 关闭/恢复 SSRF 守卫，返回恢复函数。
// 仅供测试调用。
func SetProbeGuardDisabledForTest(disabled bool) func() {
	previous := probeGuardDisabled.Swap(disabled)
	return func() { probeGuardDisabled.Store(previous) }
}

// IsBlockedProbeIP 判断该 IP 是否禁止作为探测目标。
func IsBlockedProbeIP(ip net.IP) bool {
	if probeGuardDisabled.Load() {
		return false
	}
	if ip == nil {
		return true
	}
	// IPv4-mapped IPv6（::ffff:127.0.0.1）必须按 IPv4 规则判断，
	// 否则可以用它绕过 IPv4 段的拦截。
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, network := range blockedCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// ErrBlockedProbeTarget 表示探测目标落在被禁止的地址段内。
type ErrBlockedProbeTarget struct {
	Host string
	IP   string
}

func (e *ErrBlockedProbeTarget) Error() string {
	if e.IP != "" && e.IP != e.Host {
		return fmt.Sprintf("目标地址 %s 解析到内网/保留地址 %s，已阻止；渠道探测与连通性测试只允许访问公网地址", e.Host, e.IP)
	}
	return fmt.Sprintf("目标地址 %s 属于内网/保留地址，已阻止；渠道探测与连通性测试只允许访问公网地址", e.Host)
}

// ValidateProbeHost 在发起请求前校验主机名。
//
// 解析出的**所有**地址都必须放行才算通过：只要有一个落在禁止段内就拒绝，
// 避免攻击者用同时返回公网与内网地址的域名绕过检查。
func ValidateProbeHost(ctx context.Context, host string) error {
	if probeGuardDisabled.Load() {
		return nil
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return &ErrBlockedProbeTarget{Host: host}
	}
	// 字面量 IP 无需 DNS 解析。
	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedProbeIP(ip) {
			return &ErrBlockedProbeTarget{Host: host, IP: ip.String()}
		}
		return nil
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("DNS 解析 %s 失败: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("DNS 解析 %s 未返回任何地址", host)
	}
	for _, item := range ips {
		if IsBlockedProbeIP(item.IP) {
			return &ErrBlockedProbeTarget{Host: host, IP: item.IP.String()}
		}
	}
	return nil
}

// probeGuardDialer 在实际建连前再校验一次目标 IP。
//
// 单靠 ValidateProbeHost 存在 DNS rebinding 窗口：校验时返回公网地址，
// 真正连接时 DNS 又返回内网地址。这里校验的是即将连接的地址本身，
// 因此不存在该窗口。
func probeGuardDialer(base func(context.Context, string, string) (net.Conn, error)) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if ip := net.ParseIP(host); ip != nil {
			if IsBlockedProbeIP(ip) {
				return nil, &ErrBlockedProbeTarget{Host: host, IP: ip.String()}
			}
			return base(ctx, network, address)
		}
		// Transport 通常已把主机名解析为 IP 再调用 dialer；
		// 走到这里说明拿到的仍是主机名（例如经由代理），补一次解析校验。
		if err := ValidateProbeHost(ctx, host); err != nil {
			return nil, err
		}
		return base(ctx, network, address)
	}
}
