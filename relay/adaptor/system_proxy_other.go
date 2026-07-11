//go:build !windows

package adaptor

import "os"

type systemProxySettings struct {
	HTTPProxy  string
	HTTPSProxy string
	NoProxy    string
	Source     string
}

func loadSystemProxySettings() (systemProxySettings, error) {
	return systemProxySettings{
		HTTPProxy:  firstNonEmpty(os.Getenv("HTTP_PROXY"), os.Getenv("http_proxy")),
		HTTPSProxy: firstNonEmpty(os.Getenv("HTTPS_PROXY"), os.Getenv("https_proxy")),
		NoProxy:    firstNonEmpty(os.Getenv("NO_PROXY"), os.Getenv("no_proxy")),
		Source:     "environment",
	}, nil
}
