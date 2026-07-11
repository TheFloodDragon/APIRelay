//go:build windows

package adaptor

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

type systemProxySettings struct {
	HTTPProxy  string
	HTTPSProxy string
	NoProxy    string
	Source     string
}

func loadSystemProxySettings() (systemProxySettings, error) {
	settings := systemProxySettings{Source: "windows_system"}
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		settings.Source = "windows_system_unavailable"
		return settings, nil
	}
	defer key.Close()
	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return settings, nil
	}
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		return settings, nil
	}
	bypass, _, _ := key.GetStringValue("ProxyOverride")
	bypass = strings.ReplaceAll(bypass, "<local>", "localhost;127.0.0.1;::1")
	settings.NoProxy = strings.ReplaceAll(bypass, ";", ",")
	settings.HTTPProxy, settings.HTTPSProxy = parseWindowsProxyServer(server)
	return settings, nil
}

func parseWindowsProxyServer(server string) (string, string) {
	server = strings.TrimSpace(server)
	if server == "" {
		return "", ""
	}
	if !strings.Contains(server, "=") {
		u, err := normalizeProxyURL(server)
		if err != nil {
			return "", ""
		}
		return u.String(), u.String()
	}
	values := map[string]string{}
	for _, item := range strings.Split(server, ";") {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			values[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
		}
	}
	normalize := func(value, defaultScheme string) string {
		if value == "" {
			return ""
		}
		if !strings.Contains(value, "://") && defaultScheme != "" {
			value = defaultScheme + "://" + value
		}
		u, err := normalizeProxyURL(value)
		if err != nil {
			return ""
		}
		return u.String()
	}
	httpProxy := normalize(values["http"], "http")
	if httpProxy == "" {
		httpProxy = normalize(values["socks"], "socks5")
	}
	httpsProxy := normalize(values["https"], "http")
	if httpsProxy == "" {
		httpsProxy = firstNonEmpty(httpProxy, normalize(values["socks"], "socks5"))
	}
	return httpProxy, httpsProxy
}
